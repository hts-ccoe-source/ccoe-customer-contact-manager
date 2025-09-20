package main

import (
	"strings"
	"testing"
)

func TestS3EventTester_TestS3EventDelivery(t *testing.T) {
	// Create test configuration
	manager := NewS3EventConfigManager("test-bucket")
	err := manager.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}
	err = manager.AddCustomerNotification("customer-b", "arn:aws:sqs:us-east-1:123456789012:queue2")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}

	// Create tester
	tester := NewS3EventTester(manager)

	// Test dry run
	customerCodes := []string{"customer-a", "customer-b"}
	err = tester.TestS3EventDelivery(customerCodes, true)
	if err != nil {
		t.Errorf("Dry run test failed: %v", err)
	}

	// Test with non-existent customer
	invalidCodes := []string{"non-existent"}
	err = tester.TestS3EventDelivery(invalidCodes, true)
	if err == nil {
		t.Errorf("Expected error for non-existent customer, but got none")
	}
}

func TestS3EventTester_GenerateS3EventTestPlan(t *testing.T) {
	// Create test configuration
	manager := NewS3EventConfigManager("test-metadata-bucket")
	err := manager.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:customer-a-queue")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}
	err = manager.AddCustomerNotification("customer-b", "arn:aws:sqs:us-east-1:123456789012:customer-b-queue")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}

	// Create tester
	tester := NewS3EventTester(manager)

	// Generate test plan
	testPlan, err := tester.GenerateS3EventTestPlan()
	if err != nil {
		t.Fatalf("Failed to generate test plan: %v", err)
	}

	// Verify test plan contains expected elements
	expectedElements := []string{
		"# S3 Event Delivery Test Plan",
		"test-metadata-bucket",
		"Customer Count**: 2",
		"### Test Case 1: customer-a",
		"### Test Case 2: customer-b",
		"customers/customer-a/",
		"customers/customer-b/",
		"arn:aws:sqs:us-east-1:123456789012:customer-a-queue",
		"arn:aws:sqs:us-east-1:123456789012:customer-b-queue",
		"Upload test file to",
		"Verify S3 event is generated",
		"Confirm SQS message is received",
		"go run . test-s3-events",
	}

	for _, element := range expectedElements {
		if !strings.Contains(testPlan, element) {
			t.Errorf("Test plan missing expected element: %s", element)
		}
	}

	// Verify it's properly formatted markdown
	if !strings.HasPrefix(testPlan, "# S3 Event Delivery Test Plan") {
		t.Errorf("Test plan doesn't start with proper markdown header")
	}
}

func TestS3EventTester_NilConfig(t *testing.T) {
	// Test with nil configuration
	tester := NewS3EventTester(nil)

	err := tester.TestS3EventDelivery([]string{"customer-a"}, true)
	if err == nil {
		t.Errorf("Expected error with nil configuration, but got none")
	}
	if !strings.Contains(err.Error(), "S3 event configuration is required") {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}

	_, err = tester.GenerateS3EventTestPlan()
	if err == nil {
		t.Errorf("Expected error with nil configuration, but got none")
	}
}
