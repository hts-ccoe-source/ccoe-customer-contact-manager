package datetime

import (
	"fmt"
	"strings"
	"time"
)

// Parser handles parsing of various date/time input formats
type Parser struct {
	config *DateTimeConfig
}

// NewParser creates a new Parser with the given configuration
func NewParser(config *DateTimeConfig) *Parser {
	if config == nil {
		config = DefaultConfig()
	}
	return &Parser{config: config}
}

// ParseDateTime attempts to parse a date/time string using multiple formats
func (p *Parser) ParseDateTime(input string) (time.Time, error) {
	if input == "" {
		return time.Time{}, NewDateTimeError(
			ErrInvalidFormat,
			"empty date/time input",
			input,
			nil,
		)
	}

	// Clean up the input
	input = strings.TrimSpace(input)

	// Try parsing with all supported formats
	parsed, err := p.ParseWithFormats(input, CommonInputFormats)
	if err != nil {
		return time.Time{}, NewDateTimeError(
			ErrInvalidFormat,
			fmt.Sprintf("unable to parse date/time: expected formats like '2006-01-02T15:04:05Z' or '01/02/2006 3:04 PM', got '%s'", input),
			input,
			err,
		)
	}

	// Apply default timezone if the parsed time doesn't have timezone info
	// Check if input explicitly specified timezone
	// For "-", only consider it a timezone if it appears after "T" (timezone offset, not date separator)
	hasTimezone := strings.Contains(input, "Z") || strings.Contains(input, "+") ||
		(strings.Contains(input, "T") && strings.LastIndex(input, "-") > strings.LastIndex(input, "T")) ||
		strings.HasSuffix(strings.ToUpper(input), "UTC")

	if !hasTimezone {
		// Apply default timezone - interpret the parsed time as being in the default timezone
		loc, err := time.LoadLocation(p.config.DefaultTimezone)
		if err != nil {
			return time.Time{}, NewDateTimeError(
				ErrInvalidTimezone,
				fmt.Sprintf("invalid default timezone: %s", p.config.DefaultTimezone),
				input,
				err,
			)
		}
		parsed = time.Date(
			parsed.Year(), parsed.Month(), parsed.Day(),
			parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(),
			loc,
		)
	}

	return parsed, nil
}

// ParseDate parses a date-only string and returns a time.Time at midnight in the default timezone
func (p *Parser) ParseDate(input string) (time.Time, error) {
	if input == "" {
		return time.Time{}, NewDateTimeError(
			ErrInvalidFormat,
			"empty date input",
			input,
			nil,
		)
	}

	input = strings.TrimSpace(input)

	// Date-only formats
	dateFormats := []string{
		"2006-01-02",
		"01/02/2006",
		"1/2/2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
		"2 Jan 2006",
	}

	parsed, err := p.ParseWithFormats(input, dateFormats)
	if err != nil {
		return time.Time{}, NewDateTimeError(
			ErrInvalidFormat,
			fmt.Sprintf("unable to parse date: expected formats like '2006-01-02' or '01/02/2006', got '%s'", input),
			input,
			err,
		)
	}

	// Set to midnight in the default timezone
	loc, err := time.LoadLocation(p.config.DefaultTimezone)
	if err != nil {
		return time.Time{}, NewDateTimeError(
			ErrInvalidTimezone,
			fmt.Sprintf("invalid default timezone: %s", p.config.DefaultTimezone),
			input,
			err,
		)
	}

	return time.Date(
		parsed.Year(), parsed.Month(), parsed.Day(),
		0, 0, 0, 0, loc,
	), nil
}

// ParseTime parses a time-only string and combines it with the given date
func (p *Parser) ParseTime(input string, date time.Time) (time.Time, error) {
	if input == "" {
		return time.Time{}, NewDateTimeError(
			ErrInvalidFormat,
			"empty time input",
			input,
			nil,
		)
	}

	input = strings.TrimSpace(input)

	// Time-only formats
	timeFormats := []string{
		"15:04:05",
		"15:04",
		"3:04:05 PM",
		"3:04 PM",
		"3:04:05PM",
		"3:04PM",
	}

	// Parse the time component
	parsed, err := p.ParseWithFormats(input, timeFormats)
	if err != nil {
		return time.Time{}, NewDateTimeError(
			ErrInvalidFormat,
			fmt.Sprintf("unable to parse time: expected formats like '15:04' or '3:04 PM', got '%s'", input),
			input,
			err,
		)
	}

	// Combine with the provided date in the date's timezone
	return time.Date(
		date.Year(), date.Month(), date.Day(),
		parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(),
		date.Location(),
	), nil
}

// ParseWithFormats tries to parse the input with multiple format strings
func (p *Parser) ParseWithFormats(input string, formats []string) (time.Time, error) {
	var lastErr error

	for _, format := range formats {
		parsed, err := time.Parse(format, input)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}

// ParseDateTimeWithTimezone parses a date/time string and applies the specified timezone
func (p *Parser) ParseDateTimeWithTimezone(input, timezone string) (time.Time, error) {
	// First parse the date/time
	parsed, err := p.ParseDateTime(input)
	if err != nil {
		return time.Time{}, err
	}

	// If timezone is specified, convert to that timezone
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}, NewDateTimeError(
				ErrInvalidTimezone,
				fmt.Sprintf("invalid timezone: %s", timezone),
				input,
				err,
			)
		}

		// Check if input explicitly specified timezone
		// For "-", only consider it a timezone if it appears after "T" (timezone offset, not date separator)
		hasTimezone := strings.Contains(input, "Z") || strings.Contains(input, "+") ||
			(strings.Contains(input, "T") && strings.LastIndex(input, "-") > strings.LastIndex(input, "T")) ||
			strings.HasSuffix(strings.ToUpper(input), "UTC")

		if hasTimezone {
			// If the parsed time already has timezone info, convert it to the specified timezone
			parsed = parsed.In(loc)
		} else {
			// Interpret the parsed time as being in the specified timezone
			parsed = time.Date(
				parsed.Year(), parsed.Month(), parsed.Day(),
				parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(),
				loc,
			)
		}
	}

	return parsed, nil
}
