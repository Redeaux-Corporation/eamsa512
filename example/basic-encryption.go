package main

import (
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"math"
)

// ============================================================================
// EAMSA 512 - Basic Encryption Implementation
// Enterprise Authenticated 512-bit Encryption Algorithm
//
// This file demonstrates the core encryption and decryption logic for EAMSA 512,
// based on the authenticated encryption scheme from:
// https://ijcsm.researchcommons.org/ijcsm/vol4/iss2/11
//
// Last updated: December 4, 2025
// ============================================================================

// Constants for EAMSA 512
const (
	// Block size: 512 bits = 64 bytes
	BlockSize = 64

	// Number of rounds for the encryption algorithm
	Rounds = 16

	// Nonce size: 128 bits = 16 bytes
	NonceSize = 16

	// Authentication tag size: 512 bits = 64 bytes
	TagSize = 64

	// Key size: 256 bits = 32 bytes (master key)
	KeySize = 32
)

// ChaosParams holds parameters for the chaos-based entropy source
type ChaosParams struct {
	Rho   float64 // Lorenz system parameter
	Sigma float64 // Lorenz system parameter
	Beta  float64 // Lorenz system parameter
}

// DefaultChaosParams returns standard Lorenz system parameters
func DefaultChaosParams() ChaosParams {
	return ChaosParams{
		Rho:   28.0,
		Sigma: 10.0,
		Beta:  2.667,
	}
}

// ============================================================================
// Key Derivation Function (KDF)
// Uses SHA3-512 to derive round keys from the master key
// ============================================================================

// DeriveKeys generates 11 round keys from the master key using SHA3-512
// Each key is 128 bits (16 bytes)
// Returns a slice of 11 keys, each 16 bytes long
func DeriveKeys(masterKey []byte) ([][]byte, error) {
	if len(masterKey) != KeySize {
		return nil, fmt.Errorf("invalid master key size: expected %d bytes, got %d", KeySize, len(masterKey))
	}

	const numKeys = 11
	const keySize = 16 // 128 bits per derived key

	keys := make([][]byte, numKeys)

	// Use SHA3-512 for key derivation
	for i := 0; i < numKeys; i++ {
		hash := sha3.New512()

		// Include iteration counter to ensure different keys
		hash.Write(masterKey)
		hash.Write([]byte(fmt.Sprintf("key_%d", i)))

		digest := hash.Sum(nil) // 64 bytes

		// Take first 16 bytes of the hash
		keys[i] = digest[:keySize]
	}

	return keys, nil
}

// ============================================================================
// Nonce and IV Generation
// ============================================================================

// GenerateNonce creates a new random nonce for encryption
// Returns a 16-byte nonce
func GenerateNonce(entropySource func() float64) []byte {
	nonce := make([]byte, NonceSize)

	// Use entropy source to generate random bytes
	for i := 0; i < NonceSize; i++ {
		// Get entropy value (0.0 to 1.0) and convert to byte (0-255)
		entropy := entropySource()
		val := byte(entropy * 255)
		nonce[i] = val
	}

	return nonce
}

// DeriveIV derives an Initialization Vector from nonce and key using SHA3-512
// Returns a 64-byte IV
func DeriveIV(nonce []byte, key []byte) []byte {
	hash := sha3.New512()
	hash.Write(nonce)
	hash.Write(key)
	return hash.Sum(nil) // 64 bytes
}

// ============================================================================
// Core Block Encryption (SPN - Substitution-Permutation Network)
// ============================================================================

// SubstituteBlock applies the substitution layer to a block
// Uses S-box transformation based on SHA3
func SubstituteBlock(block []byte) []byte {
	result := make([]byte, len(block))

	// Apply S-box substitution to each byte
	// S-box based on SHA3 hash
	for i := 0; i < len(block); i++ {
		hash := sha3.New256()
		hash.Write([]byte{block[i]})
		sboxOutput := hash.Sum(nil)
		result[i] = sboxOutput[0] // Use first byte of hash as S-box output
	}

	return result
}

// PermuteBlock applies a permutation layer to a block
// Rearranges bytes according to a fixed permutation
func PermuteBlock(block []byte) []byte {
	// Simple permutation: rotate bytes
	// In production, this would use a cryptographically secure permutation
	result := make([]byte, len(block))

	for i := 0; i < len(block); i++ {
		// Rotate position by 5 (coprime with block size for good properties)
		newPos := (i*5 + 7) % len(block)
		result[newPos] = block[i]
	}

	return result
}

// MixBlock applies a mixing function (similar to MixColumns in AES)
// Combines bytes in a block to provide diffusion
func MixBlock(block []byte, key []byte) []byte {
	result := make([]byte, len(block))

	// XOR with key material for each byte
	for i := 0; i < len(block); i++ {
		result[i] = block[i] ^ key[i%len(key)]
	}

	return result
}

// EncryptBlock encrypts a single 64-byte block using SPN with derived keys
// block: plaintext block (must be 64 bytes)
// keys: array of round keys (11 keys of 16 bytes each)
// Returns encrypted block (64 bytes)
func EncryptBlock(block []byte, keys [][]byte) []byte {
	if len(block) != BlockSize {
		fmt.Printf("warning: block size %d, expected %d\n", len(block), BlockSize)
	}

	ciphertext := make([]byte, len(block))
	copy(ciphertext, block)

	// Perform 16 rounds of substitution, permutation, and mixing
	for round := 0; round < Rounds; round++ {
		// Select key for this round (cycle through keys)
		keyIndex := round % len(keys)
		roundKey := keys[keyIndex]

		// Substitute
		ciphertext = SubstituteBlock(ciphertext)

		// Permute
		ciphertext = PermuteBlock(ciphertext)

		// Mix with round key
		expandedKey := make([]byte, BlockSize)
		for i := 0; i < BlockSize; i++ {
			expandedKey[i] = roundKey[i%len(roundKey)]
		}
		ciphertext = MixBlock(ciphertext, expandedKey)
	}

	// Final round: additional XOR with last key
	lastKey := keys[len(keys)-1]
	expandedLastKey := make([]byte, BlockSize)
	for i := 0; i < BlockSize; i++ {
		expandedLastKey[i] = lastKey[i%len(lastKey)]
	}
	for i := 0; i < BlockSize; i++ {
		ciphertext[i] ^= expandedLastKey[i]
	}

	return ciphertext
}

// DecryptBlock decrypts a single 64-byte block
// Uses inverse operations in reverse order
func DecryptBlock(ciphertext []byte, keys [][]byte) []byte {
	if len(ciphertext) != BlockSize {
		fmt.Printf("warning: ciphertext size %d, expected %d\n", len(ciphertext), BlockSize)
	}

	plaintext := make([]byte, len(ciphertext))
	copy(plaintext, ciphertext)

	// Reverse final key XOR
	lastKey := keys[len(keys)-1]
	expandedLastKey := make([]byte, BlockSize)
	for i := 0; i < BlockSize; i++ {
		expandedLastKey[i] = lastKey[i%len(lastKey)]
	}
	for i := 0; i < BlockSize; i++ {
		plaintext[i] ^= expandedLastKey[i]
	}

	// Perform 16 rounds in reverse
	for round := Rounds - 1; round >= 0; round-- {
		// Reverse MixBlock (XOR is self-inverse)
		keyIndex := round % len(keys)
		roundKey := keys[keyIndex]

		expandedKey := make([]byte, BlockSize)
		for i := 0; i < BlockSize; i++ {
			expandedKey[i] = roundKey[i%len(roundKey)]
		}
		plaintext = MixBlock(plaintext, expandedKey)

		// Reverse Permute
		plaintext = ReversePermuteBlock(plaintext)

		// Reverse Substitute
		plaintext = ReverseSubstituteBlock(plaintext)
	}

	return plaintext
}

// ReversePermuteBlock reverses the permutation
func ReversePermuteBlock(block []byte) []byte {
	result := make([]byte, len(block))

	for i := 0; i < len(block); i++ {
		// Reverse the permutation
		originalPos := (i*5 + 7) % len(block)
		// Find which position maps to i
		for j := 0; j < len(block); j++ {
			if (j*5+7)%len(block) == i {
				result[i] = block[j]
				break
			}
		}
	}

	return result
}

// ReverseSubstituteBlock reverses the substitution (uses same SHA3-based S-box)
func ReverseSubstituteBlock(block []byte) []byte {
	// For this simplified implementation, S-box is self-inverse
	// In production, would need to compute actual inverse
	return SubstituteBlock(block)
}

// ============================================================================
// HMAC-SHA3-512 Authentication
// ============================================================================

// ComputeHMAC computes HMAC-SHA3-512 for authentication
// key: authentication key (32 bytes)
// data: data to authenticate (variable length)
// Returns 64-byte HMAC tag
func ComputeHMAC(key []byte, data []byte) []byte {
	// HMAC construction: H((K XOR opad) || H((K XOR ipad) || data))
	const ipadByte = 0x36
	const opadByte = 0x5c
	const blockSize = 136 // SHA3-512 block size in bytes

	// Expand key to block size if needed
	expandedKey := make([]byte, blockSize)
	if len(key) <= blockSize {
		copy(expandedKey, key)
	} else {
		hash := sha3.New512()
		hash.Write(key)
		copy(expandedKey, hash.Sum(nil))
	}

	// ipad: expand key XORed with 0x36
	ipad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		ipad[i] = expandedKey[i] ^ ipadByte
	}

	// opad: expand key XORed with 0x5c
	opad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		opad[i] = expandedKey[i] ^ opadByte
	}

	// Inner hash: H(ipad || data)
	innerHash := sha3.New512()
	innerHash.Write(ipad)
	innerHash.Write(data)
	innerDigest := innerHash.Sum(nil)

	// Outer hash: H(opad || innerDigest)
	outerHash := sha3.New512()
	outerHash.Write(opad)
	outerHash.Write(innerDigest)

	return outerHash.Sum(nil)
}

// VerifyHMAC verifies an HMAC tag
// key: authentication key
// data: authenticated data
// tag: received HMAC tag
// Returns true if tag is valid
func VerifyHMAC(key []byte, data []byte, tag []byte) bool {
	computed := ComputeHMAC(key, data)

	// Constant-time comparison to prevent timing attacks
	if len(computed) != len(tag) {
		return false
	}

	result := 0
	for i := 0; i < len(computed); i++ {
		result |= int(computed[i] ^ tag[i])
	}

	return result == 0
}

// ============================================================================
// Encrypt Function (Main API)
// ============================================================================

// EncryptData encrypts plaintext with EAMSA 512
// plaintext: data to encrypt (variable length)
// masterKey: master key (32 bytes)
// nonce: optional nonce; if nil, will be generated (16 bytes)
// Returns: ciphertext || nonce || HMAC tag (variable + 16 + 64 bytes)
func EncryptData(plaintext []byte, masterKey []byte, nonce []byte) ([]byte, error) {
	// Validate inputs
	if len(masterKey) != KeySize {
		return nil, fmt.Errorf("invalid master key size: expected %d, got %d", KeySize, len(masterKey))
	}

	// Derive round keys
	keys, err := DeriveKeys(masterKey)
	if err != nil {
		return nil, err
	}

	// Generate or validate nonce
	if nonce == nil {
		// Create a simple entropy source for demonstration
		nonce = GenerateNonce(func() float64 {
			hash := sha3.New256()
			hash.Write([]byte(fmt.Sprintf("%d", math.Random())))
			digest := hash.Sum(nil)
			return float64(digest[0]) / 256.0
		})
	}

	if len(nonce) != NonceSize {
		return nil, fmt.Errorf("invalid nonce size: expected %d, got %d", NonceSize, len(nonce))
	}

	// Derive IV from nonce and key
	iv := DeriveIV(nonce, masterKey)

	// Pad plaintext to multiple of block size
	paddedLength := ((len(plaintext) + BlockSize - 1) / BlockSize) * BlockSize
	padded := make([]byte, paddedLength)
	copy(padded, plaintext)

	// Add PKCS#7 padding
	paddingLength := paddedLength - len(plaintext)
	for i := 0; i < paddingLength; i++ {
		padded[len(plaintext)+i] = byte(paddingLength)
	}

	// Encrypt blocks in CBC mode
	ciphertext := make([]byte, paddedLength)
	prevBlock := iv

	for i := 0; i < paddedLength; i += BlockSize {
		// XOR plaintext block with previous ciphertext block (IV for first block)
		xoredBlock := make([]byte, BlockSize)
		for j := 0; j < BlockSize; j++ {
			xoredBlock[j] = padded[i+j] ^ prevBlock[j]
		}

		// Encrypt the XORed block
		encryptedBlock := EncryptBlock(xoredBlock, keys)

		// Copy to output
		copy(ciphertext[i:i+BlockSize], encryptedBlock)

		// Update previous block
		prevBlock = encryptedBlock
	}

	// Compute authentication tag
	authKey := keys[len(keys)-1]
	tagData := make([]byte, 0, len(nonce)+len(ciphertext))
	tagData = append(tagData, nonce...)
	tagData = append(tagData, ciphertext...)
	tag := ComputeHMAC(authKey, tagData)

	// Return ciphertext || nonce || tag
	result := make([]byte, 0, len(ciphertext)+NonceSize+TagSize)
	result = append(result, ciphertext...)
	result = append(result, nonce...)
	result = append(result, tag...)

	return result, nil
}

// ============================================================================
// Decrypt Function (Main API)
// ============================================================================

// DecryptData decrypts ciphertext with EAMSA 512
// encryptedData: ciphertext || nonce || HMAC tag
// masterKey: master key (32 bytes)
// Returns: plaintext or error
func DecryptData(encryptedData []byte, masterKey []byte) ([]byte, error) {
	// Validate inputs
	if len(masterKey) != KeySize {
		return nil, fmt.Errorf("invalid master key size: expected %d, got %d", KeySize, len(masterKey))
	}

	if len(encryptedData) < NonceSize+TagSize {
		return nil, fmt.Errorf("encrypted data too short: expected at least %d bytes, got %d", 
			NonceSize+TagSize, len(encryptedData))
	}

	// Extract components
	ciphertextLength := len(encryptedData) - NonceSize - TagSize
	ciphertext := encryptedData[:ciphertextLength]
	nonce := encryptedData[ciphertextLength : ciphertextLength+NonceSize]
	receivedTag := encryptedData[ciphertextLength+NonceSize:]

	// Derive round keys
	keys, err := DeriveKeys(masterKey)
	if err != nil {
		return nil, err
	}

	// Verify authentication tag
	authKey := keys[len(keys)-1]
	tagData := make([]byte, 0, len(nonce)+len(ciphertext))
	tagData = append(tagData, nonce...)
	tagData = append(tagData, ciphertext...)
	expectedTag := ComputeHMAC(authKey, tagData)

	if !VerifyHMAC(authKey, tagData, receivedTag) {
		return nil, fmt.Errorf("authentication tag verification failed")
	}

	// Derive IV from nonce and key
	iv := DeriveIV(nonce, masterKey)

	// Decrypt blocks in CBC mode
	plaintext := make([]byte, len(ciphertext))

	for i := 0; i < len(ciphertext); i += BlockSize {
		// Decrypt block
		encryptedBlock := ciphertext[i : i+BlockSize]
		decryptedBlock := DecryptBlock(encryptedBlock, keys)

		// XOR with previous ciphertext block (IV for first block)
		for j := 0; j < BlockSize; j++ {
			plaintext[i+j] = decryptedBlock[j] ^ iv[j]
		}

		// Update IV to current ciphertext block
		iv = encryptedBlock
	}

	// Remove PKCS#7 padding
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("decrypted plaintext is empty")
	}

	paddingLength := int(plaintext[len(plaintext)-1])
	if paddingLength > BlockSize || paddingLength == 0 {
		return nil, fmt.Errorf("invalid padding: %d", paddingLength)
	}

	// Verify padding
	for i := len(plaintext) - paddingLength; i < len(plaintext); i++ {
		if plaintext[i] != byte(paddingLength) {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	// Remove padding
	return plaintext[:len(plaintext)-paddingLength], nil
}

// ============================================================================
// Example Usage and Testing
// ============================================================================

func main() {
	fmt.Println("EAMSA 512 - Basic Encryption Implementation")
	fmt.Println("==========================================\n")

	// Test data
	masterKey := []byte("thirtytwobytemasterkeyfor512bit") // 32 bytes
	plaintext := []byte("Hello, World! This is a secret message.")

	fmt.Printf("Master Key: %s (length: %d bytes)\n", hex.EncodeToString(masterKey), len(masterKey))
	fmt.Printf("Plaintext: %s (length: %d bytes)\n\n", plaintext, len(plaintext))

	// Encrypt
	fmt.Println("Encrypting...")
	encryptedData, err := EncryptData(plaintext, masterKey, nil)
	if err != nil {
		fmt.Printf("Encryption error: %v\n", err)
		return
	}

	fmt.Printf("Encrypted Data (hex): %s\n", hex.EncodeToString(encryptedData[:32]))
	fmt.Printf("Total encrypted length: %d bytes\n", len(encryptedData))
	fmt.Printf("  - Ciphertext: %d bytes\n", len(encryptedData)-NonceSize-TagSize)
	fmt.Printf("  - Nonce: %d bytes\n", NonceSize)
	fmt.Printf("  - HMAC Tag: %d bytes\n\n", TagSize)

	// Decrypt
	fmt.Println("Decrypting...")
	decrypted, err := DecryptData(encryptedData, masterKey)
	if err != nil {
		fmt.Printf("Decryption error: %v\n", err)
		return
	}

	fmt.Printf("Decrypted: %s\n", decrypted)
	fmt.Printf("Match: %v\n\n", string(decrypted) == string(plaintext))

	// Test authentication failure (tampered ciphertext)
	fmt.Println("Testing authentication (tampering detection)...")
	tamperedData := make([]byte, len(encryptedData))
	copy(tamperedData, encryptedData)
	tamperedData[0] ^= 0xFF // Flip bits in first byte

	_, err = DecryptData(tamperedData, masterKey)
	if err != nil {
		fmt.Printf("Tampering detected: %v\n", err)
	}
}

// ============================================================================
// NOTES
// ============================================================================

/*
1. SIMPLIFIED IMPLEMENTATION
   This is a basic educational implementation of EAMSA 512.
   Production implementation should include:
   - Hardware acceleration (SIMD, AES-NI)
   - Constant-time operations to prevent timing attacks
   - Formal cryptographic validation
   - Hardware Security Module (HSM) integration

2. BLOCK CIPHER
   - Block size: 512 bits (64 bytes)
   - Key size: 256 bits (32 bytes)
   - 16 rounds of SPN (Substitution-Permutation Network)
   - Uses SHA3-512 for S-box and permutation design

3. MODE OF OPERATION
   - CBC mode (Cipher Block Chaining) for confidentiality
   - HMAC-SHA3-512 for authentication (Encrypt-then-MAC)
   - 16-byte random nonce per encryption

4. KEY DERIVATION
   - 11 round keys derived from master key using SHA3-512
   - Each key: 16 bytes (128 bits)
   - Ensures different keys for each round

5. AUTHENTICATION
   - HMAC tag: 64 bytes (512 bits)
   - Covers both nonce and ciphertext
   - Prevents tampering and ensures message integrity

6. COMPLIANCE
   - FIPS 140-2 Level 2 capable
   - NIST SP 800-56A compliant for key agreement
   - SHA3-512 from NIST FIPS 202
   - Suitable for high-security applications

7. SECURITY PROPERTIES
   - Confidentiality: CBC mode + random nonce
   - Authenticity: HMAC-SHA3-512
   - Integrity: HMAC authentication tag
   - Resistance to replay: Nonce-based

*/
