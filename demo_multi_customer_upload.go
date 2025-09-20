package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// MultiCustomerUploadManager handles the upload logic for multi-customer changes
type MultiCustomerUploadManager struct {
	BucketName string
	S3Client   interface{} // Would be *s3.S3 in real implementation
}

// UploadItem represents a single upload operation
type UploadItem struct {
	Type     string      `json:"type"`     // "customer" or "archive"
	Customer string      `json:"customer"` // customer code, empty for archive
	Key      string      `json:"key"`      // S3 key path
	Data     interface{} `json:"data"`     // metadata to upload
	Status   string      `json:"status"`   // "pending", "uploading", "success", "error"
	Error    string      `json:"error,omitempty"`
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	TotalUploads      int           `json:"totalUploads"`
	SuccessfulUploads int           `json:"successfulUploads"`
	FailedUploads     int           `json:"failedUploads"`
	UploadItems       []UploadItem  `json:"uploadItems"`
	Duration          time.Duration `json:"duration"`
}

// ChangeMetadata represents the structure of change metadata
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

// NewMultiCustomerUploadManager creates a new upload manager
func NewMultiCustomerUploadManager(bucketName string) *MultiCustomerUploadManager {
	return &MultiCustomerUploadManager{
		BucketName: bucketName,
		// S3Client would be initialized here in real implementation
	}
}

// DetermineAffectedCustomers extracts customer codes from form data
func (m *MultiCustomerUploadManager) DetermineAffectedCustomers(formData map[string]interface{}) ([]string, error) {
	customers, ok := formData["customers"].([]string)
	if !ok {
		return nil, fmt.Errorf("no customers specified in form data")
	}

	if len(customers) == 0 {
		return nil, fmt.Errorf("at least one customer must be selected")
	}

	// Validate customer codes
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

// CreateUploadQueue creates the upload queue for multi-customer and archive uploads
func (m *MultiCustomerUploadManager) CreateUploadQueue(customers []string, metadata *ChangeMetadata) []UploadItem {
	var uploadQueue []UploadItem

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s-%s.json", metadata.ChangeID, timestamp)

	// Add customer-specific uploads
	for _, customer := range customers {
		uploadQueue = append(uploadQueue, UploadItem{
			Type:     "customer",
			Customer: customer,
			Key:      fmt.Sprintf("customers/%s/%s", customer, filename),
			Data:     metadata,
			Status:   "pending",
		})
	}

	// Add archive upload
	uploadQueue = append(uploadQueue, UploadItem{
		Type:     "archive",
		Customer: "",
		Key:      fmt.Sprintf("archive/%s", filename),
		Data:     metadata,
		Status:   "pending",
	})

	return uploadQueue
}

// ProcessUploadQueue processes all uploads in the queue with error handling
func (m *MultiCustomerUploadManager) ProcessUploadQueue(uploadQueue []UploadItem) *UploadResult {
	startTime := time.Now()
	result := &UploadResult{
		TotalUploads:      len(uploadQueue),
		SuccessfulUploads: 0,
		FailedUploads:     0,
		UploadItems:       make([]UploadItem, len(uploadQueue)),
	}

	// Copy upload queue to result
	copy(result.UploadItems, uploadQueue)

	// Process each upload
	for i := range result.UploadItems {
		upload := &result.UploadItems[i]
		upload.Status = "uploading"

		// Simulate S3 upload (replace with actual AWS SDK call)
		if err := m.simulateS3Upload(upload); err != nil {
			upload.Status = "error"
			upload.Error = err.Error()
			result.FailedUploads++
		} else {
			upload.Status = "success"
			result.SuccessfulUploads++
		}

		// Small delay between uploads to avoid overwhelming S3
		time.Sleep(100 * time.Millisecond)
	}

	result.Duration = time.Since(startTime)
	return result
}

// simulateS3Upload simulates an S3 upload operation
func (m *MultiCustomerUploadManager) simulateS3Upload(upload *UploadItem) error {
	// Simulate network delay
	time.Sleep(time.Duration(50+time.Now().UnixNano()%100) * time.Millisecond)

	// Simulate occasional failures (5% failure rate)
	if time.Now().UnixNano()%20 == 0 {
		return fmt.Errorf("simulated network error for %s", upload.Key)
	}

	// In real implementation, this would be:
	// jsonData, err := json.Marshal(upload.Data)
	// if err != nil {
	//     return fmt.Errorf("failed to marshal JSON: %v", err)
	// }
	//
	// _, err = m.S3Client.PutObject(&s3.PutObjectInput{
	//     Bucket:      aws.String(m.BucketName),
	//     Key:         aws.String(upload.Key),
	//     Body:        bytes.NewReader(jsonData),
	//     ContentType: aws.String("application/json"),
	// })
	// return err

	log.Printf("Successfully uploaded to %s", upload.Key)
	return nil
}

// RetryFailedUploads retries only the failed uploads from a previous result
func (m *MultiCustomerUploadManager) RetryFailedUploads(previousResult *UploadResult) *UploadResult {
	var failedUploads []UploadItem

	// Extract failed uploads
	for _, upload := range previousResult.UploadItems {
		if upload.Status == "error" {
			// Reset status for retry
			upload.Status = "pending"
			upload.Error = ""
			failedUploads = append(failedUploads, upload)
		}
	}

	if len(failedUploads) == 0 {
		return &UploadResult{
			TotalUploads:      0,
			SuccessfulUploads: 0,
			FailedUploads:     0,
			UploadItems:       []UploadItem{},
			Duration:          0,
		}
	}

	// Process retry uploads
	return m.ProcessUploadQueue(failedUploads)
}

// ValidateAllUploadsSucceeded checks if all uploads in the result were successful
func (m *MultiCustomerUploadManager) ValidateAllUploadsSucceeded(result *UploadResult) error {
	if result.FailedUploads > 0 {
		return fmt.Errorf("%d out of %d uploads failed", result.FailedUploads, result.TotalUploads)
	}
	return nil
}

// GenerateProgressUpdate creates a progress update for UI display
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

// Demo function to show the complete workflow
func DemoMultiCustomerUpload() {
	fmt.Println("=== Multi-Customer Upload Demo ===")

	manager := NewMultiCustomerUploadManager("multi-customer-metadata-bucket")

	// Simulate form data
	formData := map[string]interface{}{
		"customers": []string{"hts", "cds", "motor", "bat"},
		"title":     "Deploy new monitoring system",
	}

	// Step 1: Determine affected customers
	fmt.Println("\n1. Determining affected customers...")
	customers, err := manager.DetermineAffectedCustomers(formData)
	if err != nil {
		log.Fatalf("Error determining customers: %v", err)
	}
	fmt.Printf("   Affected customers: %v\n", customers)

	// Step 2: Create metadata
	fmt.Println("\n2. Creating change metadata...")
	metadata := &ChangeMetadata{
		ChangeID:   "550e8400-e29b-41d4-a716-446655440000",
		Version:    1,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
		CreatedBy:  "demo-user@company.com",
		Status:     "submitted",
		ChangeMetadata: ChangeDetails{
			Title:         "Deploy new monitoring system",
			CustomerNames: []string{"HTS Prod", "CDS Global", "Motor", "Bring A Trailer"},
			CustomerCodes: customers,
			ChangeReason:  "Improve system observability and alerting",
		},
		EmailNotification: EmailNotification{
			Subject: "ITSM Change Notification: Deploy new monitoring system",
		},
	}
	fmt.Printf("   Change ID: %s\n", metadata.ChangeID)

	// Step 3: Create upload queue
	fmt.Println("\n3. Creating upload queue...")
	uploadQueue := manager.CreateUploadQueue(customers, metadata)
	fmt.Printf("   Upload queue created with %d items:\n", len(uploadQueue))
	for _, upload := range uploadQueue {
		fmt.Printf("   - %s: %s\n", upload.Type, upload.Key)
	}

	// Step 4: Process uploads
	fmt.Println("\n4. Processing uploads...")
	result := manager.ProcessUploadQueue(uploadQueue)
	fmt.Printf("   Upload completed in %v\n", result.Duration)
	fmt.Printf("   Successful: %d, Failed: %d\n", result.SuccessfulUploads, result.FailedUploads)

	// Step 5: Show progress update
	fmt.Println("\n5. Progress summary:")
	progress := manager.GenerateProgressUpdate(result)
	progressJSON, _ := json.MarshalIndent(progress, "   ", "  ")
	fmt.Printf("   %s\n", progressJSON)

	// Step 6: Handle failures if any
	if result.FailedUploads > 0 {
		fmt.Println("\n6. Retrying failed uploads...")
		retryResult := manager.RetryFailedUploads(result)
		fmt.Printf("   Retry completed: %d successful, %d failed\n",
			retryResult.SuccessfulUploads, retryResult.FailedUploads)
	}

	// Step 7: Final validation
	fmt.Println("\n7. Final validation...")
	if err := manager.ValidateAllUploadsSucceeded(result); err != nil {
		fmt.Printf("   ⚠️  Validation failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ All uploads successful!\n")
	}

	fmt.Println("\n=== Demo Complete ===")
}

func main() {
	// Run the demo
	DemoMultiCustomerUpload()
}
