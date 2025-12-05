# EAMSA 512 API Reference

## 1. Overview

This document describes the public interfaces exposed by EAMSA 512:

- Go library API for direct embedding.

- Command-line interface (CLI) for operators and scripts.

- HTTP/JSON REST API for remote clients and services.



All APIs are designed around authenticated encryption with associated data (AEAD), key lifecycle management, and compliance reporting.



---



## 2. Go library API



### 2.1 Package layout

The primary packages are:



- `eamsa512/chaos` – Chaos-based entropy generator.

- `eamsa512/kdf` – SHA3‑512 based KDF and key agreement helpers.

- `eamsa512/cipher` – 512‑bit core encryption and decryption (Phase 2).

- `eamsa512/auth` – HMAC‑SHA3‑512 authentication (Phase 3).

- `eamsa512/hsm` – HSM integration and abstraction.

- `eamsa512/keys` – Key lifecycle management.

- `eamsa512/compliance` – Compliance checks and reporting.

  

---


### 2.2 Chaos generator



```go
package chaos

// Generator represents a chaos-based entropy source instance.

type Generator struct {

// internal state (hidden)

}



// NewGenerator creates a new chaos generator with default parameters.

func NewGenerator() (*Generator, error)



// Seed mixes external entropy into the internal state.

func (g *Generator) Seed(seed []byte) error



// Bytes returns n bytes of raw (unconditioned) chaotic output.

func (g *Generator) Bytes(n int) ([]byte, error)
```




Typical usage:


```
g, _ := chaos.NewGenerator()

raw, _ := g.Bytes(4096)
```



---



### 2.3 KDF and key agreement


```go
package kdf



// KDFParams describes inputs to the SHA3-512-based concatenation KDF.

type KDFParams struct {

Z []byte // shared secret

Nonce []byte // per-session nonce

Chaos []byte // conditioned chaos_output

OtherInfo []byte // context as specified in key-agreement-spec.md

KeyCount int // number of 128-bit keys to derive (default 11)

}



// DeriveKeys returns KeyCount 128-bit keys derived from the inputs.

func DeriveKeys(p KDFParams) ([][]byte, error)
```




Example:


```
params := kdf.KDFParams{

Z: masterSecret,

Nonce: nonce,

Chaos: chaosOutput,

OtherInfo: otherInfo,

KeyCount: 11,

}

keys, err := kdf.DeriveKeys(params)
```




---



### 2.4 Core cipher (Phase 2)





```go
package cipher



// BlockSize is the fixed block size in bytes.

const BlockSize = 64 // 512 bits



// Cipher holds expanded key material and state.

type Cipher struct {

// internal fields (hidden)

}



// NewCipher constructs a cipher instance from 11 × 128-bit subkeys.

func NewCipher(subkeys [][]byte) (*Cipher, error)



// Encrypt encrypts plaintext and returns ciphertext (may be padded or chunked).

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error)



// Decrypt decrypts ciphertext and returns the original plaintext.

func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error)
```




---



### 2.5 Authentication (Phase 3)


```go
package auth

// MACSize is the HMAC-SHA3-512 output size in bytes.
const MACSize = 64

// Auth provides per-block authentication using HMAC-SHA3-512.
type Auth struct {

// internal state

}



// NewAuth creates a new authentication context with the given key.
func NewAuth(key []byte) *Auth



// Compute computes HMAC-SHA3-512 over data and optional associatedData.
func (a *Auth) Compute(data, associatedData []byte) ([]byte, error)



// Verify recomputes the MAC and compares it in constant time.
func (a *Auth) Verify(data, associatedData, mac []byte) error
```




---



### 2.6 HSM integration


```go
package hsm



// Provider is a generic HSM abstraction interface.

type Provider interface {

GenerateKey(label string, sizeBits int) (keyID string, err error)

GetKey(labelOrID string) ([]byte, error)

DestroyKey(labelOrID string) error

Sign(labelOrID string, data []byte) ([]byte, error)

}



// Config describes HSM backend settings.

type Config struct {

Type string // e.g. "thales", "yubihsm", "aws-cloudhsm", "softhsm"

Params map[string]string // vendor-specific connection info

}



// NewProvider returns a Provider for the given configuration.

func NewProvider(cfg Config) (Provider, error)
```



---



### 2.7 Key lifecycle management


```go
package keys



// State represents the lifecycle state of a key.

type State string



const (

StateGenerated State = "generated"

StateActive State = "active"

StateRotated State = "rotated"

StateRevoked State = "revoked"

)



// KeyMetadata captures key properties and lifecycle info.

type KeyMetadata struct {

ID string

Label string

CreatedAt time.Time

ExpiresAt time.Time

State State

Algorithm string

}



// Manager orchestrates key generation, rotation, and destruction.

type Manager struct {

// internal fields (HSM or software backend)

}



// NewManager constructs a Manager bound to an HSM Provider (or software backend).

func NewManager(p hsm.Provider) *Manager



func (m *Manager) Generate(label string, sizeBits int) (KeyMetadata, error)

func (m *Manager) Rotate(label string) (KeyMetadata, error)

func (m *Manager) Get(labelOrID string) (KeyMetadata, []byte, error)

func (m *Manager) Revoke(labelOrID string) error
```




---



### 2.8 Compliance API


```go
package compliance



// Report captures the results of all internal compliance checks.

type Report struct {

SystemVersion string

ComplianceScore int

FIPS1402Level2 bool

NISTSP80056A bool

RFC2104HMAC bool

FIPS202SHA3 bool

IETFStandards bool

GoSecurityBestPractices bool

CVEVulnerabilities int

TestCoverage float64

KnownAnswerTestsPassed bool

EntropyValidationPassed bool

// ...

}



// RunFull performs all checks and returns a populated report.

func RunFull() (*Report, error)
```




---



## 3. HTTP REST API

### 3.1 Conventions

- Base URL: `https://HOST:PORT/api/v1`

- Content type: `application/json`

- Authentication: bearer tokens, mTLS, or network-level controls (implementation-dependent).

- Responses use standard HTTP status codes plus JSON bodies with `error` fields when applicable.



---



### 3.2 Endpoints overview


```
| Method | Path                          | Description                         | Auth |

|--------|-------------------------------|-------------------------------------|------|

| POST   | `/encrypt`                    | Encrypts data                       | Yes  |

| POST   | `/decrypt`                    | Decrypts data                       | Yes  |

| GET    | `/compliance/report`          | Returns compliance report           | Yes  |

| GET    | `/keys/{id}/status`           | Returns key lifecycle metadata      | Yes  |

| POST   | `/keys/{id}/rotate`           | Rotates the specified key           | Yes  |

| GET    | `/health`                     | Liveness/health probe               | No   |



Base path `/api/v1` is implied in examples.
```


---



### 3.3 `/encrypt` (POST)


Encrypts input data and returns ciphertext and MAC.

**Request**


```
POST /api/v1/encrypt

Content-Type: application/json

Authorization: Bearer <token>
```


Body:


```
{

"key_id": "primary-enc-key",

"plaintext": "base64-encoded-plaintext",

"associated_data": "base64-encoded-ad",

"nonce": "base64-encoded-nonce (optional)"

}
```



**Response**


```
{

"ciphertext": "base64-encoded-ciphertext",

"mac": "base64-encoded-mac",

"nonce": "base64-encoded-nonce"

}
```




Errors:

- `400` for invalid input.

- `401/403` for auth failures.

- `500` for internal or HSM failures.



---



### 3.4 `/decrypt` (POST)



Decrypts ciphertext and verifies its MAC.



**Request**

```
POST /api/v1/decrypt

Content-Type: application/json

Authorization: Bearer <token>
```


Body:

```
{

"key_id": "primary-enc-key",

"ciphertext": "base64-encoded-ciphertext",

"mac": "base64-encoded-mac",

"associated_data": "base64-encoded-ad",

"nonce": "base64-encoded-nonce"

}
```




**Response**


```
{

"plaintext": "base64-encoded-plaintext"

}
```




If MAC verification fails, the service returns `400` or `401` with an error and *\*does not*\* reveal which component failed.



---



### 3.5 `/compliance/report` (GET)



Returns the complete compliance report.

**Request**


```
GET /api/v1/compliance/report

Authorization: Bearer <token>
```




**Response**


```
{

"system_version": "1.1",

"compliance_score": 100,

"fips_140_2_level_2": true,

"nist_sp_800_56a": true,

"rfc_2104_hmac": true,

"nist_fips_202_sha3": true,

"ietf_standards": true,

"go_security_best_practices": true,

"cve_vulnerabilities": 0,

"test_coverage": 95.5,

"known_answer_tests_passed": true,

"entropy_validation_passed": true,

"timestamp": "2025-12-04T12:00:00Z"

}
```




---



### 3.6 `/keys/{id}/status` (GET)



Returns lifecycle metadata for a given key.



**Request**


```
GET /api/v1/keys/{id}/status

Authorization: Bearer <token>
```




**Response**


```
{

"id": "primary-enc-key",

"label": "Primary Encryption Key",

"state": "active",

"algorithm": "EAMSA512-KDF-SHA3-512",

"created_at": "2025-01-01T00:00:00Z",

"expires_at": "2026-01-01T00:00:00Z"

}
```




---



### 3.7 `/keys/{id}/rotate` (POST)

Triggers rotation of the given key through the key manager and HSM (if configured).



**Request**


```
POST /api/v1/keys/{id}/rotate

Authorization: Bearer <token>

Content-Type: application/json
```



Optional body:


```
{

"reason": "scheduled-rotation"

}
```


**Response**


```json
{

"id": "primary-enc-key",

"state": "active",

"previous_id": "primary-enc-key-2025-01",

"rotated_at": "2025-06-01T00:00:00Z"

}
```




---



### 3.8 `/health` (GET)



Basic liveness and readiness endpoint.



**Request**


```
GET /api/v1/health
```




**Response**


```json
{

"status": "ok",

"uptime\_seconds": 12345,

"self\_tests\_passed": true

}
```


This endpoint should not expose sensitive internal metrics and is suitable for Kubernetes or load balancer health checks.



---



## 4. Command-line interface (CLI)



The `eamsa512` binary provides subcommands and flags.



### 4.1 Global flags



- `-config <path>` – Path to `eamsa512.yaml` (default: `./config/eamsa512.yaml`).

- `-log-level <level>` – `DEBUG`, `INFO`, `WARN`, `ERROR`.

- `-hsm-config <path>` – Vendor-specific HSM configuration.



### 4.2 Common commands



Encryption / decryption
```
eamsa512 -encrypt -in plaintext.txt -out ciphertext.enc

eamsa512 -decrypt -in ciphertext.enc -out decrypted.txt
```


Self-tests and compliance
```
eamsa512 -test-all

eamsa512 -compliance-report
```


Server mode
```
eamsa512 -serve -config ./config/eamsa512.yaml
```


Key lifecycle operations
```
eamsa512 -generate-key -id primary-enc-key

eamsa512 -rotate-key -id primary-enc-key

eamsa512 -key-status -id primary-enc-key
```




Each command should exit with non‑zero status on failure so it can be scripted in CI/CD pipelines.



---



## 5. Error model and versioning



### 5.1 Error responses (REST)



JSON error responses follow a simple schema:


```json
{

"error": "human-readable message",

"code": "machine-readable-code",

"details": {}

}
```




Example codes:



- `INVALID_ARGUMENT`

- `UNAUTHENTICATED`

- `PERMISSION_DENIED`

- `INTERNAL`

- `HSM_ERROR`

- `COMPLIANCE_FAILURE`



### 5.2 Versioning


- API version is encoded in the base path (`/api/v1`).

- Backwards‑incompatible changes MUST result in a new version (`/api/v2`).

- The Go module should use semantic versioning tags (`v1.x.y`).



---



## 6. Best practices


- Prefer the Go library API for latency‑sensitive, in‑process use.

- Use the REST API for language‑agnostic, networked services.

- Route all key‑material operations through the key manager and HSM integration where available.

- Wrap calls in retry logic for transient HSM or network failures but avoid infinite retries.








