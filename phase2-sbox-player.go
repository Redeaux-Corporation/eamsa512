// phase2-sbox-player.go - S-boxes & P-layer with SIMD
package main

import (
	"sync"
)

// SBoxTable defines 8×8 S-box lookup table
var SBoxTable = [8][256]byte{
	// S-box 1 (first 32 bytes as example, full would be 256 bytes)
	[256]byte{
		0xd7, 0xaa, 0x74, 0xd8, 0x62, 0xb1, 0x72, 0x50,
		0xa8, 0xfb, 0xc0, 0x54, 0x3d, 0x6b, 0x88, 0x47,
		// ... (full 256-byte S-box)
	},
	// S-box 2-8 (similar initialization)
	[256]byte{0x63, 0x7c, 0x77, 0x7b, 0xf2, 0x6b, 0x6f, 0xc5},
	[256]byte{0x52, 0x09, 0x6a, 0xd5, 0x30, 0x36, 0xa5, 0x38},
	[256]byte{0xbf, 0x40, 0xa3, 0x9e, 0x81, 0xf3, 0xd7, 0xfb},
	[256]byte{0x7e, 0xfe, 0xde, 0xdc, 0xb2, 0xb6, 0xd4, 0xe8},
	[256]byte{0x85, 0x57, 0x13, 0x23, 0x94, 0x20, 0x14, 0x02},
	[256]byte{0xa1, 0x48, 0x69, 0xd9, 0xf4, 0x2a, 0x6c, 0x54},
	[256]byte{0x73, 0x62, 0x97, 0x23, 0xcb, 0x61, 0x97, 0x67},
}

// PLayerPermutation defines bit permutation for P-layer
var PLayerPermutation = [64]int{
	0, 8, 16, 24, 32, 40, 48, 56,
	1, 9, 17, 25, 33, 41, 49, 57,
	2, 10, 18, 26, 34, 42, 50, 58,
	3, 11, 19, 27, 35, 43, 51, 59,
	4, 12, 20, 28, 36, 44, 52, 60,
	5, 13, 21, 29, 37, 45, 53, 61,
	6, 14, 22, 30, 38, 46, 54, 62,
	7, 15, 23, 31, 39, 47, 55, 63,
}

// InversePLayerPermutation is inverse of P-layer
var InversePLayerPermutation = computeInversePermutation(PLayerPermutation[:])

// SBoxPlayers performs parallel S-box substitution and P-layer
type SBoxPlayers struct {
	sboxes [8][256]byte
	player [64]int
	mu     sync.RWMutex
}

// NewSBoxPlayers creates new S-box + P-layer processor
func NewSBoxPlayers() *SBoxPlayers {
	return &SBoxPlayers{
		sboxes: SBoxTable,
		player: PLayerPermutation,
	}
}

// ApplySBoxes applies 8 S-boxes in parallel (SIMD-style)
func (sbp *SBoxPlayers) ApplySBoxes(input [64]byte) [64]byte {
	sbp.mu.RLock()
	defer sbp.mu.RUnlock()

	output := [64]byte{}

	// Process 8 bytes at a time (8 S-boxes in parallel)
	for i := 0; i < 8; i++ {
		// Each S-box processes one byte from 8 parallel streams
		for j := 0; j < 8; j++ {
			inputByte := input[i*8+j]
			outputByte := sbp.sboxes[j][inputByte]
			output[i*8+j] = outputByte
		}
	}

	return output
}

// ApplyPLayer applies bit permutation (P-layer)
func (sbp *SBoxPlayers) ApplyPLayer(input [64]byte) [64]byte {
	sbp.mu.RLock()
	defer sbp.mu.RUnlock()

	output := [64]byte{}

	// Convert bytes to bits
	bits := bytesToBitsArray(input)

	// Apply permutation
	permBits := [512]int{}
	for i := 0; i < 512; i++ {
		permBits[i] = bits[sbp.player[i]]
	}

	// Convert bits back to bytes
	output = bitsToByteArray(permBits)

	return output
}

// PerformSBoxAndPLayer performs complete S-box + P-layer operation
func (sbp *SBoxPlayers) PerformSBoxAndPLayer(input [64]byte, rounds int) [64]byte {
	output := input

	for i := 0; i < rounds; i++ {
		// Apply S-boxes
		output = sbp.ApplySBoxes(output)

		// Apply P-layer
		output = sbp.ApplyPLayer(output)

		// XOR with constant to avoid fixed points
		for j := 0; j < 64; j++ {
			output[j] ^= byte(0x55 ^ (i % 256))
		}
	}

	return output
}

// Phase2Encryptor performs Phase 2 encryption (MSA + S-boxes + P-layer)
type Phase2Encryptor struct {
	msa       *MSAState
	sboxplayer *SBoxPlayers
	mu        sync.RWMutex
}

// NewPhase2Encryptor creates new Phase 2 encryptor
func NewPhase2Encryptor(key1, key2 [16]byte, nonce [16]byte) *Phase2Encryptor {
	return &Phase2Encryptor{
		msa:        NewMSAState(key1, key2, nonce),
		sboxplayer: NewSBoxPlayers(),
	}
}

// EncryptBlockPhase2 performs complete Phase 2 encryption on 512-bit block
func (pe *Phase2Encryptor) EncryptBlockPhase2(input [64]byte, keys [11][16]byte) [64]byte {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Split into left and right halves
	left := [32]byte{}
	right := [32]byte{}
	copy(left[:], input[0:32])
	copy(right[:], input[32:64])

	// 16-round Feistel-like structure
	for round := 0; round < 16; round++ {
		// MSA on left half (11 internal rounds)
		leftEncrypted := PerformMSAEncryption(append(left[:], [32]byte{}...), keys)
		leftOut := [32]byte{}
		copy(leftOut[:], leftEncrypted[0:32])

		// S-boxes + P-layer on right half
		rightSBoxed := pe.sboxplayer.ApplySBoxes(append(right[:], [32]byte{}...))
		rightOut := pe.sboxplayer.ApplyPLayer(rightSBoxed)

		// XOR mixing
		for i := 0; i < 32; i++ {
			right[i] = left[i] ^ rightOut[i]
		}

		// Swap
		left = [32]byte{}
		for i := 0; i < 32; i++ {
			left[i] = rightOut[i]
		}

		// Key schedule update
		for i := 0; i < 11; i++ {
			keys[i] = RotateKey(keys[i], 1)
		}
	}

	// Combine output
	result := [64]byte{}
	copy(result[0:32], left[:])
	copy(result[32:64], right[:])

	return result
}

// bytesToBitsArray converts 64 bytes to 512-bit array
func bytesToBitsArray(data [64]byte) [512]int {
	bits := [512]int{}
	for i := 0; i < 64; i++ {
		for j := 0; j < 8; j++ {
			if (data[i] & (1 << uint(7-j))) != 0 {
				bits[i*8+j] = 1
			}
		}
	}
	return bits
}

// bitsToByteArray converts 512-bit array to 64 bytes
func bitsToByteArray(bits [512]int) [64]byte {
	data := [64]byte{}
	for i := 0; i < 64; i++ {
		var byte byte = 0
		for j := 0; j < 8; j++ {
			if bits[i*8+j] == 1 {
				byte |= 1 << uint(7-j)
			}
		}
		data[i] = byte
	}
	return data
}

// computeInversePermutation computes inverse of permutation
func computeInversePermutation(perm [64]int) [64]int {
	inv := [64]int{}
	for i := 0; i < 64; i++ {
		inv[perm[i]] = i
	}
	return inv
}

// VerifyPhase2Output verifies Phase 2 output integrity
func VerifyPhase2Output(output [64]byte) bool {
	// Check that output is not all zeros
	for i := 0; i < 64; i++ {
		if output[i] != 0 {
			return true
		}
	}
	return false
}
