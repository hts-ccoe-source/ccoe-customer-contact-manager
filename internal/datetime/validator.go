package datetime

import (
	"fmt"
	"time"
)

// Validator handles validation of date/time values according to business rules
type Validator struct {
	config *DateTimeConfig
}

// NewValidator creates a new Validator with the given configuration
func NewValidator(config *DateTimeConfig) *Validator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Validator{config: config}
}

// ValidateDateTime performs basic validation on a time.Time value
func (v *Validator) ValidateDateTime(t time.Time) error {
	// Check if time is zero value
	if t.IsZero() {
		return NewDateTimeError(
			ErrInvalidFormat,
			"date/time cannot be zero value",
			"",
			nil,
		)
	}

	// Check if time is too far in the future (sanity check)
	maxFuture := time.Now().AddDate(10, 0, 0) // 10 years from now
	if t.After(maxFuture) {
		return NewDateTimeError(
			ErrFutureDate,
			fmt.Sprintf("date/time is too far in the future: %s", t.Format(RFC3339Format)),
			t.Format(RFC3339Format),
			nil,
		)
	}

	// Check if time is too far in the past (sanity check)
	minPast := time.Now().AddDate(-50, 0, 0) // 50 years ago
	if t.Before(minPast) {
		return NewDateTimeError(
			ErrPastDate,
			fmt.Sprintf("date/time is too far in the past: %s", t.Format(RFC3339Format)),
			t.Format(RFC3339Format),
			nil,
		)
	}

	return nil
}

// ValidateDateRange ensures that start time is before end time
func (v *Validator) ValidateDateRange(start, end time.Time) error {
	// Validate individual times first
	if err := v.ValidateDateTime(start); err != nil {
		return err
	}
	if err := v.ValidateDateTime(end); err != nil {
		return err
	}

	// Check that start is before end
	if !start.Before(end) {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("start time (%s) must be before end time (%s)",
				start.Format(RFC3339Format), end.Format(RFC3339Format)),
			fmt.Sprintf("%s to %s", start.Format(RFC3339Format), end.Format(RFC3339Format)),
			nil,
		)
	}

	// Check that the range is reasonable (not more than 1 year)
	maxDuration := 365 * 24 * time.Hour // 1 year
	if end.Sub(start) > maxDuration {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("date range is too long: maximum allowed is 1 year, got %s",
				end.Sub(start).String()),
			fmt.Sprintf("%s to %s", start.Format(RFC3339Format), end.Format(RFC3339Format)),
			nil,
		)
	}

	return nil
}

// ValidateTimezone verifies that a timezone string is valid
func (v *Validator) ValidateTimezone(tz string) error {
	if tz == "" {
		return NewDateTimeError(
			ErrInvalidTimezone,
			"timezone cannot be empty",
			tz,
			nil,
		)
	}

	_, err := time.LoadLocation(tz)
	if err != nil {
		return NewDateTimeError(
			ErrInvalidTimezone,
			fmt.Sprintf("invalid timezone: %s (must be a valid IANA timezone identifier)", tz),
			tz,
			err,
		)
	}

	return nil
}

// ValidateMeetingTime validates that a meeting time is appropriate
func (v *Validator) ValidateMeetingTime(t time.Time) error {
	// Basic validation first
	if err := v.ValidateDateTime(t); err != nil {
		return err
	}

	now := time.Now()

	// Check if meeting is in the past (with tolerance)
	if !v.config.AllowPastDates {
		earliestAllowed := now.Add(-v.config.FutureTolerance)
		if t.Before(earliestAllowed) {
			return NewDateTimeError(
				ErrPastDate,
				fmt.Sprintf("meeting time cannot be in the past (tolerance: %s): %s",
					v.config.FutureTolerance.String(), t.Format(RFC3339Format)),
				t.Format(RFC3339Format),
				nil,
			)
		}
	}

	// Check if meeting is too far in the future (business rule)
	maxFuture := now.AddDate(2, 0, 0) // 2 years from now
	if t.After(maxFuture) {
		return NewDateTimeError(
			ErrFutureDate,
			fmt.Sprintf("meeting time is too far in the future (maximum: 2 years): %s",
				t.Format(RFC3339Format)),
			t.Format(RFC3339Format),
			nil,
		)
	}

	return nil
}

// ValidateBusinessHours checks if a time falls within business hours
func (v *Validator) ValidateBusinessHours(t time.Time, timezone string) error {
	// Convert to specified timezone
	displayTime := t
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			displayTime = t.In(loc)
		}
	}

	hour := displayTime.Hour()
	weekday := displayTime.Weekday()

	// Check if it's a weekend
	if weekday == time.Saturday || weekday == time.Sunday {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("meeting time falls on weekend: %s", displayTime.Format("Monday, January 2, 2006 at 3:04 PM MST")),
			t.Format(RFC3339Format),
			nil,
		)
	}

	// Check if it's outside business hours (8 AM to 6 PM)
	if hour < 8 || hour >= 18 {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("meeting time is outside business hours (8 AM - 6 PM): %s",
				displayTime.Format("Monday, January 2, 2006 at 3:04 PM MST")),
			t.Format(RFC3339Format),
			nil,
		)
	}

	return nil
}

// ValidateScheduleWindow validates an implementation schedule window
func (v *Validator) ValidateScheduleWindow(start, end time.Time) error {
	// Basic range validation
	if err := v.ValidateDateRange(start, end); err != nil {
		return err
	}

	// Check minimum duration (at least 15 minutes)
	minDuration := 15 * time.Minute
	if end.Sub(start) < minDuration {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("schedule window is too short: minimum %s, got %s",
				minDuration.String(), end.Sub(start).String()),
			fmt.Sprintf("%s to %s", start.Format(RFC3339Format), end.Format(RFC3339Format)),
			nil,
		)
	}

	// Check maximum duration (no more than 24 hours for a single window)
	maxDuration := 24 * time.Hour
	if end.Sub(start) > maxDuration {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("schedule window is too long: maximum %s, got %s",
				maxDuration.String(), end.Sub(start).String()),
			fmt.Sprintf("%s to %s", start.Format(RFC3339Format), end.Format(RFC3339Format)),
			nil,
		)
	}

	return nil
}

// ValidateMeetingDuration validates that a meeting duration is reasonable
func (v *Validator) ValidateMeetingDuration(duration time.Duration) error {
	// Minimum 15 minutes
	if duration < 15*time.Minute {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("meeting duration is too short: minimum 15 minutes, got %s", duration.String()),
			duration.String(),
			nil,
		)
	}

	// Maximum 8 hours
	if duration > 8*time.Hour {
		return NewDateTimeError(
			ErrInvalidRange,
			fmt.Sprintf("meeting duration is too long: maximum 8 hours, got %s", duration.String()),
			duration.String(),
			nil,
		)
	}

	return nil
}

// ValidateTimezonePair validates that two times are in compatible timezones
func (v *Validator) ValidateTimezonePair(t1, t2 time.Time) error {
	// Both times should have timezone information
	if t1.Location() == nil || t2.Location() == nil {
		return NewDateTimeError(
			ErrInvalidTimezone,
			"both times must have timezone information",
			fmt.Sprintf("%s, %s", t1.Format(RFC3339Format), t2.Format(RFC3339Format)),
			nil,
		)
	}

	// This is mainly a sanity check - times can be in different timezones,
	// but we want to ensure they're both valid
	if err := v.ValidateDateTime(t1); err != nil {
		return err
	}
	if err := v.ValidateDateTime(t2); err != nil {
		return err
	}

	return nil
}
