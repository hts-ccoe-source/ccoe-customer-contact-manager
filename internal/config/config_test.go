package config

import (
	"testing"

	"ccoe-customer-contact-manager/internal/types"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with subdomain", "user@mail.example.com", true},
		{"empty email", "", false},
		{"no @ symbol", "testexample.com", false},
		{"no domain", "test@", false},
		{"no local part", "@example.com", false},
		{"no dot in domain", "test@example", false},
		{"multiple @ symbols", "test@@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidEmail(tt.email); got != tt.want {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid https URL", "https://portal.example.com", true},
		{"valid http URL", "http://portal.example.com", true},
		{"valid URL with path", "https://portal.example.com/path", true},
		{"empty URL", "", false},
		{"no protocol", "portal.example.com", false},
		{"invalid protocol", "ftp://portal.example.com", false},
		{"protocol only", "https://", false},
		{"protocol only http", "http://", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidURL(tt.url); got != tt.want {
				t.Errorf("isValidURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestValidateEmailConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid email config",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					SenderAddress:    "ccoe@nonprod.ccoe.hearst.com",
					MeetingOrganizer: "ccoe@hearst.com",
					PortalBaseURL:    "https://portal.example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "missing sender address",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					MeetingOrganizer: "ccoe@hearst.com",
					PortalBaseURL:    "https://portal.example.com",
				},
			},
			wantErr: true,
			errMsg:  "email_config.sender_address is required",
		},
		{
			name: "missing meeting organizer",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					SenderAddress: "ccoe@nonprod.ccoe.hearst.com",
					PortalBaseURL: "https://portal.example.com",
				},
			},
			wantErr: true,
			errMsg:  "email_config.meeting_organizer is required",
		},
		{
			name: "missing portal base URL",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					SenderAddress:    "ccoe@nonprod.ccoe.hearst.com",
					MeetingOrganizer: "ccoe@hearst.com",
				},
			},
			wantErr: true,
			errMsg:  "email_config.portal_base_url is required",
		},
		{
			name: "invalid sender address format",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					SenderAddress:    "invalid-email",
					MeetingOrganizer: "ccoe@hearst.com",
					PortalBaseURL:    "https://portal.example.com",
				},
			},
			wantErr: true,
			errMsg:  "invalid sender_address format",
		},
		{
			name: "invalid meeting organizer format",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					SenderAddress:    "ccoe@nonprod.ccoe.hearst.com",
					MeetingOrganizer: "invalid-email",
					PortalBaseURL:    "https://portal.example.com",
				},
			},
			wantErr: true,
			errMsg:  "invalid meeting_organizer format",
		},
		{
			name: "invalid portal base URL format",
			config: &types.Config{
				EmailConfig: types.EmailConfig{
					SenderAddress:    "ccoe@nonprod.ccoe.hearst.com",
					MeetingOrganizer: "ccoe@hearst.com",
					PortalBaseURL:    "not-a-url",
				},
			},
			wantErr: true,
			errMsg:  "invalid portal_base_url format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmailConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmailConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateEmailConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &types.Config{
				AWSRegion: "us-east-1",
				CustomerMappings: map[string]types.CustomerAccountInfo{
					"test": {
						CustomerCode: "test",
						SESRoleARN:   "arn:aws:iam::123456789012:role/TestRole",
					},
				},
				EmailConfig: types.EmailConfig{
					SenderAddress:    "ccoe@nonprod.ccoe.hearst.com",
					MeetingOrganizer: "ccoe@hearst.com",
					PortalBaseURL:    "https://portal.example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "missing email config",
			config: &types.Config{
				AWSRegion: "us-east-1",
				CustomerMappings: map[string]types.CustomerAccountInfo{
					"test": {
						CustomerCode: "test",
						SESRoleARN:   "arn:aws:iam::123456789012:role/TestRole",
					},
				},
			},
			wantErr: true,
			errMsg:  "email_config.sender_address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
