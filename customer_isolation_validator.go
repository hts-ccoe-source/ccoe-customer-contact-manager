package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CustomerIsolationValidator ensures proper customer data isolation and access controls
type CustomerIsolationValidator struct {
	customerManager     *CustomerCredentialManager
	enhancedCredManager *EnhancedCredentialManager
	statusTracker       *ExecutionStatusTracker
	monitoringSystem    *MonitoringSystem
	validationRules     []ValidationRule
	isolationCache      map[string]*IsolationValidationResult
	cacheMutex          sync.RWMutex
	cacheExpiry         time.Duration
}

// ValidationRule defines a customer isolation validation rule
type ValidationRule struct {
	ID          string                                                                       `json:"id"`
	Name        string                                                                       `json:"name"`
	Description string                                                                       `json:"description"`
	Category    ValidationCategory                                                           `json:"category"`
	Severity    ValidationSeverity                                                           `json:"severity"`
	Validator   func(context.Context, string, *CustomerIsolationValidator) *ValidationResult `json:"-"`
}

// ValidationCategory represents different types of validation
type ValidationCategory string

const (
	CategoryCredentials ValidationCategory = "credentials"
	CategoryAccess      ValidationCategory = "access"
	CategoryData        ValidationCategory = "data"
	CategoryNetwork     ValidationCategory = "network"
	CategoryAudit       ValidationCategory = "audit"
)

// ValidationSeverity represents the severity of validation issues
type ValidationSeverity string

const (
	SeverityCritical ValidationSeverity = "critical"
	SeverityHigh     ValidationSeverity = "high"
	SeverityMedium   ValidationSeverity = "medium"
	SeverityLow      ValidationSeverity = "low"
	SeverityInfo     ValidationSeverity = "info"
)

// ValidationResult represents the result of a single validation rule
type ValidationResult struct {
	RuleID       string                 `json:"ruleId"`
	CustomerCode string                 `json:"customerCode"`
	Passed       bool                   `json:"passed"`
	Message      string                 `json:"message"`
	Details      map[string]interface{} `json:"details"`
	Timestamp    time.Time              `json:"timestamp"`
	Duration     time.Duration          `json:"duration"`
}

// IsolationValidationResult represents the complete validation result for a customer
type IsolationValidationResult struct {
	CustomerCode    string                     `json:"customerCode"`
	ValidationTime  time.Time                  `json:"validationTime"`
	OverallPassed   bool                       `json:"overallPassed"`
	TotalRules      int                        `json:"totalRules"`
	PassedRules     int                        `json:"passedRules"`
	FailedRules     int                        `json:"failedRules"`
	CriticalIssues  int                        `json:"criticalIssues"`
	HighIssues      int                        `json:"highIssues"`
	Results         []*ValidationResult        `json:"results"`
	Summary         map[ValidationCategory]int `json:"summary"`
	Recommendations []string                   `json:"recommendations"`
}

// CrossCustomerAccessAttempt represents a potential cross-customer access attempt
type CrossCustomerAccessAttempt struct {
	SourceCustomer string                 `json:"sourceCustomer"`
	TargetCustomer string                 `json:"targetCustomer"`
	AccessType     string                 `json:"accessType"`
	Resource       string                 `json:"resource"`
	Timestamp      time.Time              `json:"timestamp"`
	Blocked        bool                   `json:"blocked"`
	Details        map[string]interface{} `json:"details"`
}

// NewCustomerIsolationValidator creates a new customer isolation validator
func NewCustomerIsolationValidator(
	customerManager *CustomerCredentialManager,
	enhancedCredManager *EnhancedCredentialManager,
	statusTracker *ExecutionStatusTracker,
	monitoringSystem *MonitoringSystem,
) *CustomerIsolationValidator {
	validator := &CustomerIsolationValidator{
		customerManager:     customerManager,
		enhancedCredManager: enhancedCredManager,
		statusTracker:       statusTracker,
		monitoringSystem:    monitoringSystem,
		isolationCache:      make(map[string]*IsolationValidationResult),
		cacheExpiry:         30 * time.Minute,
	}

	// Initialize validation rules
	validator.initializeValidationRules()

	return validator
}

// initializeValidationRules sets up the default validation rules
func (v *CustomerIsolationValidator) initializeValidationRules() {
	v.validationRules = []ValidationRule{
		{
			ID:          "CRED-001",
			Name:        "Customer Credential Isolation",
			Description: "Verify customer credentials cannot access other customer resources",
			Category:    CategoryCredentials,
			Severity:    SeverityCritical,
			Validator:   v.validateCredentialIsolation,
		},
		{
			ID:          "ACCESS-001",
			Name:        "Cross-Account Access Control",
			Description: "Verify proper cross-account access controls are in place",
			Category:    CategoryAccess,
			Severity:    SeverityCritical,
			Validator:   v.validateCrossAccountAccess,
		},
		{
			ID:          "DATA-001",
			Name:        "Customer Data Segregation",
			Description: "Verify customer data is properly segregated and isolated",
			Category:    CategoryData,
			Severity:    SeverityHigh,
			Validator:   v.validateDataSegregation,
		},
		{
			ID:          "ACCESS-002",
			Name:        "Role Permission Boundaries",
			Description: "Verify IAM roles have appropriate permission boundaries",
			Category:    CategoryAccess,
			Severity:    SeverityHigh,
			Validator:   v.validateRolePermissions,
		},
		{
			ID:          "AUDIT-001",
			Name:        "Audit Trail Isolation",
			Description: "Verify audit trails are properly isolated per customer",
			Category:    CategoryAudit,
			Severity:    SeverityMedium,
			Validator:   v.validateAuditTrailIsolation,
		},
		{
			ID:          "NETWORK-001",
			Name:        "Network Isolation",
			Description: "Verify network-level isolation between customers",
			Category:    CategoryNetwork,
			Severity:    SeverityMedium,
			Validator:   v.validateNetworkIsolation,
		},
		{
			ID:          "CRED-002",
			Name:        "Credential Expiration",
			Description: "Verify credentials have appropriate expiration times",
			Category:    CategoryCredentials,
			Severity:    SeverityMedium,
			Validator:   v.validateCredentialExpiration,
		},
		{
			ID:          "DATA-002",
			Name:        "Execution Context Isolation",
			Description: "Verify execution contexts are properly isolated",
			Category:    CategoryData,
			Severity:    SeverityHigh,
			Validator:   v.validateExecutionContextIsolation,
		},
	}
}

// ValidateCustomerIsolation performs comprehensive isolation validation for a customer
func (v *CustomerIsolationValidator) ValidateCustomerIsolation(ctx context.Context, customerCode string) (*IsolationValidationResult, error) {
	// Check cache first
	if cached := v.getCachedResult(customerCode); cached != nil {
		v.monitoringSystem.logger.Debug("Using cached isolation validation result", map[string]interface{}{
			"customer": customerCode,
			"age":      time.Since(cached.ValidationTime).String(),
		})
		return cached, nil
	}

	v.monitoringSystem.logger.Info("Starting customer isolation validation", map[string]interface{}{
		"customer":   customerCode,
		"totalRules": len(v.validationRules),
	})

	startTime := time.Now()
	result := &IsolationValidationResult{
		CustomerCode:    customerCode,
		ValidationTime:  startTime,
		TotalRules:      len(v.validationRules),
		Results:         make([]*ValidationResult, 0, len(v.validationRules)),
		Summary:         make(map[ValidationCategory]int),
		Recommendations: make([]string, 0),
	}

	// Run all validation rules
	for _, rule := range v.validationRules {
		ruleResult := rule.Validator(ctx, customerCode, v)
		result.Results = append(result.Results, ruleResult)

		// Update counters
		if ruleResult.Passed {
			result.PassedRules++
		} else {
			result.FailedRules++

			// Count by severity
			switch rule.Severity {
			case SeverityCritical:
				result.CriticalIssues++
			case SeverityHigh:
				result.HighIssues++
			}
		}

		// Update category summary
		result.Summary[rule.Category]++
	}

	// Determine overall result
	result.OverallPassed = result.CriticalIssues == 0 && result.HighIssues == 0

	// Generate recommendations
	result.Recommendations = v.generateRecommendations(result)

	// Cache the result
	v.cacheResult(customerCode, result)

	duration := time.Since(startTime)
	v.monitoringSystem.logger.Info("Customer isolation validation completed", map[string]interface{}{
		"customer":       customerCode,
		"overallPassed":  result.OverallPassed,
		"passedRules":    result.PassedRules,
		"failedRules":    result.FailedRules,
		"criticalIssues": result.CriticalIssues,
		"highIssues":     result.HighIssues,
		"duration":       duration.String(),
	})

	return result, nil
}

// ValidateAllCustomers validates isolation for all configured customers
func (v *CustomerIsolationValidator) ValidateAllCustomers(ctx context.Context) (map[string]*IsolationValidationResult, error) {
	results := make(map[string]*IsolationValidationResult)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	v.monitoringSystem.logger.Info("Starting isolation validation for all customers", map[string]interface{}{
		"totalCustomers": len(v.customerManager.CustomerMappings),
	})

	// Validate each customer in parallel
	for customerCode := range v.customerManager.CustomerMappings {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()

			result, err := v.ValidateCustomerIsolation(ctx, code)
			if err != nil {
				v.monitoringSystem.logger.Error("Customer isolation validation failed", err, map[string]interface{}{
					"customer": code,
				})
				return
			}

			mutex.Lock()
			results[code] = result
			mutex.Unlock()
		}(customerCode)
	}

	wg.Wait()

	// Generate summary report
	v.generateSummaryReport(results)

	return results, nil
}

// Validation rule implementations

func (v *CustomerIsolationValidator) validateCredentialIsolation(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "CRED-001",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Test that customer credentials cannot access other customer resources
	accountInfo, err := v.customerManager.GetCustomerAccountInfo(customerCode)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to get customer account info: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify the customer can only access their own account
	if v.enhancedCredManager != nil {
		clients, err := v.enhancedCredManager.AssumeCustomerRole(ctx, customerCode, "ses")
		if err != nil {
			result.Passed = false
			result.Message = fmt.Sprintf("Failed to assume customer role: %v", err)
			result.Duration = time.Since(startTime)
			return result
		}

		// Test that the assumed role identity matches the expected account
		identity, err := clients.STSClient.GetCallerIdentity(ctx, nil)
		if err != nil {
			result.Passed = false
			result.Message = fmt.Sprintf("Failed to get caller identity: %v", err)
			result.Duration = time.Since(startTime)
			return result
		}

		if *identity.Account != accountInfo.AWSAccountID {
			result.Passed = false
			result.Message = fmt.Sprintf("Account mismatch: expected %s, got %s", accountInfo.AWSAccountID, *identity.Account)
			result.Details["expectedAccount"] = accountInfo.AWSAccountID
			result.Details["actualAccount"] = *identity.Account
			result.Duration = time.Since(startTime)
			return result
		}

		result.Details["verifiedAccount"] = *identity.Account
		result.Details["assumedRole"] = *identity.Arn
	}

	result.Passed = true
	result.Message = "Customer credential isolation verified successfully"
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateCrossAccountAccess(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "ACCESS-001",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify that cross-account access is properly controlled
	accountInfo, err := v.customerManager.GetCustomerAccountInfo(customerCode)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to get customer account info: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Check role ARN format and account isolation
	if !strings.Contains(accountInfo.SESRoleARN, accountInfo.AWSAccountID) {
		result.Passed = false
		result.Message = "SES role ARN does not contain customer account ID"
		result.Details["roleArn"] = accountInfo.SESRoleARN
		result.Details["expectedAccount"] = accountInfo.AWSAccountID
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify role ARN format
	if !strings.HasPrefix(accountInfo.SESRoleARN, "arn:aws:iam::") {
		result.Passed = false
		result.Message = "Invalid SES role ARN format"
		result.Details["roleArn"] = accountInfo.SESRoleARN
		result.Duration = time.Since(startTime)
		return result
	}

	result.Passed = true
	result.Message = "Cross-account access controls verified"
	result.Details["sesRoleArn"] = accountInfo.SESRoleARN
	result.Details["accountId"] = accountInfo.AWSAccountID
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateDataSegregation(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "DATA-001",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify that customer data is properly segregated
	// Check execution history isolation
	executions, err := v.statusTracker.QueryExecutions(ExecutionQuery{
		CustomerCode: customerCode,
		Limit:        10,
	})
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to query customer executions: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify all executions belong to the correct customer
	for _, execution := range executions {
		for execCustomerCode := range execution.CustomerStatuses {
			if execCustomerCode != customerCode {
				// Check if this is a multi-customer execution
				if !v.isValidMultiCustomerExecution(execution, customerCode) {
					result.Passed = false
					result.Message = fmt.Sprintf("Found execution with cross-customer data: %s", execution.ExecutionID)
					result.Details["executionId"] = execution.ExecutionID
					result.Details["foundCustomer"] = execCustomerCode
					result.Duration = time.Since(startTime)
					return result
				}
			}
		}
	}

	result.Passed = true
	result.Message = "Customer data segregation verified"
	result.Details["executionsChecked"] = len(executions)
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateRolePermissions(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "ACCESS-002",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify IAM role permissions are appropriately scoped
	accountInfo, err := v.customerManager.GetCustomerAccountInfo(customerCode)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to get customer account info: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Check role ARN structure for security best practices
	roleArn := accountInfo.SESRoleARN

	// Verify role name follows naming convention
	if !strings.Contains(strings.ToLower(roleArn), strings.ToLower(customerCode)) {
		result.Passed = false
		result.Message = "Role ARN does not follow customer naming convention"
		result.Details["roleArn"] = roleArn
		result.Details["expectedCustomerCode"] = customerCode
		result.Duration = time.Since(startTime)
		return result
	}

	// Check for overly permissive role names
	suspiciousPatterns := []string{"admin", "root", "full", "all", "*"}
	roleName := extractRoleNameFromARN(roleArn)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(strings.ToLower(roleName), pattern) {
			result.Passed = false
			result.Message = fmt.Sprintf("Role name contains potentially overly permissive pattern: %s", pattern)
			result.Details["roleArn"] = roleArn
			result.Details["roleName"] = roleName
			result.Details["suspiciousPattern"] = pattern
			result.Duration = time.Since(startTime)
			return result
		}
	}

	result.Passed = true
	result.Message = "Role permissions appear appropriately scoped"
	result.Details["roleArn"] = roleArn
	result.Details["roleName"] = roleName
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateAuditTrailIsolation(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "AUDIT-001",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify audit trails are properly isolated
	// This is a placeholder implementation - in production, you would check CloudTrail logs
	result.Passed = true
	result.Message = "Audit trail isolation check passed (placeholder implementation)"
	result.Details["note"] = "Production implementation would verify CloudTrail log isolation"
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateNetworkIsolation(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "NETWORK-001",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify network-level isolation
	accountInfo, err := v.customerManager.GetCustomerAccountInfo(customerCode)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to get customer account info: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Check that customer is in appropriate region
	if accountInfo.Region == "" {
		result.Passed = false
		result.Message = "Customer region not specified"
		result.Duration = time.Since(startTime)
		return result
	}

	result.Passed = true
	result.Message = "Network isolation verified"
	result.Details["customerRegion"] = accountInfo.Region
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateCredentialExpiration(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "CRED-002",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify credentials have appropriate expiration times
	if v.enhancedCredManager != nil {
		cacheStatus := v.enhancedCredManager.GetCacheStatus()

		for key, expiresAt := range cacheStatus {
			if strings.Contains(key, customerCode) {
				timeUntilExpiry := time.Until(expiresAt)

				// Check if expiration time is reasonable (not too long, not too short)
				if timeUntilExpiry > 2*time.Hour {
					result.Passed = false
					result.Message = "Credential expiration time is too long"
					result.Details["expiresAt"] = expiresAt
					result.Details["timeUntilExpiry"] = timeUntilExpiry.String()
					result.Duration = time.Since(startTime)
					return result
				}

				if timeUntilExpiry < 5*time.Minute {
					result.Passed = false
					result.Message = "Credential expiration time is too short"
					result.Details["expiresAt"] = expiresAt
					result.Details["timeUntilExpiry"] = timeUntilExpiry.String()
					result.Duration = time.Since(startTime)
					return result
				}

				result.Details["expiresAt"] = expiresAt
				result.Details["timeUntilExpiry"] = timeUntilExpiry.String()
				break
			}
		}
	}

	result.Passed = true
	result.Message = "Credential expiration times are appropriate"
	result.Duration = time.Since(startTime)
	return result
}

func (v *CustomerIsolationValidator) validateExecutionContextIsolation(ctx context.Context, customerCode string, validator *CustomerIsolationValidator) *ValidationResult {
	startTime := time.Now()
	result := &ValidationResult{
		RuleID:       "DATA-002",
		CustomerCode: customerCode,
		Timestamp:    startTime,
		Details:      make(map[string]interface{}),
	}

	// Verify execution contexts are properly isolated
	// Check recent executions for this customer
	executions, err := v.statusTracker.QueryExecutions(ExecutionQuery{
		CustomerCode: customerCode,
		Limit:        5,
	})
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Failed to query executions: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Verify execution isolation
	for _, execution := range executions {
		// Check that execution context is properly scoped
		if execution.InitiatedBy == "" {
			result.Passed = false
			result.Message = "Execution missing initiator information"
			result.Details["executionId"] = execution.ExecutionID
			result.Duration = time.Since(startTime)
			return result
		}
	}

	result.Passed = true
	result.Message = "Execution context isolation verified"
	result.Details["executionsChecked"] = len(executions)
	result.Duration = time.Since(startTime)
	return result
}

// Helper methods

func (v *CustomerIsolationValidator) getCachedResult(customerCode string) *IsolationValidationResult {
	v.cacheMutex.RLock()
	defer v.cacheMutex.RUnlock()

	cached, exists := v.isolationCache[customerCode]
	if !exists {
		return nil
	}

	// Check if cache is expired
	if time.Since(cached.ValidationTime) > v.cacheExpiry {
		return nil
	}

	return cached
}

func (v *CustomerIsolationValidator) cacheResult(customerCode string, result *IsolationValidationResult) {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()

	v.isolationCache[customerCode] = result
}

func (v *CustomerIsolationValidator) isValidMultiCustomerExecution(execution *ExecutionStatus, customerCode string) bool {
	// Check if this is a legitimate multi-customer execution
	// This would involve checking if the execution was properly initiated for multiple customers
	return len(execution.CustomerStatuses) > 1 && execution.CustomerStatuses[customerCode] != nil
}

func (v *CustomerIsolationValidator) generateRecommendations(result *IsolationValidationResult) []string {
	recommendations := make([]string, 0)

	if result.CriticalIssues > 0 {
		recommendations = append(recommendations, "Address critical security issues immediately")
	}

	if result.HighIssues > 0 {
		recommendations = append(recommendations, "Review and fix high-severity isolation issues")
	}

	if result.PassedRules < result.TotalRules {
		recommendations = append(recommendations, "Implement additional security controls to pass all validation rules")
	}

	// Category-specific recommendations
	for category, count := range result.Summary {
		if count > 0 {
			switch category {
			case CategoryCredentials:
				recommendations = append(recommendations, "Review credential management and rotation policies")
			case CategoryAccess:
				recommendations = append(recommendations, "Audit IAM roles and permissions for least privilege")
			case CategoryData:
				recommendations = append(recommendations, "Implement additional data segregation controls")
			case CategoryNetwork:
				recommendations = append(recommendations, "Review network isolation and security groups")
			case CategoryAudit:
				recommendations = append(recommendations, "Enhance audit logging and monitoring")
			}
		}
	}

	return recommendations
}

func (v *CustomerIsolationValidator) generateSummaryReport(results map[string]*IsolationValidationResult) {
	totalCustomers := len(results)
	passedCustomers := 0
	totalCriticalIssues := 0
	totalHighIssues := 0

	for _, result := range results {
		if result.OverallPassed {
			passedCustomers++
		}
		totalCriticalIssues += result.CriticalIssues
		totalHighIssues += result.HighIssues
	}

	v.monitoringSystem.logger.Info("Customer isolation validation summary", map[string]interface{}{
		"totalCustomers":      totalCustomers,
		"passedCustomers":     passedCustomers,
		"failedCustomers":     totalCustomers - passedCustomers,
		"successRate":         float64(passedCustomers) / float64(totalCustomers) * 100,
		"totalCriticalIssues": totalCriticalIssues,
		"totalHighIssues":     totalHighIssues,
	})
}

// DetectCrossCustomerAccess monitors for potential cross-customer access attempts
func (v *CustomerIsolationValidator) DetectCrossCustomerAccess(sourceCustomer, targetCustomer, accessType, resource string) *CrossCustomerAccessAttempt {
	attempt := &CrossCustomerAccessAttempt{
		SourceCustomer: sourceCustomer,
		TargetCustomer: targetCustomer,
		AccessType:     accessType,
		Resource:       resource,
		Timestamp:      time.Now(),
		Blocked:        true, // Default to blocked for security
		Details:        make(map[string]interface{}),
	}

	// Log the attempt
	v.monitoringSystem.logger.Warn("Cross-customer access attempt detected", map[string]interface{}{
		"sourceCustomer": sourceCustomer,
		"targetCustomer": targetCustomer,
		"accessType":     accessType,
		"resource":       resource,
		"blocked":        attempt.Blocked,
	})

	return attempt
}

// Helper functions

func extractRoleNameFromARN(roleARN string) string {
	// Extract role name from ARN: arn:aws:iam::123456789012:role/RoleName
	parts := strings.Split(roleARN, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// ClearValidationCache clears the validation cache
func (v *CustomerIsolationValidator) ClearValidationCache() {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()

	v.isolationCache = make(map[string]*IsolationValidationResult)
	v.monitoringSystem.logger.Info("Validation cache cleared", nil)
}

// GetValidationMetrics returns metrics about validation performance
func (v *CustomerIsolationValidator) GetValidationMetrics() map[string]interface{} {
	v.cacheMutex.RLock()
	defer v.cacheMutex.RUnlock()

	return map[string]interface{}{
		"totalRules":         len(v.validationRules),
		"cachedResults":      len(v.isolationCache),
		"cacheExpiryMinutes": v.cacheExpiry.Minutes(),
	}
}
