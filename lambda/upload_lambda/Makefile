# Makefile for Upload Metadata Lambda function

.PHONY: build clean install test package

# Default target
all: build

# Install dependencies
install:
	npm install

# Build the deployment package
build: install
	./build.sh

# Clean build artifacts
clean:
	rm -f upload-metadata-lambda.zip
	rm -rf node_modules

# Create deployment package only (assumes dependencies are installed)
package:
	@echo "Creating deployment package..."
	@rm -f upload-metadata-lambda.zip
	@find . -type f \( -name "*.js" -o -name "package*.json" -o -path "./node_modules/*" \) | sort | zip -q -X -@ upload-metadata-lambda.zip
	@echo "✅ Package created: upload-metadata-lambda.zip"
	@echo "Package size: $$(du -h upload-metadata-lambda.zip | cut -f1)"

# Test the function locally (placeholder for future testing)
test:
	@echo "Local testing not implemented yet"
	@echo "Use AWS SAM or similar tools for local Lambda testing"

# Help
help:
	@echo "Available targets:"
	@echo "  install  - Install Node.js dependencies"
	@echo "  build    - Install dependencies and create deployment package"
	@echo "  package  - Create deployment package (assumes dependencies installed)"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run local tests (not implemented)"
	@echo "  help     - Show this help message"