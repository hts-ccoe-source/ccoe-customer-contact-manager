#!/bin/bash

# Build script for Email Distribution Orchestrator Service
set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
IMAGE_NAME="email-distribution-orchestrator"
VERSION="${VERSION:-latest}"
REGISTRY="${REGISTRY:-}"
PLATFORM="${PLATFORM:-linux/amd64}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.21"
    if ! printf '%s\n%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V -C; then
        log_error "Go version $GO_VERSION is less than required $REQUIRED_VERSION"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Function to run tests
run_tests() {
    log_info "Running tests..."
    
    cd "$PROJECT_DIR"
    
    # Run unit tests
    go test -v -race -coverprofile=coverage.out ./...
    
    # Generate coverage report
    go tool cover -html=coverage.out -o coverage.html
    
    # Check coverage threshold
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    THRESHOLD=80
    
    if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
        log_warning "Test coverage ($COVERAGE%) is below threshold ($THRESHOLD%)"
    else
        log_success "Test coverage: $COVERAGE%"
    fi
    
    log_success "Tests completed"
}

# Function to build binary
build_binary() {
    log_info "Building Go binary..."
    
    cd "$PROJECT_DIR"
    
    # Set build variables
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    
    # Build flags
    LDFLAGS="-w -s"
    LDFLAGS="$LDFLAGS -X main.Version=$VERSION"
    LDFLAGS="$LDFLAGS -X main.BuildTime=$BUILD_TIME"
    LDFLAGS="$LDFLAGS -X main.GitCommit=$GIT_COMMIT"
    LDFLAGS="$LDFLAGS -X main.GitBranch=$GIT_BRANCH"
    
    # Build binary
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -a -installsuffix cgo \
        -ldflags "$LDFLAGS" \
        -o orchestrator .
    
    log_success "Binary built successfully"
}

# Function to build Docker image
build_docker_image() {
    log_info "Building Docker image..."
    
    cd "$PROJECT_DIR"
    
    # Build image
    docker build \
        --platform "$PLATFORM" \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
        --build-arg GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")" \
        -t "$IMAGE_NAME:$VERSION" \
        -t "$IMAGE_NAME:latest" \
        .
    
    # Tag with registry if provided
    if [[ -n "$REGISTRY" ]]; then
        docker tag "$IMAGE_NAME:$VERSION" "$REGISTRY/$IMAGE_NAME:$VERSION"
        docker tag "$IMAGE_NAME:latest" "$REGISTRY/$IMAGE_NAME:latest"
    fi
    
    log_success "Docker image built: $IMAGE_NAME:$VERSION"
}

# Function to scan image for vulnerabilities
scan_image() {
    log_info "Scanning Docker image for vulnerabilities..."
    
    # Check if trivy is available
    if command -v trivy &> /dev/null; then
        trivy image --exit-code 1 --severity HIGH,CRITICAL "$IMAGE_NAME:$VERSION"
        log_success "Security scan completed"
    else
        log_warning "Trivy not found, skipping security scan"
    fi
}

# Function to push image
push_image() {
    if [[ -n "$REGISTRY" ]]; then
        log_info "Pushing Docker image to registry..."
        
        docker push "$REGISTRY/$IMAGE_NAME:$VERSION"
        docker push "$REGISTRY/$IMAGE_NAME:latest"
        
        log_success "Image pushed to registry: $REGISTRY/$IMAGE_NAME:$VERSION"
    else
        log_info "No registry specified, skipping push"
    fi
}

# Function to generate build info
generate_build_info() {
    log_info "Generating build information..."
    
    cat > "$PROJECT_DIR/build-info.json" << EOF
{
  "version": "$VERSION",
  "buildTime": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "gitCommit": "$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")",
  "gitBranch": "$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")",
  "goVersion": "$(go version | awk '{print $3}')",
  "platform": "$PLATFORM",
  "imageName": "$IMAGE_NAME:$VERSION"
}
EOF
    
    log_success "Build info generated: build-info.json"
}

# Function to cleanup
cleanup() {
    log_info "Cleaning up..."
    
    cd "$PROJECT_DIR"
    
    # Remove binary
    rm -f orchestrator
    
    # Remove test artifacts
    rm -f coverage.out coverage.html
    
    log_success "Cleanup completed"
}

# Main function
main() {
    log_info "Starting build process for Email Distribution Orchestrator"
    log_info "Version: $VERSION"
    log_info "Platform: $PLATFORM"
    
    # Parse command line arguments
    SKIP_TESTS=false
    SKIP_SCAN=false
    PUSH=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-tests)
                SKIP_TESTS=true
                shift
                ;;
            --skip-scan)
                SKIP_SCAN=true
                shift
                ;;
            --push)
                PUSH=true
                shift
                ;;
            --registry)
                REGISTRY="$2"
                shift 2
                ;;
            --version)
                VERSION="$2"
                shift 2
                ;;
            --platform)
                PLATFORM="$2"
                shift 2
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  --skip-tests    Skip running tests"
                echo "  --skip-scan     Skip security scanning"
                echo "  --push          Push image to registry"
                echo "  --registry      Docker registry URL"
                echo "  --version       Image version tag"
                echo "  --platform      Target platform (default: linux/amd64)"
                echo "  -h, --help      Show this help message"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Execute build steps
    check_prerequisites
    
    if [[ "$SKIP_TESTS" != true ]]; then
        run_tests
    fi
    
    build_binary
    build_docker_image
    
    if [[ "$SKIP_SCAN" != true ]]; then
        scan_image
    fi
    
    if [[ "$PUSH" == true ]]; then
        push_image
    fi
    
    generate_build_info
    cleanup
    
    log_success "Build completed successfully!"
    log_info "Image: $IMAGE_NAME:$VERSION"
    
    if [[ -n "$REGISTRY" ]]; then
        log_info "Registry: $REGISTRY/$IMAGE_NAME:$VERSION"
    fi
}

# Trap to cleanup on exit
trap cleanup EXIT

# Run main function
main "$@"