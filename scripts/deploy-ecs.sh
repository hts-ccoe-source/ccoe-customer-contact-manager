#!/bin/bash

# ECS deployment script for Email Distribution Orchestrator Service
set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CLUSTER_NAME="email-distribution-cluster"
SERVICE_NAME="email-distribution-orchestrator"
TASK_FAMILY="email-distribution-orchestrator"
IMAGE_NAME="email-distribution-orchestrator"
VERSION="${VERSION:-latest}"
REGISTRY="${REGISTRY:-}"
AWS_REGION="${AWS_REGION:-us-east-1}"
AWS_ACCOUNT_ID="${AWS_ACCOUNT_ID:-}"

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
    
    # Check AWS CLI
    if ! command -v aws &> /dev/null; then
        log_error "AWS CLI is not installed or not in PATH"
        exit 1
    fi
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        log_error "AWS credentials not configured or invalid"
        exit 1
    fi
    
    # Get AWS account ID if not provided
    if [[ -z "$AWS_ACCOUNT_ID" ]]; then
        AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
        log_info "Detected AWS Account ID: $AWS_ACCOUNT_ID"
    fi
    
    # Check jq
    if ! command -v jq &> /dev/null; then
        log_error "jq is not installed or not in PATH"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Function to create ECS cluster
create_cluster() {
    log_info "Creating ECS cluster: $CLUSTER_NAME"
    
    if aws ecs describe-clusters --clusters "$CLUSTER_NAME" --region "$AWS_REGION" &> /dev/null; then
        log_info "Cluster $CLUSTER_NAME already exists"
    else
        aws ecs create-cluster \
            --cluster-name "$CLUSTER_NAME" \
            --capacity-providers FARGATE FARGATE_SPOT \
            --default-capacity-provider-strategy capacityProvider=FARGATE,weight=1,base=2 capacityProvider=FARGATE_SPOT,weight=4 \
            --region "$AWS_REGION" \
            --tags key=Environment,value=production key=Service,value=email-distribution
        
        log_success "Cluster $CLUSTER_NAME created"
    fi
}

# Function to create CloudWatch log group
create_log_group() {
    log_info "Creating CloudWatch log group..."
    
    local log_group="/ecs/$TASK_FAMILY"
    
    if aws logs describe-log-groups --log-group-name-prefix "$log_group" --region "$AWS_REGION" | jq -e '.logGroups[] | select(.logGroupName == "'$log_group'")' &> /dev/null; then
        log_info "Log group $log_group already exists"
    else
        aws logs create-log-group \
            --log-group-name "$log_group" \
            --region "$AWS_REGION"
        
        aws logs put-retention-policy \
            --log-group-name "$log_group" \
            --retention-in-days 30 \
            --region "$AWS_REGION"
        
        log_success "Log group $log_group created"
    fi
}

# Function to update task definition
update_task_definition() {
    log_info "Updating task definition..."
    
    local image_uri="$IMAGE_NAME:$VERSION"
    if [[ -n "$REGISTRY" ]]; then
        image_uri="$REGISTRY/$IMAGE_NAME:$VERSION"
    fi
    
    # Update task definition with actual values
    local task_def_file="$PROJECT_DIR/ecs/task-definition.json"
    local updated_task_def="/tmp/task-definition-updated.json"
    
    jq --arg image "$image_uri" \
       --arg account "$AWS_ACCOUNT_ID" \
       --arg region "$AWS_REGION" \
       '.containerDefinitions[0].image = $image |
        .executionRoleArn = "arn:aws:iam::" + $account + ":role/ecsTaskExecutionRole" |
        .taskRoleArn = "arn:aws:iam::" + $account + ":role/EmailDistributionTaskRole" |
        .containerDefinitions[0].logConfiguration.options."awslogs-region" = $region |
        .containerDefinitions[1].logConfiguration.options."awslogs-region" = $region' \
       "$task_def_file" > "$updated_task_def"
    
    # Register task definition
    local task_def_arn
    task_def_arn=$(aws ecs register-task-definition \
        --cli-input-json file://"$updated_task_def" \
        --region "$AWS_REGION" \
        --query 'taskDefinition.taskDefinitionArn' \
        --output text)
    
    log_success "Task definition registered: $task_def_arn"
    echo "$task_def_arn" > /tmp/task-definition-arn
}

# Function to create or update service
create_or_update_service() {
    log_info "Creating or updating ECS service..."
    
    local task_def_arn
    task_def_arn=$(cat /tmp/task-definition-arn)
    
    # Check if service exists
    if aws ecs describe-services --cluster "$CLUSTER_NAME" --services "$SERVICE_NAME" --region "$AWS_REGION" | jq -e '.services[] | select(.serviceName == "'$SERVICE_NAME'" and .status != "INACTIVE")' &> /dev/null; then
        log_info "Updating existing service..."
        
        aws ecs update-service \
            --cluster "$CLUSTER_NAME" \
            --service "$SERVICE_NAME" \
            --task-definition "$task_def_arn" \
            --region "$AWS_REGION" \
            --force-new-deployment
        
        log_success "Service updated"
    else
        log_info "Creating new service..."
        
        # Update service definition with actual values
        local service_def_file="$PROJECT_DIR/ecs/service-definition.json"
        local updated_service_def="/tmp/service-definition-updated.json"
        
        jq --arg cluster "$CLUSTER_NAME" \
           --arg service "$SERVICE_NAME" \
           --arg task_def "$task_def_arn" \
           --arg account "$AWS_ACCOUNT_ID" \
           --arg region "$AWS_REGION" \
           '.cluster = $cluster |
            .serviceName = $service |
            .taskDefinition = $task_def |
            .loadBalancers[0].targetGroupArn = "arn:aws:elasticloadbalancing:" + $region + ":" + $account + ":targetgroup/orchestrator-tg/1234567890123456" |
            .serviceRegistries[0].registryArn = "arn:aws:servicediscovery:" + $region + ":" + $account + ":service/srv-orchestrator"' \
           "$service_def_file" > "$updated_service_def"
        
        aws ecs create-service \
            --cli-input-json file://"$updated_service_def" \
            --region "$AWS_REGION"
        
        log_success "Service created"
    fi
}

# Function to wait for deployment
wait_for_deployment() {
    log_info "Waiting for deployment to complete..."
    
    local max_wait=600  # 10 minutes
    local wait_time=0
    local check_interval=30
    
    while [[ $wait_time -lt $max_wait ]]; do
        local running_count
        running_count=$(aws ecs describe-services \
            --cluster "$CLUSTER_NAME" \
            --services "$SERVICE_NAME" \
            --region "$AWS_REGION" \
            --query 'services[0].runningCount' \
            --output text)
        
        local desired_count
        desired_count=$(aws ecs describe-services \
            --cluster "$CLUSTER_NAME" \
            --services "$SERVICE_NAME" \
            --region "$AWS_REGION" \
            --query 'services[0].desiredCount' \
            --output text)
        
        local deployment_status
        deployment_status=$(aws ecs describe-services \
            --cluster "$CLUSTER_NAME" \
            --services "$SERVICE_NAME" \
            --region "$AWS_REGION" \
            --query 'services[0].deployments[0].status' \
            --output text)
        
        log_info "Deployment status: $deployment_status, Running: $running_count/$desired_count"
        
        if [[ "$deployment_status" == "PRIMARY" && "$running_count" == "$desired_count" ]]; then
            log_success "Deployment completed successfully"
            return 0
        fi
        
        if [[ "$deployment_status" == "FAILED" ]]; then
            log_error "Deployment failed"
            return 1
        fi
        
        sleep $check_interval
        wait_time=$((wait_time + check_interval))
    done
    
    log_error "Deployment timed out after $max_wait seconds"
    return 1
}

# Function to verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    # Check service status
    local service_info
    service_info=$(aws ecs describe-services \
        --cluster "$CLUSTER_NAME" \
        --services "$SERVICE_NAME" \
        --region "$AWS_REGION")
    
    echo "$service_info" | jq '.services[0] | {serviceName, status, runningCount, desiredCount, taskDefinition}'
    
    # Check task health
    local task_arns
    task_arns=$(aws ecs list-tasks \
        --cluster "$CLUSTER_NAME" \
        --service-name "$SERVICE_NAME" \
        --region "$AWS_REGION" \
        --query 'taskArns' \
        --output text)
    
    if [[ -n "$task_arns" ]]; then
        log_info "Task health status:"
        aws ecs describe-tasks \
            --cluster "$CLUSTER_NAME" \
            --tasks $task_arns \
            --region "$AWS_REGION" \
            --query 'tasks[].{taskArn: taskArn, lastStatus: lastStatus, healthStatus: healthStatus, createdAt: createdAt}'
    fi
    
    log_success "Deployment verification completed"
}

# Function to show deployment info
show_deployment_info() {
    log_info "Deployment Information:"
    
    echo "Cluster: $CLUSTER_NAME"
    echo "Service: $SERVICE_NAME"
    echo "Task Family: $TASK_FAMILY"
    echo "Image: $IMAGE_NAME:$VERSION"
    echo "Region: $AWS_REGION"
    echo "Account: $AWS_ACCOUNT_ID"
    
    # Get service information
    echo ""
    echo "Service Details:"
    aws ecs describe-services \
        --cluster "$CLUSTER_NAME" \
        --services "$SERVICE_NAME" \
        --region "$AWS_REGION" \
        --query 'services[0] | {serviceName, status, runningCount, desiredCount, platformVersion, launchType}' \
        --output table
    
    # Get load balancer information
    echo ""
    echo "Load Balancer Targets:"
    local target_group_arn
    target_group_arn=$(aws ecs describe-services \
        --cluster "$CLUSTER_NAME" \
        --services "$SERVICE_NAME" \
        --region "$AWS_REGION" \
        --query 'services[0].loadBalancers[0].targetGroupArn' \
        --output text)
    
    if [[ "$target_group_arn" != "None" ]]; then
        aws elbv2 describe-target-health \
            --target-group-arn "$target_group_arn" \
            --region "$AWS_REGION" \
            --query 'TargetHealthDescriptions[].{Target: Target.Id, Port: Target.Port, Health: TargetHealth.State}' \
            --output table
    fi
}

# Function to rollback deployment
rollback_deployment() {
    log_warning "Rolling back deployment..."
    
    # Get previous task definition
    local previous_task_def
    previous_task_def=$(aws ecs describe-services \
        --cluster "$CLUSTER_NAME" \
        --services "$SERVICE_NAME" \
        --region "$AWS_REGION" \
        --query 'services[0].deployments[1].taskDefinition' \
        --output text)
    
    if [[ "$previous_task_def" == "None" || -z "$previous_task_def" ]]; then
        log_error "No previous task definition found for rollback"
        return 1
    fi
    
    log_info "Rolling back to: $previous_task_def"
    
    aws ecs update-service \
        --cluster "$CLUSTER_NAME" \
        --service "$SERVICE_NAME" \
        --task-definition "$previous_task_def" \
        --region "$AWS_REGION" \
        --force-new-deployment
    
    wait_for_deployment
    log_success "Rollback completed successfully"
}

# Function to cleanup
cleanup() {
    log_info "Cleaning up temporary files..."
    
    rm -f /tmp/task-definition-updated.json
    rm -f /tmp/service-definition-updated.json
    rm -f /tmp/task-definition-arn
}

# Function to delete deployment
delete_deployment() {
    log_warning "Deleting deployment..."
    
    read -p "Are you sure you want to delete the service and cluster? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Scale service to 0
        aws ecs update-service \
            --cluster "$CLUSTER_NAME" \
            --service "$SERVICE_NAME" \
            --desired-count 0 \
            --region "$AWS_REGION"
        
        # Wait for tasks to stop
        sleep 30
        
        # Delete service
        aws ecs delete-service \
            --cluster "$CLUSTER_NAME" \
            --service "$SERVICE_NAME" \
            --region "$AWS_REGION"
        
        # Delete cluster
        aws ecs delete-cluster \
            --cluster "$CLUSTER_NAME" \
            --region "$AWS_REGION"
        
        log_success "Deployment deleted"
    else
        log_info "Deletion cancelled"
    fi
}

# Main function
main() {
    log_info "Starting ECS deployment for Email Distribution Orchestrator"
    log_info "Cluster: $CLUSTER_NAME"
    log_info "Service: $SERVICE_NAME"
    log_info "Version: $VERSION"
    log_info "Region: $AWS_REGION"
    
    # Parse command line arguments
    ROLLBACK=false
    DELETE=false
    SKIP_VERIFY=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
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
            --cluster)
                CLUSTER_NAME="$2"
                shift 2
                ;;
            --service)
                SERVICE_NAME="$2"
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
            --region)
                AWS_REGION="$2"
                shift 2
                ;;
            --account-id)
                AWS_ACCOUNT_ID="$2"
                shift 2
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  --rollback      Rollback to previous deployment"
                echo "  --delete        Delete the deployment"
                echo "  --skip-verify   Skip deployment verification"
                echo "  --cluster       ECS cluster name"
                echo "  --service       ECS service name"
                echo "  --version       Image version tag"
                echo "  --registry      Docker registry URL"
                echo "  --region        AWS region"
                echo "  --account-id    AWS account ID"
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
    create_cluster
    create_log_group
    update_task_definition
    create_or_update_service
    wait_for_deployment
    
    if [[ "$SKIP_VERIFY" != true ]]; then
        verify_deployment
    fi
    
    show_deployment_info
    
    log_success "ECS deployment completed successfully!"
}

# Trap to cleanup on exit
trap cleanup EXIT

# Run main function
main "$@"