// stats.go
package main

import (
    "fmt"
    "math/rand"
    "time"
)

// Run basic randomness tests
func runBasicTests(data []byte) {
    // Monobit test
    ones := 0
    for _, b := range data {
        for i := 0; i < 8; i++ {
            if (b>>i)&1 == 1 {
                ones++
            }
        }
    }
    totalBits := len(data) * 8
    fmt.Printf("Monobit test: ones=%d, total=%d, ratio=%.4f\n", ones, totalBits, float64(ones)/float64(totalBits))
    // Additional tests can be added
}

// Example usage
func main() {
    rand.Seed(time.Now().UnixNano())
    sample := make([]byte, 1024)
    rand.Read(sample)
    runBasicTests(sample)
}
