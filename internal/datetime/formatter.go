package datetime

import (
	"fmt"
	"time"
)

// Formatter handles formatting of time.Time values for different output contexts
type Formatter struct {
	config *DateTimeConfig
}

// NewFormatter creates a new Formatter with the given configuration
func NewFormatter(config *DateTimeConfig) *Formatter {
	if config == nil {
		config = DefaultConfig()
	}
	return &Formatter{config: config}
}

// ToRFC3339 formats a time.Time to the canonical RFC3339 format with timezone
func (f *Formatter) ToRFC3339(t time.Time) string {
	return t.Format(RFC3339Format)
}

// ToMicrosoftGraph formats a time.Time for Microsoft Graph API compatibility
// Graph API expects UTC time in a specific format without timezone offset
func (f *Formatter) ToMicrosoftGraph(t time.Time) string {
	// Convert to UTC and format without timezone info
	utc := t.UTC()
	return utc.Format(GraphFormat)
}

// ToHumanReadable formats a time.Time for human-readable display
func (f *Formatter) ToHumanReadable(t time.Time, timezone string) string {
	displayTime := t

	// Convert to specified timezone if provided
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			displayTime = t.In(loc)
		}
	}

	// Format as "January 15, 2025 at 10:00 AM EST"
	date := displayTime.Format(HumanDateFormat)
	timeStr := displayTime.Format(HumanTimeFormat)

	return fmt.Sprintf("%s at %s", date, timeStr)
}

// ToICS formats a time.Time for iCalendar (ICS) files
// ICS format requires UTC time in compact format
func (f *Formatter) ToICS(t time.Time) string {
	utc := t.UTC()
	return utc.Format(ICSFormat)
}

// ToLogFormat formats a time.Time for structured logging
// Uses UTC with millisecond precision
func (f *Formatter) ToLogFormat(t time.Time) string {
	utc := t.UTC()
	return utc.Format(LogFormat)
}

// ToDateOnly formats a time.Time to date-only string
func (f *Formatter) ToDateOnly(t time.Time) string {
	return t.Format("2006-01-02")
}

// ToTimeOnly formats a time.Time to time-only string in 24-hour format
func (f *Formatter) ToTimeOnly(t time.Time) string {
	return t.Format("15:04:05")
}

// ToTimeOnly12Hour formats a time.Time to time-only string in 12-hour format
func (f *Formatter) ToTimeOnly12Hour(t time.Time) string {
	return t.Format("3:04:05 PM")
}

// ToEmailTemplate formats a time.Time for email templates with timezone context
func (f *Formatter) ToEmailTemplate(t time.Time, timezone string) string {
	displayTime := t

	// Convert to specified timezone if provided, otherwise use default
	targetTimezone := timezone
	if targetTimezone == "" {
		targetTimezone = f.config.DefaultTimezone
	}

	if loc, err := time.LoadLocation(targetTimezone); err == nil {
		displayTime = t.In(loc)
	}

	// Format as "Monday, January 15, 2025 at 10:00 AM EST"
	return displayTime.Format("Monday, January 2, 2006 at 3:04 PM MST")
}

// ToScheduleWindow formats start and end times for schedule display
func (f *Formatter) ToScheduleWindow(start, end time.Time, timezone string) string {
	startFormatted := f.ToHumanReadable(start, timezone)
	endFormatted := f.ToHumanReadable(end, timezone)

	// If same date, show date once: "January 15, 2025 from 10:00 AM to 2:00 PM EST"
	if start.Year() == end.Year() && start.Month() == end.Month() && start.Day() == end.Day() {
		displayStart := start
		displayEnd := end

		if timezone != "" {
			if loc, err := time.LoadLocation(timezone); err == nil {
				displayStart = start.In(loc)
				displayEnd = end.In(loc)
			}
		}

		date := displayStart.Format(HumanDateFormat)
		startTime := displayStart.Format(HumanTimeFormat)
		endTime := displayEnd.Format(HumanTimeFormat)

		return fmt.Sprintf("%s from %s to %s", date, startTime, endTime)
	}

	// Different dates: show full format for both
	return fmt.Sprintf("%s to %s", startFormatted, endFormatted)
}

// ToTimezone converts a time.Time to a specific timezone and returns the formatted string
func (f *Formatter) ToTimezone(t time.Time, timezone string) (string, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", NewDateTimeError(
			ErrInvalidTimezone,
			fmt.Sprintf("invalid timezone: %s", timezone),
			timezone,
			err,
		)
	}

	converted := t.In(loc)
	return f.ToRFC3339(converted), nil
}

// FormatDuration formats a duration in a human-readable way
func (f *Formatter) FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}

	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if minutes == 0 {
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	if hours == 1 {
		if minutes == 1 {
			return "1 hour 1 minute"
		}
		return fmt.Sprintf("1 hour %d minutes", minutes)
	}

	if minutes == 1 {
		return fmt.Sprintf("%d hours 1 minute", hours)
	}

	return fmt.Sprintf("%d hours %d minutes", hours, minutes)
}
