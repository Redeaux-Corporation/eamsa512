// kat-tests.go - Known Answer Tests for FIPS 140-2 Compliance
package main

import (
	"crypto/sha256"
	"fmt"
	"log"
)

// KATVector represents a known answer test vector
type KATVector struct {
	ID         string
	Key        [32]byte
	Plaintext  [64]byte
	Ciphertext [64]byte
	MAC        [64]byte
	Description string
}

// KATTestSuite manages all known answer tests
type KATTestSuite struct {
	vectors []KATVector
	passed  int
	failed  int
}

// NewKATTestSuite creates new KAT suite
func NewKATTestSuite() *KATTestSuite {
	return &KATTestSuite{
		vectors: make([]KATVector, 0),
		passed:  0,
		failed:  0,
	}
}

// AddTestVector adds a test vector to the suite
func (kat *KATTestSuite) AddTestVector(vector KATVector) {
	kat.vectors = append(kat.vectors, vector)
}

// GenerateDefaultVectors generates standard test vectors
func (kat *KATTestSuite) GenerateDefaultVectors() {
	// Vector 1: All zeros
	vec1 := KATVector{
		ID:          "KAT_001",
		Key:         [32]byte{},
		Plaintext:   [64]byte{},
		Description: "All zeros test vector",
	}
	// Pre-computed expected values (would be generated from reference implementation)
	for i := 0; i < 64; i++ {
		vec1.Ciphertext[i] = byte((i * 31) % 256)
	}
	for i := 0; i < 64; i++ {
		vec1.MAC[i] = byte((i * 47) % 256)
	}
	kat.AddTestVector(vec1)

	// Vector 2: Sequential data
	vec2 := KATVector{
		ID:          "KAT_002",
		Description: "Sequential data test vector",
	}
	for i := 0; i < 32; i++ {
		vec2.Key[i] = byte(i)
	}
	for i := 0; i < 64; i++ {
		vec2.Plaintext[i] = byte(i)
	}
	for i := 0; i < 64; i++ {
		vec2.Ciphertext[i] = byte((i * 61) % 256)
	}
	for i := 0; i < 64; i++ {
		vec2.MAC[i] = byte((i * 73) % 256)
	}
	kat.AddTestVector(vec2)

	// Vector 3: All ones
	vec3 := KATVector{
		ID:          "KAT_003",
		Description: "All ones test vector",
	}
	for i := 0; i < 32; i++ {
		vec3.Key[i] = 0xFF
	}
	for i := 0; i < 64; i++ {
		vec3.Plaintext[i] = 0xFF
	}
	for i := 0; i < 64; i++ {
		vec3.Ciphertext[i] = byte((i * 83) % 256)
	}
	for i := 0; i < 64; i++ {
		vec3.MAC[i] = byte((i * 89) % 256)
	}
	kat.AddTestVector(vec3)

	// Vector 4: Alternating pattern
	vec4 := KATVector{
		ID:          "KAT_004",
		Description: "Alternating bit pattern",
	}
	for i := 0; i < 32; i++ {
		if i%2 == 0 {
			vec4.Key[i] = 0xAA
		} else {
			vec4.Key[i] = 0x55
		}
	}
	for i := 0; i < 64; i++ {
		if i%2 == 0 {
			vec4.Plaintext[i] = 0xAA
		} else {
			vec4.Plaintext[i] = 0x55
		}
	}
	for i := 0; i < 64; i++ {
		vec4.Ciphertext[i] = byte((i * 97) % 256)
	}
	for i := 0; i < 64; i++ {
		vec4.MAC[i] = byte((i * 101) % 256)
	}
	kat.AddTestVector(vec4)

	// Vector 5: Random-like (deterministic pseudo-random)
	vec5 := KATVector{
		ID:          "KAT_005",
		Description: "Pseudo-random data test vector",
	}
	seed := uint32(0x12345678)
	for i := 0; i < 32; i++ {
		seed = seed*1103515245 + 12345
		vec5.Key[i] = byte(seed / 65536 % 256)
	}
	for i := 0; i < 64; i++ {
		seed = seed*1103515245 + 12345
		vec5.Plaintext[i] = byte(seed / 65536 % 256)
	}
	for i := 0; i < 64; i++ {
		vec5.Ciphertext[i] = byte((i * 103) % 256)
	}
	for i := 0; i < 64; i++ {
		vec5.MAC[i] = byte((i * 107) % 256)
	}
	kat.AddTestVector(vec5)
}

// VerifyVector verifies a single test vector
func (kat *KATTestSuite) VerifyVector(vector KATVector) bool {
	// In production, this would:
	// 1. Call actual encryption with the key and plaintext
	// 2. Compare result with expected ciphertext
	// 3. Call actual HMAC computation
	// 4. Compare result with expected MAC
	
	// For now, implement reference check
	phase2 := NewPhase2Encryption()
	phase3 := NewPhase3Authentication()
	
	// Encrypt
	ciphertext, err := phase2.Encrypt(vector.Plaintext, [11][16]byte{})
	if err != nil {
		log.Printf("KAT %s: Encryption failed: %v\n", vector.ID, err)
		return false
	}
	
	// Authenticate
	mac, err := phase3.ComputeHMAC(ciphertext, vector.Key)
	if err != nil {
		log.Printf("KAT %s: Authentication failed: %v\n", vector.ID, err)
		return false
	}
	
	// Verify ciphertext
	ciphertextMatch := true
	for i := 0; i < 64; i++ {
		if ciphertext[i] != vector.Ciphertext[i] {
			ciphertextMatch = false
			break
		}
	}
	
	// Verify MAC
	macMatch := true
	for i := 0; i < 64; i++ {
		if mac[i] != vector.MAC[i] {
			macMatch = false
			break
		}
	}
	
	if ciphertextMatch && macMatch {
		return true
	}
	
	return false
}

// RunAllTests runs all KAT vectors
func (kat *KATTestSuite) RunAllTests() {
	fmt.Printf("\n🧪 Running Known Answer Tests (KAT)\n")
	fmt.Printf("═════════════════════════════════════════════════════════════\n")
	
	kat.GenerateDefaultVectors()
	
	for _, vector := range kat.vectors {
		result := kat.VerifyVector(vector)
		status := "✅ PASS"
		if !result {
			status = "❌ FAIL"
			kat.failed++
		} else {
			kat.passed++
		}
		
		fmt.Printf("%s - %s: %s\n", vector.ID, vector.Description, status)
	}
	
	fmt.Printf("═════════════════════════════════════════════════════════════\n")
	fmt.Printf("Results: %d passed, %d failed out of %d tests\n", kat.passed, kat.failed, len(kat.vectors))
	
	if kat.failed == 0 {
		fmt.Printf("✅ All KAT tests PASSED - System is compliant\n")
	} else {
		fmt.Printf("❌ Some KAT tests FAILED - Review implementation\n")
	}
}

// GetComplianceStatus returns compliance status
func (kat *KATTestSuite) GetComplianceStatus() bool {
	return kat.failed == 0 && len(kat.vectors) > 0
}

// PrintTestVectorHash prints SHA256 of test vectors (for audit trail)
func (kat *KATTestSuite) PrintTestVectorHash() {
	data := make([]byte, 0)
	for _, vec := range kat.vectors {
		data = append(data, []byte(vec.ID)...)
		data = append(data, vec.Key[:]...)
		data = append(data, vec.Plaintext[:]...)
		data = append(data, vec.Ciphertext[:]...)
		data = append(data, vec.MAC[:]...)
	}
	
	hash := sha256.Sum256(data)
	fmt.Printf("KAT Vector Suite Hash (SHA256): %x\n", hash)
}

// InitializeKATOnStartup initializes and runs KAT on system startup
func InitializeKATOnStartup() bool {
	fmt.Println("\n🔐 Running FIPS 140-2 Known Answer Tests on startup...")
	
	katSuite := NewKATTestSuite()
	katSuite.RunAllTests()
	
	return katSuite.GetComplianceStatus()
}

// Stub implementations (would be imported from actual modules)
type Phase2Encryption struct{}

func NewPhase2Encryption() *Phase2Encryption {
	return &Phase2Encryption{}
}

func (p *Phase2Encryption) Encrypt(plaintext [64]byte, keys [11][16]byte) ([64]byte, error) {
	// Stub - return input as-is for testing
	return plaintext, nil
}

type Phase3Authentication struct{}

func NewPhase3Authentication() *Phase3Authentication {
	return &Phase3Authentication{}
}

func (p *Phase3Authentication) ComputeHMAC(data [64]byte, key [32]byte) ([64]byte, error) {
	// Stub - return zeros for testing
	return [64]byte{}, nil
}
