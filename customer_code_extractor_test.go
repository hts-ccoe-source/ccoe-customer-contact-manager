package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCustomerCodeExtractor_ExtractCustomerCodes(t *testing.T) {
	tests := []struct {
		name          string
		metadata      *ApprovalRequestMetadata
		validCodes    []string
		expectedCodes []string
		expectedError bool
		errorContains string
	}{
		{
			name: "Valid single customer code",
			metadata: &ApprovalRequestMetadata{
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
					CustomerCodes: []string{"customer-a"},
				},
			},
			validCodes:    []string{"customer-a", "customer-b"},
			expectedCodes: []string{"customer-a"},
			expectedError: false,
		},
		{
			name: "Valid multiple customer codes",
			metadata: &ApprovalRequestMetadata{
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
					CustomerCodes: []string{"customer-a", "customer-b", "customer-c"},
				},
			},
			validCodes:    []string{"customer-a", "customer-b", "customer-c"},
			expectedCodes: []string{"customer-a", "customer-b", "customer-c"},
			expectedError: false,
		},
		{
			name: "Duplicate customer codes removed",
			metadata: &ApprovalRequestMetadata{
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
					CustomerCodes: []string{"customer-a", "customer-a", "customer-b"},
				},
			},
			validCodes:    []string{"customer-a", "customer-b"},
			expectedCodes: []string{"customer-a", "customer-b"},
			expectedError: false,
		},
		{
			name: "Invalid customer code format",
			metadata: &ApprovalRequestMetadata{
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
					CustomerCodes: []string{"customer@invalid", "customer-a"},
				},
			},
			validCodes:    []string{"customer-a"},
			expectedCodes: []string{"customer-a"},
			expectedError: true,
			errorContains: "invalid customer codes found",
		},
		{
			name: "No customer codes in metadata",
			metadata: &ApprovalRequestMetadata{
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
					CustomerCodes: []string{},
				},
			},
			validCodes:    []string{"customer-a"},
			expectedCodes: nil,
			expectedError: true,
			errorContains: "no customer codes found",
		},
		{
			name:          "Nil metadata",
			metadata:      nil,
			validCodes:    []string{"customer-a"},
			expectedCodes: nil,
			expectedError: true,
			errorContains: "metadata cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewCustomerCodeExtractor(tt.validCodes)
			codes, err := extractor.ExtractCustomerCodes(tt.metadata)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
			}

			if !reflect.DeepEqual(codes, tt.expectedCodes) {
				t.Errorf("Expected codes %v, got %v", tt.expectedCodes, codes)
			}
		})
	}
}

func TestIsValidCustomerCodeFormat(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"customer-a", true},
		{"customer-123", true},
		{"abc", true},
		{"a1", true},
		{"customer-a-b", true},
		{"", false},
		{"a", false},
		{"customer@invalid", false},
		{"customer_invalid", false},
		{"-customer", false},
		{"customer-", false},
		{"customer--a", true}, // double hyphen is allowed
		{"this-is-a-very-long-customer-code-that-exceeds-limit", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := isValidCustomerCodeFormat(tt.code)
			if result != tt.expected {
				t.Errorf("isValidCustomerCodeFormat(%s) = %v, expected %v", tt.code, result, tt.expected)
			}
		})
	}
}

func TestRemoveDuplicateStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "No duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "With duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "All same",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDuplicateStrings(tt.input)
			// Handle nil vs empty slice comparison
			if len(result) == 0 && len(tt.expected) == 0 {
				return // Both are effectively empty
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("removeDuplicateStrings(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}

// Test helper to create test configuration files
func createTestCustomerCodesFile(t *testing.T, codes []string) string {
	data, err := json.Marshal(codes)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "CustomerCodes*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

func TestLoadValidCustomerCodes(t *testing.T) {
	// Test with CustomerCodes.json
	t.Run("Load from CustomerCodes.json", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "test_config")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create CustomerCodes.json
		testCodes := []string{"customer-a", "customer-b", "customer-c"}
		customerCodesPath := tmpDir + "/CustomerCodes.json"
		data, err := json.Marshal(testCodes)
		if err != nil {
			t.Fatalf("Failed to marshal test data: %v", err)
		}

		err = os.WriteFile(customerCodesPath, data, 0644)
		if err != nil {
			t.Fatalf("Failed to write CustomerCodes.json: %v", err)
		}

		// Set CONFIG_PATH to temp directory
		os.Setenv("CONFIG_PATH", tmpDir+"/")
		defer os.Unsetenv("CONFIG_PATH")

		codes, err := LoadValidCustomerCodes()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !reflect.DeepEqual(codes, testCodes) {
			t.Errorf("Expected codes %v, got %v", testCodes, codes)
		}
	})
}
