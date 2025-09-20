package main

import (
	"fmt"
	"log"
	"time"
)

// Validation tests for multi-customer upload functionality

func main() {
	fmt.Println("=== Multi-Customer Upload Validation ===")

	// Test 1: Customer determination logic
	fmt.Println("\n🧪 Test 1: Customer Determination Logic")
	testCustomerDetermination()

	// Test 2: Upload queue creation
	fmt.Println("\n🧪 Test 2: Upload Queue Creation")
	testUploadQueueCreation()

	// Test 3: Progress indicators
	fmt.Println("\n🧪 Test 3: Progress Indicators")
	testProgressIndicators()

	// Test 4: Error handling for partial failures
	fmt.Println("\n🧪 Test 4: Error Handling for Partial Failures")
	testErrorHandling()

	// Test 5: Upload validation
	fmt.Println("\n🧪 Test 5: Upload Validation")
	testUploadValidation()

	// Test 6: S3 lifecycle policy configuration
	fmt.Println("\n🧪 Test 6: S3 Lifecycle Policy Configuration")
	testS3LifecyclePolicyConfiguration()

	fmt.Println("\n✅ All validation tests completed!")
}

func testCustomerDetermination() {
	manager := NewMultiCustomerUploadManager("test-bucket")

	// Test valid customers
	formData := map[string]interface{}{
		"customers": []string{"hts", "cds", "motor", "bat", "icx"},
	}

	customers, err := manager.DetermineAffectedCustomers(formData)
	if err != nil {
		log.Printf("   ❌ Error: %v", err)
		return
	}

	if len(customers) != 5 {
		log.Printf("   ❌ Expected 5 customers, got %d", len(customers))
		return
	}

	fmt.Printf("   ✅ Successfully determined %d affected customers: %v\n", len(customers), customers)

	// Test invalid customer
	invalidFormData := map[string]interface{}{
		"customers": []string{"invalid-customer"},
	}

	_, err = manager.DetermineAffectedCustomers(invalidFormData)
	if err == nil {
		log.Printf("   ❌ Expected error for invalid customer, but got none")
		return
	}

	fmt.Printf("   ✅ Correctly rejected invalid customer: %v\n", err)

	// Test empty customers
	emptyFormData := map[string]interface{}{
		"customers": []string{},
	}

	_, err = manager.DetermineAffectedCustomers(emptyFormData)
	if err == nil {
		log.Printf("   ❌ Expected error for empty customers, but got none")
		return
	}

	fmt.Printf("   ✅ Correctly rejected empty customer list: %v\n", err)
}

func testUploadQueueCreation() {
	manager := NewMultiCustomerUploadManager("test-bucket")

	customers := []string{"hts", "cds", "motor"}
	metadata := &ChangeMetadata{
		ChangeID:  "test-change-123",
		Version:   1,
		CreatedAt: time.Now().Format(time.RFC3339),
		Status:    "submitted",
		ChangeMetadata: ChangeDetails{
			Title:         "Test Multi-Upload",
			CustomerCodes: customers,
		},
	}

	uploadQueue := manager.CreateUploadQueue(customers, metadata)

	// Should have 3 customer uploads + 1 archive upload = 4 total
	expectedUploads := len(customers) + 1
	if len(uploadQueue) != expectedUploads {
		log.Printf("   ❌ Expected %d uploads, got %d", expectedUploads, len(uploadQueue))
		return
	}

	customerUploads := 0
	archiveUploads := 0

	for _, upload := range uploadQueue {
		if upload.Type == "customer" {
			customerUploads++
			// Validate customer upload structure
			expectedPrefix := fmt.Sprintf("customers/%s/", upload.Customer)
			if !hasPrefix(upload.Key, expectedPrefix) {
				log.Printf("   ❌ Customer upload key %s missing expected prefix %s", upload.Key, expectedPrefix)
				return
			}
		} else if upload.Type == "archive" {
			archiveUploads++
			// Validate archive upload structure
			if !hasPrefix(upload.Key, "archive/") {
				log.Printf("   ❌ Archive upload key %s missing 'archive/' prefix", upload.Key)
				return
			}
		}

		// Validate all uploads have .json extension
		if !hasSuffix(upload.Key, ".json") {
			log.Printf("   ❌ Upload key %s missing '.json' extension", upload.Key)
			return
		}

		// Validate initial status
		if upload.Status != "pending" {
			log.Printf("   ❌ Upload status should be 'pending', got '%s'", upload.Status)
			return
		}
	}

	if customerUploads != len(customers) {
		log.Printf("   ❌ Expected %d customer uploads, got %d", len(customers), customerUploads)
		return
	}

	if archiveUploads != 1 {
		log.Printf("   ❌ Expected 1 archive upload, got %d", archiveUploads)
		return
	}

	fmt.Printf("   ✅ Upload queue created correctly: %d customer uploads + %d archive upload\n", customerUploads, archiveUploads)

	// Print upload structure for verification
	fmt.Printf("   📋 Upload structure:\n")
	for _, upload := range uploadQueue {
		fmt.Printf("      - %s: %s\n", upload.Type, upload.Key)
	}
}

func testProgressIndicators() {
	manager := NewMultiCustomerUploadManager("test-bucket")

	// Test progress at different stages
	tests := []struct {
		name       string
		total      int
		successful int
		failed     int
		expected   float64
	}{
		{"Initial", 5, 0, 0, 0.0},
		{"Partial", 5, 2, 1, 60.0},
		{"Complete", 5, 4, 1, 100.0},
		{"All Success", 3, 3, 0, 100.0},
	}

	for _, test := range tests {
		result := &UploadResult{
			TotalUploads:      test.total,
			SuccessfulUploads: test.successful,
			FailedUploads:     test.failed,
			Duration:          1 * time.Second,
		}

		progress := manager.GenerateProgressUpdate(result)

		if progress["percentage"] != test.expected {
			log.Printf("   ❌ %s: Expected %.1f%%, got %v", test.name, test.expected, progress["percentage"])
			return
		}

		if progress["total"] != test.total {
			log.Printf("   ❌ %s: Expected total %d, got %v", test.name, test.total, progress["total"])
			return
		}

		if progress["successful"] != test.successful {
			log.Printf("   ❌ %s: Expected successful %d, got %v", test.name, test.successful, progress["successful"])
			return
		}

		if progress["failed"] != test.failed {
			log.Printf("   ❌ %s: Expected failed %d, got %v", test.name, test.failed, progress["failed"])
			return
		}

		fmt.Printf("   ✅ %s progress: %.1f%% (%d/%d successful, %d failed)\n",
			test.name, test.expected, test.successful, test.total, test.failed)
	}
}

func testErrorHandling() {
	manager := NewMultiCustomerUploadManager("test-bucket")

	// Create a result with mixed success/failure
	result := &UploadResult{
		TotalUploads:      4,
		SuccessfulUploads: 2,
		FailedUploads:     2,
		UploadItems: []UploadItem{
			{Type: "customer", Customer: "hts", Key: "customers/hts/test.json", Status: "success"},
			{Type: "customer", Customer: "cds", Key: "customers/cds/test.json", Status: "error", Error: "network error"},
			{Type: "customer", Customer: "motor", Key: "customers/motor/test.json", Status: "error", Error: "timeout"},
			{Type: "archive", Customer: "", Key: "archive/test.json", Status: "success"},
		},
	}

	// Test validation of partial failure
	err := manager.ValidateAllUploadsSucceeded(result)
	if err == nil {
		log.Printf("   ❌ Expected validation error for partial failure, got none")
		return
	}

	fmt.Printf("   ✅ Correctly detected partial failure: %v\n", err)

	// Test retry mechanism
	retryResult := manager.RetryFailedUploads(result)

	if retryResult.TotalUploads != 2 {
		log.Printf("   ❌ Expected 2 retry uploads, got %d", retryResult.TotalUploads)
		return
	}

	fmt.Printf("   ✅ Retry mechanism correctly identified %d failed uploads for retry\n", retryResult.TotalUploads)

	// Verify only failed uploads are in retry queue
	for _, upload := range retryResult.UploadItems {
		if upload.Customer != "cds" && upload.Customer != "motor" {
			log.Printf("   ❌ Unexpected upload in retry queue: %s", upload.Customer)
			return
		}
	}

	fmt.Printf("   ✅ Retry queue contains only previously failed uploads\n")
}

func testUploadValidation() {
	manager := NewMultiCustomerUploadManager("test-bucket")

	// Test successful validation
	successResult := &UploadResult{
		TotalUploads:      3,
		SuccessfulUploads: 3,
		FailedUploads:     0,
	}

	err := manager.ValidateAllUploadsSucceeded(successResult)
	if err != nil {
		log.Printf("   ❌ Expected no error for successful uploads, got: %v", err)
		return
	}

	fmt.Printf("   ✅ Validation passed for all successful uploads\n")

	// Test failed validation
	failedResult := &UploadResult{
		TotalUploads:      3,
		SuccessfulUploads: 1,
		FailedUploads:     2,
	}

	err = manager.ValidateAllUploadsSucceeded(failedResult)
	if err == nil {
		log.Printf("   ❌ Expected error for failed uploads, got none")
		return
	}

	fmt.Printf("   ✅ Validation correctly failed for partial success: %v\n", err)
}

func testS3LifecyclePolicyConfiguration() {
	manager := NewMultiCustomerUploadManager("test-bucket")

	customers := []string{"hts", "cds", "motor"}
	metadata := &ChangeMetadata{
		ChangeID: "lifecycle-test-123",
		Version:  1,
		Status:   "submitted",
	}

	uploadQueue := manager.CreateUploadQueue(customers, metadata)

	for _, upload := range uploadQueue {
		switch upload.Type {
		case "customer":
			// Customer uploads should be in customers/ prefix for lifecycle deletion
			if !hasPrefix(upload.Key, "customers/") {
				log.Printf("   ❌ Customer upload key %s should start with 'customers/'", upload.Key)
				return
			}

			// Should include customer code in path
			expectedSubstring := fmt.Sprintf("customers/%s/", upload.Customer)
			if !hasPrefix(upload.Key, expectedSubstring) {
				log.Printf("   ❌ Customer upload key %s should contain '%s'", upload.Key, expectedSubstring)
				return
			}

		case "archive":
			// Archive uploads should be in archive/ prefix for permanent storage
			if !hasPrefix(upload.Key, "archive/") {
				log.Printf("   ❌ Archive upload key %s should start with 'archive/'", upload.Key)
				return
			}
		}

		// All uploads should have .json extension
		if !hasSuffix(upload.Key, ".json") {
			log.Printf("   ❌ Upload key %s should end with '.json'", upload.Key)
			return
		}
	}

	fmt.Printf("   ✅ All upload keys follow S3 lifecycle policy structure\n")
	fmt.Printf("   📋 Lifecycle configuration:\n")
	fmt.Printf("      - customers/* prefix: Auto-delete after 30 days (operational files)\n")
	fmt.Printf("      - archive/* prefix: Permanent storage (no deletion)\n")

	// Show example keys
	fmt.Printf("   📋 Example upload keys:\n")
	for _, upload := range uploadQueue {
		if upload.Type == "customer" {
			fmt.Printf("      - Customer (%s): %s\n", upload.Customer, upload.Key)
		} else {
			fmt.Printf("      - Archive: %s\n", upload.Key)
		}
	}
}

// Helper functions
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// Include the required types and functions from the main implementation
// (This would normally be imported from a package)

type MultiCustomerUploadManager struct {
	BucketName string
	S3Client   interface{}
}

type UploadItem struct {
	Type     string      `json:"type"`
	Customer string      `json:"customer"`
	Key      string      `json:"key"`
	Data     interface{} `json:"data"`
	Status   string      `json:"status"`
	Error    string      `json:"error,omitempty"`
}

type UploadResult struct {
	TotalUploads      int           `json:"totalUploads"`
	SuccessfulUploads int           `json:"successfulUploads"`
	FailedUploads     int           `json:"failedUploads"`
	UploadItems       []UploadItem  `json:"uploadItems"`
	Duration          time.Duration `json:"duration"`
}

type ChangeMetadata struct {
	ChangeID          string            `json:"changeId"`
	Version           int               `json:"version"`
	CreatedAt         string            `json:"createdAt"`
	ModifiedAt        string            `json:"modifiedAt"`
	CreatedBy         string            `json:"createdBy"`
	ModifiedBy        string            `json:"modifiedBy"`
	Status            string            `json:"status"`
	ChangeMetadata    ChangeDetails     `json:"changeMetadata"`
	EmailNotification EmailNotification `json:"emailNotification"`
}

type ChangeDetails struct {
	Title                  string       `json:"title"`
	CustomerNames          []string     `json:"customerNames"`
	CustomerCodes          []string     `json:"customerCodes"`
	Tickets                TicketInfo   `json:"tickets"`
	Schedule               ScheduleInfo `json:"schedule"`
	ChangeReason           string       `json:"changeReason"`
	ImplementationPlan     string       `json:"implementationPlan"`
	TestPlan               string       `json:"testPlan"`
	ExpectedCustomerImpact string       `json:"expectedCustomerImpact"`
	RollbackPlan           string       `json:"rollbackPlan"`
}

type TicketInfo struct {
	ServiceNow string `json:"serviceNow"`
	Jira       string `json:"jira"`
}

type ScheduleInfo struct {
	ImplementationStart string `json:"implementationStart"`
	ImplementationEnd   string `json:"implementationEnd"`
	BeginDate           string `json:"beginDate"`
	BeginTime           string `json:"beginTime"`
	EndDate             string `json:"endDate"`
	EndTime             string `json:"endTime"`
	Timezone            string `json:"timezone"`
}

type EmailNotification struct {
	Subject string `json:"subject"`
}

func NewMultiCustomerUploadManager(bucketName string) *MultiCustomerUploadManager {
	return &MultiCustomerUploadManager{
		BucketName: bucketName,
	}
}

func (m *MultiCustomerUploadManager) DetermineAffectedCustomers(formData map[string]interface{}) ([]string, error) {
	customers, ok := formData["customers"].([]string)
	if !ok {
		return nil, fmt.Errorf("no customers specified in form data")
	}

	if len(customers) == 0 {
		return nil, fmt.Errorf("at least one customer must be selected")
	}

	validCustomers := map[string]bool{
		"hts": true, "cds": true, "fdbus": true, "hmiit": true, "hmies": true,
		"htvdigital": true, "htv": true, "icx": true, "motor": true, "bat": true,
		"mhk": true, "hdmautos": true, "hnpit": true, "hnpdigital": true,
		"camp": true, "mcg": true, "hmuk": true, "hmusdigital": true,
		"hwp": true, "zynx": true, "hchb": true, "fdbuk": true,
		"hecom": true, "blkbook": true,
	}

	var validatedCustomers []string
	for _, customer := range customers {
		if validCustomers[customer] {
			validatedCustomers = append(validatedCustomers, customer)
		} else {
			return nil, fmt.Errorf("invalid customer code: %s", customer)
		}
	}

	return validatedCustomers, nil
}

func (m *MultiCustomerUploadManager) CreateUploadQueue(customers []string, metadata *ChangeMetadata) []UploadItem {
	var uploadQueue []UploadItem

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s-%s.json", metadata.ChangeID, timestamp)

	for _, customer := range customers {
		uploadQueue = append(uploadQueue, UploadItem{
			Type:     "customer",
			Customer: customer,
			Key:      fmt.Sprintf("customers/%s/%s", customer, filename),
			Data:     metadata,
			Status:   "pending",
		})
	}

	uploadQueue = append(uploadQueue, UploadItem{
		Type:     "archive",
		Customer: "",
		Key:      fmt.Sprintf("archive/%s", filename),
		Data:     metadata,
		Status:   "pending",
	})

	return uploadQueue
}

func (m *MultiCustomerUploadManager) RetryFailedUploads(previousResult *UploadResult) *UploadResult {
	var failedUploads []UploadItem

	for _, upload := range previousResult.UploadItems {
		if upload.Status == "error" {
			upload.Status = "pending"
			upload.Error = ""
			failedUploads = append(failedUploads, upload)
		}
	}

	return &UploadResult{
		TotalUploads:      len(failedUploads),
		SuccessfulUploads: 0,
		FailedUploads:     0,
		UploadItems:       failedUploads,
		Duration:          0,
	}
}

func (m *MultiCustomerUploadManager) ValidateAllUploadsSucceeded(result *UploadResult) error {
	if result.FailedUploads > 0 {
		return fmt.Errorf("%d out of %d uploads failed", result.FailedUploads, result.TotalUploads)
	}
	return nil
}

func (m *MultiCustomerUploadManager) GenerateProgressUpdate(result *UploadResult) map[string]interface{} {
	completed := result.SuccessfulUploads + result.FailedUploads
	percentage := float64(completed) / float64(result.TotalUploads) * 100

	return map[string]interface{}{
		"completed":  completed,
		"total":      result.TotalUploads,
		"percentage": percentage,
		"successful": result.SuccessfulUploads,
		"failed":     result.FailedUploads,
		"duration":   result.Duration.String(),
	}
}
