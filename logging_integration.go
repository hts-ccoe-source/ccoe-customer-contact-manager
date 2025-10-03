package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"aws-alternate-contact-manager/logging"
)

// Global logging manager
var loggingManager *logging.LoggingManager

// initializeLogging initializes the comprehensive logging system
func initializeLogging(config *Config) error {
	// Determine log level from config or environment
	logLevel := logging.INFO
	if config.LogLevel != "" {
		switch strings.ToUpper(config.LogLevel) {
		case "DEBUG":
			logLevel = logging.DEBUG
		case "INFO":
			logLevel = logging.INFO
		case "WARN":
			logLevel = logging.WARN
		case "ERROR":
			logLevel = logging.ERROR
		case "AUDIT":
			logLevel = logging.AUDIT
		}
	}

	// Get customer codes from config
	var customerCodes []string
	for code := range config.CustomerMappings {
		customerCodes = append(customerCodes, code)
	}

	// Configure logging
	loggingConfig := logging.LoggingConfig{
		Service:         "email-distribution",
		Version:         Version,
		LogLevel:        logLevel,
		CloudWatchGroup: "/aws/email-distribution",
		S3Bucket:        config.S3Config.BucketName + "-logs", // Use separate bucket for logs
		AWSRegion:       config.AWSRegion,
		CustomerCodes:   customerCodes,
		EnableAudit:     true,
		EnableRetention: true,
	}

	// Override from environment variables
	if cwGroup := os.Getenv("CLOUDWATCH_LOG_GROUP"); cwGroup != "" {
		loggingConfig.CloudWatchGroup = cwGroup
	}
	if s3Bucket := os.Getenv("LOG_S3_BUCKET"); s3Bucket != "" {
		loggingConfig.S3Bucket = s3Bucket
	}

	var err error
	loggingManager, err = logging.InitializeLogging(loggingConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %v", err)
	}

	// Log system startup
	loggingManager.LogSystemEvent("SYSTEM_STARTUP", "initialize_logging", nil, map[string]interface{}{
		"version":           Version,
		"build_time":        BuildTime,
		"git_commit":        GitCommit,
		"log_level":         logLevel,
		"customer_count":    len(customerCodes),
		"audit_enabled":     true,
		"retention_enabled": true,
	})

	return nil
}

// Enhanced logging functions for the main application

// logCustomerContactUpdate logs alternate contact updates with full audit trail
func logCustomerContactUpdate(customerCode string, contactType string, startTime time.Time, err error, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	operation := fmt.Sprintf("update_%s_contact", contactType)
	resource := "alternate_contacts"

	if details == nil {
		details = make(map[string]interface{})
	}
	details["contact_type"] = contactType

	loggingManager.LogCustomerOperation(customerCode, operation, resource, startTime, err, details)
}

// logEmailSending logs email sending operations with detailed metrics
func logEmailSending(customerCode string, emailType string, recipientCount int, startTime time.Time, err error, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	operation := fmt.Sprintf("send_%s_email", emailType)

	if details == nil {
		details = make(map[string]interface{})
	}
	details["email_type"] = emailType
	details["recipient_count"] = recipientCount

	loggingManager.LogEmailOperation(customerCode, operation, recipientCount, startTime, err, details)
}

// logSQSProcessing logs SQS message processing
func logSQSProcessing(customerCode string, messageID string, startTime time.Time, err error, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	operation := "process_sqs_message"
	resource := "sqs_queue"

	if details == nil {
		details = make(map[string]interface{})
	}
	details["message_id"] = messageID

	loggingManager.LogCustomerOperation(customerCode, operation, resource, startTime, err, details)
}

// logUserWebAction logs user actions from the web interface
func logUserWebAction(userID string, action string, customerCodes []string, startTime time.Time, err error, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	resource := "web_interface"

	if details == nil {
		details = make(map[string]interface{})
	}
	details["customer_codes"] = customerCodes
	details["source"] = "web_ui"

	loggingManager.LogUserAction(userID, action, resource, startTime, err, details)
}

// logSecurityEvent logs security-related events
func logSecurityEvent(eventType string, userID string, action string, ipAddress string, userAgent string, err error, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	loggingManager.LogSecurityEvent(eventType, userID, action, ipAddress, userAgent, err, details)
}

// logConfigurationChange logs configuration changes
func logConfigurationChange(userID string, configType string, operation string, oldValue interface{}, newValue interface{}, err error) {
	if loggingManager == nil {
		return
	}

	result := "SUCCESS"
	if err != nil {
		result = "FAILURE"
	}

	if loggingManager.AuditLogger != nil {
		loggingManager.AuditLogger.LogConfigurationChange(userID, configType, operation, oldValue, newValue, result)
	}
}

// logDataAccess logs data access for compliance
func logDataAccess(userID string, customerCode string, dataType string, operation string, err error, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	result := "SUCCESS"
	if err != nil {
		result = "FAILURE"
	}

	if loggingManager.AuditLogger != nil {
		loggingManager.AuditLogger.LogDataAccess(userID, customerCode, dataType, operation, result, details)
	}
}

// Enhanced error logging with context
func logError(message string, err error, context map[string]interface{}) {
	if loggingManager == nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", message, err)
		return
	}

	loggingManager.Logger.Error(message, err, context)
}

// Enhanced info logging with context
func logInfo(message string, context map[string]interface{}) {
	if loggingManager == nil {
		fmt.Printf("INFO: %s\n", message)
		return
	}

	loggingManager.Logger.Info(message, context)
}

// Enhanced debug logging with context
func logDebug(message string, context map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	loggingManager.Logger.Debug(message, context)
}

// Enhanced warning logging with context
func logWarn(message string, context map[string]interface{}) {
	if loggingManager == nil {
		fmt.Printf("WARN: %s\n", message)
		return
	}

	loggingManager.Logger.Warn(message, context)
}

// Audit logging functions

// auditUserLogin logs user login events
func auditUserLogin(userID string, ipAddress string, userAgent string, success bool, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	_ = "SUCCESS"
	if !success {
		_ = "FAILURE"
	}

	loggingManager.LogSecurityEvent("USER_LOGIN", userID, "login", ipAddress, userAgent, nil, details)
}

// auditUserLogout logs user logout events
func auditUserLogout(userID string, ipAddress string, userAgent string, details map[string]interface{}) {
	if loggingManager == nil {
		return
	}

	loggingManager.LogSecurityEvent("USER_LOGOUT", userID, "logout", ipAddress, userAgent, nil, details)
}

// auditPermissionChange logs permission changes
func auditPermissionChange(adminUserID string, targetUserID string, permission string, action string, err error) {
	if loggingManager == nil {
		return
	}

	details := map[string]interface{}{
		"target_user_id": targetUserID,
		"permission":     permission,
		"action":         action,
	}

	result := "SUCCESS"
	if err != nil {
		result = "FAILURE"
		details["error"] = err.Error()
	}

	if loggingManager.AuditLogger != nil {
		loggingManager.AuditLogger.LogUserAction(adminUserID, fmt.Sprintf("permission_%s", action), "user_permissions", result, details)
	}
}

// auditCustomerDataAccess logs customer data access
func auditCustomerDataAccess(userID string, customerCode string, dataType string, operation string, recordCount int, err error) {
	if loggingManager == nil {
		return
	}

	details := map[string]interface{}{
		"data_type":    dataType,
		"record_count": recordCount,
	}

	logDataAccess(userID, customerCode, dataType, operation, err, details)
}

// Performance monitoring functions

// measureOperation measures the duration of an operation and logs it
func measureOperation(operationName string, customerCode string, fn func() error) error {
	if loggingManager == nil {
		return fn()
	}

	return loggingManager.LogWithCustomerTiming(customerCode, operationName, fn)
}

// measureUserOperation measures a user operation
func measureUserOperation(operationName string, userID string, fn func() error) error {
	if loggingManager == nil {
		return fn()
	}

	return loggingManager.LogWithUserTiming(userID, operationName, fn)
}

// Log aggregation and search functions

// searchApplicationLogs searches application logs with criteria
func searchApplicationLogs(ctx context.Context, criteria logging.SearchCriteria) (*logging.LogSearchResult, error) {
	if loggingManager == nil {
		return nil, fmt.Errorf("logging manager not initialized")
	}

	return loggingManager.SearchLogs(ctx, criteria)
}

// getApplicationLogStatistics gets log statistics
func getApplicationLogStatistics(ctx context.Context, criteria logging.SearchCriteria) (*logging.LogStatistics, error) {
	if loggingManager == nil {
		return nil, fmt.Errorf("logging manager not initialized")
	}

	return loggingManager.GetLogStatistics(ctx, criteria)
}

// exportApplicationLogs exports logs in various formats
func exportApplicationLogs(ctx context.Context, criteria logging.SearchCriteria, format string) ([]byte, error) {
	if loggingManager == nil {
		return nil, fmt.Errorf("logging manager not initialized")
	}

	return loggingManager.ExportLogs(ctx, criteria, format)
}

// Retention management functions

// applyLogRetentionPolicies applies log retention policies
func applyLogRetentionPolicies(ctx context.Context) error {
	if loggingManager == nil {
		return fmt.Errorf("logging manager not initialized")
	}

	return loggingManager.ApplyRetentionPolicies(ctx)
}

// getLogRetentionStatus gets retention policy status
func getLogRetentionStatus(ctx context.Context) (*logging.RetentionStatus, error) {
	if loggingManager == nil {
		return nil, fmt.Errorf("logging manager not initialized")
	}

	return loggingManager.GetRetentionStatus(ctx)
}

// Cleanup function
func closeLogging() {
	if loggingManager != nil {
		// Log system shutdown
		loggingManager.LogSystemEvent("SYSTEM_SHUTDOWN", "close_logging", nil, map[string]interface{}{
			"uptime_seconds": time.Since(time.Now()).Seconds(), // This would need to be tracked properly
		})

		loggingManager.Close()
	}
}
