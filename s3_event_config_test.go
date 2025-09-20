package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestS3EventConfigManager_AddCustomerNotification(t *testing.T) {
	tests := []struct {
		name          string
		customerCode  string
		sqsQueueArn   string
		expectedError bool
		errorContains string
	}{
		{
			name:          "Valid customer notification",
			customerCode:  "customer-a",
			sqsQueueArn:   "arn:aws:sqs:us-east-1:123456789012:customer-a-change-queue",
			expectedError: false,
		},
		{
			name:          "Empty customer code",
			customerCode:  "",
			sqsQueueArn:   "arn:aws:sqs:us-east-1:123456789012:test-queue",
			expectedError: true,
			errorContains: "customer code cannot be empty",
		},
		{
			name:          "Empty SQS queue ARN",
			customerCode:  "customer-a",
			sqsQueueArn:   "",
			expectedError: true,
			errorContains: "SQS queue ARN cannot be empty",
		},
		{
			name:          "Invalid customer code format",
			customerCode:  "customer@invalid",
			sqsQueueArn:   "arn:aws:sqs:us-east-1:123456789012:test-queue",
			expectedError: true,
			errorContains: "invalid customer code format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewS3EventConfigManager("test-bucket")
			err := manager.AddCustomerNotification(tt.customerCode, tt.sqsQueueArn)

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

				// Verify the notification was added correctly
				notification, err := manager.GetCustomerNotification(tt.customerCode)
				if err != nil {
					t.Errorf("Failed to get added notification: %v", err)
					return
				}

				if notification.CustomerCode != tt.customerCode {
					t.Errorf("Expected customer code %s, got %s", tt.customerCode, notification.CustomerCode)
				}
				if notification.SQSQueueArn != tt.sqsQueueArn {
					t.Errorf("Expected SQS ARN %s, got %s", tt.sqsQueueArn, notification.SQSQueueArn)
				}

				expectedPrefix := "customers/" + tt.customerCode + "/"
				if notification.Prefix != expectedPrefix {
					t.Errorf("Expected prefix %s, got %s", expectedPrefix, notification.Prefix)
				}
				if notification.Suffix != ".json" {
					t.Errorf("Expected suffix .json, got %s", notification.Suffix)
				}
			}
		})
	}
}

func TestS3EventConfigManager_DuplicateCustomer(t *testing.T) {
	manager := NewS3EventConfigManager("test-bucket")

	// Add first notification
	err := manager.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
	if err != nil {
		t.Fatalf("Failed to add first notification: %v", err)
	}

	// Try to add duplicate
	err = manager.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue2")
	if err == nil {
		t.Errorf("Expected error for duplicate customer, but got none")
	}
	if !strings.Contains(err.Error(), "customer notification already exists") {
		t.Errorf("Expected duplicate error, got: %s", err.Error())
	}
}

func TestS3EventConfigManager_RemoveCustomerNotification(t *testing.T) {
	manager := NewS3EventConfigManager("test-bucket")

	// Add notification
	err := manager.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}

	// Remove notification
	err = manager.RemoveCustomerNotification("customer-a")
	if err != nil {
		t.Errorf("Failed to remove notification: %v", err)
	}

	// Verify it's gone
	_, err = manager.GetCustomerNotification("customer-a")
	if err == nil {
		t.Errorf("Expected error when getting removed notification, but got none")
	}

	// Try to remove non-existent
	err = manager.RemoveCustomerNotification("non-existent")
	if err == nil {
		t.Errorf("Expected error when removing non-existent notification, but got none")
	}
}

func TestS3EventConfigManager_ValidateConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*S3EventConfigManager)
		expectedError bool
		errorContains string
	}{
		{
			name: "Valid configuration",
			setupFunc: func(m *S3EventConfigManager) {
				m.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
			},
			expectedError: false,
		},
		{
			name: "Empty bucket name",
			setupFunc: func(m *S3EventConfigManager) {
				m.Config.BucketName = ""
				m.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
			},
			expectedError: true,
			errorContains: "bucket name cannot be empty",
		},
		{
			name: "No customer notifications",
			setupFunc: func(m *S3EventConfigManager) {
				// Don't add any notifications
			},
			expectedError: true,
			errorContains: "no customer notifications configured",
		},
		{
			name: "Invalid SQS ARN format",
			setupFunc: func(m *S3EventConfigManager) {
				m.Config.CustomerNotifications = append(m.Config.CustomerNotifications, CustomerNotificationConfig{
					CustomerCode: "customer-a",
					SQSQueueArn:  "invalid-arn",
					Prefix:       "customers/customer-a/",
					Suffix:       ".json",
				})
			},
			expectedError: true,
			errorContains: "invalid SQS queue ARN format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewS3EventConfigManager("test-bucket")
			tt.setupFunc(manager)

			err := manager.ValidateConfiguration()

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

func TestS3EventConfigManager_SaveAndLoadConfiguration(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "s3_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set CONFIG_PATH to temp directory
	os.Setenv("CONFIG_PATH", tmpDir+"/")
	defer os.Unsetenv("CONFIG_PATH")

	// Create and configure manager
	manager1 := NewS3EventConfigManager("test-bucket")
	err = manager1.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:queue1")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}
	err = manager1.AddCustomerNotification("customer-b", "arn:aws:sqs:us-east-1:123456789012:queue2")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}

	// Save configuration
	filename := "test-s3-config.json"
	err = manager1.SaveConfiguration(filename)
	if err != nil {
		t.Fatalf("Failed to save configuration: %v", err)
	}

	// Load configuration into new manager
	manager2 := NewS3EventConfigManager("")
	err = manager2.LoadConfiguration(filename)
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Compare configurations
	if !reflect.DeepEqual(manager1.Config, manager2.Config) {
		t.Errorf("Loaded configuration doesn't match saved configuration")
		t.Errorf("Original: %+v", manager1.Config)
		t.Errorf("Loaded: %+v", manager2.Config)
	}
}

func TestS3EventConfigManager_GenerateTerraformConfig(t *testing.T) {
	manager := NewS3EventConfigManager("my-metadata-bucket")

	err := manager.AddCustomerNotification("customer-a", "arn:aws:sqs:us-east-1:123456789012:customer-a-queue")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}

	err = manager.AddCustomerNotification("customer-b", "arn:aws:sqs:us-east-1:123456789012:customer-b-queue")
	if err != nil {
		t.Fatalf("Failed to add notification: %v", err)
	}

	terraformConfig, err := manager.GenerateTerraformConfig()
	if err != nil {
		t.Fatalf("Failed to generate Terraform config: %v", err)
	}

	// Check that the config contains expected elements
	expectedElements := []string{
		"resource \"aws_s3_bucket_notification\"",
		"my_metadata_bucket_notifications",
		"bucket = \"my-metadata-bucket\"",
		"arn:aws:sqs:us-east-1:123456789012:customer-a-queue",
		"arn:aws:sqs:us-east-1:123456789012:customer-b-queue",
		"customers/customer-a/",
		"customers/customer-b/",
		"filter_suffix = \".json\"",
		"events    = [\"s3:ObjectCreated:*\"]",
	}

	for _, element := range expectedElements {
		if !strings.Contains(terraformConfig, element) {
			t.Errorf("Generated Terraform config missing expected element: %s", element)
		}
	}

	// Verify it's valid Terraform syntax (basic check)
	if !strings.Contains(terraformConfig, "resource \"aws_s3_bucket_notification\"") {
		t.Errorf("Generated config doesn't appear to be valid Terraform")
	}
}

func TestLoadS3EventConfigFromCustomerCodes(t *testing.T) {
	bucketName := "test-bucket"
	customerSQSMapping := map[string]string{
		"customer-a": "arn:aws:sqs:us-east-1:123456789012:customer-a-queue",
		"customer-b": "arn:aws:sqs:us-east-1:123456789012:customer-b-queue",
		"customer-c": "arn:aws:sqs:us-east-1:123456789012:customer-c-queue",
	}

	manager, err := LoadS3EventConfigFromCustomerCodes(bucketName, customerSQSMapping)
	if err != nil {
		t.Fatalf("Failed to load S3 event config: %v", err)
	}

	// Verify bucket name
	if manager.Config.BucketName != bucketName {
		t.Errorf("Expected bucket name %s, got %s", bucketName, manager.Config.BucketName)
	}

	// Verify all customers were added
	if len(manager.Config.CustomerNotifications) != len(customerSQSMapping) {
		t.Errorf("Expected %d notifications, got %d", len(customerSQSMapping), len(manager.Config.CustomerNotifications))
	}

	// Verify each customer notification
	for customerCode, expectedArn := range customerSQSMapping {
		notification, err := manager.GetCustomerNotification(customerCode)
		if err != nil {
			t.Errorf("Failed to get notification for %s: %v", customerCode, err)
			continue
		}

		if notification.SQSQueueArn != expectedArn {
			t.Errorf("Expected ARN %s for %s, got %s", expectedArn, customerCode, notification.SQSQueueArn)
		}

		expectedPrefix := "customers/" + customerCode + "/"
		if notification.Prefix != expectedPrefix {
			t.Errorf("Expected prefix %s for %s, got %s", expectedPrefix, customerCode, notification.Prefix)
		}
	}

	// Test with invalid customer code
	invalidMapping := map[string]string{
		"invalid@code": "arn:aws:sqs:us-east-1:123456789012:queue",
	}

	_, err = LoadS3EventConfigFromCustomerCodes(bucketName, invalidMapping)
	if err == nil {
		t.Errorf("Expected error for invalid customer code, but got none")
	}
}
