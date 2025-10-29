# Implementation Plan

- [ ] 1. Initialize slog in Lambda handler
  - Add slog initialization at start of Lambda handler in main.go
  - Default to JSON output for Lambda mode (CloudWatch)
  - Default to text output for CLI mode (human-readable)
  - Allow override via LOG_FORMAT environment variable
  - Set as default logger with `slog.SetDefault(logger)`
  - Set default level to Info (Debug only for troubleshooting)
  - _Requirements: 4.2, 4.3_

- [ ] 2. Audit and categorize all log statements in main.go
  - Search for all `log.Printf()` calls in main.go Lambda handler
  - Categorize each as: ERROR (keep), WARN (keep selective), INFO (keep critical only), or DELETE
  - Target: Reduce from ~200+ logs to ~20-30 logs
  - Document which logs to keep vs remove
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 3.4_

- [ ] 3. Migrate logging in main.go Lambda handler to slog
  - Convert ERROR logs: `log.Printf("❌...")` → `logger.Error("...", "error", err)`
  - Convert WARN logs: `log.Printf("⚠️...")` → `logger.Warn("...")`
  - Convert INFO logs: `log.Printf("✅...")` → `logger.Info("...")`
  - DELETE verbose success logs (don't convert to Debug)
  - Remove all emoji characters
  - Add single summary log at end: `logger.Info("lambda complete", "processed", count, "errors", errCount, "duration_ms", ms)`
  - _Requirements: 1.1, 1.2, 2.1, 2.2, 2.3, 2.4, 4.1, 4.2, 4.3_

- [ ] 4. Add logger parameter to AnnouncementProcessor
  - Add `logger *slog.Logger` field to AnnouncementProcessor struct
  - Update NewAnnouncementProcessor to accept logger parameter
  - Pass logger through to all processor methods
  - _Requirements: 4.2_

- [ ] 5. Migrate logging in internal/processors/announcement_processor.go to slog
  - Audit all log.Printf calls (~100+ logs)
  - DELETE verbose step-by-step processing logs (don't convert)
  - Convert ERROR logs to logger.Error with structured fields
  - Convert meeting scheduled/cancelled to logger.Info (critical operations)
  - Convert email sent to logger.Info with recipient count
  - Target: Reduce from ~100+ logs to ~15-20 logs
  - _Requirements: 1.1, 2.1, 2.2, 3.1, 3.2, 3.3, 4.1, 4.2, 4.3_

- [ ] 6. Migrate logging in internal/lambda/handlers.go to slog
  - Add logger parameter to handler functions
  - DELETE verbose SQS message processing logs
  - DELETE successful S3 event parsing logs
  - DELETE customer code extraction success logs
  - Convert ERROR logs to logger.Error
  - Convert WARN logs to logger.Warn
  - Target: Reduce from ~50+ logs to ~5-10 logs
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 4.1, 4.2, 4.3_

- [ ] 7. Migrate logging in internal/ses/meetings.go to slog
  - Add logger parameter to meeting functions
  - DELETE verbose meeting creation step logs
  - Convert meeting scheduled to logger.Info with meeting_id
  - Convert meeting cancelled to logger.Info with result
  - Convert ERROR logs to logger.Error
  - Target: Reduce from ~30+ logs to ~5-8 logs
  - _Requirements: 1.1, 2.1, 3.1, 3.2, 4.1, 4.2, 4.3_

- [ ] 8. Migrate logging in internal/ses/operations.go to slog
  - Add logger parameter to SES operation functions
  - DELETE verbose SES operation success logs
  - Convert email sent to logger.Info with type and recipient_count
  - Convert ERROR logs to logger.Error
  - Convert WARN logs to logger.Warn
  - Target: Reduce from ~50+ logs to ~10-15 logs
  - _Requirements: 1.1, 2.1, 3.3, 4.1, 4.2, 4.3_

- [ ] 7. Review and preserve concurrent logging patterns
  - Review CustomerLogBuffer usage in processCustomer()
  - Review ImportAllAWSContactsWithLogger() buffered logging
  - Remove emoji from buffered log messages
  - Reduce verbosity but keep customer grouping intact
  - Preserve logBuffer.Printf() and logBuffer.Flush() pattern
  - _Requirements: 1.1, 4.1, 4.2, 4.3_

- [ ] 8. Search for and clean up logging in any remaining files
  - Use grep to find all log.Printf and slog calls
  - Apply same cleanup rules to any other files
  - Ensure consistency across entire codebase
  - _Requirements: 1.1, 4.1, 4.2, 4.3_

- [ ] 9. Design and implement comprehensive summary statistics
  - Identify all metrics currently logged individually (processed count, error count, email count, meeting count, etc.)
  - Create summary data structure to track all metrics throughout Lambda execution
  - Include email filtering stats: sent, filtered by restricted_recipients, total before filter
  - Include meeting attendee stats: total, filtered by restricted_recipients, manual attendees added, final count
  - Add counters/trackers at each critical operation point
  - Verify each deleted log statement has corresponding metric in summary
  - Document mapping: "deleted log X" → "tracked in summary field Y"
  - _Requirements: 1.1, 1.2_

- [ ] 10. Add summary logging to Lambda handler
  - Log single comprehensive summary line at end of handler
  - Include: processed count, error count, email count, meeting count, duration, customer codes
  - Format: `logger.Info("lambda complete", "processed", count, "errors", errCount, "emails_sent", emailCount, "meetings_scheduled", meetingCount, "duration_ms", ms, "customers", codes)`
  - Verify summary contains all information previously in individual logs
  - _Requirements: 1.1, 1.2_

- [ ] 11. Verify summary statistics completeness
  - For each deleted log, verify corresponding data in summary
  - Test that summary provides same troubleshooting information as individual logs
  - Create test scenarios and verify summary captures all relevant data
  - Document any gaps and add missing metrics to summary
  - _Requirements: 1.1, 1.2_

- [ ] 12. Test in non-production environment
  - Deploy changes to non-prod Lambda
  - Trigger test change request workflow
  - Trigger test announcement workflow
  - Trigger test meeting scheduling
  - Trigger concurrent customer import (test buffered logging)
  - Review CloudWatch logs for each workflow
  - Verify summary statistics match expected values
  - _Requirements: All_

- [ ] 13. Validate log quality and completeness
  - Verify all errors are still logged with context
  - Verify warnings are still logged
  - Verify critical operations are logged (meetings, emails)
  - Verify no emoji characters remain
  - Verify log volume reduced significantly (80%+)
  - Verify logs are still useful for troubleshooting
  - Verify concurrent customer logs are still grouped properly
  - Verify summary statistics provide complete picture of execution
  - Compare old logs vs new logs for same workflow - ensure no information loss
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 3.4, 4.1, 4.2, 4.3_

- [ ] 14. Document logging standards
  - Update code comments with logging guidelines
  - Document what should/shouldn't be logged (Error/Warn/Info/Debug)
  - Add examples of proper slog format with structured fields
  - Document CustomerLogBuffer pattern and when to use it
  - Document summary statistics structure and what metrics to track
  - Create troubleshooting guide using new log format
  - _Requirements: 4.2, 4.3_

- [ ] 15. Deploy to production
  - Deploy cleaned up logging to production Lambda
  - Monitor CloudWatch logs for 24 hours
  - Verify no critical information lost
  - Measure log volume reduction (target: 80%+)
  - Verify concurrent operations still log correctly
  - Verify summary statistics are accurate and complete
  - Collect feedback from team on log usefulness
  - _Requirements: All_
