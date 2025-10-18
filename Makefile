# CCOE Customer Contact Manager Makefile

# Variables
BINARY_NAME=ccoe-customer-contact-manager
LAMBDA_BINARY=bootstrap
VERSION?=latest
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD)

# Go build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Default target - build for Lambda deployment
.PHONY: all
all: build-lambda

# Build for Lambda deployment (ARM64/Graviton - recommended)
.PHONY: build build-lambda
build build-lambda:
	@echo "Building Lambda function for Graviton (ARM64)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(LAMBDA_BINARY) .

# Build for Lambda on x86_64 (alternative architecture)
.PHONY: build-lambda-x86
build-lambda-x86:
	@echo "Building Lambda function for x86_64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(LAMBDA_BINARY) .

# Build for local development/testing (current architecture)
.PHONY: build-local
build-local:
	@echo "Building $(BINARY_NAME) for local development..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Build for Linux x86_64 (if needed for ECS/EC2)
.PHONY: build-linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux x86_64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME) .

# Create Lambda deployment package for Graviton
.PHONY: package-golang-lambda
package-golang-lambda: build-lambda
	@echo "Creating Lambda deployment package..."
	@rm -f ccoe-customer-contact-manager-lambda.zip
	@zip -q ccoe-customer-contact-manager-lambda.zip $(LAMBDA_BINARY)
	@if [ -f config.json ]; then \
		echo "Adding config.json to package..."; \
		zip -q ccoe-customer-contact-manager-lambda.zip config.json; \
	fi
	@if [ -f SESConfig.json ]; then \
		echo "Adding SESConfig.json to package..."; \
		zip -q ccoe-customer-contact-manager-lambda.zip SESConfig.json; \
	fi
	@echo "Lambda deployment package created: ccoe-customer-contact-manager-lambda.zip"
	@ls -lh ccoe-customer-contact-manager-lambda.zip
	@echo "Copying deployment package to Terraform applications directory..."
	@cp ccoe-customer-contact-manager-lambda.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/
	@echo "✅ Deployment package copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/ccoe-customer-contact-manager-lambda.zip"

# Create Lambda deployment package for x86_64
.PHONY: package-golang-lambda-x86
package-golang-lambda-x86: build-lambda-x86
	@echo "Creating Lambda deployment package for x86_64..."
	@rm -f ccoe-customer-contact-manager-lambda-x86.zip
	@zip -q ccoe-customer-contact-manager-lambda-x86.zip $(LAMBDA_BINARY)
	@if [ -f config.json ]; then \
		echo "Adding config.json to package..."; \
		zip -q ccoe-customer-contact-manager-lambda-x86.zip config.json; \
	fi
	@if [ -f SESConfig.json ]; then \
		echo "Adding SESConfig.json to package..."; \
		zip -q ccoe-customer-contact-manager-lambda-x86.zip SESConfig.json; \
	fi
	@echo "Lambda deployment package created: ccoe-customer-contact-manager-lambda-x86.zip"
	@ls -lh ccoe-customer-contact-manager-lambda-x86.zip
	@echo "Copying deployment package to Terraform applications directory..."
	@cp ccoe-customer-contact-manager-lambda-x86.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/
	@echo "✅ Deployment package copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/ccoe-customer-contact-manager-lambda-x86.zip"

# Sync datetime utilities to lambda directories
.PHONY: sync-datetime-utilities
sync-datetime-utilities:
	@echo "Syncing datetime utilities to lambda directories..."
	@cp datetime/index.js lambda/upload_lambda/datetime/index.js
	@cp datetime/parser.js lambda/upload_lambda/datetime/parser.js
	@cp datetime/formatter.js lambda/upload_lambda/datetime/formatter.js
	@cp datetime/validator.js lambda/upload_lambda/datetime/validator.js
	@cp datetime/types.js lambda/upload_lambda/datetime/types.js
	@cp datetime/index.js lambda/saml_auth/datetime/index.js
	@cp datetime/parser.js lambda/saml_auth/datetime/parser.js
	@cp datetime/formatter.js lambda/saml_auth/datetime/formatter.js
	@cp datetime/validator.js lambda/saml_auth/datetime/validator.js
	@cp datetime/types.js lambda/saml_auth/datetime/types.js
	@echo "✅ Datetime utilities synced to all lambda directories"

# Package JavaScript Lambda functions
.PHONY: package-upload-lambda
package-upload-lambda: sync-datetime-utilities
	@echo "Building upload Lambda..."
	@cd lambda/upload_lambda && $(MAKE) build
	@echo "Copying deployment package to Terraform applications directory..."
	@mkdir -p ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/
	@cp lambda/upload_lambda/upload-metadata-lambda.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/
	@cp lambda/upload_lambda/upload-metadata-lambda.js ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/
	@echo "✅ Upload Lambda package copied"

.PHONY: package-saml-lambda
package-saml-lambda: sync-datetime-utilities
	@echo "Building SAML auth Lambda with dependencies..."
	@cd lambda/saml_auth && $(MAKE) build
	@echo "Copying deployment package and source to Terraform applications directory..."
	@mkdir -p ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/
	@cp lambda/saml_auth/lambda-edge-samlify.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/
	@cp lambda/saml_auth/lambda-edge-samlify.js ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/
	@echo "✅ SAML Lambda package and source copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/"

.PHONY: package-all-lambdas
package-all-lambdas: package-golang-lambda package-upload-lambda package-saml-lambda
	@echo "✅ All Lambda packages created and copied to Terraform directory"

# Build Docker image
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

# Build multi-arch Docker image for ARM64 and AMD64
.PHONY: docker-build-multiarch
docker-build-multiarch:
	@echo "Building multi-architecture Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(BINARY_NAME):$(VERSION) .

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...
	@echo "Running tests for internal packages..."
	go test -v ./internal/...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	@echo "Running tests with coverage for internal packages..."
	go test -v -coverprofile=coverage-internal.out ./internal/...
	@echo "Merging coverage reports..."
	@if command -v gocovmerge >/dev/null 2>&1; then \
		gocovmerge coverage.out coverage-internal.out > coverage-merged.out; \
		go tool cover -html=coverage-merged.out -o coverage.html; \
		echo "Merged coverage report generated: coverage.html"; \
	else \
		go tool cover -html=coverage.out -o coverage.html; \
		echo "Coverage report generated: coverage.html (install gocovmerge for merged reports)"; \
	fi

# Test only internal packages
.PHONY: test-internal
test-internal:
	@echo "Running tests for internal packages only..."
	go test -v ./internal/...

# Validate internal package structure
.PHONY: validate-structure
validate-structure:
	@echo "Validating internal package structure..."
	@for pkg in aws config contacts lambda ses types; do \
		if [ ! -d "internal/$$pkg" ]; then \
			echo "❌ Missing internal/$$pkg directory"; \
			exit 1; \
		else \
			echo "✅ internal/$$pkg directory exists"; \
		fi; \
	done
	@echo "✅ Internal package structure validation complete"

# Lint code
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run
	@echo "Running linter for internal packages..."
	golangci-lint run ./internal/...

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Formatting internal packages..."
	go fmt ./internal/...

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f $(LAMBDA_BINARY)
	rm -f ccoe-customer-contact-manager-lambda.zip
	rm -f ccoe-customer-contact-manager-lambda-x86.zip
	rm -f coverage.out
	rm -f coverage-internal.out
	rm -f coverage-merged.out
	rm -f coverage.html
	@echo "Cleaning JavaScript Lambda artifacts..."
	@cd lambda/upload_lambda && $(MAKE) clean
	@cd lambda/saml_auth && $(MAKE) clean

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download

# Development setup
.PHONY: dev-setup
dev-setup: deps
	@echo "Setting up development environment..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi

# Quick development build and test
.PHONY: dev
dev: fmt tidy build test

# Release build (all architectures)
.PHONY: release
release: clean fmt tidy test build-linux package-golang-lambda
	@echo "Release build complete!"
	@echo "Artifacts created:"
	@ls -lh $(BINARY_NAME) lambda-deployment.zip

# Deploy Lambda function (requires AWS CLI)
.PHONY: deploy-lambda
deploy-lambda: package-golang-lambda
	@echo "Deploying Lambda function..."
	@if [ -z "$(FUNCTION_NAME)" ]; then \
		echo "Error: FUNCTION_NAME environment variable is required"; \
		echo "Usage: make deploy-lambda FUNCTION_NAME=my-function"; \
		exit 1; \
	fi
	aws lambda update-function-code \
		--function-name $(FUNCTION_NAME) \
		--zip-file fileb://lambda-deployment.zip
	@echo "Lambda function $(FUNCTION_NAME) updated successfully!"

# Create Lambda function (requires AWS CLI)
.PHONY: create-lambda
create-lambda: package-golang-lambda
	@echo "Creating Lambda function..."
	@if [ -z "$(FUNCTION_NAME)" ] || [ -z "$(ROLE_ARN)" ]; then \
		echo "Error: FUNCTION_NAME and ROLE_ARN environment variables are required"; \
		echo "Usage: make create-lambda FUNCTION_NAME=my-function ROLE_ARN=arn:aws:iam::123456789012:role/lambda-role"; \
		exit 1; \
	fi
	aws lambda create-function \
		--function-name $(FUNCTION_NAME) \
		--runtime provided.al2 \
		--role $(ROLE_ARN) \
		--handler bootstrap \
		--zip-file fileb://lambda-deployment.zip \
		--architectures arm64 \
		--timeout 300 \
		--memory-size 512
	@echo "Lambda function $(FUNCTION_NAME) created successfully!"

# Show help
.PHONY: help
help:
	@echo "CCOE Customer Contact Manager - Available Make Targets:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build              Build for Lambda deployment (ARM64/Graviton)"
	@echo "  build-lambda       Same as 'build' - Lambda deployment (ARM64/Graviton)"
	@echo "  build-lambda-x86   Build for Lambda deployment (x86_64)"
	@echo "  build-local        Build for local development/testing"
	@echo "  build-linux        Build for Linux x86_64 (ECS/EC2)"
	@echo "  build-lambda       Build Lambda function for Graviton (ARM64)"
	@echo "  build-lambda-x86   Build Lambda function for x86_64"
	@echo ""
	@echo "Package Commands:"
	@echo "  sync-datetime-utilities   Sync datetime utilities to lambda directories"
	@echo "  package-golang-lambda     Create Go Lambda deployment package (Graviton)"
	@echo "  package-golang-lambda-x86 Create Go Lambda deployment package (x86_64)"
	@echo "  package-upload-lambda     Create JavaScript upload Lambda package"
	@echo "  package-saml-lambda       Create JavaScript SAML auth Lambda package"
	@echo "  package-all-lambdas       Create all Lambda packages"
	@echo ""
	@echo "Docker Commands:"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-build-multiarch Build multi-arch Docker image"
	@echo ""
	@echo "Development Commands:"
	@echo "  test               Run tests (all packages)"
	@echo "  test-internal      Run tests for internal packages only"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  lint               Run linter (all packages)"
	@echo "  fmt                Format code (all packages)"
	@echo "  tidy               Tidy dependencies"
	@echo "  dev                Quick dev build (fmt + tidy + build + test)"
	@echo "  dev-setup          Setup development environment"
	@echo ""
	@echo "Deployment Commands:"
	@echo "  deploy-lambda      Deploy Lambda function (requires FUNCTION_NAME)"
	@echo "  create-lambda      Create Lambda function (requires FUNCTION_NAME and ROLE_ARN)"
	@echo ""
	@echo "Utility Commands:"
	@echo "  clean              Clean build artifacts"
	@echo "  deps               Install dependencies"
	@echo "  validate-structure Validate internal package structure"
	@echo "  release            Complete release build"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make package-golang-lambda             # Create Go Lambda package (Graviton)"
	@echo "  make package-upload-lambda             # Create JavaScript upload Lambda package"
	@echo "  make package-saml-lambda               # Create JavaScript SAML auth Lambda package"
	@echo "  make package-all-lambdas               # Create all Lambda packages"
	@echo "  make deploy-lambda FUNCTION_NAME=my-fn # Deploy to existing function"
	@echo "  make create-lambda FUNCTION_NAME=my-fn ROLE_ARN=arn:aws:iam::123:role/lambda-role"