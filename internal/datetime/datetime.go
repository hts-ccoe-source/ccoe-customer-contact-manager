// Package datetime provides standardized date/time handling utilities
// for consistent parsing, formatting, and validation across all components.
//
// This package implements a unified approach to date/time operations that works
// consistently across Go Lambda backend, Node.js frontend API, and Node.js edge
// authorizer components.
//
// Key features:
//   - Standardized RFC3339 internal format with timezone support
//   - Multiple input format parsing (ISO8601, common date formats, etc.)
//   - Context-aware output formatting (Microsoft Graph, human-readable, etc.)
//   - Comprehensive validation with configurable business rules

// Example usage:
//
//	// Create a datetime manager with default config
//	dt := datetime.New(nil)
//
//	// Parse various input formats
//	parsed, err := dt.Parse("2025-01-15 10:00 AM")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Format for different contexts
//	rfc3339 := dt.Format(parsed).ToRFC3339()
//	graphAPI := dt.Format(parsed).ToMicrosoftGraph()
//	human := dt.Format(parsed).ToHumanReadable("America/New_York")
//
//	// Validate business rules
//	if err := dt.Validate(parsed).MeetingTime(); err != nil {
//		log.Printf("Invalid meeting time: %v", err)
//	}
package datetime

import "time"

// Manager provides a unified interface to all date/time operations
type Manager struct {
	parser    *Parser
	formatter *Formatter
	validator *Validator
	config    *DateTimeConfig
}

// New creates a new datetime Manager with the given configuration
// If config is nil, uses DefaultConfig()
func New(config *DateTimeConfig) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		parser:    NewParser(config),
		formatter: NewFormatter(config),
		validator: NewValidator(config),
		config:    config,
	}
}

// Parse returns a parser interface for the manager
func (m *Manager) Parse(input string) (time.Time, error) {
	return m.parser.ParseDateTime(input)
}

// ParseDate parses a date-only string
func (m *Manager) ParseDate(input string) (time.Time, error) {
	return m.parser.ParseDate(input)
}

// ParseTime parses a time-only string and combines with the given date
func (m *Manager) ParseTime(input string, date time.Time) (time.Time, error) {
	return m.parser.ParseTime(input, date)
}

// ParseWithTimezone parses date/time and applies the specified timezone
func (m *Manager) ParseWithTimezone(input, timezone string) (time.Time, error) {
	return m.parser.ParseDateTimeWithTimezone(input, timezone)
}

// Format returns a formatter interface for the given time
func (m *Manager) Format(t time.Time) *TimeFormatter {
	return &TimeFormatter{
		time:      t,
		formatter: m.formatter,
	}
}

// Validate returns a validator interface for the given time
func (m *Manager) Validate(t time.Time) *TimeValidator {
	return &TimeValidator{
		time:      t,
		validator: m.validator,
	}
}

// ValidateTimezone validates a timezone string
func (m *Manager) ValidateTimezone(tz string) error {
	return m.validator.ValidateTimezone(tz)
}

// ValidateRange validates a date/time range
func (m *Manager) ValidateRange(start, end time.Time) error {
	return m.validator.ValidateDateRange(start, end)
}

// Config returns the current configuration
func (m *Manager) Config() *DateTimeConfig {
	return m.config
}

// TimeFormatter provides formatting methods for a specific time
type TimeFormatter struct {
	time      time.Time
	formatter *Formatter
}

// ToRFC3339 formats to the canonical RFC3339 format
func (tf *TimeFormatter) ToRFC3339() string {
	return tf.formatter.ToRFC3339(tf.time)
}

// ToMicrosoftGraph formats for Microsoft Graph API
func (tf *TimeFormatter) ToMicrosoftGraph() string {
	return tf.formatter.ToMicrosoftGraph(tf.time)
}

// ToHumanReadable formats for human display
func (tf *TimeFormatter) ToHumanReadable(timezone string) string {
	return tf.formatter.ToHumanReadable(tf.time, timezone)
}

// ToICS formats for iCalendar files
func (tf *TimeFormatter) ToICS() string {
	return tf.formatter.ToICS(tf.time)
}

// ToLogFormat formats for structured logging
func (tf *TimeFormatter) ToLogFormat() string {
	return tf.formatter.ToLogFormat(tf.time)
}

// ToEmailTemplate formats for email templates
func (tf *TimeFormatter) ToEmailTemplate(timezone string) string {
	return tf.formatter.ToEmailTemplate(tf.time, timezone)
}

// ToDateOnly formats to date-only string
func (tf *TimeFormatter) ToDateOnly() string {
	return tf.formatter.ToDateOnly(tf.time)
}

// ToTimeOnly formats to time-only string (24-hour)
func (tf *TimeFormatter) ToTimeOnly() string {
	return tf.formatter.ToTimeOnly(tf.time)
}

// ToTimeOnly12Hour formats to time-only string (12-hour)
func (tf *TimeFormatter) ToTimeOnly12Hour() string {
	return tf.formatter.ToTimeOnly12Hour(tf.time)
}

// ToTimezone converts to a specific timezone
func (tf *TimeFormatter) ToTimezone(timezone string) (string, error) {
	return tf.formatter.ToTimezone(tf.time, timezone)
}

// TimeValidator provides validation methods for a specific time
type TimeValidator struct {
	time      time.Time
	validator *Validator
}

// DateTime performs basic date/time validation
func (tv *TimeValidator) DateTime() error {
	return tv.validator.ValidateDateTime(tv.time)
}

// MeetingTime validates as a meeting time
func (tv *TimeValidator) MeetingTime() error {
	return tv.validator.ValidateMeetingTime(tv.time)
}

// BusinessHours validates against business hours
func (tv *TimeValidator) BusinessHours(timezone string) error {
	return tv.validator.ValidateBusinessHours(tv.time, timezone)
}

// RangeFormatter provides formatting methods for time ranges
type RangeFormatter struct {
	start     time.Time
	end       time.Time
	formatter *Formatter
}

// FormatRange returns a range formatter for start and end times
func (m *Manager) FormatRange(start, end time.Time) *RangeFormatter {
	return &RangeFormatter{
		start:     start,
		end:       end,
		formatter: m.formatter,
	}
}

// ToScheduleWindow formats as a schedule window
func (rf *RangeFormatter) ToScheduleWindow(timezone string) string {
	return rf.formatter.ToScheduleWindow(rf.start, rf.end, timezone)
}

// ValidateScheduleWindow validates the range as a schedule window
func (rf *RangeFormatter) ValidateScheduleWindow(validator *Validator) error {
	return validator.ValidateScheduleWindow(rf.start, rf.end)
}

// Convenience functions for common operations

// ParseDateTime is a convenience function that uses default configuration
func ParseDateTime(input string) (time.Time, error) {
	manager := New(nil)
	return manager.Parse(input)
}

// FormatRFC3339 is a convenience function for RFC3339 formatting
func FormatRFC3339(t time.Time) string {
	manager := New(nil)
	return manager.Format(t).ToRFC3339()
}

// FormatMicrosoftGraph is a convenience function for Microsoft Graph formatting
func FormatMicrosoftGraph(t time.Time) string {
	manager := New(nil)
	return manager.Format(t).ToMicrosoftGraph()
}

// ValidateMeetingTime is a convenience function for meeting time validation
func ValidateMeetingTime(t time.Time) error {
	manager := New(nil)
	return manager.Validate(t).MeetingTime()
}
