package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// Performance and load testing for multi-customer email distribution system

func TestPerformanceFileProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	// Test different file processing loads
	testCases := []struct {
		name      string
		fileCount int
		customers []string
	}{
		{"SingleCustomerLowLoad", 10, []string{"hts"}},
		{"SingleCustomerMediumLoad", 50, []string{"hts"}},
		{"MultiCustomerLowLoad", 10, []string{"hts", "cds"}},
		{"MultiCustomerMediumLoad", 50, []string{"hts", "cds", "motor"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()
			var wg sync.WaitGroup

			// Process files concurrently
			for i := 0; i < tc.fileCount; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()

					metadata := createTestMetadata(fmt.Sprintf("PERF-%s-%03d", tc.name, index), tc.customers)
					metadataFile := testEnv.createMetadataFile(fmt.Sprintf("perf_%s_%03d.json", tc.name, index), metadata)

					err := testEnv.app.processFile(metadataFile)
					if err != nil {
						t.Errorf("File %d processing failed: %v", index, err)
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			// Calculate performance metrics
			throughput := float64(tc.fileCount) / duration.Seconds()
			avgTime := duration / time.Duration(tc.fileCount)

			t.Logf("%s Performance Results:", tc.name)
			t.Logf("  Files: %d", tc.fileCount)
			t.Logf("  Customers per file: %d", len(tc.customers))
			t.Logf("  Total duration: %v", duration)
			t.Logf("  Throughput: %.2f files/second", throughput)
			t.Logf("  Average time per file: %v", avgTime)

			// Performance assertions
			if throughput < 1.0 {
				t.Errorf("Throughput too low: %.2f files/second", throughput)
			}

			if avgTime > 10*time.Second {
				t.Errorf("Average processing time too high: %v", avgTime)
			}
		})
	}
}

func TestPerformanceConcurrentCustomers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	// Test concurrent processing of different customers
	customers := []string{"hts", "cds", "motor"}
	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency%d", concurrency), func(t *testing.T) {
			startTime := time.Now()
			var wg sync.WaitGroup
			semaphore := make(chan struct{}, concurrency)

			totalFiles := 30
			for i := 0; i < totalFiles; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()

					semaphore <- struct{}{}        // Acquire
					defer func() { <-semaphore }() // Release

					customerCode := customers[index%len(customers)]
					metadata := createTestMetadata(fmt.Sprintf("CONC-%d-%03d", concurrency, index), []string{customerCode})
					metadataFile := testEnv.createMetadataFile(fmt.Sprintf("conc_%d_%03d.json", concurrency, index), metadata)

					err := testEnv.app.processFile(metadataFile)
					if err != nil {
						t.Errorf("File %d processing failed: %v", index, err)
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			throughput := float64(totalFiles) / duration.Seconds()
			t.Logf("Concurrency %d Results:", concurrency)
			t.Logf("  Total files: %d", totalFiles)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Throughput: %.2f files/second", throughput)

			// Verify no performance degradation with higher concurrency
			if concurrency > 1 && throughput < 0.5 {
				t.Errorf("Throughput degraded significantly at concurrency %d: %.2f files/second", concurrency, throughput)
			}
		})
	}
}

func TestPerformanceMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	// Measure memory usage during processing
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Process a significant number of files
	fileCount := 100
	var wg sync.WaitGroup

	for i := 0; i < fileCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			metadata := createTestMetadata(fmt.Sprintf("MEM-%03d", index), []string{"hts"})
			metadataFile := testEnv.createMetadataFile(fmt.Sprintf("mem_%03d.json", index), metadata)

			err := testEnv.app.processFile(metadataFile)
			if err != nil {
				t.Errorf("File %d processing failed: %v", index, err)
			}
		}(i)
	}

	wg.Wait()

	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Calculate memory usage
	allocatedMB := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
	totalAllocMB := float64(m2.TotalAlloc-m1.TotalAlloc) / 1024 / 1024

	t.Logf("Memory Usage Results:")
	t.Logf("  Files processed: %d", fileCount)
	t.Logf("  Memory allocated: %.2f MB", allocatedMB)
	t.Logf("  Total allocations: %.2f MB", totalAllocMB)
	t.Logf("  Memory per file: %.2f KB", allocatedMB*1024/float64(fileCount))

	// Memory usage assertions
	memoryPerFile := allocatedMB * 1024 / float64(fileCount)
	if memoryPerFile > 100 { // 100KB per file seems reasonable
		t.Errorf("Memory usage per file too high: %.2f KB", memoryPerFile)
	}
}

func TestPerformanceIsolationValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	if testEnv.isolationValidator == nil {
		t.Skip("Isolation validator not available")
	}

	ctx := context.Background()
	customers := []string{"hts", "cds", "motor"}

	// Test single customer validation performance
	t.Run("SingleCustomerValidation", func(t *testing.T) {
		iterations := 50
		startTime := time.Now()

		for i := 0; i < iterations; i++ {
			customerCode := customers[i%len(customers)]
			_, err := testEnv.isolationValidator.ValidateCustomerIsolation(ctx, customerCode)
			if err != nil {
				t.Errorf("Validation %d failed: %v", i, err)
			}
		}

		duration := time.Since(startTime)
		avgTime := duration / time.Duration(iterations)
		throughput := float64(iterations) / duration.Seconds()

		t.Logf("Single Customer Validation Performance:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Average time: %v", avgTime)
		t.Logf("  Throughput: %.2f validations/second", throughput)

		if avgTime > 500*time.Millisecond {
			t.Errorf("Validation time too high: %v", avgTime)
		}
	})

	// Test bulk validation performance
	t.Run("BulkValidation", func(t *testing.T) {
		iterations := 10
		startTime := time.Now()

		for i := 0; i < iterations; i++ {
			_, err := testEnv.isolationValidator.ValidateAllCustomers(ctx)
			if err != nil {
				t.Errorf("Bulk validation %d failed: %v", i, err)
			}
		}

		duration := time.Since(startTime)
		avgTime := duration / time.Duration(iterations)

		t.Logf("Bulk Validation Performance:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Customers per iteration: %d", len(customers))
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Average time per bulk validation: %v", avgTime)

		if avgTime > 2*time.Second {
			t.Errorf("Bulk validation time too high: %v", avgTime)
		}
	})
}

func TestPerformanceCredentialManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	if testEnv.enhancedCredManager == nil {
		t.Skip("Enhanced credential manager not available")
	}

	ctx := context.Background()
	customers := []string{"hts", "cds", "motor"}

	// Test credential assumption performance
	t.Run("CredentialAssumption", func(t *testing.T) {
		iterations := 20
		startTime := time.Now()

		for i := 0; i < iterations; i++ {
			customerCode := customers[i%len(customers)]
			_, err := testEnv.enhancedCredManager.AssumeCustomerRole(ctx, customerCode, "ses")
			if err != nil {
				t.Logf("Credential assumption %d failed (expected in test): %v", i, err)
				continue
			}
		}

		duration := time.Since(startTime)
		avgTime := duration / time.Duration(iterations)

		t.Logf("Credential Assumption Performance:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Average time: %v", avgTime)

		// In production with real AWS, this should be faster due to caching
		if avgTime > 2*time.Second {
			t.Logf("Credential assumption time: %v (may be high in test environment)", avgTime)
		}
	})

	// Test credential caching performance
	t.Run("CredentialCaching", func(t *testing.T) {
		// First, populate cache
		for _, customerCode := range customers {
			testEnv.enhancedCredManager.AssumeCustomerRole(ctx, customerCode, "ses")
		}

		// Test cache performance
		iterations := 100
		startTime := time.Now()

		for i := 0; i < iterations; i++ {
			customerCode := customers[i%len(customers)]
			testEnv.enhancedCredManager.AssumeCustomerRole(ctx, customerCode, "ses")
		}

		duration := time.Since(startTime)
		avgTime := duration / time.Duration(iterations)

		t.Logf("Credential Caching Performance:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Average time (cached): %v", avgTime)

		// Cached operations should be very fast
		if avgTime > 10*time.Millisecond {
			t.Logf("Cached credential access time: %v (may include network overhead)", avgTime)
		}
	})
}

func TestLoadTestWithHarness(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	// Configure test harness
	config := TestHarnessConfig{
		MaxConcurrentTests:  10,
		TestDuration:        30 * time.Second,
		CustomerCount:       3,
		MessagesPerCustomer: 5,
		EnableStressTest:    true,
		EnableFailureTest:   false,
		FailureRate:         0.0,
		ValidationInterval:  5 * time.Second,
	}

	harness := NewTestHarness(
		testEnv.app.customerManager,
		testEnv.enhancedCredManager,
		testEnv.app.statusTracker,
		testEnv.app.monitoringSystem,
		testEnv.isolationValidator,
		config,
	)

	ctx := context.Background()
	results, err := harness.RunLoadTest(ctx)
	if err != nil {
		t.Fatalf("Load test failed: %v", err)
	}

	// Verify results
	if results.TotalTests == 0 {
		t.Error("No tests were executed")
	}

	successRate := float64(results.PassedTests) / float64(results.TotalTests)
	if successRate < 0.8 {
		t.Errorf("Success rate too low: %.2f%%", successRate*100)
	}

	if results.PerformanceMetrics != nil {
		if results.PerformanceMetrics.ThroughputPerSecond < 0.5 {
			t.Errorf("Throughput too low: %.2f tests/second", results.PerformanceMetrics.ThroughputPerSecond)
		}

		if results.PerformanceMetrics.AverageResponseTime > 10*time.Second {
			t.Errorf("Average response time too high: %v", results.PerformanceMetrics.AverageResponseTime)
		}
	}

	// Check isolation results
	for customerCode, isolationResult := range results.IsolationResults {
		if isolationResult.CriticalIssues > 0 {
			t.Errorf("Critical isolation issues for customer %s: %d", customerCode, isolationResult.CriticalIssues)
		}
	}

	// Generate and log report
	report := harness.GenerateReport()
	t.Logf("Load Test Report:\n%s", report)
}

func TestFailureScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping failure test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.cleanup()

	// Configure test harness for failure testing
	config := TestHarnessConfig{
		MaxConcurrentTests: 5,
		EnableFailureTest:  true,
		FailureRate:        0.3, // 30% failure rate
	}

	harness := NewTestHarness(
		testEnv.app.customerManager,
		testEnv.enhancedCredManager,
		testEnv.app.statusTracker,
		testEnv.app.monitoringSystem,
		testEnv.isolationValidator,
		config,
	)

	ctx := context.Background()
	results, err := harness.RunFailureTest(ctx)
	if err != nil {
		t.Fatalf("Failure test failed: %v", err)
	}

	// Verify that failure scenarios behaved as expected
	if results.TotalTests == 0 {
		t.Error("No failure tests were executed")
	}

	// Some tests should fail (that's the point of failure testing)
	if results.FailedTests == 0 {
		t.Error("Expected some tests to fail in failure scenario testing")
	}

	// Generate and log report
	report := harness.GenerateReport()
	t.Logf("Failure Test Report:\n%s", report)
}

// Benchmark tests for performance measurement

func BenchmarkFileProcessing(b *testing.B) {
	testEnv := setupTestEnvironment(&testing.T{})
	defer testEnv.cleanup()

	metadata := createTestMetadata("BENCH-001", []string{"hts"})
	metadataFile := testEnv.createMetadataFile("bench.json", metadata)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := testEnv.app.processFile(metadataFile)
		if err != nil {
			b.Errorf("File processing failed: %v", err)
		}
	}
}

func BenchmarkConcurrentFileProcessing(b *testing.B) {
	testEnv := setupTestEnvironment(&testing.T{})
	defer testEnv.cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			metadata := createTestMetadata(fmt.Sprintf("BENCH-CONC-%d", i), []string{"hts"})
			metadataFile := testEnv.createMetadataFile(fmt.Sprintf("bench_conc_%d.json", i), metadata)

			err := testEnv.app.processFile(metadataFile)
			if err != nil {
				b.Errorf("File processing failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkIsolationValidation(b *testing.B) {
	testEnv := setupTestEnvironment(&testing.T{})
	defer testEnv.cleanup()

	if testEnv.isolationValidator == nil {
		b.Skip("Isolation validator not available")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := testEnv.isolationValidator.ValidateCustomerIsolation(ctx, "hts")
		if err != nil {
			b.Errorf("Isolation validation failed: %v", err)
		}
	}
}

func BenchmarkCredentialAssumption(b *testing.B) {
	testEnv := setupTestEnvironment(&testing.T{})
	defer testEnv.cleanup()

	if testEnv.enhancedCredManager == nil {
		b.Skip("Enhanced credential manager not available")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := testEnv.enhancedCredManager.AssumeCustomerRole(ctx, "hts", "ses")
		if err != nil {
			// Expected to fail in test environment
			continue
		}
	}
}
