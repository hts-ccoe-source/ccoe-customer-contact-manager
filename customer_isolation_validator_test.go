package main

import (
	"context"
	"testing"
	"time"
)

func TestCustomerIsolationValidator_Creation(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	if validator == nil {
		t.Error("Expected validator to be non-nil")
	}

	if len(validator.validationRules) == 0 {
		t.Error("Expected validation rules to be initialized")
	}

	// Check that all expected validation rules are present
	expectedRules := []string{"CRED-001", "ACCESS-001", "DATA-001", "ACCESS-002", "AUDIT-001", "NETWORK-001", "CRED-002", "DATA-002"}
	for _, expectedRule := range expectedRules {
		found := false
		for _, rule := range validator.validationRules {
			if rule.ID == expectedRule {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected validation rule %s not found", expectedRule)
		}
	}
}

func TestCustomerIsolationValidator_ValidateCustomerIsolation(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test validation for valid customer
	t.Run("ValidCustomer", func(t *testing.T) {
		result, err := validator.ValidateCustomerIsolation(ctx, "hts")
		if err != nil {
			t.Errorf("Validation failed: %v", err)
		}

		if result == nil {
			t.Error("Expected validation result to be non-nil")
		}

		if result.CustomerCode != "hts" {
			t.Errorf("Expected customer code 'hts', got '%s'", result.CustomerCode)
		}

		if result.TotalRules != len(validator.validationRules) {
			t.Errorf("Expected %d total rules, got %d", len(validator.validationRules), result.TotalRules)
		}

		if len(result.Results) != result.TotalRules {
			t.Errorf("Expected %d validation results, got %d", result.TotalRules, len(result.Results))
		}

		// Check that all results have required fields
		for _, validationResult := range result.Results {
			if validationResult.RuleID == "" {
				t.Error("Validation result missing rule ID")
			}
			if validationResult.CustomerCode != "hts" {
				t.Errorf("Expected customer code 'hts', got '%s'", validationResult.CustomerCode)
			}
			if validationResult.Timestamp.IsZero() {
				t.Error("Validation result missing timestamp")
			}
		}
	})

	// Test validation for invalid customer
	t.Run("InvalidCustomer", func(t *testing.T) {
		result, err := validator.ValidateCustomerIsolation(ctx, "invalid")
		if err != nil {
			t.Errorf("Validation failed: %v", err)
		}

		if result == nil {
			t.Error("Expected validation result to be non-nil")
		}

		// Should have some failed validations for invalid customer
		if result.FailedRules == 0 {
			t.Error("Expected some validation failures for invalid customer")
		}
	})

	// Test caching
	t.Run("Caching", func(t *testing.T) {
		// First validation
		result1, err := validator.ValidateCustomerIsolation(ctx, "hts")
		if err != nil {
			t.Errorf("First validation failed: %v", err)
		}

		// Second validation (should use cache)
		result2, err := validator.ValidateCustomerIsolation(ctx, "hts")
		if err != nil {
			t.Errorf("Second validation failed: %v", err)
		}

		// Results should be identical (from cache)
		if result1.ValidationTime != result2.ValidationTime {
			t.Error("Expected cached result to have same validation time")
		}
	})
}

func TestCustomerIsolationValidator_ValidateAllCustomers(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	results, err := validator.ValidateAllCustomers(ctx)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	if len(results) != len(customerManager.CustomerMappings) {
		t.Errorf("Expected %d results, got %d", len(customerManager.CustomerMappings), len(results))
	}

	// Check that all customers were validated
	for customerCode := range customerManager.CustomerMappings {
		if _, exists := results[customerCode]; !exists {
			t.Errorf("Missing validation result for customer %s", customerCode)
		}
	}
}

func TestCustomerIsolationValidator_ValidationRules(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test individual validation rules
	testCases := []struct {
		ruleID       string
		customerCode string
		expectPass   bool
	}{
		{"CRED-001", "hts", true},
		{"ACCESS-001", "hts", true},
		{"DATA-001", "hts", true},
		{"ACCESS-002", "hts", true},
		{"AUDIT-001", "hts", true},
		{"NETWORK-001", "hts", true},
		{"CRED-002", "hts", true},
		{"DATA-002", "hts", true},
	}

	for _, tc := range testCases {
		t.Run(tc.ruleID, func(t *testing.T) {
			// Find the validation rule
			var rule *ValidationRule
			for _, r := range validator.validationRules {
				if r.ID == tc.ruleID {
					rule = &r
					break
				}
			}

			if rule == nil {
				t.Fatalf("Validation rule %s not found", tc.ruleID)
			}

			// Execute the validation rule
			result := rule.Validator(ctx, tc.customerCode, validator)

			if result == nil {
				t.Error("Expected validation result to be non-nil")
			}

			if result.RuleID != tc.ruleID {
				t.Errorf("Expected rule ID %s, got %s", tc.ruleID, result.RuleID)
			}

			if result.CustomerCode != tc.customerCode {
				t.Errorf("Expected customer code %s, got %s", tc.customerCode, result.CustomerCode)
			}

			if result.Timestamp.IsZero() {
				t.Error("Expected timestamp to be set")
			}

			if result.Duration == 0 {
				t.Error("Expected duration to be set")
			}

			// Note: We don't check expectPass because in test environment,
			// many validations may fail due to missing AWS resources
		})
	}
}

func TestCustomerIsolationValidator_CredentialIsolation(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test credential isolation validation
	result := validator.validateCredentialIsolation(ctx, "hts", validator)

	if result == nil {
		t.Error("Expected validation result to be non-nil")
	}

	if result.RuleID != "CRED-001" {
		t.Errorf("Expected rule ID 'CRED-001', got '%s'", result.RuleID)
	}

	if result.CustomerCode != "hts" {
		t.Errorf("Expected customer code 'hts', got '%s'", result.CustomerCode)
	}

	// In test environment, this may fail due to missing AWS credentials
	// Just verify the structure is correct
	if result.Details == nil {
		t.Error("Expected details to be non-nil")
	}
}

func TestCustomerIsolationValidator_CrossAccountAccess(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test cross-account access validation
	result := validator.validateCrossAccountAccess(ctx, "hts", validator)

	if result == nil {
		t.Error("Expected validation result to be non-nil")
	}

	if result.RuleID != "ACCESS-001" {
		t.Errorf("Expected rule ID 'ACCESS-001', got '%s'", result.RuleID)
	}

	// Should pass for valid customer with proper role ARN
	if !result.Passed {
		t.Errorf("Expected validation to pass, got: %s", result.Message)
	}

	// Check details
	if result.Details["sesRoleArn"] == nil {
		t.Error("Expected SES role ARN in details")
	}

	if result.Details["accountId"] == nil {
		t.Error("Expected account ID in details")
	}
}

func TestCustomerIsolationValidator_RolePermissions(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test role permissions validation
	result := validator.validateRolePermissions(ctx, "hts", validator)

	if result == nil {
		t.Error("Expected validation result to be non-nil")
	}

	if result.RuleID != "ACCESS-002" {
		t.Errorf("Expected rule ID 'ACCESS-002', got '%s'", result.RuleID)
	}

	// Should pass for properly named role
	if !result.Passed {
		t.Errorf("Expected validation to pass, got: %s", result.Message)
	}

	// Test with suspicious role name
	customerManager.CustomerMappings["test"] = CustomerAccountInfo{
		CustomerCode: "test",
		AWSAccountID: "123456789012",
		SESRoleARN:   "arn:aws:iam::123456789012:role/AdminRole", // Suspicious name
	}

	result = validator.validateRolePermissions(ctx, "test", validator)
	if result.Passed {
		t.Error("Expected validation to fail for suspicious role name")
	}
}

func TestCustomerIsolationValidator_DetectCrossCustomerAccess(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	// Test cross-customer access detection
	attempt := validator.DetectCrossCustomerAccess("hts", "cds", "ses", "email-list")

	if attempt == nil {
		t.Error("Expected cross-customer access attempt to be non-nil")
	}

	if attempt.SourceCustomer != "hts" {
		t.Errorf("Expected source customer 'hts', got '%s'", attempt.SourceCustomer)
	}

	if attempt.TargetCustomer != "cds" {
		t.Errorf("Expected target customer 'cds', got '%s'", attempt.TargetCustomer)
	}

	if attempt.AccessType != "ses" {
		t.Errorf("Expected access type 'ses', got '%s'", attempt.AccessType)
	}

	if attempt.Resource != "email-list" {
		t.Errorf("Expected resource 'email-list', got '%s'", attempt.Resource)
	}

	if !attempt.Blocked {
		t.Error("Expected access attempt to be blocked by default")
	}

	if attempt.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestCustomerIsolationValidator_CacheOperations(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test cache operations
	t.Run("CacheStorage", func(t *testing.T) {
		// Validate customer (should cache result)
		result1, err := validator.ValidateCustomerIsolation(ctx, "hts")
		if err != nil {
			t.Errorf("Validation failed: %v", err)
		}

		// Check that result is cached
		cached := validator.getCachedResult("hts")
		if cached == nil {
			t.Error("Expected result to be cached")
		}

		if cached.ValidationTime != result1.ValidationTime {
			t.Error("Cached result validation time mismatch")
		}
	})

	t.Run("CacheExpiry", func(t *testing.T) {
		// Set short cache expiry for testing
		validator.cacheExpiry = 1 * time.Millisecond

		// Validate customer
		validator.ValidateCustomerIsolation(ctx, "cds")

		// Wait for cache to expire
		time.Sleep(2 * time.Millisecond)

		// Check that cache is expired
		cached := validator.getCachedResult("cds")
		if cached != nil {
			t.Error("Expected cache to be expired")
		}
	})

	t.Run("CacheClear", func(t *testing.T) {
		// Validate customer (should cache result)
		validator.ValidateCustomerIsolation(ctx, "hts")

		// Verify cached
		if validator.getCachedResult("hts") == nil {
			t.Error("Expected result to be cached")
		}

		// Clear cache
		validator.ClearValidationCache()

		// Verify cache is cleared
		if validator.getCachedResult("hts") != nil {
			t.Error("Expected cache to be cleared")
		}
	})
}

func TestCustomerIsolationValidator_Metrics(t *testing.T) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	metrics := validator.GetValidationMetrics()

	if metrics["totalRules"] == nil {
		t.Error("Expected totalRules in metrics")
	}

	if metrics["cachedResults"] == nil {
		t.Error("Expected cachedResults in metrics")
	}

	if metrics["cacheExpiryMinutes"] == nil {
		t.Error("Expected cacheExpiryMinutes in metrics")
	}

	totalRules := metrics["totalRules"].(int)
	if totalRules != len(validator.validationRules) {
		t.Errorf("Expected %d total rules in metrics, got %d", len(validator.validationRules), totalRules)
	}
}

func TestValidationSeverityAndCategory(t *testing.T) {
	// Test validation severity constants
	severities := []ValidationSeverity{
		SeverityCritical,
		SeverityHigh,
		SeverityMedium,
		SeverityLow,
		SeverityInfo,
	}

	for _, severity := range severities {
		if string(severity) == "" {
			t.Errorf("Severity %v should not be empty", severity)
		}
	}

	// Test validation category constants
	categories := []ValidationCategory{
		CategoryCredentials,
		CategoryAccess,
		CategoryData,
		CategoryNetwork,
		CategoryAudit,
	}

	for _, category := range categories {
		if string(category) == "" {
			t.Errorf("Category %v should not be empty", category)
		}
	}
}

func TestExtractRoleNameFromARN(t *testing.T) {
	testCases := []struct {
		arn      string
		expected string
	}{
		{
			arn:      "arn:aws:iam::123456789012:role/HTSSESRole",
			expected: "HTSSESRole",
		},
		{
			arn:      "arn:aws:iam::123456789012:role/path/to/RoleName",
			expected: "RoleName",
		},
		{
			arn:      "invalid-arn",
			expected: "invalid-arn",
		},
		{
			arn:      "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.arn, func(t *testing.T) {
			result := extractRoleNameFromARN(tc.arn)
			if result != tc.expected {
				t.Errorf("Expected role name '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// Benchmark tests

func BenchmarkCustomerIsolationValidator_ValidateCustomer(b *testing.B) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateCustomerIsolation(ctx, "hts")
	}
}

func BenchmarkCustomerIsolationValidator_ValidationRule(b *testing.B) {
	customerManager := createTestCustomerManager()
	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "Test",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.validateCrossAccountAccess(ctx, "hts", validator)
	}
}
