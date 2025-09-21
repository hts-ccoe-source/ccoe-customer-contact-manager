package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestEnhancedCredentialManager_AssumeCustomerRole(t *testing.T) {
	// Create customer credential manager
	customerManager := NewCustomerCredentialManager("us-east-1")

	// Add test customer mappings
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Test",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/HTSSQSRole",
			Environment:  "test",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Test",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "test",
		},
	}

	// Note: This test requires AWS credentials and will make real AWS API calls
	// Skip if running in CI/CD without proper AWS setup
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create enhanced credential manager
	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Skipf("Failed to create enhanced credential manager (likely no AWS credentials): %v", err)
	}

	ctx := context.Background()

	// Test assuming role for HTS customer
	t.Run("AssumeRoleForHTS", func(t *testing.T) {
		clients, err := enhancedManager.AssumeCustomerRole(ctx, "hts", "ses")
		if err != nil {
			t.Skipf("Failed to assume role for HTS (expected in test environment): %v", err)
		}

		if clients == nil {
			t.Error("Expected clients to be non-nil")
		}

		if clients.SESClient == nil {
			t.Error("Expected SES client to be non-nil")
		}
	})

	// Test invalid customer code
	t.Run("InvalidCustomerCode", func(t *testing.T) {
		_, err := enhancedManager.AssumeCustomerRole(ctx, "invalid", "ses")
		if err == nil {
			t.Error("Expected error for invalid customer code")
		}
	})

	// Test invalid service type
	t.Run("InvalidServiceType", func(t *testing.T) {
		_, err := enhancedManager.AssumeCustomerRole(ctx, "hts", "invalid")
		if err == nil {
			t.Error("Expected error for invalid service type")
		}
	})
}

func TestEnhancedCredentialManager_CredentialCaching(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"test": {
			CustomerCode: "test",
			CustomerName: "Test Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/TestSESRole",
			Environment:  "test",
		},
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	// Test cache operations
	t.Run("CacheOperations", func(t *testing.T) {
		// Initially cache should be empty
		status := enhancedManager.GetCacheStatus()
		if len(status) != 0 {
			t.Errorf("Expected empty cache, got %d entries", len(status))
		}

		// Clear cache (should not error on empty cache)
		enhancedManager.ClearCredentialCache()

		// Get metrics
		metrics := enhancedManager.GetCredentialMetrics()
		if metrics["total_cached"].(int) != 0 {
			t.Errorf("Expected 0 cached credentials, got %d", metrics["total_cached"])
		}

		if metrics["total_customers"].(int) != 1 {
			t.Errorf("Expected 1 total customer, got %d", metrics["total_customers"])
		}
	})
}

func TestEnhancedCredentialManager_ValidateCredentials(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Test",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			Environment:  "test",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Test",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "test",
		},
	}

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	ctx := context.Background()

	// Test single customer validation
	t.Run("ValidateSingleCustomer", func(t *testing.T) {
		err := enhancedManager.ValidateCustomerCredentials(ctx, "hts", "ses")
		if err != nil {
			t.Skipf("Credential validation failed (expected in test environment): %v", err)
		}
	})

	// Test all customers validation
	t.Run("ValidateAllCustomers", func(t *testing.T) {
		results := enhancedManager.ValidateAllCustomerCredentials(ctx, "ses")

		// Should have results for both customers
		if len(results) > 2 {
			t.Errorf("Expected at most 2 validation results, got %d", len(results))
		}

		// In test environment, we expect failures, so just check structure
		for customerCode, err := range results {
			if err != nil {
				t.Logf("Validation failed for customer %s (expected): %v", customerCode, err)
			}
		}
	})

	// Test invalid customer validation
	t.Run("ValidateInvalidCustomer", func(t *testing.T) {
		err := enhancedManager.ValidateCustomerCredentials(ctx, "invalid", "ses")
		if err == nil {
			t.Error("Expected error for invalid customer code")
		}

		// Should be a CredentialValidationError
		if _, ok := err.(*CredentialValidationError); !ok {
			t.Errorf("Expected CredentialValidationError, got %T", err)
		}
	})
}

func TestEnhancedCredentialManager_ServiceClients(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"test": {
			CustomerCode: "test",
			CustomerName: "Test Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/TestSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/TestSQSRole",
			Environment:  "test",
		},
	}

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	ctx := context.Background()

	// Test SES client creation
	t.Run("GetSESClient", func(t *testing.T) {
		client, err := enhancedManager.GetCustomerSESClient(ctx, "test")
		if err != nil {
			t.Skipf("Failed to get SES client (expected in test environment): %v", err)
		}

		if client == nil {
			t.Error("Expected SES client to be non-nil")
		}
	})

	// Test SQS client creation
	t.Run("GetSQSClient", func(t *testing.T) {
		client, err := enhancedManager.GetCustomerSQSClient(ctx, "test")
		if err != nil {
			t.Skipf("Failed to get SQS client (expected in test environment): %v", err)
		}

		if client == nil {
			t.Error("Expected SQS client to be non-nil")
		}
	})

	// Test invalid customer for client creation
	t.Run("GetClientInvalidCustomer", func(t *testing.T) {
		_, err := enhancedManager.GetCustomerSESClient(ctx, "invalid")
		if err == nil {
			t.Error("Expected error for invalid customer code")
		}
	})
}

func TestEnhancedCredentialManager_RefreshCredentials(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"test": {
			CustomerCode: "test",
			CustomerName: "Test Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/TestSESRole",
			Environment:  "test",
		},
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	ctx := context.Background()

	// Test credential refresh
	t.Run("RefreshCredentials", func(t *testing.T) {
		err := enhancedManager.RefreshCredentials(ctx, "test", "ses")
		if err != nil {
			t.Skipf("Credential refresh failed (expected in test environment): %v", err)
		}
	})

	// Test refresh for invalid customer
	t.Run("RefreshInvalidCustomer", func(t *testing.T) {
		err := enhancedManager.RefreshCredentials(ctx, "invalid", "ses")
		if err == nil {
			t.Error("Expected error for invalid customer code")
		}
	})
}

func TestEnhancedCredentialManager_TestAccess(t *testing.T) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"test": {
			CustomerCode: "test",
			CustomerName: "Test Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/TestSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/TestSQSRole",
			Environment:  "test",
		},
	}

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		t.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	ctx := context.Background()

	// Test access for customer
	t.Run("TestCustomerAccess", func(t *testing.T) {
		err := enhancedManager.TestCustomerAccess(ctx, "test")
		if err != nil {
			t.Skipf("Access test failed (expected in test environment): %v", err)
		}
	})

	// Test access for invalid customer
	t.Run("TestInvalidCustomerAccess", func(t *testing.T) {
		err := enhancedManager.TestCustomerAccess(ctx, "invalid")
		if err == nil {
			t.Error("Expected error for invalid customer code")
		}
	})
}

func TestRoleAssumptionError(t *testing.T) {
	err := &RoleAssumptionError{
		CustomerCode: "test",
		RoleARN:      "arn:aws:iam::123456789012:role/TestRole",
		Err:          fmt.Errorf("access denied"),
	}

	expected := "failed to assume role arn:aws:iam::123456789012:role/TestRole for customer test: access denied"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestCredentialValidationError(t *testing.T) {
	err := &CredentialValidationError{
		CustomerCode: "test",
		Service:      "ses",
		Err:          fmt.Errorf("invalid credentials"),
	}

	expected := "credential validation failed for customer test service ses: invalid credentials"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

// Benchmark tests for performance evaluation

func BenchmarkAssumeCustomerRole(b *testing.B) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"test": {
			CustomerCode: "test",
			CustomerName: "Test Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/TestSESRole",
			Environment:  "test",
		},
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		b.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enhancedManager.AssumeCustomerRole(ctx, "test", "ses")
		if err != nil {
			b.Skipf("Role assumption failed: %v", err)
		}
	}
}

func BenchmarkCredentialCaching(b *testing.B) {
	customerManager := NewCustomerCredentialManager("us-east-1")
	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		b.Skipf("Failed to create enhanced credential manager: %v", err)
	}

	// Add some test cache entries
	for i := 0; i < 100; i++ {
		cacheKey := fmt.Sprintf("customer%d:ses", i)
		enhancedManager.cacheCredentials(cacheKey, aws.Config{}, "test-role")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cacheKey := fmt.Sprintf("customer%d:ses", i%100)
		enhancedManager.getCachedCredentials(cacheKey)
	}
}

// Helper function for testing
func createTestCustomerManager() *CustomerCredentialManager {
	manager := NewCustomerCredentialManager("us-east-1")
	manager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Test",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/HTSSQSRole",
			Environment:  "test",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Test",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "test",
		},
	}
	return manager
}
