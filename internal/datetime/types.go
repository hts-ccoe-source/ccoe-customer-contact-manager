// Package datetime provides standardized date/time handling utilities
// for consistent parsing, formatting, and validation across all components.
package datetime

import "time"

// Standard format constants for consistent date/time handling
const (
	// RFC3339Format is the canonical internal format for all date/time storage
	RFC3339Format = "2006-01-02T15:04:05Z07:00"

	// GraphFormat is the format required by Microsoft Graph API
	GraphFormat = "2006-01-02T15:04:05.0000000"

	// ICSFormat is the format used for calendar files (iCalendar)
	ICSFormat = "20060102T150405Z"

	// LogFormat is the format used for structured logging with milliseconds
	LogFormat = "2006-01-02T15:04:05.000Z"

	// HumanDateFormat is a human-readable date format
	HumanDateFormat = "January 2, 2006"

	// HumanTimeFormat is a human-readable time format
	HumanTimeFormat = "3:04 PM MST"
)

// Common input formats that the parser should accept
var CommonInputFormats = []string{
	// ISO8601/RFC3339 formats (order matters - more specific first)
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",

	// Date only formats
	"2006-01-02",
	"01/02/2006",
	"1/2/2006",
	"January 2, 2006",
	"Jan 2, 2006",
	"2 January 2006",
	"2 Jan 2006",

	// Time only formats
	"15:04:05",
	"15:04",
	"3:04:05 PM",
	"3:04 PM",
	"3:04:05PM",
	"3:04PM",

	// Combined date/time formats
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"01/02/2006 15:04:05",
	"01/02/2006 15:04",
	"01/02/2006 3:04 PM",
	"January 2, 2006 3:04 PM",
	"January 2, 2006 at 3:04 PM",
}

// DateTimeConfig holds configuration for date/time operations
type DateTimeConfig struct {
	// DefaultTimezone is used when no timezone is specified in input
	DefaultTimezone string

	// AllowPastDates controls whether past dates are allowed in validation
	AllowPastDates bool

	// FutureTolerance is the grace period for "future" date validation
	// (e.g., allow dates up to 5 minutes in the past for meeting times)
	FutureTolerance time.Duration
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *DateTimeConfig {
	return &DateTimeConfig{
		DefaultTimezone: "America/New_York", // EST/EDT
		AllowPastDates:  false,
		FutureTolerance: 5 * time.Minute,
	}
}

// Error types for standardized error handling
const (
	ErrInvalidFormat   = "INVALID_FORMAT"
	ErrInvalidTimezone = "INVALID_TIMEZONE"
	ErrInvalidRange    = "INVALID_RANGE"
	ErrPastDate        = "PAST_DATE"
	ErrFutureDate      = "FUTURE_DATE"
)

// DateTimeError represents a standardized date/time error
type DateTimeError struct {
	Type    string
	Message string
	Input   string
	Cause   error
}

// Error implements the error interface
func (e *DateTimeError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// NewDateTimeError creates a new DateTimeError
func NewDateTimeError(errorType, message, input string, cause error) *DateTimeError {
	return &DateTimeError{
		Type:    errorType,
		Message: message,
		Input:   input,
		Cause:   cause,
	}
}
