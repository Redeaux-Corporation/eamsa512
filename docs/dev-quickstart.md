# EAMSA 512 - Developer Quick Start Guide

## 🚀 5-Minute Setup

### Step 1: Clone Repository
```bash
git clone https://github.com/Redeaux-Corporation/eamsa512.git
cd eamsa512
```

### Step 2: Initialize & Build
```bash
go mod init eamsa512
go get -u golang.org/x/crypto
go build -o eamsa512
```

### Step 3: Verify Installation
```bash
./eamsa512 -version
./eamsa512 -compliance-report
```

### Step 4: Run First Encryption
```bash
# Create test data
echo "Hello EAMSA512!" > plaintext.txt

# Encrypt
./eamsa512 -encrypt -input plaintext.txt -key prod-key-001

# Decrypt
./eamsa512 -decrypt -input ciphertext.bin -key prod-key-001
```

---

## 📦 Complete File Structure

### Essential Files (Must Have)

```
eamsa512/
├── Core Implementation (5200+ lines)
│   ├── chaos.go                    # Lorenz + Hyperchaotic systems
│   ├── kdf.go                      # SHA3-512 key derivation
│   ├── stats.go                    # NIST statistical validation
│   ├── phase2-msa.go               # Modified SALSA20
│   ├── phase2-sbox-player.go       # S-boxes + P-layer
│   ├── phase3-sha3-updated.go      # HMAC-SHA3-512
│   ├── main.go                     # CLI interface
│   └── go.mod                      # Dependencies
│
├── Compliance Files (750+ lines)
│   ├── hsm-integration.go          # HSM abstraction + tamper detection
│   ├── key-lifecycle.go            # Key management lifecycle
│   ├── kat-tests.go                # Known answer tests
│   ├── rbac.go                     # Role-based access control
│   ├── kdf-compliance.go           # NIST SP 800-56A
│   └── compliance-report.go        # Compliance verification
│
├── Configuration
│   ├── config/
│   │   ├── eamsa512.yaml           # Main configuration
│   │   ├── hsm-creds.txt           # HSM credentials (private)
│   │   └── rbac-config.yaml        # RBAC roles
│   │
│   └── Dockerfile                  # Docker build file
│
├── Documentation
│   ├── README.md                   # Overview
│   ├── deployment-guide.md         # Production deployment
│   ├── fips-140-2-compliance.md    # Compliance guide
│   ├── key-agreement-spec.md       # Key agreement protocol
│   ├── entropy-source-spec.md      # Entropy validation
│   └── api-reference.md            # API documentation
│
├── Tests & Examples
│   ├── examples/
│   │   ├── basic-encryption.go     # Basic example
│   │   ├── key-rotation.go         # Key rotation
│   │   ├── hsm-integration.go      # HSM setup
│   │   └── web-server.go           # HTTP server
│   │
│   └── tests/
│       ├── encryption_test.go      # Unit tests
│       ├── compliance_test.go      # Compliance tests
│       └── performance_test.go     # Benchmarks
│
└── Deployment
    ├── k8s/
    │   ├── deployment.yaml         # Kubernetes deployment
    │   ├── service.yaml            # K8s service
    │   └── configmap.yaml          # K8s config
    │
    ├── systemd/
    │   └── eamsa512.service        # Systemd service file
    │
    └── docker-compose.yml          # Docker Compose
```

---

## 🔧 Installation for Developers

### Option 1: Development Setup

```bash
# 1. Clone or download all files
mkdir ~/projects/eamsa512
cd ~/projects/eamsa512
# Copy all Go files here

# 2. Setup Go workspace
go mod init eamsa512
go mod tidy

# 3. Build
go build -o eamsa512

# 4. Install locally
go install ./...

# 5. Verify
eamsa512 -version
```

### Option 2: Docker Development

```bash
# Build Docker image
docker build -t eamsa512:dev .

# Run container
docker run -it --rm eamsa512:dev /bin/sh

# Inside container:
eamsa512 -version
eamsa512 -test-all
```

### Option 3: Cloud Development (AWS)

```bash
# Launch EC2 instance (Go 1.21+ AMI)
aws ec2 run-instances \
  --image-id ami-0c55b159cbfafe1f0 \
  --instance-type t3.medium

# SSH into instance
ssh -i key.pem ubuntu@instance-ip

# Clone and build
cd /home/ubuntu
git clone https://github.com/yourorg/eamsa512.git
cd eamsa512
go build -o eamsa512
```

---

## 💻 Building Applications on EAMSA 512

### Example 1: Simple Encryption Utility

```go
// examples/basic-encryption.go
package main

import (
    "eamsa512"
    "flag"
    "fmt"
    "io/ioutil"
    "os"
)

func main() {
    var (
        mode     = flag.String("mode", "encrypt", "encrypt or decrypt")
        input    = flag.String("input", "", "input file")
        output   = flag.String("output", "", "output file")
        keyFile  = flag.String("key", "", "key file")
    )
    flag.Parse()

    // Load key
    keyData, err := ioutil.ReadFile(*keyFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading key: %v\n", err)
        os.Exit(1)
    }

    // Load input data
    inputData, err := ioutil.ReadFile(*input)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
        os.Exit(1)
    }

    var outputData []byte

    // Process based on mode
    switch *mode {
    case "encrypt":
        phase2 := eamsa512.NewPhase2Encryption()
        phase3 := eamsa512.NewPhase3Authentication()
        
        // Split into 512-bit blocks
        for i := 0; i < len(inputData); i += 64 {
            block := [64]byte{}
            copy(block[:], inputData[i:])
            
            // Encrypt
            ciphertext, _ := phase2.Encrypt(block, [11][16]byte{})
            mac, _ := phase3.ComputeHMAC(ciphertext, [32]byte{})
            
            outputData = append(outputData, ciphertext[:]...)
            outputData = append(outputData, mac[:]...)
        }

    case "decrypt":
        phase2 := eamsa512.NewPhase2Encryption()
        
        // Process encrypted blocks
        for i := 0; i < len(inputData); i += 128 {
            ciphertext := [64]byte{}
            copy(ciphertext[:], inputData[i:i+64])
            
            plaintext, _ := phase2.Decrypt(ciphertext, [11][16]byte{})
            outputData = append(outputData, plaintext[:]...)
        }
    }

    // Write output
    ioutil.WriteFile(*output, outputData, 0644)
    fmt.Printf("Successfully processed: %s -> %s\n", *input, *output)
}
```

**Build & Run:**
```bash
go run examples/basic-encryption.go \
  -mode encrypt \
  -input plaintext.txt \
  -key master.key \
  -output ciphertext.bin
```

### Example 2: HTTP Encryption Server

```go
// examples/web-server.go
package main

import (
    "eamsa512"
    "encoding/json"
    "log"
    "net/http"
)

type EncryptRequest struct {
    Plaintext string `json:"plaintext"`
    KeyID     string `json:"key_id"`
}

type EncryptResponse struct {
    Ciphertext string `json:"ciphertext"`
    MAC        string `json:"mac"`
}

func encryptHandler(w http.ResponseWriter, r *http.Request) {
    var req EncryptRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Get key from lifecycle manager
    phase2 := eamsa512.NewPhase2Encryption()
    phase3 := eamsa512.NewPhase3Authentication()

    // Convert plaintext to block
    plaintext := [64]byte{}
    copy(plaintext[:], []byte(req.Plaintext))

    // Encrypt
    ciphertext, _ := phase2.Encrypt(plaintext, [11][16]byte{})
    mac, _ := phase3.ComputeHMAC(ciphertext, [32]byte{})

    // Return response
    resp := EncryptResponse{
        Ciphertext: string(ciphertext[:]),
        MAC:        string(mac[:]),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func main() {
    http.HandleFunc("/api/v1/encrypt", encryptHandler)
    log.Println("Server listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**Run:**
```bash
go run examples/web-server.go

# Test in another terminal
curl -X POST http://localhost:8080/api/v1/encrypt \
  -H "Content-Type: application/json" \
  -d '{
    "plaintext": "Hello World",
    "key_id": "prod-key-001"
  }'
```

### Example 3: Database Encryption Wrapper

```go
// examples/database-encryption.go
package main

import (
    "database/sql"
    "eamsa512"
    _ "github.com/lib/pq"
)

type EncryptedDB struct {
    db  *sql.DB
    enc *eamsa512.Phase2Encryption
    auth *eamsa512.Phase3Authentication
}

func NewEncryptedDB(connString string) (*EncryptedDB, error) {
    db, err := sql.Open("postgres", connString)
    if err != nil {
        return nil, err
    }

    return &EncryptedDB{
        db:   db,
        enc:  eamsa512.NewPhase2Encryption(),
        auth: eamsa512.NewPhase3Authentication(),
    }, nil
}

func (edb *EncryptedDB) InsertEncrypted(
    table string,
    column string,
    plaintext []byte,
) error {
    // Split into blocks
    for i := 0; i < len(plaintext); i += 64 {
        block := [64]byte{}
        copy(block[:], plaintext[i:i+64])

        // Encrypt
        ciphertext, _ := edb.enc.Encrypt(block, [11][16]byte{})
        mac, _ := edb.auth.ComputeHMAC(ciphertext, [32]byte{})

        // Insert encrypted data
        _, err := edb.db.Exec(
            "INSERT INTO "+table+" ("+column+", mac) VALUES ($1, $2)",
            ciphertext[:],
            mac[:],
        )
        if err != nil {
            return err
        }
    }
    return nil
}

func (edb *EncryptedDB) SelectEncrypted(
    table string,
    column string,
    id int,
) ([]byte, error) {
    var ciphertext []byte
    var mac []byte

    row := edb.db.QueryRow(
        "SELECT "+column+", mac FROM "+table+" WHERE id = $1",
        id,
    )
    err := row.Scan(&ciphertext, &mac)
    if err != nil {
        return nil, err
    }

    // Decrypt
    cblock := [64]byte{}
    copy(cblock[:], ciphertext)
    plaintext, _ := edb.enc.Decrypt(cblock, [11][16]byte{})

    return plaintext[:], nil
}
```

### Example 4: Key Rotation Service

```go
// examples/key-rotation.go
package main

import (
    "eamsa512"
    "fmt"
    "time"
)

func rotateKeysDaily(klm *eamsa512.KeyLifecycleManager) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        // Get keys needing rotation
        keysToRotate := klm.GetKeysNeedingRotation()

        for _, keyID := range keysToRotate {
            fmt.Printf("Rotating key: %s\n", keyID)

            // Rotate key
            newKey, err := klm.RotateKey(keyID, "rotation-service")
            if err != nil {
                fmt.Printf("Error rotating key: %v\n", err)
                continue
            }

            fmt.Printf("Successfully rotated key: %s\n", newKey.KeyID)
        }
    }
}

func monitorComplianceStatus(hsm *eamsa512.HSMIntegration) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        status := hsm.GetStatus()
        fmt.Printf("HSM Status:\n")
        fmt.Printf("  Online: %v\n", status.Online)
        fmt.Printf("  Tamper: %v\n", status.TamperDetected)
        fmt.Printf("  Events: %d\n", status.SecurityEvents)
    }
}

func main() {
    // Initialize HSM
    hsmConfig := eamsa512.HSMConfig{
        HSMType:      "softhsm",
        TamperSensor: true,
        AuditLog:     "/var/log/hsm/",
    }
    hsm := eamsa512.NewHSMIntegration(hsmConfig)

    // Initialize key lifecycle manager
    klm := eamsa512.NewKeyLifecycleManager(hsm)

    // Start rotation service
    go rotateKeysDaily(klm)

    // Start monitoring
    go monitorComplianceStatus(hsm)

    // Keep running
    select {}
}
```

---

## 🧪 Testing Your Application

### Unit Tests

```go
// tests/encryption_test.go
package tests

import (
    "eamsa512"
    "testing"
)

func TestEncryptionRoundTrip(t *testing.T) {
    phase2 := eamsa512.NewPhase2Encryption()
    
    // Create test data
    plaintext := [64]byte{}
    for i := 0; i < 64; i++ {
        plaintext[i] = byte(i)
    }
    
    keys := [11][16]byte{}
    
    // Encrypt
    ciphertext, err := phase2.Encrypt(plaintext, keys)
    if err != nil {
        t.Fatalf("Encryption failed: %v", err)
    }
    
    // Decrypt
    decrypted, err := phase2.Decrypt(ciphertext, keys)
    if err != nil {
        t.Fatalf("Decryption failed: %v", err)
    }
    
    // Verify
    if plaintext != decrypted {
        t.Fatal("Plaintext mismatch after encryption/decryption")
    }
}
```

**Run:**
```bash
go test -v ./tests/encryption_test.go
```

### Benchmark Tests

```go
// tests/performance_test.go
package tests

import (
    "eamsa512"
    "testing"
)

func BenchmarkEncryption(b *testing.B) {
    phase2 := eamsa512.NewPhase2Encryption()
    plaintext := [64]byte{}
    keys := [11][16]byte{}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        phase2.Encrypt(plaintext, keys)
    }
}

func BenchmarkKeyDerivation(b *testing.B) {
    kdf := eamsa512.NewKDFVectorized()
    masterKey := [32]byte{}
    nonce := [16]byte{}
    trajectory := make([]float64, 1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        kdf.DeriveKeys(masterKey, nonce, trajectory)
    }
}
```

**Run:**
```bash
go test -bench=. -benchmem ./tests/performance_test.go
```

---

## 📚 API Quick Reference

### Key Functions

```go
// Phase 1: Key Generation
chaos := eamsa512.NewChaosGenerator()
sequence := chaos.GenerateSequence(1000)

// Phase 1: Key Derivation
kdf := eamsa512.NewKDFVectorized()
keys := kdf.DeriveKeys(masterKey, nonce, sequence)

// Phase 2: Encryption
phase2 := eamsa512.NewPhase2Encryption()
ciphertext, _ := phase2.Encrypt(plaintext, keys)
plaintext, _ := phase2.Decrypt(ciphertext, keys)

// Phase 3: Authentication
phase3 := eamsa512.NewPhase3Authentication()
mac, _ := phase3.ComputeHMAC(ciphertext, authKey)
verified, _ := phase3.VerifyHMAC(ciphertext, mac, authKey)

// HSM Integration
hsm := eamsa512.NewHSMIntegration(config)
hsm.ImportKey(keyMaterial)
hsm.DetectTamper()

// Key Lifecycle
klm := eamsa512.NewKeyLifecycleManager(hsm)
key, _ := klm.GenerateKey("key-id", "operator")
klm.RotateKey("key-id", "operator")
klm.ZeroizeKey("key-id", "operator")

// Compliance
report := eamsa512.GenerateComplianceReport()
```

---

## 🔐 Security Checklist for Developers

- [ ] Never hardcode encryption keys
- [ ] Use HSM for production key storage
- [ ] Enable TLS for network communication
- [ ] Implement proper error handling
- [ ] Log security events (audit trail)
- [ ] Validate all inputs
- [ ] Use constant-time comparisons
- [ ] Implement key rotation
- [ ] Monitor for tamper alerts
- [ ] Test with known answer tests
- [ ] Run compliance checks regularly
- [ ] Keep dependencies updated

---

## 📞 Support Resources

### Documentation
- Full Deployment Guide: `deployment-guide.md`
- Compliance Guide: `fips-140-2-compliance.md`
- API Reference: `api-reference.md`

### Examples
```
examples/
├── basic-encryption.go
├── key-rotation.go
├── hsm-integration.go
└── web-server.go
```

### Testing
```
tests/
├── encryption_test.go
├── compliance_test.go
└── performance_test.go
```
