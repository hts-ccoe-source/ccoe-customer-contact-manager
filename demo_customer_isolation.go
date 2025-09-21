package main

import (
	"context"
	"fmt"
	"time"
)

// Demo application showcasing Task 13: Customer isolation validation

func main() {
	fmt.Println("=== Task 13: Customer Isolation Validation Demo ===")

	// Demo 1: Basic isolation validation
	fmt.Println("\n🔒 Demo 1: Basic Customer Isolation Validation")
	demoBasicIsolationValidation()

	// Demo 2: Comprehensive validation rules
	fmt.Println("\n📋 Demo 2: Comprehensive Validation Rules")
	demoComprehensiveValidationRules()

	// Demo 3: Cross-customer access detection
	fmt.Println("\n🚨 Demo 3: Cross-Customer Access Detection")
	demoCrossCustomerAccessDetection()

	// Demo 4: Bulk validation and reporting
	fmt.Println("\n📊 Demo 4: Bulk Validation and Reporting")
	demoBulkValidationAndReporting()

	// Demo 5: Security monitoring and alerts
	fmt.Println("\n🛡️  Demo 5: Security Monitoring and Alerts")
	demoSecurityMonitoringAndAlerts()

	fmt.Println("\n=== Customer Isolation Validation Demo Complete ===")
}

func demoBasicIsolationValidation() {
	fmt.Printf("🔒 Basic Customer Isolation Validation Demo\n")

	// Create test environment
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"hts": {
			CustomerCode: "hts",
			CustomerName: "HTS Production",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/HTSSESRole",
			SQSRoleARN:   "arn:aws:iam::123456789012:role/HTSSQSRole",
			Environment:  "production",
		},
		"cds": {
			CustomerCode: "cds",
			CustomerName: "CDS Global",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/CDSSESRole",
			Environment:  "production",
		},
		"motor": {
			CustomerCode: "motor",
			CustomerName: "Motor Staging",
			AWSAccountID: "345678901234",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::345678901234:role/MotorSESRole",
			Environment:  "staging",
		},
		"insecure": {
			CustomerCode: "insecure",
			CustomerName: "Insecure Customer",
			AWSAccountID: "999999999999",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::999999999999:role/AdminRole", // Suspicious role name
			Environment:  "test",
		},
	}

	enhancedManager, err := NewEnhancedCredentialManager(customerManager)
	if err != nil {
		fmt.Printf("   ⚠️  Enhanced credential manager creation failed: %v\n", err)
		enhancedManager = nil // Continue with basic validation
	}

	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "EmailDistribution",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	// Create isolation validator
	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	fmt.Printf("   ✅ Customer isolation validator created\n")
	fmt.Printf("   📋 Validation rules loaded: %d\n", len(validator.validationRules))

	// Display validation rules
	fmt.Printf("\n   📝 Available Validation Rules:\n")
	for _, rule := range validator.validationRules {
		fmt.Printf("      %s: %s (%s - %s)\n",
			rule.ID, rule.Name, rule.Category, rule.Severity)
	}

	ctx := context.Background()

	// Validate individual customers
	fmt.Printf("\n   🔍 Individual Customer Validation:\n")
	testCustomers := []string{"hts", "cds", "motor", "insecure"}

	for _, customerCode := range testCustomers {
		fmt.Printf("      🏢 Validating customer: %s\n", customerCode)

		result, err := validator.ValidateCustomerIsolation(ctx, customerCode)
		if err != nil {
			fmt.Printf("         ❌ Validation failed: %v\n", err)
			continue
		}

		fmt.Printf("         📊 Results: %d/%d rules passed\n", result.PassedRules, result.TotalRules)
		fmt.Printf("         🚨 Critical issues: %d\n", result.CriticalIssues)
		fmt.Printf("         ⚠️  High issues: %d\n", result.HighIssues)
		fmt.Printf("         ✅ Overall passed: %t\n", result.OverallPassed)

		// Show failed validations
		if result.FailedRules > 0 {
			fmt.Printf("         ❌ Failed validations:\n")
			for _, validationResult := range result.Results {
				if !validationResult.Passed {
					fmt.Printf("            %s: %s\n", validationResult.RuleID, validationResult.Message)
				}
			}
		}

		// Show recommendations
		if len(result.Recommendations) > 0 {
			fmt.Printf("         💡 Recommendations:\n")
			for _, recommendation := range result.Recommendations {
				fmt.Printf("            - %s\n", recommendation)
			}
		}

		fmt.Printf("\n")
	}
}

func demoComprehensiveValidationRules() {
	fmt.Printf("📋 Comprehensive Validation Rules Demo\n")

	// Create test environment
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"secure": {
			CustomerCode: "secure",
			CustomerName: "Secure Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/SecureSESRole",
			Environment:  "production",
		},
		"vulnerable": {
			CustomerCode: "vulnerable",
			CustomerName: "Vulnerable Customer",
			AWSAccountID: "234567890123",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::234567890123:role/FullAccessRole", // Suspicious
			Environment:  "production",
		},
	}

	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "EmailDistribution",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	ctx := context.Background()

	// Test each validation rule individually
	fmt.Printf("   🔍 Testing Individual Validation Rules:\n")

	validationTests := []struct {
		ruleID      string
		customer    string
		description string
	}{
		{"CRED-001", "secure", "Customer Credential Isolation"},
		{"ACCESS-001", "secure", "Cross-Account Access Control"},
		{"DATA-001", "secure", "Customer Data Segregation"},
		{"ACCESS-002", "vulnerable", "Role Permission Boundaries (should fail)"},
		{"AUDIT-001", "secure", "Audit Trail Isolation"},
		{"NETWORK-001", "secure", "Network Isolation"},
		{"CRED-002", "secure", "Credential Expiration"},
		{"DATA-002", "secure", "Execution Context Isolation"},
	}

	for _, test := range validationTests {
		fmt.Printf("      🧪 Testing %s: %s\n", test.ruleID, test.description)

		// Find the validation rule
		var rule *ValidationRule
		for _, r := range validator.validationRules {
			if r.ID == test.ruleID {
				rule = &r
				break
			}
		}

		if rule == nil {
			fmt.Printf("         ❌ Rule not found: %s\n", test.ruleID)
			continue
		}

		// Execute the validation rule
		startTime := time.Now()
		result := rule.Validator(ctx, test.customer, validator)
		duration := time.Since(startTime)

		fmt.Printf("         📊 Result: %s\n", getResultStatus(result.Passed))
		fmt.Printf("         💬 Message: %s\n", result.Message)
		fmt.Printf("         ⏱️  Duration: %v\n", duration)

		// Show details if available
		if len(result.Details) > 0 {
			fmt.Printf("         📋 Details:\n")
			for key, value := range result.Details {
				fmt.Printf("            %s: %v\n", key, value)
			}
		}

		fmt.Printf("\n")
	}

	// Test validation categories
	fmt.Printf("   📊 Validation by Category:\n")
	categories := []ValidationCategory{
		CategoryCredentials,
		CategoryAccess,
		CategoryData,
		CategoryNetwork,
		CategoryAudit,
	}

	for _, category := range categories {
		fmt.Printf("      📂 Category: %s\n", category)

		categoryRules := 0
		for _, rule := range validator.validationRules {
			if rule.Category == category {
				categoryRules++
				fmt.Printf("         - %s: %s (%s)\n", rule.ID, rule.Name, rule.Severity)
			}
		}

		fmt.Printf("         Total rules: %d\n\n", categoryRules)
	}
}

func demoCrossCustomerAccessDetection() {
	fmt.Printf("🚨 Cross-Customer Access Detection Demo\n")

	// Create test environment
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"customer-a": {
			CustomerCode: "customer-a",
			CustomerName: "Customer A",
			AWSAccountID: "111111111111",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::111111111111:role/CustomerASESRole",
			Environment:  "production",
		},
		"customer-b": {
			CustomerCode: "customer-b",
			CustomerName: "Customer B",
			AWSAccountID: "222222222222",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::222222222222:role/CustomerBSESRole",
			Environment:  "production",
		},
	}

	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "EmailDistribution",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	// Simulate cross-customer access attempts
	fmt.Printf("   🚨 Simulating Cross-Customer Access Attempts:\n")

	accessAttempts := []struct {
		source      string
		target      string
		accessType  string
		resource    string
		description string
	}{
		{
			source:      "customer-a",
			target:      "customer-b",
			accessType:  "ses",
			resource:    "email-list",
			description: "Customer A trying to access Customer B's email list",
		},
		{
			source:      "customer-b",
			target:      "customer-a",
			accessType:  "sqs",
			resource:    "message-queue",
			description: "Customer B trying to access Customer A's message queue",
		},
		{
			source:      "customer-a",
			target:      "customer-b",
			accessType:  "s3",
			resource:    "metadata-bucket",
			description: "Customer A trying to access Customer B's metadata bucket",
		},
		{
			source:      "customer-b",
			target:      "customer-a",
			accessType:  "execution",
			resource:    "status-data",
			description: "Customer B trying to access Customer A's execution status",
		},
	}

	for i, attempt := range accessAttempts {
		fmt.Printf("      %d. %s\n", i+1, attempt.description)

		// Detect and log the access attempt
		detection := validator.DetectCrossCustomerAccess(
			attempt.source,
			attempt.target,
			attempt.accessType,
			attempt.resource,
		)

		fmt.Printf("         🔍 Detection Result:\n")
		fmt.Printf("            Source: %s\n", detection.SourceCustomer)
		fmt.Printf("            Target: %s\n", detection.TargetCustomer)
		fmt.Printf("            Access Type: %s\n", detection.AccessType)
		fmt.Printf("            Resource: %s\n", detection.Resource)
		fmt.Printf("            Blocked: %s\n", getBooleanStatus(detection.Blocked))
		fmt.Printf("            Timestamp: %s\n", detection.Timestamp.Format("15:04:05"))

		// Simulate security response
		if detection.Blocked {
			fmt.Printf("         🛡️  Security Response:\n")
			fmt.Printf("            - Access attempt blocked\n")
			fmt.Printf("            - Security alert generated\n")
			fmt.Printf("            - Incident logged for review\n")
		}

		fmt.Printf("\n")
	}

	// Demonstrate isolation validation after access attempts
	fmt.Printf("   🔒 Post-Incident Isolation Validation:\n")
	ctx := context.Background()

	for _, customerCode := range []string{"customer-a", "customer-b"} {
		fmt.Printf("      🏢 Re-validating customer: %s\n", customerCode)

		result, err := validator.ValidateCustomerIsolation(ctx, customerCode)
		if err != nil {
			fmt.Printf("         ❌ Validation failed: %v\n", err)
			continue
		}

		fmt.Printf("         📊 Isolation Status: %s\n", getResultStatus(result.OverallPassed))
		fmt.Printf("         🚨 Critical Issues: %d\n", result.CriticalIssues)
		fmt.Printf("         ⚠️  High Issues: %d\n", result.HighIssues)

		if !result.OverallPassed {
			fmt.Printf("         🚨 SECURITY ALERT: Customer isolation compromised!\n")
		}
	}
}

func demoBulkValidationAndReporting() {
	fmt.Printf("📊 Bulk Validation and Reporting Demo\n")

	// Create test environment with multiple customers
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"enterprise-a": {
			CustomerCode: "enterprise-a",
			CustomerName: "Enterprise Customer A",
			AWSAccountID: "111111111111",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::111111111111:role/EnterpriseSESRole",
			Environment:  "production",
		},
		"enterprise-b": {
			CustomerCode: "enterprise-b",
			CustomerName: "Enterprise Customer B",
			AWSAccountID: "222222222222",
			Region:       "us-west-2",
			SESRoleARN:   "arn:aws:iam::222222222222:role/EnterpriseSESRole",
			Environment:  "production",
		},
		"startup-c": {
			CustomerCode: "startup-c",
			CustomerName: "Startup Customer C",
			AWSAccountID: "333333333333",
			Region:       "eu-west-1",
			SESRoleARN:   "arn:aws:iam::333333333333:role/StartupSESRole",
			Environment:  "staging",
		},
		"test-d": {
			CustomerCode: "test-d",
			CustomerName: "Test Customer D",
			AWSAccountID: "444444444444",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::444444444444:role/AdminRole", // Suspicious
			Environment:  "test",
		},
		"secure-e": {
			CustomerCode: "secure-e",
			CustomerName: "Secure Customer E",
			AWSAccountID: "555555555555",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::555555555555:role/SecureEmailRole",
			Environment:  "production",
		},
	}

	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "EmailDistribution",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	fmt.Printf("   🏢 Validating %d customers in parallel...\n", len(customerManager.CustomerMappings))

	ctx := context.Background()
	startTime := time.Now()

	// Perform bulk validation
	results, err := validator.ValidateAllCustomers(ctx)
	if err != nil {
		fmt.Printf("   ❌ Bulk validation failed: %v\n", err)
		return
	}

	duration := time.Since(startTime)
	fmt.Printf("   ✅ Bulk validation completed in %v\n", duration)

	// Generate comprehensive report
	fmt.Printf("\n   📊 Validation Summary Report:\n")

	totalCustomers := len(results)
	passedCustomers := 0
	totalCriticalIssues := 0
	totalHighIssues := 0
	totalMediumIssues := 0

	categoryStats := make(map[ValidationCategory]int)
	severityStats := make(map[ValidationSeverity]int)

	for customerCode, result := range results {
		if result.OverallPassed {
			passedCustomers++
		}

		totalCriticalIssues += result.CriticalIssues
		totalHighIssues += result.HighIssues

		// Count by category
		for category, count := range result.Summary {
			categoryStats[category] += count
		}

		fmt.Printf("      🏢 %s: %s (%d/%d rules passed)\n",
			customerCode, getResultStatus(result.OverallPassed),
			result.PassedRules, result.TotalRules)

		if result.CriticalIssues > 0 {
			fmt.Printf("         🚨 Critical Issues: %d\n", result.CriticalIssues)
		}
		if result.HighIssues > 0 {
			fmt.Printf("         ⚠️  High Issues: %d\n", result.HighIssues)
		}
	}

	// Overall statistics
	fmt.Printf("\n   📈 Overall Statistics:\n")
	fmt.Printf("      Total Customers: %d\n", totalCustomers)
	fmt.Printf("      Passed: %d (%.1f%%)\n", passedCustomers, float64(passedCustomers)/float64(totalCustomers)*100)
	fmt.Printf("      Failed: %d (%.1f%%)\n", totalCustomers-passedCustomers, float64(totalCustomers-passedCustomers)/float64(totalCustomers)*100)
	fmt.Printf("      Total Critical Issues: %d\n", totalCriticalIssues)
	fmt.Printf("      Total High Issues: %d\n", totalHighIssues)

	// Category breakdown
	fmt.Printf("\n   📂 Issues by Category:\n")
	for category, count := range categoryStats {
		fmt.Printf("      %s: %d issues\n", category, count)
	}

	// Risk assessment
	fmt.Printf("\n   🎯 Risk Assessment:\n")
	if totalCriticalIssues > 0 {
		fmt.Printf("      🚨 HIGH RISK: %d critical security issues require immediate attention\n", totalCriticalIssues)
	} else if totalHighIssues > 0 {
		fmt.Printf("      ⚠️  MEDIUM RISK: %d high-severity issues should be addressed soon\n", totalHighIssues)
	} else {
		fmt.Printf("      ✅ LOW RISK: No critical or high-severity issues detected\n")
	}

	// Recommendations
	fmt.Printf("\n   💡 Top Recommendations:\n")
	if totalCriticalIssues > 0 {
		fmt.Printf("      1. Address all critical security issues immediately\n")
		fmt.Printf("      2. Implement emergency security controls\n")
		fmt.Printf("      3. Conduct security incident review\n")
	} else {
		fmt.Printf("      1. Continue regular security validation\n")
		fmt.Printf("      2. Monitor for new security threats\n")
		fmt.Printf("      3. Review and update security policies\n")
	}
}

func demoSecurityMonitoringAndAlerts() {
	fmt.Printf("🛡️  Security Monitoring and Alerts Demo\n")

	// Create test environment
	customerManager := NewCustomerCredentialManager("us-east-1")
	customerManager.CustomerMappings = map[string]CustomerAccountInfo{
		"monitored": {
			CustomerCode: "monitored",
			CustomerName: "Monitored Customer",
			AWSAccountID: "123456789012",
			Region:       "us-east-1",
			SESRoleARN:   "arn:aws:iam::123456789012:role/MonitoredSESRole",
			Environment:  "production",
		},
	}

	enhancedManager, _ := NewEnhancedCredentialManager(customerManager)
	statusTracker := NewExecutionStatusTracker(customerManager)
	errorHandler := NewErrorHandler(customerManager, statusTracker)

	monitoringConfig := MonitoringConfiguration{
		EnableCloudWatch: false,
		EnableXRay:       false,
		MetricsNamespace: "EmailDistribution",
	}
	monitoringSystem := NewMonitoringSystem(monitoringConfig, customerManager, errorHandler, statusTracker)

	validator := NewCustomerIsolationValidator(
		customerManager,
		enhancedManager,
		statusTracker,
		monitoringSystem,
	)

	fmt.Printf("   🔍 Security Monitoring Configuration:\n")
	fmt.Printf("      Validation Cache Expiry: %v\n", validator.cacheExpiry)
	fmt.Printf("      Total Validation Rules: %d\n", len(validator.validationRules))

	// Demonstrate continuous monitoring
	fmt.Printf("\n   📊 Continuous Security Monitoring:\n")

	ctx := context.Background()

	// Simulate monitoring cycles
	for cycle := 1; cycle <= 3; cycle++ {
		fmt.Printf("      🔄 Monitoring Cycle %d:\n", cycle)

		// Validate customer isolation
		result, err := validator.ValidateCustomerIsolation(ctx, "monitored")
		if err != nil {
			fmt.Printf("         ❌ Validation failed: %v\n", err)
			continue
		}

		fmt.Printf("         📊 Validation Result: %s\n", getResultStatus(result.OverallPassed))
		fmt.Printf("         ⏱️  Validation Time: %s\n", result.ValidationTime.Format("15:04:05"))

		// Check for security alerts
		if result.CriticalIssues > 0 {
			fmt.Printf("         🚨 SECURITY ALERT: Critical issues detected!\n")
			fmt.Printf("         📧 Alert sent to security team\n")
			fmt.Printf("         📱 SMS notification sent to on-call engineer\n")
		} else if result.HighIssues > 0 {
			fmt.Printf("         ⚠️  WARNING: High-severity issues detected\n")
			fmt.Printf("         📧 Warning sent to operations team\n")
		} else {
			fmt.Printf("         ✅ All security checks passed\n")
		}

		// Simulate time between monitoring cycles
		if cycle < 3 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Demonstrate metrics and caching
	fmt.Printf("\n   📈 Validation Metrics:\n")
	metrics := validator.GetValidationMetrics()
	for key, value := range metrics {
		fmt.Printf("      %s: %v\n", key, value)
	}

	// Cache management
	fmt.Printf("\n   💾 Cache Management:\n")
	fmt.Printf("      Cache status before clear:\n")
	if validator.getCachedResult("monitored") != nil {
		fmt.Printf("         ✅ Customer 'monitored' result cached\n")
	} else {
		fmt.Printf("         ❌ No cached result for 'monitored'\n")
	}

	validator.ClearValidationCache()
	fmt.Printf("      Cache cleared\n")

	if validator.getCachedResult("monitored") != nil {
		fmt.Printf("         ❌ Cache clear failed\n")
	} else {
		fmt.Printf("         ✅ Cache successfully cleared\n")
	}

	// Security incident simulation
	fmt.Printf("\n   🚨 Security Incident Simulation:\n")
	fmt.Printf("      Simulating potential security breach...\n")

	// Simulate cross-customer access attempt
	attempt := validator.DetectCrossCustomerAccess(
		"monitored",
		"external-attacker",
		"unauthorized",
		"sensitive-data",
	)

	fmt.Printf("      🔍 Incident Details:\n")
	fmt.Printf("         Source: %s\n", attempt.SourceCustomer)
	fmt.Printf("         Target: %s\n", attempt.TargetCustomer)
	fmt.Printf("         Access Type: %s\n", attempt.AccessType)
	fmt.Printf("         Resource: %s\n", attempt.Resource)
	fmt.Printf("         Blocked: %s\n", getBooleanStatus(attempt.Blocked))
	fmt.Printf("         Timestamp: %s\n", attempt.Timestamp.Format("2006-01-02 15:04:05"))

	fmt.Printf("\n      🛡️  Automated Security Response:\n")
	fmt.Printf("         1. ✅ Access attempt automatically blocked\n")
	fmt.Printf("         2. 🚨 Critical security alert triggered\n")
	fmt.Printf("         3. 📧 Incident response team notified\n")
	fmt.Printf("         4. 📊 Security metrics updated\n")
	fmt.Printf("         5. 🔍 Forensic logging initiated\n")
	fmt.Printf("         6. 🔒 Additional security controls activated\n")
}

// Helper functions

func getResultStatus(passed bool) string {
	if passed {
		return "✅ PASSED"
	}
	return "❌ FAILED"
}

func getBooleanStatus(value bool) string {
	if value {
		return "✅ YES"
	}
	return "❌ NO"
}
