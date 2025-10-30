package concurrent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// CustomerResult represents the result of an operation for a single customer
type CustomerResult struct {
	CustomerCode   string
	CustomerName   string
	Success        bool
	Error          error
	Data           interface{}
	ProcessingTime time.Duration
}

// MultiCustomerSummary aggregates results from multiple customer operations
type MultiCustomerSummary struct {
	TotalCustomers  int
	SuccessfulCount int
	FailedCount     int
	SkippedCount    int
	Results         []CustomerResult
	TotalDuration   time.Duration
}

// CustomerOperation is a function type that performs an operation on a single customer
// It receives the customer code and returns an error if the operation fails
type CustomerOperation func(customerCode string) (interface{}, error)

// ProcessCustomersConcurrently processes multiple customers concurrently using a worker pool pattern
// Parameters:
//   - customerCodes: list of customer codes to process
//   - operation: function to execute for each customer
//   - maxConcurrency: maximum number of concurrent workers (0 or negative means unlimited)
//
// Returns a slice of CustomerResult containing the outcome for each customer
func ProcessCustomersConcurrently(
	customerCodes []string,
	customerNames map[string]string,
	operation CustomerOperation,
	maxConcurrency int,
) []CustomerResult {
	if len(customerCodes) == 0 {
		return []CustomerResult{}
	}

	// Default to processing all customers concurrently if maxConcurrency is not set
	if maxConcurrency <= 0 || maxConcurrency > len(customerCodes) {
		maxConcurrency = len(customerCodes)
	}

	// Create worker pool with semaphore for concurrency control
	semaphore := make(chan struct{}, maxConcurrency)
	results := make(chan CustomerResult, len(customerCodes))
	var wg sync.WaitGroup

	// Launch goroutines for each customer
	for _, custCode := range customerCodes {
		wg.Add(1)
		go func(customerCode string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process customer with timing
			startTime := time.Now()
			result := CustomerResult{
				CustomerCode: customerCode,
				CustomerName: getCustomerName(customerCode, customerNames),
			}

			// Handle panics gracefully
			defer func() {
				result.ProcessingTime = time.Since(startTime)
				if r := recover(); r != nil {
					result.Success = false
					result.Error = fmt.Errorf("panic: %v", r)
				}
				results <- result
			}()

			// Execute operation
			data, err := operation(customerCode)
			if err != nil {
				result.Success = false
				result.Error = err
			} else {
				result.Success = true
				result.Data = data
			}
		}(custCode)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results from all customers
	var allResults []CustomerResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

// AggregateResults aggregates a slice of CustomerResult into a MultiCustomerSummary
func AggregateResults(results []CustomerResult) MultiCustomerSummary {
	summary := MultiCustomerSummary{
		TotalCustomers: len(results),
		Results:        results,
	}

	var totalDuration time.Duration
	for _, result := range results {
		totalDuration += result.ProcessingTime
		if result.Success {
			summary.SuccessfulCount++
		} else if result.Error != nil {
			summary.FailedCount++
		} else {
			summary.SkippedCount++
		}
	}

	// Calculate average duration (total time across all goroutines)
	summary.TotalDuration = totalDuration

	return summary
}

// DisplayCustomerResult displays a single customer result with formatted output
func DisplayCustomerResult(result CustomerResult) {
	status := ""
	statusText := "Success"
	if !result.Success {
		status = ""
		statusText = "Failed"
	}

	customerLabel := result.CustomerCode
	if result.CustomerName != "" {
		customerLabel = fmt.Sprintf("%s (%s)", result.CustomerCode, result.CustomerName)
	}

	fmt.Printf("%s %s: %s (%.2fs)\n", status, customerLabel, statusText, result.ProcessingTime.Seconds())

	if result.Error != nil {
		fmt.Printf("   Error: %v\n", result.Error)
	}
}

// DisplaySummary displays a formatted summary of multi-customer operation results
func DisplaySummary(summary MultiCustomerSummary) {
	fmt.Println()
	fmt.Printf("=" + strings.Repeat("=", 70) + "\n")
	fmt.Printf(" OPERATION SUMMARY\n")
	fmt.Printf("=" + strings.Repeat("=", 70) + "\n")
	fmt.Printf("Total customers: %d\n", summary.TotalCustomers)
	fmt.Printf(" Successful: %d\n", summary.SuccessfulCount)
	fmt.Printf(" Failed: %d\n", summary.FailedCount)
	fmt.Printf(" Skipped: %d\n", summary.SkippedCount)
	fmt.Printf(" Total processing time: %.2fs\n", summary.TotalDuration.Seconds())

	// Display successful customers
	if summary.SuccessfulCount > 0 {
		fmt.Printf("\n Successful customers:\n")
		for _, result := range summary.Results {
			if result.Success {
				customerLabel := result.CustomerCode
				if result.CustomerName != "" {
					customerLabel = fmt.Sprintf("%s (%s)", result.CustomerCode, result.CustomerName)
				}
				fmt.Printf("   - %s (%.2fs)\n", customerLabel, result.ProcessingTime.Seconds())
			}
		}
	}

	// Display failed customers
	if summary.FailedCount > 0 {
		fmt.Printf("\n Failed customers:\n")
		for _, result := range summary.Results {
			if !result.Success && result.Error != nil {
				customerLabel := result.CustomerCode
				if result.CustomerName != "" {
					customerLabel = fmt.Sprintf("%s (%s)", result.CustomerCode, result.CustomerName)
				}
				fmt.Printf("   - %s: %v\n", customerLabel, result.Error)
			}
		}
	}

	// Display skipped customers
	if summary.SkippedCount > 0 {
		fmt.Printf("\n Skipped customers:\n")
		for _, result := range summary.Results {
			if !result.Success && result.Error == nil {
				customerLabel := result.CustomerCode
				if result.CustomerName != "" {
					customerLabel = fmt.Sprintf("%s (%s)", result.CustomerCode, result.CustomerName)
				}
				fmt.Printf("   - %s\n", customerLabel)
			}
		}
	}

	fmt.Printf("=" + strings.Repeat("=", 70) + "\n")
}

// getCustomerName retrieves the customer name from the map, returns empty string if not found
func getCustomerName(customerCode string, customerNames map[string]string) string {
	if customerNames == nil {
		return ""
	}
	return customerNames[customerCode]
}
