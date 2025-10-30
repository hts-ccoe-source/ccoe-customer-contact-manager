package config

import (
	"testing"
)

// TestLoadActualConfigFile tests loading the actual config.json file from the project root
func TestLoadActualConfigFile(t *testing.T) {
	// Try to load the actual config.json file
	cfg, err := LoadConfig("../../config.json")
	if err != nil {
		t.Fatalf("Failed to load config.json: %v", err)
	}

	// Validate the loaded configuration
	err = ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Verify email config fields are present
	if cfg.EmailConfig.SenderAddress == "" {
		t.Error("EmailConfig.SenderAddress is empty")
	}
	if cfg.EmailConfig.MeetingOrganizer == "" {
		t.Error("EmailConfig.MeetingOrganizer is empty")
	}
	if cfg.EmailConfig.PortalBaseURL == "" {
		t.Error("EmailConfig.PortalBaseURL is empty")
	}

	t.Logf(" Successfully loaded and validated config.json")
	t.Logf("   Sender Address: %s", cfg.EmailConfig.SenderAddress)
	t.Logf("   Meeting Organizer: %s", cfg.EmailConfig.MeetingOrganizer)
	t.Logf("   Portal Base URL: %s", cfg.EmailConfig.PortalBaseURL)
}
