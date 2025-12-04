// phase2-msa.go - Modified SALSA20 (MSA) with SIMD Vectorization
package main

import (
	"encoding/binary"
	"sync"
)

// MSAState represents Modified SALSA20 state (4×4 matrix)
type MSAState struct {
	Matrix [4][4]uint32
	mu     sync.RWMutex
}

// NewMSAState creates new MSA state
func NewMSAState(key1, key2 [16]byte, nonce [16]byte) *MSAState {
	state := &MSAState{}

	// Initialize 4×4 matrix from keys and nonce
	state.Matrix[0][0] = binary.LittleEndian.Uint32(key1[0:4])
	state.Matrix[0][1] = binary.LittleEndian.Uint32(key1[4:8])
	state.Matrix[0][2] = binary.LittleEndian.Uint32(key1[8:12])
	state.Matrix[0][3] = binary.LittleEndian.Uint32(key1[12:16])

	state.Matrix[1][0] = binary.LittleEndian.Uint32(key2[0:4])
	state.Matrix[1][1] = binary.LittleEndian.Uint32(key2[4:8])
	state.Matrix[1][2] = binary.LittleEndian.Uint32(key2[8:12])
	state.Matrix[1][3] = binary.LittleEndian.Uint32(key2[12:16])

	state.Matrix[2][0] = binary.LittleEndian.Uint32(nonce[0:4])
	state.Matrix[2][1] = binary.LittleEndian.Uint32(nonce[4:8])
	state.Matrix[2][2] = binary.LittleEndian.Uint32(nonce[8:12])
	state.Matrix[2][3] = binary.LittleEndian.Uint32(nonce[12:16])

	// Counter row
	state.Matrix[3][0] = 0
	state.Matrix[3][1] = 0
	state.Matrix[3][2] = 0
	state.Matrix[3][3] = 0

	return state
}

// MSAStepDiagonal performs diagonal operations with SIMD-style parallelism
func (ms *MSAState) MSAStepDiagonal() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Diagonal operations (can be parallelized)
	// T = T XOR rotate(T, 7) XOR rotate(T, 1)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			val := ms.Matrix[i][j]
			rotated7 := rotateLeft(val, 7)
			rotated1 := rotateLeft(val, 1)
			ms.Matrix[i][j] ^= rotated7 ^ rotated1
		}
	}
}

// MSAStepCrossDiagonal performs cross-diagonal operations
func (ms *MSAState) MSAStepCrossDiagonal() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Cross-diagonal operations
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			val := ms.Matrix[i][j]
			nextVal := ms.Matrix[(i+1)%4][(j+1)%4]
			prevVal := ms.Matrix[(i-1+4)%4][(j-1+4)%4]

			ms.Matrix[i][j] ^= (val + nextVal) ^ (val + prevVal)
		}
	}
}

// MSAFinalStep performs final transpose-based mixing
func (ms *MSAState) MSAFinalStep() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Transpose-like operation (row/column mix)
	temp := [4][4]uint32{}

	// Copy and transpose
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			temp[i][j] = ms.Matrix[i][j]
		}
	}

	// Mix rows with columns
	for i := 0; i < 4; i++ {
		ms.Matrix[i][0] ^= temp[i][1] + temp[i][2] + temp[i][3]
		ms.Matrix[i][1] ^= temp[i][0] + temp[i][2] + temp[i][3]
		ms.Matrix[i][2] ^= temp[i][0] + temp[i][1] + temp[i][3]
		ms.Matrix[i][3] ^= temp[i][0] + temp[i][1] + temp[i][2]
	}
}

// MSAround performs one complete MSA round
func (ms *MSAState) MSAround() {
	ms.MSAStepDiagonal()
	ms.MSAStepCrossDiagonal()
	ms.MSAFinalStep()
}

// GetOutput extracts 64-byte output from MSA state
func (ms *MSAState) GetOutput() [64]byte {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var output [64]byte
	idx := 0

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			binary.LittleEndian.PutUint32(output[idx:idx+4], ms.Matrix[i][j])
			idx += 4
		}
	}

	return output
}

// PerformMSAEncryption performs 11-round MSA encryption
func PerformMSAEncryption(input [64]byte, keys [11][16]byte) [64]byte {
	// Split input into two 32-byte halves
	left := input[0:32]
	right := input[32:64]

	// Create MSA state from keys 7-11
	msa := NewMSAState(keys[7], keys[8], keys[9][:])

	// 11 rounds of MSA encryption
	for round := 0; round < 11; round++ {
		msa.MSAround()

		// XOR with keys
		output := msa.GetOutput()
		for i := 0; i < 32; i++ {
			left[i] ^= output[i]
			right[i] ^= output[i+32]
		}

		// Mix keys with rotation
		for i := 0; i < 16; i++ {
			keys[7][i] = rotateLeft8(keys[7][i])
			keys[8][i] = rotateLeft8(keys[8][i])
		}
	}

	// Combine output
	result := [64]byte{}
	copy(result[0:32], left)
	copy(result[32:64], right)

	return result
}

// rotateLeft rotates uint32 left by n bits
func rotateLeft(val uint32, n uint) uint32 {
	return (val << n) | (val >> (32 - n))
}

// rotateLeft8 rotates byte left by 1 bit
func rotateLeft8(val byte) byte {
	return (val << 1) | (val >> 7)
}

// IncrementCounter increments MSA counter
func (ms *MSAState) IncrementCounter() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.Matrix[3][3]++
}

// SetCounter sets MSA counter to specific value
func (ms *MSAState) SetCounter(value uint32) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.Matrix[3][3] = value
}
