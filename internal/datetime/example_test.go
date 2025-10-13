package datetime_test

import (
	"fmt"
	"log"
	"time"

	"ccoe-customer-contact-manager/internal/datetime"
)

// Example demonstrates basic usage of the datetime package
func Example() {
	// Create a datetime manager with default configuration
	dt := datetime.New(nil)

	// Parse various input formats
	examples := []string{
		"2025-01-15T10:00:00-05:00",    // RFC3339 with timezone
		"2025-01-15 10:00 AM",          // Common format
		"01/15/2025 10:00",             // US date format
		"January 15, 2025 at 10:00 AM", // Human-readable
	}

	for _, input := range examples {
		parsed, err := dt.Parse(input)
		if err != nil {
			log.Printf("Failed to parse %s: %v", input, err)
			continue
		}

		// Format for different contexts
		fmt.Printf("Input: %s\n", input)
		fmt.Printf("  RFC3339: %s\n", dt.Format(parsed).ToRFC3339())
		fmt.Printf("  Microsoft Graph: %s\n", dt.Format(parsed).ToMicrosoftGraph())
		fmt.Printf("  Human Readable: %s\n", dt.Format(parsed).ToHumanReadable("America/New_York"))
		fmt.Printf("  Log Format: %s\n", dt.Format(parsed).ToLogFormat())
		fmt.Println()
	}
}

// ExampleLegacyParsing demonstrates parsing legacy date/time fields
func ExampleLegacyParsing() {
	dt := datetime.New(nil)

	// Parse combined date and time with timezone
	dateTime := "2025-01-15 10:00:00"
	timezone := "America/New_York"

	parsed, err := dt.ParseWithTimezone(dateTime, timezone)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Parsed date/time with timezone:\n")
	fmt.Printf("  Input: %s, Timezone: %s\n", dateTime, timezone)
	fmt.Printf("  Result: %s\n", dt.Format(parsed).ToRFC3339())
	fmt.Printf("  Human: %s\n", dt.Format(parsed).ToHumanReadable(timezone))
}

// ExampleValidation demonstrates date/time validation
func ExampleValidation() {
	dt := datetime.New(nil)

	// Parse a meeting time
	meetingTime, err := dt.Parse("2025-01-15 14:00")
	if err != nil {
		log.Fatal(err)
	}

	// Validate as meeting time
	if err := dt.Validate(meetingTime).MeetingTime(); err != nil {
		fmt.Printf("Invalid meeting time: %v\n", err)
	} else {
		fmt.Printf("Valid meeting time: %s\n", dt.Format(meetingTime).ToHumanReadable("America/New_York"))
	}

	// Validate business hours
	if err := dt.Validate(meetingTime).BusinessHours("America/New_York"); err != nil {
		fmt.Printf("Outside business hours: %v\n", err)
	} else {
		fmt.Printf("Within business hours\n")
	}

	// Validate date range
	endTime, _ := dt.Parse("2025-01-15 16:00")
	if err := dt.ValidateRange(meetingTime, endTime); err != nil {
		fmt.Printf("Invalid range: %v\n", err)
	} else {
		fmt.Printf("Valid range: %s\n", dt.FormatRange(meetingTime, endTime).ToScheduleWindow("America/New_York"))
	}
}

// ExampleMicrosoftGraphIntegration demonstrates formatting for Microsoft Graph API
func ExampleMicrosoftGraphIntegration() {
	dt := datetime.New(nil)

	// Parse meeting start time
	startTime, err := dt.Parse("2025-01-15 14:00")
	if err != nil {
		log.Fatal(err)
	}

	// Calculate end time (1 hour meeting)
	endTime := startTime.Add(1 * time.Hour)

	// Format for Microsoft Graph API
	fmt.Printf("Microsoft Graph API format:\n")
	fmt.Printf("  Start: %s\n", dt.Format(startTime).ToMicrosoftGraph())
	fmt.Printf("  End: %s\n", dt.Format(endTime).ToMicrosoftGraph())

	// Format for human display in email
	fmt.Printf("\nEmail template format:\n")
	fmt.Printf("  Meeting: %s\n", dt.Format(startTime).ToEmailTemplate("America/New_York"))
	fmt.Printf("  Duration: %s\n", dt.FormatRange(startTime, endTime).ToScheduleWindow("America/New_York"))
}

// ExampleTimezoneHandling demonstrates timezone conversion and handling
func ExampleTimezoneHandling() {
	dt := datetime.New(nil)

	// Parse time in one timezone
	eastCoast, err := dt.ParseWithTimezone("2025-01-15 14:00", "America/New_York")
	if err != nil {
		log.Fatal(err)
	}

	// Convert to different timezones
	timezones := []string{
		"America/New_York",
		"America/Chicago",
		"America/Denver",
		"America/Los_Angeles",
		"UTC",
	}

	fmt.Printf("Meeting time across timezones:\n")
	for _, tz := range timezones {
		converted, err := dt.Format(eastCoast).ToTimezone(tz)
		if err != nil {
			log.Printf("Error converting to %s: %v", tz, err)
			continue
		}

		// Parse back to get timezone-aware time for human formatting
		tzTime, _ := dt.ParseWithTimezone(converted, tz)
		human := dt.Format(tzTime).ToHumanReadable(tz)

		fmt.Printf("  %s: %s\n", tz, human)
	}
}
