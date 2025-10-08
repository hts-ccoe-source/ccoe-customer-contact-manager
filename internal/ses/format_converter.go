// Package ses provides format conversion utilities for handling different JSON formats.
package ses

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"ccoe-customer-contact-manager/internal/types"
)

// FlatChangeMetadata represents the flat JSON format from the frontend
type FlatChangeMetadata struct {
	ChangeTitle             string   `json:"changeTitle"`
	Customers               []string `json:"customers"`
	ChangeReason            string   `json:"changeReason"`
	ImplementationPlan      string   `json:"implementationPlan"`
	TestPlan                string   `json:"testPlan"`
	CustomerImpact          string   `json:"customerImpact"`
	RollbackPlan            string   `json:"rollbackPlan"`
	ImplementationBeginDate string   `json:"implementationBeginDate"`
	ImplementationBeginTime string   `json:"implementationBeginTime"`
	ImplementationEndDate   string   `json:"implementationEndDate"`
	ImplementationEndTime   string   `json:"implementationEndTime"`
	Timezone                string   `json:"timezone"`
	SnowTicket              string   `json:"snowTicket,omitempty"`
	JiraTicket              string   `json:"jiraTicket,omitempty"`
	MeetingRequired         string   `json:"meetingRequired,omitempty"`
	MeetingTitle            string   `json:"meetingTitle,omitempty"`
	MeetingDate             string   `json:"meetingDate,omitempty"`
	MeetingDuration         string   `json:"meetingDuration,omitempty"`
	MeetingLocation         string   `json:"meetingLocation,omitempty"`
	Description             string   `json:"description,omitempty"`
}

// LoadMetadataFromFile loads metadata from a JSON file and automatically detects the format
func LoadMetadataFromFile(filePath string) (*types.ApprovalRequestMetadata, error) {
	data, err := readFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Try to detect the format by attempting to unmarshal as nested format first
	var nestedMetadata types.ApprovalRequestMetadata
	if err := json.Unmarshal(data, &nestedMetadata); err == nil {
		// Check if it's actually the nested format by looking for required nested fields
		if nestedMetadata.ChangeMetadata.Title != "" || len(nestedMetadata.ChangeMetadata.CustomerNames) > 0 {
			return &nestedMetadata, nil
		}
	}

	// Try flat format
	var flatMetadata FlatChangeMetadata
	if err := json.Unmarshal(data, &flatMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata as either nested or flat format: %w", err)
	}

	// Convert flat to nested format
	converted, err := ConvertFlatToNested(&flatMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to convert flat format to nested: %w", err)
	}

	return converted, nil
}

// ConvertFlatToNested converts flat JSON format to nested ApprovalRequestMetadata format
func ConvertFlatToNested(flat *FlatChangeMetadata) (*types.ApprovalRequestMetadata, error) {
	if flat == nil {
		return nil, fmt.Errorf("flat metadata cannot be nil")
	}

	// Create the nested structure
	metadata := &types.ApprovalRequestMetadata{
		ChangeMetadata: struct {
			Title         string   `json:"changeTitle"`
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
			Description string `json:"description"`
			ApprovedBy  string `json:"approvedBy,omitempty"`
			ApprovedAt  string `json:"approvedAt,omitempty"`
		}{
			Title:                  flat.ChangeTitle,
			CustomerNames:          flat.Customers,
			CustomerCodes:          flat.Customers, // Use same as names for now
			ChangeReason:           flat.ChangeReason,
			ImplementationPlan:     flat.ImplementationPlan,
			TestPlan:               flat.TestPlan,
			ExpectedCustomerImpact: flat.CustomerImpact,
			RollbackPlan:           flat.RollbackPlan,
			Description:            flat.Description,
		},
		EmailNotification: struct {
			Subject         string   `json:"subject"`
			CustomerNames   []string `json:"customerNames"`
			CustomerCodes   []string `json:"customerCodes"`
			ScheduledWindow struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"scheduledWindow"`
			Tickets struct {
				Snow string `json:"snow"`
				Jira string `json:"jira"`
			} `json:"tickets"`
		}{
			Subject:       generateEmailSubject(flat),
			CustomerNames: flat.Customers,
			CustomerCodes: flat.Customers,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		GeneratedBy: "ccoe-customer-contact-manager",
	}

	// Set tickets
	metadata.ChangeMetadata.Tickets.ServiceNow = flat.SnowTicket
	metadata.ChangeMetadata.Tickets.Jira = flat.JiraTicket
	metadata.EmailNotification.Tickets.Snow = flat.SnowTicket
	metadata.EmailNotification.Tickets.Jira = flat.JiraTicket

	// Set schedule information
	metadata.ChangeMetadata.Schedule.BeginDate = flat.ImplementationBeginDate
	metadata.ChangeMetadata.Schedule.BeginTime = flat.ImplementationBeginTime
	metadata.ChangeMetadata.Schedule.EndDate = flat.ImplementationEndDate
	metadata.ChangeMetadata.Schedule.EndTime = flat.ImplementationEndTime
	metadata.ChangeMetadata.Schedule.Timezone = flat.Timezone

	// Generate implementation start/end timestamps
	if flat.ImplementationBeginDate != "" && flat.ImplementationBeginTime != "" {
		metadata.ChangeMetadata.Schedule.ImplementationStart = fmt.Sprintf("%sT%s",
			flat.ImplementationBeginDate, flat.ImplementationBeginTime)
		metadata.EmailNotification.ScheduledWindow.Start = metadata.ChangeMetadata.Schedule.ImplementationStart
	}

	if flat.ImplementationEndDate != "" && flat.ImplementationEndTime != "" {
		metadata.ChangeMetadata.Schedule.ImplementationEnd = fmt.Sprintf("%sT%s",
			flat.ImplementationEndDate, flat.ImplementationEndTime)
		metadata.EmailNotification.ScheduledWindow.End = metadata.ChangeMetadata.Schedule.ImplementationEnd
	}

	// Handle meeting information if present
	if flat.MeetingRequired == "yes" || flat.MeetingRequired == "true" || flat.MeetingTitle != "" {
		meetingInvite := &struct {
			Title           string   `json:"title"`
			StartTime       string   `json:"startTime"`
			Duration        int      `json:"duration"`
			DurationMinutes int      `json:"durationMinutes"`
			Attendees       []string `json:"attendees"`
			Location        string   `json:"location"`
		}{
			Title:    flat.MeetingTitle,
			Location: flat.MeetingLocation,
		}

		// Set meeting start time
		if flat.MeetingDate != "" {
			meetingInvite.StartTime = flat.MeetingDate
		} else if flat.ImplementationBeginDate != "" && flat.ImplementationBeginTime != "" {
			// Default to implementation start time
			meetingInvite.StartTime = fmt.Sprintf("%sT%s", flat.ImplementationBeginDate, flat.ImplementationBeginTime)
		}

		// Parse meeting duration
		if flat.MeetingDuration != "" {
			if duration, err := parseDurationMinutes(flat.MeetingDuration); err == nil {
				meetingInvite.Duration = duration
				meetingInvite.DurationMinutes = duration
			} else {
				// Default to 60 minutes if parsing fails
				meetingInvite.Duration = 60
				meetingInvite.DurationMinutes = 60
			}
		} else {
			// Default meeting duration
			meetingInvite.Duration = 60
			meetingInvite.DurationMinutes = 60
		}

		// Set default title if not provided
		if meetingInvite.Title == "" {
			meetingInvite.Title = fmt.Sprintf("Change Implementation Meeting: %s", flat.ChangeTitle)
		}

		metadata.MeetingInvite = meetingInvite
	}

	return metadata, nil
}

// generateEmailSubject creates an appropriate email subject from flat metadata
func generateEmailSubject(flat *FlatChangeMetadata) string {
	if flat.ChangeTitle == "" {
		return "ITSM Change Notification"
	}

	// Truncate title if too long for email subject
	title := flat.ChangeTitle
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	return fmt.Sprintf("ITSM Change Notification: %s", title)
}

// parseDurationMinutes parses duration string to minutes
func parseDurationMinutes(duration string) (int, error) {
	duration = strings.TrimSpace(strings.ToLower(duration))

	// Handle common formats
	if strings.HasSuffix(duration, "min") || strings.HasSuffix(duration, "minutes") {
		duration = strings.TrimSuffix(duration, "min")
		duration = strings.TrimSuffix(duration, "utes")
		duration = strings.TrimSpace(duration)
	} else if strings.HasSuffix(duration, "h") || strings.HasSuffix(duration, "hour") || strings.HasSuffix(duration, "hours") {
		duration = strings.TrimSuffix(duration, "h")
		duration = strings.TrimSuffix(duration, "our")
		duration = strings.TrimSuffix(duration, "ours")
		duration = strings.TrimSpace(duration)

		// Convert hours to minutes
		if hours := parseInt(duration); hours > 0 {
			return hours * 60, nil
		}
	}

	// Try to parse as plain number (assume minutes)
	if minutes := parseInt(duration); minutes > 0 {
		return minutes, nil
	}

	return 0, fmt.Errorf("unable to parse duration: %s", duration)
}

// parseInt safely parses a string to int
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	var result int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			return 0 // Invalid character
		}
	}
	return result
}

// readFileContent reads file content using os.ReadFile
func readFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}
