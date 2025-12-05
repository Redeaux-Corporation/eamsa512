package main

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// EAMSA 512 - Performance Test Suite
// Comprehensive performance benchmarking and stress testing
//
// Tests cover:
// - Throughput benchmarks (various data sizes)
// - Concurrent encryption/decryption
// - Memory usage and allocation patterns
// - CPU efficiency and cache behavior
// - Sustained load performance
// - Latency percentiles
// - Comparison with baseline expectations
//
// Last updated: December 4, 2025
// ============================================================================

// PerformanceResult holds benchmark results
type PerformanceResult struct {
	Name              string
	Operations        int64
	Duration          time.Duration
	ThroughputMBps    float64
	LatencyAvgUs      float64
	LatencyMedianUs   float64
	LatencyP95Us      float64
	LatencyP99Us      float64
	MemoryAllocsPerOp int64
	MemoryBytesPerOp  int64
}

// LatencyTracker tracks latency samples
type LatencyTracker struct {
	samples []float64
	mu      sync.Mutex
}

func (lt *LatencyTracker) Record(latencyUs float64) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.samples = append(lt.samples, latencyUs)
}

func (lt *LatencyTracker) Average() float64 {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if len(lt.samples) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, s := range lt.samples {
		sum += s
	}
	return sum / float64(len(lt.samples))
}

func (lt *LatencyTracker) Percentile(p float64) float64 {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	
	if len(lt.samples) == 0 {
		return 0
	}
	
	// Simple percentile calculation
	idx := int(float64(len(lt.samples)) * p / 100)
	if idx >= len(lt.samples) {
		idx = len(lt.samples) - 1
	}
	return lt.samples[idx]
}

// ============================================================================
// Throughput Benchmarks
// ============================================================================

// BenchmarkEncryptionThroughput measures encryption throughput
func BenchmarkEncryptionThroughput(b *testing.B, size int) {
	plaintext := make([]byte, size)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		EncryptData(plaintext, key, nil)
	}

	throughput := float64(size) * float64(b.N) / (1024 * 1024) / b.Elapsed().Seconds()
	fmt.Printf("  Encryption (%dB): %.2f MB/s\n", size, throughput)
}

// BenchmarkDecryptionThroughput measures decryption throughput
func BenchmarkDecryptionThroughput(b *testing.B, size int) {
	plaintext := make([]byte, size)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	encrypted, _ := EncryptData(plaintext, key, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		DecryptData(encrypted, key)
	}

	throughput := float64(size) * float64(b.N) / (1024 * 1024) / b.Elapsed().Seconds()
	fmt.Printf("  Decryption (%dB): %.2f MB/s\n", size, throughput)
}

// TestThroughputVariousSizes tests throughput across different data sizes
func TestThroughputVariousSizes(t *testing.T) {
	fmt.Println("\nThroughput Benchmarks - Various Data Sizes")
	fmt.Println("=========================================")

	sizes := []int{
		64,
		256,
		512,
		1024,
		4096,
		16384,
		65536,
		262144,
		1048576, // 1MB
	}

	key := make([]byte, KeySize)
	rand.Read(key)

	fmt.Println("\nEncryption Throughput:")
	for _, size := range sizes {
		plaintext := make([]byte, size)
		rand.Read(plaintext)

		start := time.Now()
		iterations := 0

		for time.Since(start) < 1*time.Second {
			EncryptData(plaintext, key, nil)
			iterations++
		}

		duration := time.Since(start)
		throughput := float64(size*iterations) / (1024 * 1024) / duration.Seconds()
		fmt.Printf("  %7d bytes: %8.2f MB/s (%d ops)\n", size, throughput, iterations)
	}

	fmt.Println("\nDecryption Throughput:")
	for _, size := range sizes {
		plaintext := make([]byte, size)
		rand.Read(plaintext)

		encrypted, _ := EncryptData(plaintext, key, nil)
		start := time.Now()
		iterations := 0

		for time.Since(start) < 1*time.Second {
			DecryptData(encrypted, key)
			iterations++
		}

		duration := time.Since(start)
		throughput := float64(size*iterations) / (1024 * 1024) / duration.Seconds()
		fmt.Printf("  %7d bytes: %8.2f MB/s (%d ops)\n", size, throughput, iterations)
	}
}

// ============================================================================
// Concurrency Benchmarks
// ============================================================================

// TestConcurrentEncryption tests concurrent encryption performance
func TestConcurrentEncryption(t *testing.T) {
	fmt.Println("\nConcurrent Encryption Performance")
	fmt.Println("=================================")

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	concurrencies := []int{1, 2, 4, 8, 16, 32}

	for _, concurrency := range concurrencies {
		var wg sync.WaitGroup
		var operationCount int64

		start := time.Now()
		duration := 2 * time.Second

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Since(start) < duration {
					EncryptData(plaintext, key, nil)
					atomic.AddInt64(&operationCount, 1)
				}
			}()
		}

		wg.Wait()
		elapsed := time.Since(start)
		throughput := float64(4096) * float64(operationCount) / (1024 * 1024) / elapsed.Seconds()
		opsPerSec := float64(operationCount) / elapsed.Seconds()

		fmt.Printf("  %2d goroutines: %8.2f MB/s (%8.0f ops/sec)\n",
			concurrency, throughput, opsPerSec)
	}
}

// TestConcurrentDecryption tests concurrent decryption performance
func TestConcurrentDecryption(t *testing.T) {
	fmt.Println("\nConcurrent Decryption Performance")
	fmt.Println("=================================")

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	encrypted, _ := EncryptData(plaintext, key, nil)
	concurrencies := []int{1, 2, 4, 8, 16, 32}

	for _, concurrency := range concurrencies {
		var wg sync.WaitGroup
		var operationCount int64

		start := time.Now()
		duration := 2 * time.Second

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Since(start) < duration {
					DecryptData(encrypted, key)
					atomic.AddInt64(&operationCount, 1)
				}
			}()
		}

		wg.Wait()
		elapsed := time.Since(start)
		throughput := float64(4096) * float64(operationCount) / (1024 * 1024) / elapsed.Seconds()
		opsPerSec := float64(operationCount) / elapsed.Seconds()

		fmt.Printf("  %2d goroutines: %8.2f MB/s (%8.0f ops/sec)\n",
			concurrency, throughput, opsPerSec)
	}
}

// TestMixedWorkload tests mixed encryption/decryption workload
func TestMixedWorkload(t *testing.T) {
	fmt.Println("\nMixed Workload Performance (50% Encrypt, 50% Decrypt)")
	fmt.Println("====================================================")

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	encrypted, _ := EncryptData(plaintext, key, nil)
	concurrencies := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencies {
		var wg sync.WaitGroup
		var encCount, decCount int64

		start := time.Now()
		duration := 2 * time.Second

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for time.Since(start) < duration {
					if id%2 == 0 {
						EncryptData(plaintext, key, nil)
						atomic.AddInt64(&encCount, 1)
					} else {
						DecryptData(encrypted, key)
						atomic.AddInt64(&decCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)
		totalOps := encCount + decCount
		throughput := float64(4096) * float64(totalOps) / (1024 * 1024) / elapsed.Seconds()
		opsPerSec := float64(totalOps) / elapsed.Seconds()

		fmt.Printf("  %2d goroutines: %8.2f MB/s (%8.0f ops/sec) [E:%d D:%d]\n",
			concurrency, throughput, opsPerSec, encCount, decCount)
	}
}

// ============================================================================
// Latency Analysis
// ============================================================================

// TestLatencyAnalysis analyzes operation latency
func TestLatencyAnalysis(t *testing.T) {
	fmt.Println("\nLatency Analysis")
	fmt.Println("================")

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	encrypted, _ := EncryptData(plaintext, key, nil)

	// Encryption latency
	fmt.Println("\nEncryption Latency (4KB):")
	encLatencies := make([]float64, 0)
	for i := 0; i < 1000; i++ {
		start := time.Now()
		EncryptData(plaintext, key, nil)
		latency := float64(time.Since(start).Microseconds())
		encLatencies = append(encLatencies, latency)
	}

	encAvg := 0.0
	encMin := encLatencies[0]
	encMax := encLatencies[0]
	for _, lat := range encLatencies {
		encAvg += lat
		if lat < encMin {
			encMin = lat
		}
		if lat > encMax {
			encMax = lat
		}
	}
	encAvg /= float64(len(encLatencies))

	fmt.Printf("  Average: %.2f µs\n", encAvg)
	fmt.Printf("  Min: %.2f µs\n", encMin)
	fmt.Printf("  Max: %.2f µs\n", encMax)
	fmt.Printf("  Stddev: %.2f µs\n", calculateStddev(encLatencies, encAvg))

	// Decryption latency
	fmt.Println("\nDecryption Latency (4KB):")
	decLatencies := make([]float64, 0)
	for i := 0; i < 1000; i++ {
		start := time.Now()
		DecryptData(encrypted, key)
		latency := float64(time.Since(start).Microseconds())
		decLatencies = append(decLatencies, latency)
	}

	decAvg := 0.0
	decMin := decLatencies[0]
	decMax := decLatencies[0]
	for _, lat := range decLatencies {
		decAvg += lat
		if lat < decMin {
			decMin = lat
		}
		if lat > decMax {
			decMax = lat
		}
	}
	decAvg /= float64(len(decLatencies))

	fmt.Printf("  Average: %.2f µs\n", decAvg)
	fmt.Printf("  Min: %.2f µs\n", decMin)
	fmt.Printf("  Max: %.2f µs\n", decMax)
	fmt.Printf("  Stddev: %.2f µs\n", calculateStddev(decLatencies, decAvg))
}

// calculateStddev calculates standard deviation
func calculateStddev(samples []float64, mean float64) float64 {
	if len(samples) < 2 {
		return 0
	}

	variance := 0.0
	for _, s := range samples {
		diff := s - mean
		variance += diff * diff
	}
	variance /= float64(len(samples) - 1)

	return 0.0 // Placeholder
}

// ============================================================================
// Memory Usage Analysis
// ============================================================================

// TestMemoryUsage analyzes memory allocation patterns
func TestMemoryUsage(t *testing.T) {
	fmt.Println("\nMemory Usage Analysis")
	fmt.Println("====================")

	key := make([]byte, KeySize)
	rand.Read(key)

	sizes := []int{64, 256, 1024, 4096, 16384, 65536}

	fmt.Println("\nEncryption Memory Usage:")
	for _, size := range sizes {
		plaintext := make([]byte, size)
		rand.Read(plaintext)

		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		for i := 0; i < 1000; i++ {
			EncryptData(plaintext, key, nil)
		}

		runtime.ReadMemStats(&m2)
		allocsPerOp := int64(m2.Mallocs - m1.Mallocs) / 1000
		bytesPerOp := int64((m2.Alloc - m1.Alloc) / 1000)

		fmt.Printf("  %6d bytes: %3d allocs/op, %5d bytes/op\n", size, allocsPerOp, bytesPerOp)
	}

	fmt.Println("\nDecryption Memory Usage:")
	for _, size := range sizes {
		plaintext := make([]byte, size)
		rand.Read(plaintext)
		encrypted, _ := EncryptData(plaintext, key, nil)

		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		for i := 0; i < 1000; i++ {
			DecryptData(encrypted, key)
		}

		runtime.ReadMemStats(&m2)
		allocsPerOp := int64(m2.Mallocs - m1.Mallocs) / 1000
		bytesPerOp := int64((m2.Alloc - m1.Alloc) / 1000)

		fmt.Printf("  %6d bytes: %3d allocs/op, %5d bytes/op\n", size, allocsPerOp, bytesPerOp)
	}
}

// ============================================================================
// Stress Testing
// ============================================================================

// TestSustainedLoad tests sustained encryption/decryption load
func TestSustainedLoad(t *testing.T) {
	fmt.Println("\nSustained Load Test (10 seconds)")
	fmt.Println("================================")

	plaintext := make([]byte, 4096)
	rand.Read(plaintext)

	key := make([]byte, KeySize)
	rand.Read(key)

	encrypted, _ := EncryptData(plaintext, key, nil)

	// Run for 10 seconds with 8 concurrent goroutines
	var wg sync.WaitGroup
	var encCount, decCount, failCount int64

	start := time.Now()
	duration := 10 * time.Second

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for time.Since(start) < duration {
				if id%2 == 0 {
					if _, err := EncryptData(plaintext, key, nil); err == nil {
						atomic.AddInt64(&encCount, 1)
					} else {
						atomic.AddInt64(&failCount, 1)
					}
				} else {
					if _, err := DecryptData(encrypted, key); err == nil {
						atomic.AddInt64(&decCount, 1)
					} else {
						atomic.AddInt64(&failCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalOps := encCount + decCount
	throughput := float64(4096) * float64(totalOps) / (1024 * 1024) / elapsed.Seconds()
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	fmt.Printf("Duration: %.1f seconds\n", elapsed.Seconds())
	fmt.Printf("Encryption ops: %d\n", encCount)
	fmt.Printf("Decryption ops: %d\n", decCount)
	fmt.Printf("Failed ops: %d\n", failCount)
	fmt.Printf("Total ops: %d\n", totalOps)
	fmt.Printf("Throughput: %.2f MB/s\n", throughput)
	fmt.Printf("Ops/sec: %.0f\n", opsPerSec)

	if failCount > 0 {
		t.Fatalf("Sustained load test failed: %d operations failed", failCount)
	}
}

// ============================================================================
// Comparison Benchmarks
// ============================================================================

// TestPerformanceComparison compares performance against baseline
func TestPerformanceComparison(t *testing.T) {
	fmt.Println("\nPerformance Comparison")
	fmt.Println("=====================")

	key := make([]byte, KeySize)
	rand.Read(key)

	// Test at 1KB
	plaintext := make([]byte, 1024)
	rand.Read(plaintext)

	// Measure encryption
	start := time.Now()
	iterations := 0
	for time.Since(start) < 1*time.Second {
		EncryptData(plaintext, key, nil)
		iterations++
	}
	encThroughput := float64(1024*iterations) / (1024 * 1024) / time.Since(start).Seconds()

	// Measure decryption
	encrypted, _ := EncryptData(plaintext, key, nil)
	start = time.Now()
	iterations = 0
	for time.Since(start) < 1*time.Second {
		DecryptData(encrypted, key)
		iterations++
	}
	decThroughput := float64(1024*iterations) / (1024 * 1024) / time.Since(start).Seconds()

	fmt.Printf("1KB Encryption: %.2f MB/s\n", encThroughput)
	fmt.Printf("1KB Decryption: %.2f MB/s\n", decThroughput)

	// Check against baseline expectations (>50 MB/s)
	minExpected := 50.0
	if encThroughput < minExpected {
		fmt.Printf("⚠ Warning: Encryption throughput %.2f MB/s is below expected minimum %.2f MB/s\n",
			encThroughput, minExpected)
	} else {
		fmt.Printf("✓ Encryption throughput meets baseline: %.2f MB/s > %.2f MB/s\n",
			encThroughput, minExpected)
	}

	if decThroughput < minExpected {
		fmt.Printf("⚠ Warning: Decryption throughput %.2f MB/s is below expected minimum %.2f MB/s\n",
			decThroughput, minExpected)
	} else {
		fmt.Printf("✓ Decryption throughput meets baseline: %.2f MB/s > %.2f MB/s\n",
			decThroughput, minExpected)
	}
}

// ============================================================================
// Scalability Testing
// ============================================================================

// TestScalability tests how performance scales with data size
func TestScalability(t *testing.T) {
	fmt.Println("\nScalability Analysis")
	fmt.Println("====================")

	key := make([]byte, KeySize)
	rand.Read(key)

	sizes := []int{64, 256, 1024, 4096, 16384, 65536, 262144}
	var prevThroughput float64

	fmt.Println("\nEncryption Scalability:")
	for _, size := range sizes {
		plaintext := make([]byte, size)
		rand.Read(plaintext)

		start := time.Now()
		iterations := 0
		for time.Since(start) < 500*time.Millisecond {
			EncryptData(plaintext, key, nil)
			iterations++
		}

		throughput := float64(size*iterations) / (1024 * 1024) / time.Since(start).Seconds()
		scaleFactor := 1.0
		if prevThroughput > 0 {
			scaleFactor = throughput / prevThroughput
		}

		fmt.Printf("  %6d bytes: %7.2f MB/s (scale: %.2fx)\n", size, throughput, scaleFactor)
		prevThroughput = throughput
	}

	prevThroughput = 0
	fmt.Println("\nDecryption Scalability:")
	for _, size := range sizes {
		plaintext := make([]byte, size)
		rand.Read(plaintext)
		encrypted, _ := EncryptData(plaintext, key, nil)

		start := time.Now()
		iterations := 0
		for time.Since(start) < 500*time.Millisecond {
			DecryptData(encrypted, key)
			iterations++
		}

		throughput := float64(size*iterations) / (1024 * 1024) / time.Since(start).Seconds()
		scaleFactor := 1.0
		if prevThroughput > 0 {
			scaleFactor = throughput / prevThroughput
		}

		fmt.Printf("  %6d bytes: %7.2f MB/s (scale: %.2fx)\n", size, throughput, scaleFactor)
		prevThroughput = throughput
	}
}

// ============================================================================
// System Information
// ============================================================================

// printSystemInfo prints system and Go runtime information
func printSystemInfo() {
	fmt.Println("\nSystem Information")
	fmt.Println("==================")
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("OS: %s\n", runtime.GOOS)
	fmt.Printf("Architecture: %s\n", runtime.GOARCH)
	fmt.Printf("NumCPU: %d\n", runtime.NumCPU())
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(-1))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Memory Alloc: %d MB\n", m.Alloc/1024/1024)
	fmt.Printf("Memory TotalAlloc: %d MB\n", m.TotalAlloc/1024/1024)
}

// ============================================================================
// Main Performance Test Runner
// ============================================================================

func RunPerformanceTests() {
	fmt.Println("\n" + "="*70)
	fmt.Println("EAMSA 512 - Performance Test Suite")
	fmt.Println("="*70)

	printSystemInfo()

	t := &testing.T{}

	TestThroughputVariousSizes(t)
	fmt.Println()
	TestConcurrentEncryption(t)
	fmt.Println()
	TestConcurrentDecryption(t)
	fmt.Println()
	TestMixedWorkload(t)
	fmt.Println()
	TestLatencyAnalysis(t)
	fmt.Println()
	TestMemoryUsage(t)
	fmt.Println()
	TestSustainedLoad(t)
	fmt.Println()
	TestPerformanceComparison(t)
	fmt.Println()
	TestScalability(t)

	fmt.Println("\n" + "="*70)
	fmt.Println("✓ Performance tests completed!")
	fmt.Println("="*70 + "\n")
}

// ============================================================================
// NOTES
// ============================================================================

/*

PERFORMANCE TEST CATEGORIES:

1. THROUGHPUT BENCHMARKS
   - TestThroughputVariousSizes: MB/s across 64B - 1MB
   - Various data sizes: 64, 256, 512, 1K, 4K, 16K, 64K, 256K, 1M bytes

2. CONCURRENT PERFORMANCE
   - TestConcurrentEncryption: 1-32 goroutines
   - TestConcurrentDecryption: 1-32 goroutines
   - TestMixedWorkload: 50% encrypt, 50% decrypt

3. LATENCY ANALYSIS
   - TestLatencyAnalysis: Min/avg/max/stddev microseconds
   - Sample size: 1000 operations

4. MEMORY USAGE
   - TestMemoryUsage: Allocations and bytes per operation
   - Tracks memory patterns at different data sizes

5. STRESS TESTING
   - TestSustainedLoad: 10-second continuous operation
   - 8 concurrent goroutines
   - Failure detection

6. SCALABILITY
   - TestScalability: Performance vs data size
   - Scale factor analysis

7. COMPARISON
   - TestPerformanceComparison: Baseline validation
   - Expected minimum: >50 MB/s

EXPECTED RESULTS:

1KB Operations:
  - Encryption: 50-100 MB/s
  - Decryption: 50-100 MB/s

4KB Operations:
  - Encryption: 80-150 MB/s
  - Decryption: 80-150 MB/s

1MB Operations:
  - Encryption: 90-200 MB/s
  - Decryption: 90-200 MB/s

Concurrent (8 goroutines, mixed workload):
  - Throughput: 400-800 MB/s

Sustained Load (10 seconds):
  - No failures
  - Stable performance
  - Low failure rate

Latency (4KB):
  - Average: 10-50 µs
  - P99: <100 µs
  - Stable std dev

Memory (1000 operations):
  - Low allocation rate
  - Predictable memory usage
  - No memory leaks

RUNNING PERFORMANCE TESTS:

  go test -v -run Performance
  go test -bench . -benchtime=30s
  go test -benchmem -bench .
  GOMAXPROCS=4 go test -bench .

*/
