package main

import (
	"fmt"
	"log"
	"testing"
	"time"
)

// Integration tests for multi-customer upload workflow

func TestMultiCustomerUploadWorkflow(t *testing.T) {
	manager := NewMultiCustomerUploadManager("test-integration-bucket")

	// Test complete workflow from form data to successful upload
	formData := map[string]interface{}{
		"customers":               []string{"hts", "cds", "motor"},
		"changeTitle":             "Test Multi-Customer Change",
		"changeReason":            "Integration testing",
		"implementationPlan":      "Deploy test changes",
		"testPlan":                "Validate functionality",
		"expectedCustomerImpact":  "No impact expected",
		"rollbackPlan":            "Revert changes if needed",
		"implementationBeginDate": "2025-09-21",
		"implementationBeginTime": "10:00",
		"implementationEndDate":   "2025-09-21",
		"implementationEndTime":   "17:00",
		"timezone":                "America/New_York",
	}

	// Step 1: Determine affected customers
	customers, err := manager.DetermineAffectedCustomers(formData)
	if err != nil {
		t.Fatalf("Failed to determine customers: %v", err)
	}

	if len(customers) != 3 {
		t.Errorf("Expected 3 customers, got %d", len(customers))
	}

	// Step 2: Create metadata
	metadata := createTestMetadata(formData, customers)

	// Step 3: Create upload queue
	uploadQueue := manager.CreateUploadQueue(customers, metadata)

	// Should have 3 customer uploads + 1 archive = 4 total
	expectedUploads := 4
	if len(uploadQueue) != expectedUploads {
		t.Errorf("Expected %d uploads, got %d", expectedUploads, len(uploadQueue))
	}

	// Step 4: Process uploads
	result := manager.ProcessUploadQueue(uploadQueue)

	// Verify results
	if result.TotalUploads != expectedUploads {
		t.Errorf("Expected %d total uploads, got %d", expectedUploads, result.TotalUploads)
	}

	if result.SuccessfulUploads+result.FailedUploads != result.TotalUploads {
		t.Errorf("Successful + Failed should equal total uploads")
	}

	// Step 5: Validate upload structure
	validateUploadStructure(t, uploadQueue, customers)

	// Step 6: Test progress tracking
	progress := manager.GenerateProgressUpdate(result)
	validateProgressUpdate(t, progress, result)
}

func TestPartialUploadFailureAndRetry(t *testing.T) {
	manager := NewMultiCustomerUploadManager("test-retry-bucket")

	customers := []string{"hts", "cds", "motor", "bat"}
	metadata := createTestMetadata(map[string]interface{}{
		"customers":   customers,
		"changeTitle": "Test Retry Scenario",
	}, customers)

	uploadQueue := manager.CreateUploadQueue(customers, metadata)

	// Process uploads (some may fail due to simulated errors)
	result := manager.ProcessUploadQueue(uploadQueue)

	// If there are failures, test retry mechanism
	if result.FailedUploads > 0 {
		t.Logf("Initial upload had %d failures, testing retry...", result.FailedUploads)

		retryResult := manager.RetryFailedUploads(result)

		// Verify retry only processes failed uploads
		if retryResult.TotalUploads != result.FailedUploads {
			t.Errorf("Retry should only process failed uploads. Expected %d, got %d",
				result.FailedUploads, retryResult.TotalUploads)
		}

		// Test validation after retry
		combinedSuccessful := result.SuccessfulUploads + retryResult.SuccessfulUploads
		combinedFailed := retryResult.FailedUploads

		t.Logf("After retry: %d successful, %d failed", combinedSuccessful, combinedFailed)

		if combinedSuccessful+combinedFailed != result.TotalUploads {
			t.Errorf("Combined results don't match original total")
		}
	} else {
		t.Log("All uploads succeeded on first attempt")
	}
}

func TestUploadValidation(t *testing.T) {
	manager := NewMultiCustomerUploadManager("test-validation-bucket")

	tests := []struct {
		name           string
		customers      []string
		expectArchive  bool
		expectCustomer int
	}{
		{
			name:           "Single customer",
			customers:      []string{"hts"},
			expectArchive:  true,
			expectCustomer: 1,
		},
		{
			name:           "Multiple customers",
			customers:      []string{"hts", "cds", "motor", "bat", "icx"},
			expectArchive:  true,
			expectCustomer: 5,
		},
		{
			name:           "All customers",
			customers:      []string{"hts", "cds", "fdbus", "hmiit", "hmies", "htvdigital", "htv", "icx", "motor", "bat"},
			expectArchive:  true,
			expectCustomer: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := createTestMetadata(map[string]interface{}{
				"customers":   tt.customers,
				"changeTitle": fmt.Sprintf("Test %s", tt.name),
			}, tt.customers)

			uploadQueue := manager.CreateUploadQueue(tt.customers, metadata)

			customerUploads := 0
			archiveUploads := 0

			for _, upload := range uploadQueue {
				switch upload.Type {
				case "customer":
					customerUploads++
					// Validate customer upload structure
					if upload.Customer == "" {
						t.Errorf("Customer upload missing customer code")
					}
					expectedPrefix := fmt.Sprintf("customers/%s/", upload.Customer)
					if !hasPrefix(upload.Key, expectedPrefix) {
						t.Errorf("Customer upload key %s missing expected prefix %s", upload.Key, expectedPrefix)
					}
				case "archive":
					archiveUploads++
					// Validate archive upload structure
					if upload.Customer != "" {
						t.Errorf("Archive upload should not have customer code")
					}
					if !hasPrefix(upload.Key, "archive/") {
						t.Errorf("Archive upload key %s missing 'archive/' prefix", upload.Key)
					}
				default:
					t.Errorf("Unknown upload type: %s", upload.Type)
				}
			}

			if customerUploads != tt.expectCustomer {
				t.Errorf("Expected %d customer uploads, got %d", tt.expectCustomer, customerUploads)
			}

			expectedArchive := 0
			if tt.expectArchive {
				expectedArchive = 1
			}
			if archiveUploads != expectedArchive {
				t.Errorf("Expected %d archive uploads, got %d", expectedArchive, archiveUploads)
			}
		})
	}
}

func TestErrorHandlingScenarios(t *testing.T) {
	manager := NewMultiCustomerUploadManager("test-error-bucket")

	// Test invalid form data scenarios
	errorTests := []struct {
		name        string
		formData    map[string]interface{}
		expectError bool
	}{
		{
			name: "No customers",
			formData: map[string]interface{}{
				"customers":   []string{},
				"changeTitle": "Test Change",
			},
			expectError: true,
		},
		{
			name: "Invalid customer code",
			formData: map[string]interface{}{
				"customers":   []string{"invalid-customer"},
				"changeTitle": "Test Change",
			},
			expectError: true,
		},
		{
			name: "Missing customers field",
			formData: map[string]interface{}{
				"changeTitle": "Test Change",
			},
			expectError: true,
		},
		{
			name: "Valid customers",
			formData: map[string]interface{}{
				"customers":   []string{"hts", "cds"},
				"changeTitle": "Test Change",
			},
			expectError: false,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.DetermineAffectedCustomers(tt.formData)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestUploadProgressTracking(t *testing.T) {
	manager := NewMultiCustomerUploadManager("test-progress-bucket")

	customers := []string{"hts", "cds", "motor"}
	metadata := createTestMetadata(map[string]interface{}{
		"customers":   customers,
		"changeTitle": "Test Progress Tracking",
	}, customers)

	uploadQueue := manager.CreateUploadQueue(customers, metadata)

	// Test progress at different stages
	initialResult := &UploadResult{
		TotalUploads:      len(uploadQueue),
		SuccessfulUploads: 0,
		FailedUploads:     0,
		Duration:          0,
	}

	progress := manager.GenerateProgressUpdate(initialResult)
	if progress["percentage"] != 0.0 {
		t.Errorf("Initial progress should be 0%%, got %v", progress["percentage"])
	}

	// Simulate partial completion
	partialResult := &UploadResult{
		TotalUploads:      len(uploadQueue),
		SuccessfulUploads: 2,
		FailedUploads:     0,
		Duration:          1 * time.Second,
	}

	progress = manager.GenerateProgressUpdate(partialResult)
	expectedPercentage := 50.0 // 2 out of 4 uploads
	if progress["percentage"] != expectedPercentage {
		t.Errorf("Partial progress should be %v%%, got %v", expectedPercentage, progress["percentage"])
	}

	// Simulate completion
	completeResult := &UploadResult{
		TotalUploads:      len(uploadQueue),
		SuccessfulUploads: len(uploadQueue),
		FailedUploads:     0,
		Duration:          2 * time.Second,
	}

	progress = manager.GenerateProgressUpdate(completeResult)
	if progress["percentage"] != 100.0 {
		t.Errorf("Complete progress should be 100%%, got %v", progress["percentage"])
	}
}

func TestS3LifecyclePolicyConfiguration(t *testing.T) {
	// Test that upload keys follow the expected pattern for lifecycle policies
	manager := NewMultiCustomerUploadManager("test-lifecycle-bucket")

	customers := []string{"hts", "cds", "motor"}
	metadata := createTestMetadata(map[string]interface{}{
		"customers":   customers,
		"changeTitle": "Test Lifecycle Policy",
	}, customers)

	uploadQueue := manager.CreateUploadQueue(customers, metadata)

	for _, upload := range uploadQueue {
		switch upload.Type {
		case "customer":
			// Customer uploads should be in customers/ prefix for lifecycle deletion
			if !hasPrefix(upload.Key, "customers/") {
				t.Errorf("Customer upload key %s should start with 'customers/'", upload.Key)
			}

			// Should include customer code in path
			expectedSubstring := fmt.Sprintf("customers/%s/", upload.Customer)
			if !hasPrefix(upload.Key, expectedSubstring) {
				t.Errorf("Customer upload key %s should contain '%s'", upload.Key, expectedSubstring)
			}

		case "archive":
			// Archive uploads should be in archive/ prefix for permanent storage
			if !hasPrefix(upload.Key, "archive/") {
				t.Errorf("Archive upload key %s should start with 'archive/'", upload.Key)
			}
		}

		// All uploads should have .json extension
		if !hasSuffix(upload.Key, ".json") {
			t.Errorf("Upload key %s should end with '.json'", upload.Key)
		}
	}
}

// Helper functions for integration tests

func createTestMetadata(formData map[string]interface{}, customers []string) *ChangeMetadata {
	changeTitle, _ := formData["changeTitle"].(string)
	if changeTitle == "" {
		changeTitle = "Test Change"
	}

	// Create customer names
	customerNames := make([]string, len(customers))
	customerMapping := map[string]string{
		"hts":        "HTS Prod",
		"cds":        "CDS Global",
		"motor":      "Motor",
		"bat":        "Bring A Trailer",
		"icx":        "iCrossing",
		"fdbus":      "FDBUS",
		"hmiit":      "Hearst Magazines Italy",
		"hmies":      "Hearst Magazines Spain",
		"htvdigital": "HTV Digital",
		"htv":        "HTV",
	}

	for i, code := range customers {
		if name, exists := customerMapping[code]; exists {
			customerNames[i] = name
		} else {
			customerNames[i] = code
		}
	}

	return &ChangeMetadata{
		ChangeID:   fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Version:    1,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
		CreatedBy:  "integration-test",
		Status:     "submitted",
		ChangeMetadata: ChangeDetails{
			Title:                  changeTitle,
			CustomerNames:          customerNames,
			CustomerCodes:          customers,
			ChangeReason:           "Integration test change",
			ImplementationPlan:     "Test implementation",
			TestPlan:               "Test validation",
			ExpectedCustomerImpact: "No impact expected",
			RollbackPlan:           "Revert if needed",
			Schedule: ScheduleInfo{
				ImplementationStart: "2025-09-21T10:00:00",
				ImplementationEnd:   "2025-09-21T17:00:00",
				BeginDate:           "2025-09-21",
				BeginTime:           "10:00",
				EndDate:             "2025-09-21",
				EndTime:             "17:00",
				Timezone:            "America/New_York",
			},
			Tickets: TicketInfo{
				ServiceNow: "CHG0123456",
				Jira:       "TEST-123",
			},
		},
		EmailNotification: EmailNotification{
			Subject: fmt.Sprintf("ITSM Change Notification: %s", changeTitle),
		},
	}
}

func validateUploadStructure(t *testing.T, uploadQueue []UploadItem, customers []string) {
	customerUploads := make(map[string]bool)
	archiveFound := false

	for _, upload := range uploadQueue {
		// Validate common fields
		if upload.Status != "pending" {
			t.Errorf("Upload status should be 'pending', got '%s'", upload.Status)
		}

		if upload.Data == nil {
			t.Errorf("Upload data should not be nil")
		}

		switch upload.Type {
		case "customer":
			if upload.Customer == "" {
				t.Errorf("Customer upload missing customer code")
			}
			customerUploads[upload.Customer] = true

		case "archive":
			if upload.Customer != "" {
				t.Errorf("Archive upload should not have customer code")
			}
			archiveFound = true

		default:
			t.Errorf("Unknown upload type: %s", upload.Type)
		}
	}

	// Verify all customers have uploads
	for _, customer := range customers {
		if !customerUploads[customer] {
			t.Errorf("Missing upload for customer: %s", customer)
		}
	}

	// Verify archive upload exists
	if !archiveFound {
		t.Errorf("Missing archive upload")
	}
}

func validateProgressUpdate(t *testing.T, progress map[string]interface{}, result *UploadResult) {
	if progress["total"] != result.TotalUploads {
		t.Errorf("Progress total %v doesn't match result total %d", progress["total"], result.TotalUploads)
	}

	if progress["successful"] != result.SuccessfulUploads {
		t.Errorf("Progress successful %v doesn't match result successful %d", progress["successful"], result.SuccessfulUploads)
	}

	if progress["failed"] != result.FailedUploads {
		t.Errorf("Progress failed %v doesn't match result failed %d", progress["failed"], result.FailedUploads)
	}

	completed := result.SuccessfulUploads + result.FailedUploads
	if progress["completed"] != completed {
		t.Errorf("Progress completed %v doesn't match calculated completed %d", progress["completed"], completed)
	}

	expectedPercentage := float64(completed) / float64(result.TotalUploads) * 100
	if progress["percentage"] != expectedPercentage {
		t.Errorf("Progress percentage %v doesn't match expected %v", progress["percentage"], expectedPercentage)
	}
}

// Helper functions
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// Demo function for integration testing
func DemoIntegrationWorkflow() {
	fmt.Println("=== Multi-Customer Upload Integration Demo ===")

	manager := NewMultiCustomerUploadManager("integration-demo-bucket")

	// Simulate realistic form data
	formData := map[string]interface{}{
		"customers":               []string{"hts", "cds", "motor", "bat", "icx"},
		"changeTitle":             "Deploy Enhanced Monitoring Dashboard",
		"changeReason":            "Improve system observability and reduce MTTR",
		"implementationPlan":      "1. Deploy monitoring agents\n2. Configure dashboards\n3. Set up alerting rules\n4. Test notification channels",
		"testPlan":                "1. Verify metrics collection\n2. Test dashboard functionality\n3. Validate alert triggers\n4. Confirm notification delivery",
		"expectedCustomerImpact":  "Minimal impact during deployment. Improved monitoring capabilities post-deployment.",
		"rollbackPlan":            "1. Disable new monitoring agents\n2. Restore previous dashboard configuration\n3. Revert alerting rules\n4. Validate system stability",
		"implementationBeginDate": "2025-09-21",
		"implementationBeginTime": "02:00",
		"implementationEndDate":   "2025-09-21",
		"implementationEndTime":   "06:00",
		"timezone":                "America/New_York",
		"snowTicket":              "CHG0987654",
		"jiraTicket":              "INFRA-5432",
	}

	fmt.Printf("Form data: %d customers selected\n", len(formData["customers"].([]string)))

	// Step 1: Validate and extract customers
	fmt.Println("\n1. Validating customer selection...")
	customers, err := manager.DetermineAffectedCustomers(formData)
	if err != nil {
		log.Fatalf("Customer validation failed: %v", err)
	}
	fmt.Printf("   ✅ Validated %d customers: %v\n", len(customers), customers)

	// Step 2: Generate metadata
	fmt.Println("\n2. Generating change metadata...")
	metadata := createTestMetadata(formData, customers)
	fmt.Printf("   ✅ Generated metadata for change ID: %s\n", metadata.ChangeID)
	fmt.Printf("   📋 Change: %s\n", metadata.ChangeMetadata.Title)
	fmt.Printf("   🎫 Tickets: SNOW=%s, JIRA=%s\n",
		metadata.ChangeMetadata.Tickets.ServiceNow,
		metadata.ChangeMetadata.Tickets.Jira)

	// Step 3: Create upload plan
	fmt.Println("\n3. Creating upload plan...")
	uploadQueue := manager.CreateUploadQueue(customers, metadata)
	fmt.Printf("   📤 Upload plan created: %d total uploads\n", len(uploadQueue))

	customerCount := 0
	archiveCount := 0
	for _, upload := range uploadQueue {
		if upload.Type == "customer" {
			customerCount++
		} else if upload.Type == "archive" {
			archiveCount++
		}
	}
	fmt.Printf("   - Customer uploads: %d\n", customerCount)
	fmt.Printf("   - Archive uploads: %d\n", archiveCount)

	// Step 4: Execute uploads with progress tracking
	fmt.Println("\n4. Executing uploads...")
	startTime := time.Now()

	// Simulate progress updates during upload
	go func() {
		for i := 0; i <= len(uploadQueue); i++ {
			time.Sleep(200 * time.Millisecond)
			percentage := float64(i) / float64(len(uploadQueue)) * 100
			fmt.Printf("   📊 Progress: %d/%d uploads (%.1f%%)\n", i, len(uploadQueue), percentage)
		}
	}()

	result := manager.ProcessUploadQueue(uploadQueue)
	duration := time.Since(startTime)

	fmt.Printf("\n   ⏱️  Upload completed in %v\n", duration)
	fmt.Printf("   ✅ Successful uploads: %d\n", result.SuccessfulUploads)
	fmt.Printf("   ❌ Failed uploads: %d\n", result.FailedUploads)

	// Step 5: Handle failures if any
	if result.FailedUploads > 0 {
		fmt.Println("\n5. Handling upload failures...")
		fmt.Printf("   ⚠️  %d uploads failed, initiating retry...\n", result.FailedUploads)

		retryResult := manager.RetryFailedUploads(result)
		fmt.Printf("   🔄 Retry completed: %d successful, %d still failed\n",
			retryResult.SuccessfulUploads, retryResult.FailedUploads)

		totalSuccessful := result.SuccessfulUploads + retryResult.SuccessfulUploads
		totalFailed := retryResult.FailedUploads

		if totalFailed == 0 {
			fmt.Printf("   ✅ All uploads successful after retry!\n")
		} else {
			fmt.Printf("   ⚠️  Final result: %d successful, %d failed\n", totalSuccessful, totalFailed)
		}
	} else {
		fmt.Println("\n5. All uploads successful on first attempt! 🎉")
	}

	// Step 6: Final validation
	fmt.Println("\n6. Final validation...")
	if err := manager.ValidateAllUploadsSucceeded(result); err != nil {
		fmt.Printf("   ⚠️  Validation warning: %v\n", err)
		fmt.Printf("   📋 Some customers may not receive change notifications\n")
	} else {
		fmt.Printf("   ✅ All validations passed!\n")
		fmt.Printf("   📧 Change notifications will be sent to all %d customers\n", len(customers))
	}

	// Step 7: Generate final report
	fmt.Println("\n7. Upload summary report:")
	progress := manager.GenerateProgressUpdate(result)
	fmt.Printf("   📊 Total uploads: %v\n", progress["total"])
	fmt.Printf("   ✅ Successful: %v (%.1f%%)\n", progress["successful"],
		float64(progress["successful"].(int))/float64(progress["total"].(int))*100)
	fmt.Printf("   ❌ Failed: %v\n", progress["failed"])
	fmt.Printf("   ⏱️  Duration: %v\n", progress["duration"])

	fmt.Println("\n=== Integration Demo Complete ===")
	fmt.Printf("Change %s is ready for distribution! 🚀\n", metadata.ChangeID)
}

func main() {
	// Run integration demo
	DemoIntegrationWorkflow()
}
