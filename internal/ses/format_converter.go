// Package ses provides format conversion utilities for handling different JSON formats.
package ses

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

// LoadMetadataFromFile loads metadata from a JSON file as ApprovalRequestMetadata for backward compatibility
func LoadMetadataFromFile(filePath string) (*types.ApprovalRequestMetadata, error) {
	data, err := readFileContent(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Try to parse as flat ChangeMetadata first
	var flatMetadata types.ChangeMetadata
	if err := json.Unmarshal(data, &flatMetadata); err == nil && flatMetadata.ChangeTitle != "" {
		// Convert flat to nested for backward compatibility
		return convertFlatToNested(&flatMetadata), nil
	}

	// Try to parse as nested ApprovalRequestMetadata
	var nestedMetadata types.ApprovalRequestMetadata
	if err := json.Unmarshal(data, &nestedMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata as either ChangeMetadata or ApprovalRequestMetadata: %w", err)
	}

	return &nestedMetadata, nil
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

// convertFlatToNested converts flat ChangeMetadata to nested ApprovalRequestMetadata for backward compatibility
func convertFlatToNested(flat *types.ChangeMetadata) *types.ApprovalRequestMetadata {
	// Helper function to get customer names from codes
	getCustomerNames := func(codes []string) []string {
		customerMapping := map[string]string{
			"hts":         "HTS Prod",
			"htsnonprod":  "HTS NonProd",
			"cds":         "CDS Global",
			"fdbus":       "FDBUS",
			"hmiit":       "Hearst Magazines Italy",
			"hmies":       "Hearst Magazines Spain",
			"htvdigital":  "HTV Digital",
			"htv":         "HTV",
			"icx":         "iCrossing",
			"motor":       "Motor",
			"bat":         "Bring A Trailer",
			"mhk":         "MHK",
			"hdmautos":    "Autos",
			"hnpit":       "HNP IT",
			"hnpdigital":  "HNP Digital",
			"camp":        "CAMP Systems",
			"mcg":         "MCG",
			"hmuk":        "Hearst Magazines UK",
			"hmusdigital": "Hearst Magazines Digital",
			"hwp":         "Hearst Western Properties",
			"zynx":        "Zynx",
			"hchb":        "HCHB",
			"fdbuk":       "FDBUK",
			"hecom":       "Hearst ECommerce",
			"blkbook":     "Black Book",
		}

		var names []string
		for _, code := range codes {
			if name, exists := customerMapping[code]; exists {
				names = append(names, name)
			} else {
				names = append(names, code)
			}
		}
		return names
	}

	nested := &types.ApprovalRequestMetadata{
		GeneratedAt: flat.CreatedAt,
		GeneratedBy: flat.CreatedBy,
	}

	// Populate ChangeMetadata
	nested.ChangeMetadata.Title = flat.ChangeTitle
	nested.ChangeMetadata.CustomerNames = getCustomerNames(flat.Customers)
	nested.ChangeMetadata.CustomerCodes = flat.Customers
	nested.ChangeMetadata.ChangeReason = flat.ChangeReason
	nested.ChangeMetadata.ImplementationPlan = flat.ImplementationPlan
	nested.ChangeMetadata.TestPlan = flat.TestPlan
	nested.ChangeMetadata.ExpectedCustomerImpact = flat.CustomerImpact
	nested.ChangeMetadata.RollbackPlan = flat.RollbackPlan
	nested.ChangeMetadata.Description = flat.ChangeReason
	nested.ChangeMetadata.ApprovedBy = flat.ApprovedBy
	nested.ChangeMetadata.ApprovedAt = flat.ApprovedAt

	// Populate tickets
	nested.ChangeMetadata.Tickets.ServiceNow = flat.SnowTicket
	nested.ChangeMetadata.Tickets.Jira = flat.JiraTicket

	// Populate schedule
	nested.ChangeMetadata.Schedule.BeginDate = flat.ImplementationBeginDate
	nested.ChangeMetadata.Schedule.BeginTime = flat.ImplementationBeginTime
	nested.ChangeMetadata.Schedule.EndDate = flat.ImplementationEndDate
	nested.ChangeMetadata.Schedule.EndTime = flat.ImplementationEndTime
	nested.ChangeMetadata.Schedule.Timezone = flat.Timezone
	nested.ChangeMetadata.Schedule.ImplementationStart = fmt.Sprintf("%sT%s", flat.ImplementationBeginDate, flat.ImplementationBeginTime)
	nested.ChangeMetadata.Schedule.ImplementationEnd = fmt.Sprintf("%sT%s", flat.ImplementationEndDate, flat.ImplementationEndTime)

	// Populate EmailNotification
	nested.EmailNotification.Subject = fmt.Sprintf("ITSM Change Notification: %s", flat.ChangeTitle)
	nested.EmailNotification.CustomerNames = getCustomerNames(flat.Customers)
	nested.EmailNotification.CustomerCodes = flat.Customers
	nested.EmailNotification.ScheduledWindow.Start = nested.ChangeMetadata.Schedule.ImplementationStart
	nested.EmailNotification.ScheduledWindow.End = nested.ChangeMetadata.Schedule.ImplementationEnd
	nested.EmailNotification.Tickets.Snow = flat.SnowTicket
	nested.EmailNotification.Tickets.Jira = flat.JiraTicket

	return nested
}
