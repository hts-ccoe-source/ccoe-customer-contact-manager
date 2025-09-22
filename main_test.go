package main

import (
	"testing"
)

func TestValidateCustomerCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid code", "hts", false},
		{"valid with numbers", "customer-123", false},
		{"valid with hyphens", "multi-customer-code", false},
		{"empty code", "", true},
		{"uppercase", "HTS", true},
		{"with underscore", "customer_code", true},
		{"with space", "customer code", true},
		{"too long", "this-is-a-very-long-customer-code-that-exceeds-the-fifty-character-limit", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCustomerCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCustomerCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid with subdomain", "user@mail.example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"empty email", "", true},
		{"no @", "testexample.com", true},
		{"no domain", "test@", true},
		{"no local part", "@example.com", true},
		{"invalid format", "test@", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !Contains(slice, "banana") {
		t.Error("Contains() should return true for existing item")
	}

	if Contains(slice, "grape") {
		t.Error("Contains() should return false for non-existing item")
	}
}

func TestRemoveDuplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	expected := []string{"a", "b", "c", "d"}

	result := RemoveDuplicates(input)

	if len(result) != len(expected) {
		t.Errorf("RemoveDuplicates() length = %d, want %d", len(result), len(expected))
	}

	for i, v := range expected {
		if i >= len(result) || result[i] != v {
			t.Errorf("RemoveDuplicates() = %v, want %v", result, expected)
			break
		}
	}
}

func TestLoadConfig(t *testing.T) {
	// Test default config
	config, err := LoadConfig("")
	if err != nil {
		t.Errorf("LoadConfig() with empty path should return default config, got error: %v", err)
	}

	if config.AWSRegion != "us-east-1" {
		t.Errorf("Default config AWSRegion = %s, want us-east-1", config.AWSRegion)
	}

	if len(config.CustomerMappings) == 0 {
		t.Error("Default config should have at least one customer mapping")
	}
}

func TestValidateConfig(t *testing.T) {
	// Test valid config
	config := &Config{
		AWSRegion: "us-east-1",
		CustomerMappings: map[string]CustomerAccountInfo{
			"test": {
				CustomerCode: "test",
				AWSAccountID: "123456789012",
				SESRoleARN:   "arn:aws:iam::123456789012:role/TestRole",
			},
		},
	}

	if err := ValidateConfig(config); err != nil {
		t.Errorf("ValidateConfig() with valid config should not return error, got: %v", err)
	}

	// Test invalid config - empty region
	config.AWSRegion = ""
	if err := ValidateConfig(config); err == nil {
		t.Error("ValidateConfig() with empty region should return error")
	}
}
