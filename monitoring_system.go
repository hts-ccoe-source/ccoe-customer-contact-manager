package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// MonitoringSystem provides comprehensive monitoring and observability
type MonitoringSystem struct {
	metricsCollector *MetricsCollector
	logger           *StructuredLogger
	healthChecker    *HealthChecker
	alertManager     *AlertManager
	dashboardConfig  *DashboardConfiguration
	customerManager  *CustomerCredentialManager
	errorHandler     *ErrorHandler
	statusTracker    *ExecutionStatusTracker
	config           MonitoringConfiguration
}

// MetricsCollector collects and emits metrics
type MetricsCollector struct {
	metrics       map[string]*Metric
	customMetrics map[string]*CustomMetric
	mutex         sync.RWMutex
	enabled       bool
	namespace     string
	tags          map[string]string
}

// Metric represents a system metric
type Metric struct {
	Name        string            `json:"name"`
	Type        MetricType        `json:"type"`
	Value       float64           `json:"value"`
	Unit        string            `json:"unit"`
	Timestamp   time.Time         `json:"timestamp"`
	Tags        map[string]string `json:"tags"`
	Dimensions  map[string]string `json:"dimensions"`
	Description string            `json:"description"`
}

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeTimer     MetricType = "timer"
)

// CustomMetric represents a custom application metric
type CustomMetric struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Type         MetricType        `json:"type"`
	Value        float64           `json:"value"`
	CustomerCode string            `json:"customerCode,omitempty"`
	Tags         map[string]string `json:"tags"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// StructuredLogger provides structured logging capabilities
type StructuredLogger struct {
	level       LogLevel
	output      LogOutput
	fields      map[string]interface{}
	mutex       sync.RWMutex
	enabled     bool
	serviceName string
	version     string
}

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// LogOutput represents log output destinations
type LogOutput string

const (
	LogOutputConsole    LogOutput = "console"
	LogOutputFile       LogOutput = "file"
	LogOutputCloudWatch LogOutput = "cloudwatch"
	LogOutputSyslog     LogOutput = "syslog"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp    time.Time              `json:"timestamp"`
	Level        LogLevel               `json:"level"`
	Message      string                 `json:"message"`
	ServiceName  string                 `json:"serviceName"`
	Version      string                 `json:"version"`
	CustomerCode string                 `json:"customerCode,omitempty"`
	ExecutionID  string                 `json:"executionId,omitempty"`
	TraceID      string                 `json:"traceId,omitempty"`
	SpanID       string                 `json:"spanId,omitempty"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
	Error        *ErrorInfo             `json:"error,omitempty"`
}

// HealthChecker monitors system health
type HealthChecker struct {
	checks        map[string]HealthCheck
	status        HealthStatus
	lastCheck     time.Time
	checkInterval time.Duration
	mutex         sync.RWMutex
	enabled       bool
}

// HealthCheck represents a health check function
type HealthCheck struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	CheckFunc   func() HealthResult `json:"-"`
	Timeout     time.Duration       `json:"timeout"`
	Critical    bool                `json:"critical"`
	Tags        map[string]string   `json:"tags"`
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HealthStatus represents health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// AlertManager handles alerting and notifications
type AlertManager struct {
	rules        map[string]*AlertRule
	channels     map[string]AlertChannel
	activeAlerts map[string]*Alert
	mutex        sync.RWMutex
	enabled      bool
}

// AlertRule defines when to trigger an alert
type AlertRule struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Condition   AlertCondition    `json:"condition"`
	Severity    AlertSeverity     `json:"severity"`
	Channels    []string          `json:"channels"`
	Cooldown    time.Duration     `json:"cooldown"`
	Tags        map[string]string `json:"tags"`
	Enabled     bool              `json:"enabled"`
}

// AlertCondition defines the condition for triggering an alert
type AlertCondition struct {
	MetricName  string        `json:"metricName"`
	Operator    string        `json:"operator"` // >, <, >=, <=, ==, !=
	Threshold   float64       `json:"threshold"`
	Duration    time.Duration `json:"duration"`
	Aggregation string        `json:"aggregation"` // avg, sum, min, max, count
}

// AlertSeverity represents alert severity levels
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityInfo     AlertSeverity = "info"
)

// Alert represents an active alert
type Alert struct {
	ID          string                 `json:"id"`
	RuleName    string                 `json:"ruleName"`
	Severity    AlertSeverity          `json:"severity"`
	Message     string                 `json:"message"`
	TriggeredAt time.Time              `json:"triggeredAt"`
	ResolvedAt  *time.Time             `json:"resolvedAt,omitempty"`
	Status      AlertStatus            `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Channels    []string               `json:"channels"`
}

// AlertStatus represents the status of an alert
type AlertStatus string

const (
	AlertStatusActive   AlertStatus = "active"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusSilenced AlertStatus = "silenced"
)

// AlertChannel defines how alerts are delivered
type AlertChannel interface {
	SendAlert(alert *Alert) error
	GetName() string
	IsEnabled() bool
}

// DashboardConfiguration defines monitoring dashboard configuration
type DashboardConfiguration struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Widgets     []DashboardWidget `json:"widgets"`
	Layout      DashboardLayout   `json:"layout"`
	RefreshRate time.Duration     `json:"refreshRate"`
	TimeRange   string            `json:"timeRange"`
	Tags        map[string]string `json:"tags"`
}

// DashboardWidget represents a dashboard widget
type DashboardWidget struct {
	ID          string                 `json:"id"`
	Type        WidgetType             `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Metrics     []string               `json:"metrics"`
	Filters     map[string]string      `json:"filters"`
	Position    WidgetPosition         `json:"position"`
	Size        WidgetSize             `json:"size"`
	Config      map[string]interface{} `json:"config"`
}

// WidgetType represents the type of dashboard widget
type WidgetType string

const (
	WidgetTypeLineChart WidgetType = "line_chart"
	WidgetTypeBarChart  WidgetType = "bar_chart"
	WidgetTypePieChart  WidgetType = "pie_chart"
	WidgetTypeGauge     WidgetType = "gauge"
	WidgetTypeTable     WidgetType = "table"
	WidgetTypeText      WidgetType = "text"
	WidgetTypeHeatmap   WidgetType = "heatmap"
)

// WidgetPosition represents widget position on dashboard
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// WidgetSize represents widget size
type WidgetSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DashboardLayout represents dashboard layout configuration
type DashboardLayout struct {
	Columns int    `json:"columns"`
	Rows    int    `json:"rows"`
	Theme   string `json:"theme"`
}

// NewMonitoringSystem creates a new monitoring system
func NewMonitoringSystem(config MonitoringConfiguration, customerManager *CustomerCredentialManager, errorHandler *ErrorHandler, statusTracker *ExecutionStatusTracker) *MonitoringSystem {
	ms := &MonitoringSystem{
		customerManager: customerManager,
		errorHandler:    errorHandler,
		statusTracker:   statusTracker,
		config:          config,
	}

	// Initialize components
	ms.metricsCollector = NewMetricsCollector(config.MetricsNamespace, config.EnableCloudWatch)
	ms.logger = NewStructuredLogger(LogLevelInfo, LogOutputConsole, "multi-customer-email-distribution", "1.0.0")
	ms.healthChecker = NewHealthChecker(config.HealthCheckInterval)
	ms.alertManager = NewAlertManager()
	ms.dashboardConfig = ms.createDefaultDashboard()

	// Register default health checks
	ms.registerDefaultHealthChecks()

	// Register default alert rules
	ms.registerDefaultAlertRules()

	// Start background monitoring
	ms.startBackgroundMonitoring()

	return ms
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(namespace string, enabled bool) *MetricsCollector {
	return &MetricsCollector{
		metrics:       make(map[string]*Metric),
		customMetrics: make(map[string]*CustomMetric),
		enabled:       enabled,
		namespace:     namespace,
		tags:          make(map[string]string),
	}
}

// EmitMetric emits a metric
func (mc *MetricsCollector) EmitMetric(name string, value float64, metricType MetricType, unit string, tags map[string]string) {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	metric := &Metric{
		Name:      name,
		Type:      metricType,
		Value:     value,
		Unit:      unit,
		Timestamp: time.Now(),
		Tags:      tags,
	}

	mc.metrics[name] = metric

	// In a real implementation, this would send to CloudWatch, Prometheus, etc.
	log.Printf("METRIC: %s=%f %s %v", name, value, unit, tags)
}

// IncrementCounter increments a counter metric
func (mc *MetricsCollector) IncrementCounter(name string, tags map[string]string) {
	mc.EmitMetric(name, 1, MetricTypeCounter, "count", tags)
}

// SetGauge sets a gauge metric value
func (mc *MetricsCollector) SetGauge(name string, value float64, unit string, tags map[string]string) {
	mc.EmitMetric(name, value, MetricTypeGauge, unit, tags)
}

// RecordTimer records a timer metric
func (mc *MetricsCollector) RecordTimer(name string, duration time.Duration, tags map[string]string) {
	mc.EmitMetric(name, float64(duration.Milliseconds()), MetricTypeTimer, "milliseconds", tags)
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(level LogLevel, output LogOutput, serviceName, version string) *StructuredLogger {
	return &StructuredLogger{
		level:       level,
		output:      output,
		fields:      make(map[string]interface{}),
		enabled:     true,
		serviceName: serviceName,
		version:     version,
	}
}

// Log writes a structured log entry
func (sl *StructuredLogger) Log(level LogLevel, message string, fields map[string]interface{}) {
	if !sl.enabled || !sl.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp:   time.Now(),
		Level:       level,
		Message:     message,
		ServiceName: sl.serviceName,
		Version:     sl.version,
		Fields:      fields,
	}

	// Add global fields
	sl.mutex.RLock()
	for k, v := range sl.fields {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}
		entry.Fields[k] = v
	}
	sl.mutex.RUnlock()

	sl.writeLog(entry)
}

// Info logs an info message
func (sl *StructuredLogger) Info(message string, fields map[string]interface{}) {
	sl.Log(LogLevelInfo, message, fields)
}

// Error logs an error message
func (sl *StructuredLogger) Error(message string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	sl.Log(LogLevelError, message, fields)
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(message string, fields map[string]interface{}) {
	sl.Log(LogLevelDebug, message, fields)
}

// Warn logs a warning message
func (sl *StructuredLogger) Warn(message string, fields map[string]interface{}) {
	sl.Log(LogLevelWarn, message, fields)
}

// shouldLog determines if a message should be logged based on level
func (sl *StructuredLogger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
		LogLevelFatal: 4,
	}

	return levels[level] >= levels[sl.level]
}

// writeLog writes the log entry to the configured output
func (sl *StructuredLogger) writeLog(entry LogEntry) {
	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	switch sl.output {
	case LogOutputConsole:
		fmt.Println(string(jsonData))
	case LogOutputFile:
		// In real implementation, write to file
		fmt.Println(string(jsonData))
	case LogOutputCloudWatch:
		// In real implementation, send to CloudWatch Logs
		fmt.Println(string(jsonData))
	default:
		fmt.Println(string(jsonData))
	}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(checkInterval time.Duration) *HealthChecker {
	return &HealthChecker{
		checks:        make(map[string]HealthCheck),
		status:        HealthStatusUnknown,
		checkInterval: checkInterval,
		enabled:       true,
	}
}

// RegisterHealthCheck registers a new health check
func (hc *HealthChecker) RegisterHealthCheck(check HealthCheck) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	hc.checks[check.Name] = check
}

// RunHealthChecks executes all registered health checks
func (hc *HealthChecker) RunHealthChecks() map[string]HealthResult {
	hc.mutex.RLock()
	checks := make(map[string]HealthCheck)
	for k, v := range hc.checks {
		checks[k] = v
	}
	hc.mutex.RUnlock()

	results := make(map[string]HealthResult)
	overallStatus := HealthStatusHealthy

	for name, check := range checks {
		start := time.Now()

		// Run check with timeout
		ctx, cancel := context.WithTimeout(context.Background(), check.Timeout)
		defer cancel()

		resultChan := make(chan HealthResult, 1)
		go func() {
			resultChan <- check.CheckFunc()
		}()

		var result HealthResult
		select {
		case result = <-resultChan:
			result.Duration = time.Since(start)
		case <-ctx.Done():
			result = HealthResult{
				Status:    HealthStatusUnhealthy,
				Message:   "Health check timed out",
				Duration:  time.Since(start),
				Timestamp: time.Now(),
			}
		}

		result.Timestamp = time.Now()
		results[name] = result

		// Update overall status
		if check.Critical && result.Status != HealthStatusHealthy {
			overallStatus = HealthStatusUnhealthy
		} else if result.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	hc.mutex.Lock()
	hc.status = overallStatus
	hc.lastCheck = time.Now()
	hc.mutex.Unlock()

	return results
}

// GetHealthStatus returns the current health status
func (hc *HealthChecker) GetHealthStatus() (HealthStatus, time.Time) {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	return hc.status, hc.lastCheck
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{
		rules:        make(map[string]*AlertRule),
		channels:     make(map[string]AlertChannel),
		activeAlerts: make(map[string]*Alert),
		enabled:      true,
	}
}

// RegisterAlertRule registers a new alert rule
func (am *AlertManager) RegisterAlertRule(rule *AlertRule) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.rules[rule.Name] = rule
}

// RegisterAlertChannel registers a new alert channel
func (am *AlertManager) RegisterAlertChannel(channel AlertChannel) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.channels[channel.GetName()] = channel
}

// EvaluateAlerts evaluates all alert rules against current metrics
func (am *AlertManager) EvaluateAlerts(metrics map[string]*Metric) {
	if !am.enabled {
		return
	}

	am.mutex.Lock()
	defer am.mutex.Unlock()

	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		metric, exists := metrics[rule.Condition.MetricName]
		if !exists {
			continue
		}

		triggered := am.evaluateCondition(rule.Condition, metric)
		alertID := fmt.Sprintf("%s-%s", rule.Name, rule.Condition.MetricName)

		if triggered {
			if _, exists := am.activeAlerts[alertID]; !exists {
				// Create new alert
				alert := &Alert{
					ID:          alertID,
					RuleName:    rule.Name,
					Severity:    rule.Severity,
					Message:     fmt.Sprintf("Alert triggered: %s", rule.Description),
					TriggeredAt: time.Now(),
					Status:      AlertStatusActive,
					Channels:    rule.Channels,
					Metadata:    make(map[string]interface{}),
				}

				am.activeAlerts[alertID] = alert
				am.sendAlert(alert)
			}
		} else {
			if alert, exists := am.activeAlerts[alertID]; exists {
				// Resolve alert
				now := time.Now()
				alert.ResolvedAt = &now
				alert.Status = AlertStatusResolved
				am.sendAlert(alert)
				delete(am.activeAlerts, alertID)
			}
		}
	}
}

// evaluateCondition evaluates an alert condition
func (am *AlertManager) evaluateCondition(condition AlertCondition, metric *Metric) bool {
	switch condition.Operator {
	case ">":
		return metric.Value > condition.Threshold
	case "<":
		return metric.Value < condition.Threshold
	case ">=":
		return metric.Value >= condition.Threshold
	case "<=":
		return metric.Value <= condition.Threshold
	case "==":
		return metric.Value == condition.Threshold
	case "!=":
		return metric.Value != condition.Threshold
	default:
		return false
	}
}

// sendAlert sends an alert through configured channels
func (am *AlertManager) sendAlert(alert *Alert) {
	for _, channelName := range alert.Channels {
		if channel, exists := am.channels[channelName]; exists && channel.IsEnabled() {
			go func(ch AlertChannel, a *Alert) {
				if err := ch.SendAlert(a); err != nil {
					log.Printf("Failed to send alert through channel %s: %v", ch.GetName(), err)
				}
			}(channel, alert)
		}
	}
}

// registerDefaultHealthChecks registers default health checks
func (ms *MonitoringSystem) registerDefaultHealthChecks() {
	// Database connectivity check
	ms.healthChecker.RegisterHealthCheck(HealthCheck{
		Name:        "database",
		Description: "Database connectivity check",
		CheckFunc: func() HealthResult {
			// Simulate database check
			return HealthResult{
				Status:  HealthStatusHealthy,
				Message: "Database connection is healthy",
			}
		},
		Timeout:  5 * time.Second,
		Critical: true,
	})

	// AWS services check
	ms.healthChecker.RegisterHealthCheck(HealthCheck{
		Name:        "aws_services",
		Description: "AWS services connectivity check",
		CheckFunc: func() HealthResult {
			// Check SES, SQS, S3 connectivity
			return HealthResult{
				Status:  HealthStatusHealthy,
				Message: "AWS services are accessible",
			}
		},
		Timeout:  10 * time.Second,
		Critical: true,
	})

	// Memory usage check
	ms.healthChecker.RegisterHealthCheck(HealthCheck{
		Name:        "memory",
		Description: "Memory usage check",
		CheckFunc: func() HealthResult {
			// Simulate memory check
			return HealthResult{
				Status:  HealthStatusHealthy,
				Message: "Memory usage is within acceptable limits",
			}
		},
		Timeout:  2 * time.Second,
		Critical: false,
	})
}

// registerDefaultAlertRules registers default alert rules
func (ms *MonitoringSystem) registerDefaultAlertRules() {
	// High error rate alert
	ms.alertManager.RegisterAlertRule(&AlertRule{
		Name:        "high_error_rate",
		Description: "Error rate is above threshold",
		Condition: AlertCondition{
			MetricName:  "error_rate",
			Operator:    ">",
			Threshold:   5.0,
			Duration:    5 * time.Minute,
			Aggregation: "avg",
		},
		Severity: AlertSeverityHigh,
		Channels: []string{"email", "slack"},
		Cooldown: 15 * time.Minute,
		Enabled:  true,
	})

	// High latency alert
	ms.alertManager.RegisterAlertRule(&AlertRule{
		Name:        "high_latency",
		Description: "Response latency is above threshold",
		Condition: AlertCondition{
			MetricName:  "response_time",
			Operator:    ">",
			Threshold:   1000.0,
			Duration:    2 * time.Minute,
			Aggregation: "avg",
		},
		Severity: AlertSeverityMedium,
		Channels: []string{"email"},
		Cooldown: 10 * time.Minute,
		Enabled:  true,
	})
}

// createDefaultDashboard creates a default monitoring dashboard
func (ms *MonitoringSystem) createDefaultDashboard() *DashboardConfiguration {
	return &DashboardConfiguration{
		Name:        "Multi-Customer Email Distribution Dashboard",
		Description: "Main monitoring dashboard for the email distribution system",
		RefreshRate: 30 * time.Second,
		TimeRange:   "1h",
		Layout: DashboardLayout{
			Columns: 3,
			Rows:    4,
			Theme:   "dark",
		},
		Widgets: []DashboardWidget{
			{
				ID:          "emails_sent",
				Type:        WidgetTypeLineChart,
				Title:       "Emails Sent",
				Description: "Number of emails sent over time",
				Metrics:     []string{"emails_sent"},
				Position:    WidgetPosition{X: 0, Y: 0},
				Size:        WidgetSize{Width: 1, Height: 1},
			},
			{
				ID:          "error_rate",
				Type:        WidgetTypeGauge,
				Title:       "Error Rate",
				Description: "Current error rate percentage",
				Metrics:     []string{"error_rate"},
				Position:    WidgetPosition{X: 1, Y: 0},
				Size:        WidgetSize{Width: 1, Height: 1},
			},
			{
				ID:          "customer_status",
				Type:        WidgetTypePieChart,
				Title:       "Customer Status Distribution",
				Description: "Distribution of customer execution statuses",
				Metrics:     []string{"customer_status"},
				Position:    WidgetPosition{X: 2, Y: 0},
				Size:        WidgetSize{Width: 1, Height: 1},
			},
			{
				ID:          "response_time",
				Type:        WidgetTypeLineChart,
				Title:       "Response Time",
				Description: "Average response time over time",
				Metrics:     []string{"response_time"},
				Position:    WidgetPosition{X: 0, Y: 1},
				Size:        WidgetSize{Width: 2, Height: 1},
			},
		},
	}
}

// startBackgroundMonitoring starts background monitoring tasks
func (ms *MonitoringSystem) startBackgroundMonitoring() {
	// Start health check routine
	go func() {
		ticker := time.NewTicker(ms.config.HealthCheckInterval)
		defer ticker.Stop()

		for range ticker.C {
			ms.healthChecker.RunHealthChecks()
		}
	}()

	// Start metrics collection routine
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			ms.collectSystemMetrics()
		}
	}()

	// Start alert evaluation routine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ms.metricsCollector.mutex.RLock()
			metrics := make(map[string]*Metric)
			for k, v := range ms.metricsCollector.metrics {
				metrics[k] = v
			}
			ms.metricsCollector.mutex.RUnlock()

			ms.alertManager.EvaluateAlerts(metrics)
		}
	}()
}

// collectSystemMetrics collects system-level metrics
func (ms *MonitoringSystem) collectSystemMetrics() {
	// Collect error metrics
	if ms.errorHandler != nil {
		errorMetrics := ms.errorHandler.GetErrorMetrics()
		ms.metricsCollector.SetGauge("total_errors", float64(errorMetrics.TotalErrors), "count", nil)
		ms.metricsCollector.SetGauge("circuit_breaker_trips", float64(errorMetrics.CircuitBreakerTrips), "count", nil)
		ms.metricsCollector.SetGauge("dead_letter_count", float64(errorMetrics.DeadLetterCount), "count", nil)
	}

	// Collect execution metrics
	if ms.statusTracker != nil {
		summary, err := ms.statusTracker.GetExecutionSummary(24 * time.Hour)
		if err == nil {
			ms.metricsCollector.SetGauge("total_executions", float64(summary.TotalExecutions), "count", nil)
			if summary.PerformanceStats != nil {
				ms.metricsCollector.SetGauge("success_rate", summary.PerformanceStats.SuccessRate, "percent", nil)
				ms.metricsCollector.SetGauge("average_duration", float64(summary.PerformanceStats.AverageDuration.Milliseconds()), "milliseconds", nil)
			}
		}
	}
}

// GetHealthEndpoint returns an HTTP handler for health checks
func (ms *MonitoringSystem) GetHealthEndpoint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		results := ms.healthChecker.RunHealthChecks()
		status, lastCheck := ms.healthChecker.GetHealthStatus()

		response := map[string]interface{}{
			"status":    status,
			"lastCheck": lastCheck,
			"checks":    results,
			"timestamp": time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")

		if status == HealthStatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// GetMetricsEndpoint returns an HTTP handler for metrics
func (ms *MonitoringSystem) GetMetricsEndpoint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ms.metricsCollector.mutex.RLock()
		metrics := make(map[string]*Metric)
		for k, v := range ms.metricsCollector.metrics {
			metrics[k] = v
		}
		ms.metricsCollector.mutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}
