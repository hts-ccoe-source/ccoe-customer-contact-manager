package lambda

import (
	"context"
	"time"
)

// ExecutionSummary tracks comprehensive metrics for Lambda execution.
//
// Instead of logging every operation, we track metrics in this structure and log
// once at the end of Lambda execution. This reduces log volume by 80%+ while
// maintaining complete visibility into Lambda operations.
//
// Usage:
//  1. Initialize in Handler: summary := NewExecutionSummary()
//  2. Store in context: ctx = ContextWithSummary(ctx, summary)
//  3. Track operations: summary.RecordEmailSent(count)
//  4. Log at end: logger.Info("lambda execution complete", "emails_sent", summary.EmailsSent, ...)
//
// See docs/logging-standards.md for complete documentation.
// See .kiro/specs/reduce-backend-logging/summary-metrics-mapping.md for metric definitions.
type ExecutionSummary struct {
	// Timing
	StartTime time.Time
	EndTime   time.Time

	// Message Processing
	TotalMessages      int
	SuccessfulMessages int
	RetryableErrors    int
	NonRetryableErrors int
	DiscardedEvents    int // Backend-generated events that were discarded

	// Customer Processing
	CustomersProcessed []string // List of customer codes processed

	// Email Statistics
	EmailsSent         int
	EmailsFiltered     int // Filtered by restricted_recipients
	EmailsBeforeFilter int // Total before filtering
	EmailErrors        int

	// Meeting Statistics
	MeetingsScheduled  int
	MeetingsCancelled  int
	MeetingsUpdated    int
	MeetingErrors      int
	TotalAttendees     int
	FilteredAttendees  int // Filtered by restricted_recipients
	ManualAttendees    int // Manually added attendees
	FinalAttendeeCount int

	// S3 Operations
	S3Downloads int
	S3Uploads   int
	S3Deletes   int
	S3Errors    int

	// Change Request Processing
	ApprovalRequests int
	ApprovedChanges  int
	CompletedChanges int
	CancelledChanges int

	// Error Details
	ErrorMessages []string // Detailed error messages for troubleshooting
}

// NewExecutionSummary creates a new execution summary
func NewExecutionSummary() *ExecutionSummary {
	return &ExecutionSummary{
		StartTime:          time.Now(),
		CustomersProcessed: make([]string, 0),
		ErrorMessages:      make([]string, 0),
	}
}

// RecordSuccess increments successful message count
func (s *ExecutionSummary) RecordSuccess() {
	s.SuccessfulMessages++
}

// RecordRetryableError increments retryable error count and records error message
func (s *ExecutionSummary) RecordRetryableError(err error) {
	s.RetryableErrors++
	s.ErrorMessages = append(s.ErrorMessages, err.Error())
}

// RecordNonRetryableError increments non-retryable error count and records error message
func (s *ExecutionSummary) RecordNonRetryableError(err error) {
	s.NonRetryableErrors++
	s.ErrorMessages = append(s.ErrorMessages, err.Error())
}

// RecordDiscardedEvent increments discarded event count
func (s *ExecutionSummary) RecordDiscardedEvent() {
	s.DiscardedEvents++
}

// RecordCustomer adds a customer code to the processed list if not already present
func (s *ExecutionSummary) RecordCustomer(customerCode string) {
	for _, code := range s.CustomersProcessed {
		if code == customerCode {
			return
		}
	}
	s.CustomersProcessed = append(s.CustomersProcessed, customerCode)
}

// RecordEmailSent increments email sent count
func (s *ExecutionSummary) RecordEmailSent(recipientCount int) {
	s.EmailsSent++
	s.FinalAttendeeCount += recipientCount
}

// RecordEmailFiltering records email filtering statistics
func (s *ExecutionSummary) RecordEmailFiltering(beforeFilter, afterFilter, filtered int) {
	s.EmailsBeforeFilter += beforeFilter
	s.EmailsFiltered += filtered
}

// RecordEmailError increments email error count
func (s *ExecutionSummary) RecordEmailError() {
	s.EmailErrors++
}

// RecordMeetingScheduled increments meeting scheduled count
func (s *ExecutionSummary) RecordMeetingScheduled(attendeeCount int) {
	s.MeetingsScheduled++
	s.TotalAttendees += attendeeCount
}

// RecordMeetingCancelled increments meeting cancelled count
func (s *ExecutionSummary) RecordMeetingCancelled() {
	s.MeetingsCancelled++
}

// RecordMeetingUpdated increments meeting updated count
func (s *ExecutionSummary) RecordMeetingUpdated() {
	s.MeetingsUpdated++
}

// RecordMeetingError increments meeting error count
func (s *ExecutionSummary) RecordMeetingError() {
	s.MeetingErrors++
}

// RecordMeetingAttendeeFiltering records meeting attendee filtering statistics
func (s *ExecutionSummary) RecordMeetingAttendeeFiltering(total, filtered, manual, final int) {
	s.TotalAttendees += total
	s.FilteredAttendees += filtered
	s.ManualAttendees += manual
	s.FinalAttendeeCount += final
}

// RecordS3Download increments S3 download count
func (s *ExecutionSummary) RecordS3Download() {
	s.S3Downloads++
}

// RecordS3Upload increments S3 upload count
func (s *ExecutionSummary) RecordS3Upload() {
	s.S3Uploads++
}

// RecordS3Delete increments S3 delete count
func (s *ExecutionSummary) RecordS3Delete() {
	s.S3Deletes++
}

// RecordS3Error increments S3 error count
func (s *ExecutionSummary) RecordS3Error() {
	s.S3Errors++
}

// RecordApprovalRequest increments approval request count
func (s *ExecutionSummary) RecordApprovalRequest() {
	s.ApprovalRequests++
}

// RecordApprovedChange increments approved change count
func (s *ExecutionSummary) RecordApprovedChange() {
	s.ApprovedChanges++
}

// RecordCompletedChange increments completed change count
func (s *ExecutionSummary) RecordCompletedChange() {
	s.CompletedChanges++
}

// RecordCancelledChange increments cancelled change count
func (s *ExecutionSummary) RecordCancelledChange() {
	s.CancelledChanges++
}

// Finalize marks the end time and calculates duration
func (s *ExecutionSummary) Finalize() {
	s.EndTime = time.Now()
}

// DurationMs returns the execution duration in milliseconds
func (s *ExecutionSummary) DurationMs() int64 {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime).Milliseconds()
	}
	return s.EndTime.Sub(s.StartTime).Milliseconds()
}

// Context key for execution summary
type contextKey string

const summaryContextKey contextKey = "execution_summary"

// ContextWithSummary adds the execution summary to the context
func ContextWithSummary(ctx context.Context, summary *ExecutionSummary) context.Context {
	return context.WithValue(ctx, summaryContextKey, summary)
}

// SummaryFromContext retrieves the execution summary from the context
// Returns nil if no summary is found
func SummaryFromContext(ctx context.Context) *ExecutionSummary {
	if summary, ok := ctx.Value(summaryContextKey).(*ExecutionSummary); ok {
		return summary
	}
	return nil
}
