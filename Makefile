# AWS Alternate Contact Manager Makefile

# Variables
BINARY_NAME=aws-alternate-contact-manager
LAMBDA_BINARY=bootstrap
VERSION?=latest
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD)

# Go build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Default target
.PHONY: all
all: build

# Build for local development (current architecture)
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) for local development..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Build for Linux x86_64 (standard ECS/EC2)
.PHONY: build-linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux x86_64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME) .

# Build for Lambda on Graviton (ARM64)
.PHONY: build-lambda
build-lambda:
	@echo "Building Lambda function for Graviton (ARM64)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(LAMBDA_BINARY) .

# Build for Lambda on x86_64 (if needed)
.PHONY: build-lambda-x86
build-lambda-x86:
	@echo "Building Lambda function for x86_64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(LAMBDA_BINARY) .

# Create Lambda deployment package for Graviton
.PHONY: package-lambda
package-lambda: build-lambda
	@echo "Creating Lambda deployment package..."
	@rm -f aws-alternate-contact-manager-lambda.zip
	zip aws-alternate-contact-manager-lambda.zip $(LAMBDA_BINARY)
	@if [ -f config.json ]; then \
		echo "Adding config.json to package..."; \
		zip aws-alternate-contact-manager-lambda.zip config.json; \
	fi
	@if [ -f SESConfig.json ]; then \
		echo "Adding SESConfig.json to package..."; \
		zip aws-alternate-contact-manager-lambda.zip SESConfig.json; \
	fi
	@echo "Lambda deployment package created: aws-alternate-contact-manager-lambda.zip"
	@ls -lh aws-alternate-contact-manager-lambda.zip
	@echo "Copying deployment package to Terraform applications directory..."
	@cp aws-alternate-contact-manager-lambda.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/
	@echo "✅ Deployment package copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/aws-alternate-contact-manager-lambda.zip"

# Create Lambda deployment package for x86_64
.PHONY: package-lambda-x86
package-lambda-x86: build-lambda-x86
	@echo "Creating Lambda deployment package for x86_64..."
	@rm -f aws-alternate-contact-manager-lambda-x86.zip
	zip aws-alternate-contact-manager-lambda-x86.zip $(LAMBDA_BINARY)
	@if [ -f config.json ]; then \
		echo "Adding config.json to package..."; \
		zip aws-alternate-contact-manager-lambda-x86.zip config.json; \
	fi
	@if [ -f SESConfig.json ]; then \
		echo "Adding SESConfig.json to package..."; \
		zip aws-alternate-contact-manager-lambda-x86.zip SESConfig.json; \
	fi
	@echo "Lambda deployment package created: aws-alternate-contact-manager-lambda-x86.zip"
	@ls -lh aws-alternate-contact-manager-lambda-x86.zip
	@echo "Copying deployment package to Terraform applications directory..."
	@cp aws-alternate-contact-manager-lambda-x86.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/
	@echo "✅ Deployment package copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/golang_lambda/aws-alternate-contact-manager-lambda-x86.zip"

# Package JavaScript Lambda functions
.PHONY: package-upload-lambda
package-upload-lambda:
	@echo "Creating upload Lambda deployment package..."
	@rm -f upload-metadata-lambda.zip
	@if [ ! -f enhanced-metadata-lambda.js ]; then \
		echo "Error: enhanced-metadata-lambda.js not found"; \
		exit 1; \
	fi
	zip upload-metadata-lambda.zip enhanced-metadata-lambda.js
	@echo "Upload Lambda deployment package created: upload-metadata-lambda.zip"
	@ls -lh upload-metadata-lambda.zip
	@echo "Copying deployment package and source to Terraform applications directory..."
	@mkdir -p ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/
	@cp upload-metadata-lambda.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/
	@cp enhanced-metadata-lambda.js ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/
	@echo "✅ Upload Lambda package and source copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/upload_lambda/"

.PHONY: package-saml-lambda
package-saml-lambda:
	@echo "Creating SAML auth Lambda deployment package..."
	@rm -f lambda-edge-samlify.zip
	@if [ ! -f lambda-edge-samlify.js ]; then \
		echo "Error: lambda-edge-samlify.js not found"; \
		exit 1; \
	fi
	@echo "Installing samlify dependency..."
	@rm -rf node_modules package.json package-lock.json
	@npm init -y > /dev/null 2>&1
	@npm install samlify @authenio/samlify-node-xmllint > /dev/null 2>&1
	zip -r lambda-edge-samlify.zip lambda-edge-samlify.js node_modules/
	@echo "SAML Lambda deployment package created: lambda-edge-samlify.zip"
	@ls -lh lambda-edge-samlify.zip
	@echo "Copying deployment package and source to Terraform applications directory..."
	@mkdir -p ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/
	@cp lambda-edge-samlify.zip ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/
	@cp lambda-edge-samlify.js ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/
	@echo "✅ SAML Lambda package and source copied to: ../terraform/hts-terraform-applications/hts-aws-com-std-app-orchestration-email-distro-prod-use1/saml_auth/"
	@echo "Cleaning up temporary files..."
	@rm -rf node_modules package.json package-lock.json

.PHONY: package-all-lambdas
package-all-lambdas: package-lambda package-upload-lambda package-saml-lambda
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

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint code
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

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
	rm -f aws-alternate-contact-manager-lambda.zip
	rm -f aws-alternate-contact-manager-lambda-x86.zip
	rm -f upload-metadata-lambda.zip
	rm -f lambda-edge-samlify.zip
	rm -f coverage.out
	rm -f coverage.html

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
release: clean fmt tidy test build-linux package-lambda
	@echo "Release build complete!"
	@echo "Artifacts created:"
	@ls -lh $(BINARY_NAME) lambda-deployment.zip

# Deploy Lambda function (requires AWS CLI)
.PHONY: deploy-lambda
deploy-lambda: package-lambda
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
create-lambda: package-lambda
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
	@echo "AWS Alternate Contact Manager - Available Make Targets:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build              Build for local development"
	@echo "  build-linux        Build for Linux x86_64 (ECS/EC2)"
	@echo "  build-lambda       Build Lambda function for Graviton (ARM64)"
	@echo "  build-lambda-x86   Build Lambda function for x86_64"
	@echo ""
	@echo "Package Commands:"
	@echo "  package-lambda        Create Go Lambda deployment package (Graviton)"
	@echo "  package-lambda-x86    Create Go Lambda deployment package (x86_64)"
	@echo "  package-upload-lambda Create JavaScript upload Lambda package"
	@echo "  package-saml-lambda   Create JavaScript SAML auth Lambda package"
	@echo "  package-all-lambdas   Create all Lambda packages"
	@echo ""
	@echo "Docker Commands:"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-build-multiarch Build multi-arch Docker image"
	@echo ""
	@echo "Development Commands:"
	@echo "  test               Run tests"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  lint               Run linter"
	@echo "  fmt                Format code"
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
	@echo "  release            Complete release build"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make package-lambda                    # Create Go Lambda package (Graviton)"
	@echo "  make package-upload-lambda             # Create JavaScript upload Lambda package"
	@echo "  make package-saml-lambda               # Create JavaScript SAML auth Lambda package"
	@echo "  make package-all-lambdas               # Create all Lambda packages"
	@echo "  make deploy-lambda FUNCTION_NAME=my-fn # Deploy to existing function"
	@echo "  make create-lambda FUNCTION_NAME=my-fn ROLE_ARN=arn:aws:iam::123:role/lambda-role"