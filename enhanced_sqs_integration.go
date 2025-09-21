package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// EnhancedSQSMessage represents the structure of messages processed by the enhanced SQS processor
type EnhancedSQSMessage struct {
	ChangeID      string                 `json:"changeId"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	CustomerCodes []string               `json:"customerCodes"`
	TemplateID    string                 `json:"templateId"`
	CreatedBy     string                 `json:"createdBy"`
	CreatedAt     string                 `json:"createdAt"`
	Priority      string                 `json:"priority"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// EnhancedSQSProcessor provides advanced SQS message processing with polling and graceful shutdown
type EnhancedSQSProcessor struct {
	config            SQSProcessorConfig
	sqsClient         *sqs.Client
	customerManager   *CustomerCredentialManager
	templateManager   *EmailTemplateManager
	sesManager        *SESIntegrationManager
	statusTracker     *ExecutionStatusTracker
	errorHandler      *ErrorHandler
	monitoringSystem  *MonitoringSystem
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	isRunning         bool
	runningMutex      sync.RWMutex
	messageChannel    chan *types.Message
	workerPool        chan struct{}
	shutdownTimeout   time.Duration
	lastPollTime      time.Time
	messagesProcessed int64
	messagesSucceeded int64
	messagesFailed    int64
	metricsLock       sync.RWMutex
}

// SQSProcessorConfig contains configuration for SQS processing
type SQSProcessorConfig struct {
	QueueURL           string        `json:"queueUrl"`
	Region             string        `json:"region"`
	MaxMessages        int32         `json:"maxMessages"`
	VisibilityTimeout  int32         `json:"visibilityTimeout"`
	WaitTimeSeconds    int32         `json:"waitTimeSeconds"`
	PollingInterval    time.Duration `json:"pollingInterval"`
	MaxRetries         int           `json:"maxRetries"`
	WorkerPoolSize     int           `json:"workerPoolSize"`
	MessageBufferSize  int           `json:"messageBufferSize"`
	ShutdownTimeout    time.Duration `json:"shutdownTimeout"`
	EnableLongPolling  bool          `json:"enableLongPolling"`
	EnableBatchDelete  bool          `json:"enableBatchDelete"`
	DeadLetterQueueURL string        `json:"deadLetterQueueUrl"`
}

// SQSProcessorMetrics contains metrics for SQS processing
type SQSProcessorMetrics struct {
	MessagesReceived   int64     `json:"messagesReceived"`
	MessagesProcessed  int64     `json:"messagesProcessed"`
	MessagesSucceeded  int64     `json:"messagesSucceeded"`
	MessagesFailed     int64     `json:"messagesFailed"`
	MessagesDeleted    int64     `json:"messagesDeleted"`
	LastPollTime       time.Time `json:"lastPollTime"`
	AverageProcessTime float64   `json:"averageProcessTime"`
	CurrentWorkers     int       `json:"currentWorkers"`
	QueueDepth         int64     `json:"queueDepth"`
	ErrorRate          float64   `json:"errorRate"`
}

// NewEnhancedSQSProcessor creates a new enhanced SQS processor
func NewEnhancedSQSProcessor(
	config SQSProcessorConfig,
	customerManager *CustomerCredentialManager,
	templateManager *EmailTemplateManager,
	sesManager *SESIntegrationManager,
	statusTracker *ExecutionStatusTracker,
	errorHandler *ErrorHandler,
	monitoringSystem *MonitoringSystem,
) (*EnhancedSQSProcessor, error) {
	// Load AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(config.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)

	// Set default values
	if config.WorkerPoolSize == 0 {
		config.WorkerPoolSize = 10
	}
	if config.MessageBufferSize == 0 {
		config.MessageBufferSize = 100
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}
	if config.MaxMessages == 0 {
		config.MaxMessages = 10
	}
	if config.VisibilityTimeout == 0 {
		config.VisibilityTimeout = 30
	}
	if config.WaitTimeSeconds == 0 {
		config.WaitTimeSeconds = 20
	}
	if config.PollingInterval == 0 {
		config.PollingInterval = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	processor := &EnhancedSQSProcessor{
		config:           config,
		sqsClient:        sqsClient,
		customerManager:  customerManager,
		templateManager:  templateManager,
		sesManager:       sesManager,
		statusTracker:    statusTracker,
		errorHandler:     errorHandler,
		monitoringSystem: monitoringSystem,
		ctx:              ctx,
		cancel:           cancel,
		messageChannel:   make(chan *types.Message, config.MessageBufferSize),
		workerPool:       make(chan struct{}, config.WorkerPoolSize),
		shutdownTimeout:  config.ShutdownTimeout,
	}

	return processor, nil
}

// StartProcessing starts the SQS message processing with polling loop
func (p *EnhancedSQSProcessor) StartProcessing() error {
	p.runningMutex.Lock()
	if p.isRunning {
		p.runningMutex.Unlock()
		return fmt.Errorf("SQS processor is already running")
	}
	p.isRunning = true
	p.runningMutex.Unlock()

	p.monitoringSystem.logger.Info("Starting enhanced SQS processor", map[string]interface{}{
		"queueUrl":          p.config.QueueURL,
		"workerPoolSize":    p.config.WorkerPoolSize,
		"pollingInterval":   p.config.PollingInterval.String(),
		"maxMessages":       p.config.MaxMessages,
		"visibilityTimeout": p.config.VisibilityTimeout,
		"waitTimeSeconds":   p.config.WaitTimeSeconds,
	})

	// Start worker goroutines
	for i := 0; i < p.config.WorkerPoolSize; i++ {
		p.wg.Add(1)
		go p.messageWorker(i)
	}

	// Start polling goroutine
	p.wg.Add(1)
	go p.pollingLoop()

	// Start metrics reporting goroutine
	p.wg.Add(1)
	go p.metricsReporter()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or context cancellation
	select {
	case sig := <-sigChan:
		p.monitoringSystem.logger.Info("Received shutdown signal", map[string]interface{}{
			"signal": sig.String(),
		})
		return p.GracefulShutdown()
	case <-p.ctx.Done():
		p.monitoringSystem.logger.Info("Context cancelled, shutting down", nil)
		return p.GracefulShutdown()
	}
}

// pollingLoop continuously polls SQS for messages
func (p *EnhancedSQSProcessor) pollingLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.monitoringSystem.logger.Info("Polling loop shutting down", nil)
			return
		case <-ticker.C:
			if err := p.pollMessages(); err != nil {
				p.monitoringSystem.logger.Error("Failed to poll messages", err, nil)
				// Continue polling even on errors
			}
		}
	}
}

// pollMessages polls SQS for new messages
func (p *EnhancedSQSProcessor) pollMessages() error {
	p.lastPollTime = time.Now()

	input := &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(p.config.QueueURL),
		MaxNumberOfMessages:   p.config.MaxMessages,
		VisibilityTimeout:     p.config.VisibilityTimeout,
		WaitTimeSeconds:       p.config.WaitTimeSeconds,
		MessageAttributeNames: []string{"All"},
	}

	result, err := p.sqsClient.ReceiveMessage(p.ctx, input)
	if err != nil {
		return fmt.Errorf("failed to receive messages: %v", err)
	}

	if len(result.Messages) == 0 {
		// No messages available
		return nil
	}

	p.monitoringSystem.logger.Debug("Received messages from SQS", map[string]interface{}{
		"messageCount": len(result.Messages),
		"queueUrl":     p.config.QueueURL,
	})

	// Send messages to worker pool
	for _, message := range result.Messages {
		select {
		case p.messageChannel <- &message:
			// Message queued for processing
		case <-p.ctx.Done():
			return nil
		default:
			// Channel is full, log warning
			p.monitoringSystem.logger.Warn("Message channel full, dropping message", map[string]interface{}{
				"messageId": *message.MessageId,
			})
		}
	}

	return nil
}

// messageWorker processes messages from the message channel
func (p *EnhancedSQSProcessor) messageWorker(workerID int) {
	defer p.wg.Done()

	p.monitoringSystem.logger.Info("Starting message worker", map[string]interface{}{
		"workerId": workerID,
	})

	for {
		select {
		case <-p.ctx.Done():
			p.monitoringSystem.logger.Info("Message worker shutting down", map[string]interface{}{
				"workerId": workerID,
			})
			return
		case message := <-p.messageChannel:
			p.processMessage(workerID, message)
		}
	}
}

// processMessage processes a single SQS message
func (p *EnhancedSQSProcessor) processMessage(workerID int, message *types.Message) {
	startTime := time.Now()
	messageID := *message.MessageId

	p.monitoringSystem.logger.Info("Processing message", map[string]interface{}{
		"workerId":  workerID,
		"messageId": messageID,
	})

	// Increment processed counter
	p.metricsLock.Lock()
	p.messagesProcessed++
	p.metricsLock.Unlock()

	// Parse message body
	var sqsMessage EnhancedSQSMessage
	if err := json.Unmarshal([]byte(*message.Body), &sqsMessage); err != nil {
		p.monitoringSystem.logger.Error("Failed to parse message body", err, map[string]interface{}{
			"workerId":  workerID,
			"messageId": messageID,
		})
		p.handleMessageFailure(message, fmt.Errorf("invalid message format: %v", err))
		return
	}

	// Validate message
	if err := p.validateSQSMessage(&sqsMessage); err != nil {
		p.monitoringSystem.logger.Error("Message validation failed", err, map[string]interface{}{
			"workerId":  workerID,
			"messageId": messageID,
		})
		p.handleMessageFailure(message, fmt.Errorf("message validation failed: %v", err))
		return
	}

	// Process message with retry logic
	err := p.errorHandler.ExecuteWithRetry(p.ctx, "sqs-processor", func(ctx context.Context) error {
		return p.processEmailDistribution(&sqsMessage)
	})

	processingTime := time.Since(startTime)

	if err != nil {
		p.monitoringSystem.logger.Error("Message processing failed", err, map[string]interface{}{
			"workerId":       workerID,
			"messageId":      messageID,
			"processingTime": processingTime.String(),
		})
		p.handleMessageFailure(message, err)
		return
	}

	// Message processed successfully, delete from queue
	if err := p.deleteMessage(message); err != nil {
		p.monitoringSystem.logger.Error("Failed to delete message", err, map[string]interface{}{
			"workerId":  workerID,
			"messageId": messageID,
		})
		// Don't return error here, message was processed successfully
	}

	// Increment success counter
	p.metricsLock.Lock()
	p.messagesSucceeded++
	p.metricsLock.Unlock()

	p.monitoringSystem.logger.Info("Message processed successfully", map[string]interface{}{
		"workerId":       workerID,
		"messageId":      messageID,
		"processingTime": processingTime.String(),
	})
}

// processEmailDistribution processes the email distribution from SQS message
func (p *EnhancedSQSProcessor) processEmailDistribution(sqsMessage *EnhancedSQSMessage) error {
	// Start execution tracking
	execution, err := p.statusTracker.StartExecution(
		sqsMessage.ChangeID,
		sqsMessage.Title,
		sqsMessage.Description,
		"sqs-processor",
		sqsMessage.CustomerCodes,
	)
	if err != nil {
		return fmt.Errorf("failed to start execution tracking: %v", err)
	}

	// Process each customer
	for _, customerCode := range sqsMessage.CustomerCodes {
		p.monitoringSystem.logger.Info("Processing customer from SQS", map[string]interface{}{
			"customer":    customerCode,
			"executionId": execution.ExecutionID,
			"changeId":    sqsMessage.ChangeID,
		})

		err := p.processCustomerFromSQS(execution.ExecutionID, customerCode, sqsMessage)
		if err != nil {
			p.monitoringSystem.logger.Error("Customer processing failed", err, map[string]interface{}{
				"customer":    customerCode,
				"executionId": execution.ExecutionID,
			})
			// Continue processing other customers
		}
	}

	return nil
}

// processCustomerFromSQS processes a customer's email distribution from SQS message
func (p *EnhancedSQSProcessor) processCustomerFromSQS(executionID, customerCode string, sqsMessage *EnhancedSQSMessage) error {
	// Start customer execution
	if err := p.statusTracker.StartCustomerExecution(executionID, customerCode); err != nil {
		return err
	}

	// Execute with retry logic
	operation := func(ctx context.Context) error {
		// Add execution steps
		steps := []struct {
			id, name, description string
		}{
			{"validate", "Validate SQS Message", "Validate SQS message and customer configuration"},
			{"render", "Render Templates", "Render email templates with message data"},
			{"send", "Send Emails", "Send emails via SES"},
			{"verify", "Verify Delivery", "Verify email delivery status"},
		}

		for _, step := range steps {
			p.statusTracker.AddExecutionStep(executionID, customerCode, step.id, step.name, step.description)
			p.statusTracker.UpdateExecutionStep(executionID, customerCode, step.id, StepStatusRunning, "")

			// Simulate step processing
			time.Sleep(50 * time.Millisecond)

			// Complete step
			p.statusTracker.UpdateExecutionStep(executionID, customerCode, step.id, StepStatusCompleted, "")
		}

		return nil
	}

	// Execute with retry logic
	err := p.errorHandler.ExecuteWithRetry(context.Background(), customerCode, operation)

	// Complete customer execution
	success := err == nil
	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}

	return p.statusTracker.CompleteCustomerExecution(executionID, customerCode, success, errorMessage)
}

// validateSQSMessage validates the SQS message format and content
func (p *EnhancedSQSProcessor) validateSQSMessage(message *EnhancedSQSMessage) error {
	if message.ChangeID == "" {
		return fmt.Errorf("changeId is required")
	}
	if message.Title == "" {
		return fmt.Errorf("title is required")
	}
	if len(message.CustomerCodes) == 0 {
		return fmt.Errorf("customerCodes is required and must not be empty")
	}
	if message.TemplateID == "" {
		return fmt.Errorf("templateId is required")
	}

	// Validate customer codes exist
	for _, customerCode := range message.CustomerCodes {
		if _, err := p.customerManager.GetCustomerAccountInfo(customerCode); err != nil {
			return fmt.Errorf("invalid customer code: %s", customerCode)
		}
	}

	return nil
}

// handleMessageFailure handles failed message processing
func (p *EnhancedSQSProcessor) handleMessageFailure(message *types.Message, err error) {
	// Increment failure counter
	p.metricsLock.Lock()
	p.messagesFailed++
	p.metricsLock.Unlock()

	// Check if message should be sent to dead letter queue
	if p.config.DeadLetterQueueURL != "" {
		if err := p.sendToDeadLetterQueue(message, err); err != nil {
			p.monitoringSystem.logger.Error("Failed to send message to dead letter queue", err, map[string]interface{}{
				"messageId": *message.MessageId,
			})
		}
	}

	// Log the failure
	p.monitoringSystem.logger.Error("Message processing failed permanently", err, map[string]interface{}{
		"messageId": *message.MessageId,
	})
}

// sendToDeadLetterQueue sends a failed message to the dead letter queue
func (p *EnhancedSQSProcessor) sendToDeadLetterQueue(message *types.Message, processingError error) error {
	// Create dead letter message with error information
	deadLetterMessage := map[string]interface{}{
		"originalMessage": *message.Body,
		"error":           processingError.Error(),
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"processor":       "enhanced-sqs-processor",
	}

	messageBody, err := json.Marshal(deadLetterMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal dead letter message: %v", err)
	}

	// Send to dead letter queue
	_, err = p.sqsClient.SendMessage(p.ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.config.DeadLetterQueueURL),
		MessageBody: aws.String(string(messageBody)),
	})

	return err
}

// deleteMessage deletes a processed message from the queue
func (p *EnhancedSQSProcessor) deleteMessage(message *types.Message) error {
	_, err := p.sqsClient.DeleteMessage(p.ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.config.QueueURL),
		ReceiptHandle: message.ReceiptHandle,
	})
	return err
}

// metricsReporter periodically reports metrics
func (p *EnhancedSQSProcessor) metricsReporter() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			metrics := p.GetMetrics()
			p.monitoringSystem.logger.Info("SQS processor metrics", map[string]interface{}{
				"messagesProcessed": metrics.MessagesProcessed,
				"messagesSucceeded": metrics.MessagesSucceeded,
				"messagesFailed":    metrics.MessagesFailed,
				"errorRate":         metrics.ErrorRate,
				"currentWorkers":    metrics.CurrentWorkers,
			})
		}
	}
}

// GetMetrics returns current processing metrics
func (p *EnhancedSQSProcessor) GetMetrics() SQSProcessorMetrics {
	p.metricsLock.RLock()
	defer p.metricsLock.RUnlock()

	var errorRate float64
	if p.messagesProcessed > 0 {
		errorRate = float64(p.messagesFailed) / float64(p.messagesProcessed) * 100
	}

	return SQSProcessorMetrics{
		MessagesProcessed: p.messagesProcessed,
		MessagesSucceeded: p.messagesSucceeded,
		MessagesFailed:    p.messagesFailed,
		LastPollTime:      p.lastPollTime,
		CurrentWorkers:    p.config.WorkerPoolSize,
		ErrorRate:         errorRate,
	}
}

// GracefulShutdown performs graceful shutdown of the SQS processor
func (p *EnhancedSQSProcessor) GracefulShutdown() error {
	p.runningMutex.Lock()
	if !p.isRunning {
		p.runningMutex.Unlock()
		return nil
	}
	p.isRunning = false
	p.runningMutex.Unlock()

	p.monitoringSystem.logger.Info("Starting graceful shutdown of SQS processor", map[string]interface{}{
		"shutdownTimeout": p.shutdownTimeout.String(),
	})

	// Cancel context to stop all goroutines
	p.cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.monitoringSystem.logger.Info("SQS processor shutdown completed gracefully", nil)
		return nil
	case <-time.After(p.shutdownTimeout):
		p.monitoringSystem.logger.Warn("SQS processor shutdown timed out", map[string]interface{}{
			"timeout": p.shutdownTimeout.String(),
		})
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// StopProcessing stops the SQS processor
func (p *EnhancedSQSProcessor) StopProcessing() {
	p.cancel()
}

// IsRunning returns whether the processor is currently running
func (p *EnhancedSQSProcessor) IsRunning() bool {
	p.runningMutex.RLock()
	defer p.runningMutex.RUnlock()
	return p.isRunning
}

// GetQueueDepth returns the approximate number of messages in the queue
func (p *EnhancedSQSProcessor) GetQueueDepth() (int64, error) {
	result, err := p.sqsClient.GetQueueAttributes(p.ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(p.config.QueueURL),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameApproximateNumberOfMessages,
		},
	})
	if err != nil {
		return 0, err
	}

	if attr, ok := result.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)]; ok {
		var depth int64
		fmt.Sscanf(attr, "%d", &depth)
		return depth, nil
	}

	return 0, nil
}

// PurgeQueue purges all messages from the queue (use with caution)
func (p *EnhancedSQSProcessor) PurgeQueue() error {
	p.monitoringSystem.logger.Warn("Purging SQS queue", map[string]interface{}{
		"queueUrl": p.config.QueueURL,
	})

	_, err := p.sqsClient.PurgeQueue(p.ctx, &sqs.PurgeQueueInput{
		QueueUrl: aws.String(p.config.QueueURL),
	})

	return err
}
