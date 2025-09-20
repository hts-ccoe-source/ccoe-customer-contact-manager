package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSQSMessageSender_CreateSQSMessage(t *testing.T) {
	sender := NewSQSMessageSender(nil)

	// Create sample metadata
	metadata := &ApprovalRequestMetadata{
		ChangeMetadata: struct {
			Title         string   `json:"title"`
			CustomerNames []string `json:"customerNames"`
			CustomerCodes []string `json:"customerCodes"`
			Tickets       struct {
				ServiceNow string `json:"serviceNow"`
				Jira       string `json:"jira"`
			} `json:"tickets"`
			ChangeReason           string `json:"changeReason"`
			ImplementationPlan     string `json:"implementationPlan"`
			TestPlan               string `json:"testPlan"`
			ExpectedCustomerImpact string `json:"expectedCustomerImpact"`
			RollbackPlan           string `json:"rollbackPlan"`
			Schedule               struct {
				ImplementationStart string `json:"implementationStart"`
				ImplementationEnd   string `json:"implementationEnd"`
				BeginDate           string `json:"beginDate"`
				BeginTime           string `json:"beginTime"`
				EndDate             string `json:"endDate"`
				EndTime             string `json:"endTime"`
				Timezone            string `json:"timezone"`
			} `json:"schedule"`
		}{
			Title:         "Test Change",
			CustomerNames: []string{"Customer A"},
			CustomerCodes: []string{"customer-a"},
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		GeneratedBy: "test",
	}

	tests := []struct {
		name          string
		customerCode  string
		actionType    string
		metadata      *ApprovalRequestMetadata
		expectedError bool
		errorContains string
	}{
		{
			name:          "Valid message creation",
			customerCode:  "customer-a",
			actionType:    "send-change-notification",
			metadata:      metadata,
			expectedError: false,
		},
		{
			name:          "Empty customer code",
			customerCode:  "",
			actionType:    "send-change-notification",
			metadata:      metadata,
			expectedError: true,
			errorContains: "customer code cannot be empty",
		},
		{
			name:          "Empty action type",
			customerCode:  "customer-a",
			actionType:    "",
			metadata:      metadata,
			expectedError: true,
			errorContains: "action type cannot be empty",
		},
		{
			name:          "Nil metadata",
			customerCode:  "customer-a",
			actionType:    "send-change-notification",
			metadata:      nil,
			expectedError: true,
			errorContains: "metadata cannot be nil",
		},
		{
			name:          "Invalid customer code format",
			customerCode:  "invalid@code",
			actionType:    "send-change-notification",
			metadata:      metadata,
			expectedError: true,
			errorContains: "invalid customer code format",
		},
		{
			name:          "Invalid action type",
			customerCode:  "customer-a",
			actionType:    "invalid-action",
			metadata:      metadata,
			expectedError: true,
			errorContains: "invalid action type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := sender.CreateSQSMessage(tt.customerCode, tt.actionType, tt.metadata)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}

				// Validate message structure
				if message.CustomerCode != tt.customerCode {
					t.Errorf("Expected customer code %s, got %s", tt.customerCode, message.CustomerCode)
				}
				if message.ActionType != tt.actionType {
					t.Errorf("Expected action type %s, got %s", tt.actionType, message.ActionType)
				}
				if message.Metadata != tt.metadata {
					t.Errorf("Expected metadata to match input")
				}
				if message.ExecutionID == "" {
					t.Errorf("Expected execution ID to be generated")
				}
				if message.Timestamp == "" {
					t.Errorf("Expected timestamp to be generated")
				}
				if message.RetryCount != 0 {
					t.Errorf("Expected retry count to be 0, got %d", message.RetryCount)
				}

				// Validate timestamp format
				_, err := time.Parse(time.RFC3339, message.Timestamp)
				if err != nil {
					t.Errorf("Invalid timestamp format: %s", message.Timestamp)
				}

				// Validate execution ID format
				expectedPrefix := tt.customerCode + "-" + tt.actionType + "-"
				if !strings.HasPrefix(message.ExecutionID, expectedPrefix) {
					t.Errorf("Expected execution ID to start with '%s', got: %s", expectedPrefix, message.ExecutionID)
				}
			}
		})
	}
}

func TestSQSMessageSender_ValidateSQSMessage(t *testing.T) {
	sender := NewSQSMessageSender(nil)

	// Create valid message
	validMessage := &SQSMessage{
		ExecutionID:  "customer-a-send-change-notification-123456",
		ActionType:   "send-change-notification",
		CustomerCode: "customer-a",
		Timestamp:    time.Now().Format(time.RFC3339),
		RetryCount:   0,
		Metadata:     &ApprovalRequestMetadata{},
	}

	tests := []struct {
		name          string
		message       *SQSMessage
		expectedError bool
		errorContains string
	}{
		{
			name:          "Valid message",
			message:       validMessage,
			expectedError: false,
		},
		{
			name:          "Nil message",
			message:       nil,
			expectedError: true,
			errorContains: "message cannot be nil",
		},
		{
			name: "Empty execution ID",
			message: &SQSMessage{
				ExecutionID:  "",
				ActionType:   "send-change-notification",
				CustomerCode: "customer-a",
				Timestamp:    time.Now().Format(time.RFC3339),
				RetryCount:   0,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "execution ID cannot be empty",
		},
		{
			name: "Empty action type",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "",
				CustomerCode: "customer-a",
				Timestamp:    time.Now().Format(time.RFC3339),
				RetryCount:   0,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "action type cannot be empty",
		},
		{
			name: "Empty customer code",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "send-change-notification",
				CustomerCode: "",
				Timestamp:    time.Now().Format(time.RFC3339),
				RetryCount:   0,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "customer code cannot be empty",
		},
		{
			name: "Empty timestamp",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "send-change-notification",
				CustomerCode: "customer-a",
				Timestamp:    "",
				RetryCount:   0,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "timestamp cannot be empty",
		},
		{
			name: "Nil metadata",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "send-change-notification",
				CustomerCode: "customer-a",
				Timestamp:    time.Now().Format(time.RFC3339),
				RetryCount:   0,
				Metadata:     nil,
			},
			expectedError: true,
			errorContains: "metadata cannot be nil",
		},
		{
			name: "Invalid timestamp format",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "send-change-notification",
				CustomerCode: "customer-a",
				Timestamp:    "invalid-timestamp",
				RetryCount:   0,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "invalid timestamp format",
		},
		{
			name: "Invalid customer code format",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "send-change-notification",
				CustomerCode: "invalid@code",
				Timestamp:    time.Now().Format(time.RFC3339),
				RetryCount:   0,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "invalid customer code format",
		},
		{
			name: "Negative retry count",
			message: &SQSMessage{
				ExecutionID:  "test-id",
				ActionType:   "send-change-notification",
				CustomerCode: "customer-a",
				Timestamp:    time.Now().Format(time.RFC3339),
				RetryCount:   -1,
				Metadata:     &ApprovalRequestMetadata{},
			},
			expectedError: true,
			errorContains: "retry count cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sender.ValidateSQSMessage(tt.message)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSQSMessageSender_SendSQSMessage(t *testing.T) {
	// Create test configuration
	config := NewS3EventConfigManager("test-bucket")
	err := config.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
	if err != nil {
		t.Fatalf("Failed to add customer notification: %v", err)
	}

	sender := NewSQSMessageSender(config)

	// Create valid message
	metadata := &ApprovalRequestMetadata{
		GeneratedAt: time.Now().Format(time.RFC3339),
		GeneratedBy: "test",
	}

	message, err := sender.CreateSQSMessage("customer-a", "send-change-notification", metadata)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Test dry run
	err = sender.SendSQSMessage(message, true)
	if err != nil {
		t.Errorf("Dry run failed: %v", err)
	}

	// Test with invalid message
	invalidMessage := &SQSMessage{}
	err = sender.SendSQSMessage(invalidMessage, true)
	if err == nil {
		t.Errorf("Expected error for invalid message, but got none")
	}

	// Test with no config
	senderNoConfig := NewSQSMessageSender(nil)
	err = senderNoConfig.SendSQSMessage(message, true)
	if err == nil {
		t.Errorf("Expected error with no config, but got none")
	}
}

func TestSQSMessageSender_SendMultiCustomerMessages(t *testing.T) {
	// Create test configuration
	config := NewS3EventConfigManager("test-bucket")
	err := config.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
	if err != nil {
		t.Fatalf("Failed to add customer notification: %v", err)
	}
	err = config.AddCustomerNotification("customer-b", "arn:aws:sqs:us-east-1:123456789012:queue2")
	if err != nil {
		t.Fatalf("Failed to add customer notification: %v", err)
	}

	sender := NewSQSMessageSender(config)

	// Create metadata
	metadata := &ApprovalRequestMetadata{
		GeneratedAt: time.Now().Format(time.RFC3339),
		GeneratedBy: "test",
	}

	// Test successful multi-customer send
	customerCodes := []string{"customer-a", "customer-b"}
	results, err := sender.SendMultiCustomerMessages(customerCodes, "send-change-notification", metadata, true)
	if err != nil {
		t.Errorf("Multi-customer send failed: %v", err)
	}

	// Check results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	for customerCode, result := range results {
		if result != nil {
			t.Errorf("Expected success for %s, got error: %v", customerCode, result)
		}
	}

	// Test with invalid customer
	invalidCodes := []string{"customer-a", "non-existent"}
	results, err = sender.SendMultiCustomerMessages(invalidCodes, "send-change-notification", metadata, true)
	if err == nil {
		t.Errorf("Expected error for invalid customer, but got none")
	}

	// Check that customer-a succeeded but non-existent failed
	if results["customer-a"] != nil {
		t.Errorf("Expected success for customer-a, got error: %v", results["customer-a"])
	}
	if results["non-existent"] == nil {
		t.Errorf("Expected error for non-existent customer, but got none")
	}

	// Test with empty customer codes
	_, err = sender.SendMultiCustomerMessages([]string{}, "send-change-notification", metadata, true)
	if err == nil {
		t.Errorf("Expected error for empty customer codes, but got none")
	}
}

func TestSQSMessageSender_RetryFailedMessage(t *testing.T) {
	sender := NewSQSMessageSender(nil)

	// Create original message
	originalMessage := &SQSMessage{
		ExecutionID:  "original-id",
		ActionType:   "send-change-notification",
		CustomerCode: "customer-a",
		Timestamp:    time.Now().Format(time.RFC3339),
		RetryCount:   0,
		Metadata:     &ApprovalRequestMetadata{},
	}

	// Create retry message
	retryMessage, err := sender.RetryFailedMessage(originalMessage)
	if err != nil {
		t.Fatalf("Failed to create retry message: %v", err)
	}

	// Validate retry message
	if retryMessage.RetryCount != 1 {
		t.Errorf("Expected retry count 1, got %d", retryMessage.RetryCount)
	}
	if retryMessage.ActionType != originalMessage.ActionType {
		t.Errorf("Expected action type to match original")
	}
	if retryMessage.CustomerCode != originalMessage.CustomerCode {
		t.Errorf("Expected customer code to match original")
	}
	if retryMessage.Metadata != originalMessage.Metadata {
		t.Errorf("Expected metadata to match original")
	}
	if !strings.Contains(retryMessage.ExecutionID, "retry") {
		t.Errorf("Expected retry execution ID to contain 'retry', got: %s", retryMessage.ExecutionID)
	}

	// Test with nil message
	_, err = sender.RetryFailedMessage(nil)
	if err == nil {
		t.Errorf("Expected error for nil message, but got none")
	}
}

func TestSQSMessageSender_GenerateSQSMessageTemplate(t *testing.T) {
	sender := NewSQSMessageSender(nil)

	template, err := sender.GenerateSQSMessageTemplate()
	if err != nil {
		t.Fatalf("Failed to generate template: %v", err)
	}

	// Validate template is valid JSON
	var message SQSMessage
	err = json.Unmarshal([]byte(template), &message)
	if err != nil {
		t.Errorf("Generated template is not valid JSON: %v", err)
	}

	// Validate template contains expected fields
	expectedFields := []string{
		"execution_id",
		"action_type",
		"customer_code",
		"timestamp",
		"retry_count",
		"metadata",
	}

	for _, field := range expectedFields {
		if !strings.Contains(template, field) {
			t.Errorf("Template missing expected field: %s", field)
		}
	}

	// Validate message structure
	if message.CustomerCode != "customer-a" {
		t.Errorf("Expected customer code 'customer-a', got: %s", message.CustomerCode)
	}
	if message.ActionType != "send-change-notification" {
		t.Errorf("Expected action type 'send-change-notification', got: %s", message.ActionType)
	}
	if message.RetryCount != 0 {
		t.Errorf("Expected retry count 0, got: %d", message.RetryCount)
	}
}
