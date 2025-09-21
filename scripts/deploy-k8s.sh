#!/bin/bash

# Kubernetes deployment script for Email Distribution Orchestrator Service
set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
NAMESPACE="email-distribution"
DEPLOYMENT_NAME="orchestrator"
IMAGE_NAME="email-distribution-orchestrator"
VERSION="${VERSION:-latest}"
REGISTRY="${REGISTRY:-}"
ENVIRONMENT="${ENVIRONMENT:-production}"

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
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check helm (optional)
    if command -v helm &> /dev/null; then
        log_info "Helm detected: $(helm version --short)"
    else
        log_warning "Helm not found, using kubectl for deployment"
    fi
    
    log_success "Prerequisites check passed"
}

# Function to create namespace
create_namespace() {
    log_info "Creating namespace: $NAMESPACE"
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace $NAMESPACE already exists"
    else
        kubectl apply -f "$PROJECT_DIR/k8s/namespace.yaml"
        log_success "Namespace $NAMESPACE created"
    fi
}

# Function to apply RBAC
apply_rbac() {
    log_info "Applying RBAC configuration..."
    
    # Update service account annotation with actual account ID
    if [[ -n "${AWS_ACCOUNT_ID:-}" ]]; then
        sed -i.bak "s/ACCOUNT_ID/$AWS_ACCOUNT_ID/g" "$PROJECT_DIR/k8s/rbac.yaml"
    fi
    
    kubectl apply -f "$PROJECT_DIR/k8s/rbac.yaml"
    log_success "RBAC configuration applied"
}

# Function to apply secrets
apply_secrets() {
    log_info "Applying secrets..."
    
    # Check if secrets exist
    if kubectl get secret orchestrator-secrets -n "$NAMESPACE" &> /dev/null; then
        log_warning "Secrets already exist, skipping creation"
        log_warning "Update secrets manually if needed"
    else
        kubectl apply -f "$PROJECT_DIR/k8s/secret.yaml"
        log_success "Secrets applied"
    fi
}

# Function to apply configmaps
apply_configmaps() {
    log_info "Applying ConfigMaps..."
    
    kubectl apply -f "$PROJECT_DIR/k8s/configmap.yaml"
    log_success "ConfigMaps applied"
}

# Function to update deployment image
update_deployment_image() {
    log_info "Updating deployment image..."
    
    local image_tag="$IMAGE_NAME:$VERSION"
    if [[ -n "$REGISTRY" ]]; then
        image_tag="$REGISTRY/$IMAGE_NAME:$VERSION"
    fi
    
    # Update deployment YAML with new image
    sed -i.bak "s|image: email-distribution-orchestrator:latest|image: $image_tag|g" "$PROJECT_DIR/k8s/deployment.yaml"
    
    log_info "Using image: $image_tag"
}

# Function to apply deployment
apply_deployment() {
    log_info "Applying deployment..."
    
    kubectl apply -f "$PROJECT_DIR/k8s/deployment.yaml"
    
    # Wait for rollout to complete
    log_info "Waiting for deployment rollout..."
    kubectl rollout status deployment/$DEPLOYMENT_NAME -n "$NAMESPACE" --timeout=600s
    
    log_success "Deployment applied and rolled out successfully"
}

# Function to apply services
apply_services() {
    log_info "Applying services..."
    
    kubectl apply -f "$PROJECT_DIR/k8s/service.yaml"
    log_success "Services applied"
}

# Function to apply ingress
apply_ingress() {
    log_info "Applying ingress..."
    
    kubectl apply -f "$PROJECT_DIR/k8s/ingress.yaml"
    log_success "Ingress applied"
}

# Function to apply autoscaling
apply_autoscaling() {
    log_info "Applying autoscaling configuration..."
    
    kubectl apply -f "$PROJECT_DIR/k8s/hpa.yaml"
    log_success "Autoscaling configuration applied"
}

# Function to verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    # Check pod status
    log_info "Checking pod status..."
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=orchestrator
    
    # Check service endpoints
    log_info "Checking service endpoints..."
    kubectl get endpoints -n "$NAMESPACE"
    
    # Check ingress
    log_info "Checking ingress..."
    kubectl get ingress -n "$NAMESPACE"
    
    # Wait for pods to be ready
    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=orchestrator -n "$NAMESPACE" --timeout=300s
    
    # Test health endpoint
    log_info "Testing health endpoint..."
    if kubectl get service orchestrator-internal -n "$NAMESPACE" &> /dev/null; then
        kubectl port-forward service/orchestrator-internal 8081:8081 -n "$NAMESPACE" &
        PORT_FORWARD_PID=$!
        sleep 5
        
        if curl -f http://localhost:8081/health &> /dev/null; then
            log_success "Health check passed"
        else
            log_warning "Health check failed"
        fi
        
        kill $PORT_FORWARD_PID 2>/dev/null || true
    fi
    
    log_success "Deployment verification completed"
}

# Function to show deployment info
show_deployment_info() {
    log_info "Deployment Information:"
    
    echo "Namespace: $NAMESPACE"
    echo "Deployment: $DEPLOYMENT_NAME"
    echo "Image: $IMAGE_NAME:$VERSION"
    echo "Environment: $ENVIRONMENT"
    
    # Get service information
    echo ""
    echo "Services:"
    kubectl get services -n "$NAMESPACE" -o wide
    
    # Get ingress information
    echo ""
    echo "Ingress:"
    kubectl get ingress -n "$NAMESPACE" -o wide
    
    # Get pod information
    echo ""
    echo "Pods:"
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=orchestrator -o wide
    
    # Get HPA information
    echo ""
    echo "Horizontal Pod Autoscaler:"
    kubectl get hpa -n "$NAMESPACE"
}

# Function to rollback deployment
rollback_deployment() {
    log_warning "Rolling back deployment..."
    
    kubectl rollout undo deployment/$DEPLOYMENT_NAME -n "$NAMESPACE"
    kubectl rollout status deployment/$DEPLOYMENT_NAME -n "$NAMESPACE" --timeout=300s
    
    log_success "Deployment rolled back successfully"
}

# Function to cleanup
cleanup() {
    log_info "Cleaning up temporary files..."
    
    # Restore original files
    if [[ -f "$PROJECT_DIR/k8s/deployment.yaml.bak" ]]; then
        mv "$PROJECT_DIR/k8s/deployment.yaml.bak" "$PROJECT_DIR/k8s/deployment.yaml"
    fi
    
    if [[ -f "$PROJECT_DIR/k8s/rbac.yaml.bak" ]]; then
        mv "$PROJECT_DIR/k8s/rbac.yaml.bak" "$PROJECT_DIR/k8s/rbac.yaml"
    fi
}

# Function to delete deployment
delete_deployment() {
    log_warning "Deleting deployment..."
    
    read -p "Are you sure you want to delete the deployment? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kubectl delete -f "$PROJECT_DIR/k8s/" --ignore-not-found=true
        log_success "Deployment deleted"
    else
        log_info "Deletion cancelled"
    fi
}

# Main function
main() {
    log_info "Starting Kubernetes deployment for Email Distribution Orchestrator"
    log_info "Environment: $ENVIRONMENT"
    log_info "Namespace: $NAMESPACE"
    log_info "Version: $VERSION"
    
    # Parse command line arguments
    DRY_RUN=false
    ROLLBACK=false
    DELETE=false
    SKIP_VERIFY=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --rollback)
                ROLLBACK=true
                shift
                ;;
            --delete)
                DELETE=true
                shift
                ;;
            --skip-verify)
                SKIP_VERIFY=true
                shift
                ;;
            --namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            --version)
                VERSION="$2"
                shift 2
                ;;
            --registry)
                REGISTRY="$2"
                shift 2
                ;;
            --environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  --dry-run       Show what would be deployed without applying"
                echo "  --rollback      Rollback to previous deployment"
                echo "  --delete        Delete the deployment"
                echo "  --skip-verify   Skip deployment verification"
                echo "  --namespace     Kubernetes namespace (default: email-distribution)"
                echo "  --version       Image version tag (default: latest)"
                echo "  --registry      Docker registry URL"
                echo "  --environment   Deployment environment (default: production)"
                echo "  -h, --help      Show this help message"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Handle special operations
    if [[ "$DELETE" == true ]]; then
        delete_deployment
        exit 0
    fi
    
    if [[ "$ROLLBACK" == true ]]; then
        rollback_deployment
        exit 0
    fi
    
    # Execute deployment steps
    check_prerequisites
    
    if [[ "$DRY_RUN" == true ]]; then
        log_info "DRY RUN MODE - No changes will be applied"
        kubectl apply --dry-run=client -f "$PROJECT_DIR/k8s/"
        exit 0
    fi
    
    create_namespace
    apply_rbac
    apply_secrets
    apply_configmaps
    update_deployment_image
    apply_deployment
    apply_services
    apply_ingress
    apply_autoscaling
    
    if [[ "$SKIP_VERIFY" != true ]]; then
        verify_deployment
    fi
    
    show_deployment_info
    
    log_success "Kubernetes deployment completed successfully!"
}

# Trap to cleanup on exit
trap cleanup EXIT

# Run main function
main "$@"