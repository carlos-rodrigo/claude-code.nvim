#!/bin/bash

# Deploy script for claude-code-intelligence
set -e

# Configuration
PROJECT_NAME="claude-code-intelligence"
DOCKER_IMAGE="${PROJECT_NAME}:latest"
DEPLOYMENT_TYPE="docker-compose" # Options: docker-compose, kubernetes, docker
BUILD_CONTEXT="../../"
DOCKER_COMPOSE_FILE="../docker-compose/docker-compose.yml"
KUBERNETES_MANIFESTS="./kubernetes"

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

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] COMMAND

Deploy claude-code-intelligence service

Commands:
    build           Build Docker image
    deploy          Deploy the service
    destroy         Stop and remove the service
    logs            Show service logs
    status          Show service status
    update          Update the service (build + deploy)
    backup          Create database backup
    restore         Restore from backup
    
Options:
    -t, --type TYPE     Deployment type (docker-compose|kubernetes|docker)
    -e, --env ENV       Environment (dev|staging|prod)
    -h, --help          Show this help
    -v, --verbose       Verbose output
    --no-build          Skip building Docker image
    --force             Force operation without confirmation

Examples:
    $0 -t docker-compose deploy
    $0 -t kubernetes -e prod deploy
    $0 --no-build deploy
    $0 logs
    $0 status

EOF
}

# Function to parse command line arguments
parse_args() {
    POSITIONAL=()
    while [[ $# -gt 0 ]]; do
        case $1 in
            -t|--type)
                DEPLOYMENT_TYPE="$2"
                shift 2
                ;;
            -e|--env)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            --no-build)
                NO_BUILD=true
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            *)
                POSITIONAL+=("$1")
                shift
                ;;
        esac
    done
    set -- "${POSITIONAL[@]}"
    
    COMMAND="$1"
}

# Function to validate deployment type
validate_deployment_type() {
    case $DEPLOYMENT_TYPE in
        docker-compose|kubernetes|docker)
            ;;
        *)
            log_error "Invalid deployment type: $DEPLOYMENT_TYPE"
            log_info "Valid types: docker-compose, kubernetes, docker"
            exit 1
            ;;
    esac
}

# Function to check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    case $DEPLOYMENT_TYPE in
        docker-compose)
            if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
                log_error "Docker Compose is not installed"
                exit 1
            fi
            ;;
        kubernetes)
            if ! command -v kubectl &> /dev/null; then
                log_error "kubectl is not installed"
                exit 1
            fi
            if ! kubectl cluster-info &> /dev/null; then
                log_error "kubectl is not connected to a Kubernetes cluster"
                exit 1
            fi
            ;;
        docker)
            if ! command -v docker &> /dev/null; then
                log_error "Docker is not installed"
                exit 1
            fi
            ;;
    esac
    
    log_success "Prerequisites check passed"
}

# Function to build Docker image
build_image() {
    if [[ "$NO_BUILD" == "true" ]]; then
        log_info "Skipping build (--no-build specified)"
        return
    fi
    
    log_info "Building Docker image: $DOCKER_IMAGE"
    
    # Set build arguments
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    
    docker build \
        --build-arg BUILD_DATE="$BUILD_DATE" \
        --build-arg GIT_COMMIT="$GIT_COMMIT" \
        --build-arg VERSION="1.0.0" \
        -t "$DOCKER_IMAGE" \
        -f ../docker/Dockerfile \
        "$BUILD_CONTEXT"
    
    log_success "Docker image built successfully"
}

# Function to deploy with Docker Compose
deploy_docker_compose() {
    log_info "Deploying with Docker Compose..."
    
    # Set environment variables
    export BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    export GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    
    # Create necessary directories
    mkdir -p ../../data ../../logs
    
    # Start services
    docker-compose -f "$DOCKER_COMPOSE_FILE" up -d
    
    # Wait for services to be ready
    log_info "Waiting for services to be ready..."
    sleep 10
    
    # Check service health
    if curl -f http://localhost:7345/health >/dev/null 2>&1; then
        log_success "Service deployed successfully"
        log_info "Service available at: http://localhost:7345"
    else
        log_error "Service health check failed"
        docker-compose -f "$DOCKER_COMPOSE_FILE" logs
        exit 1
    fi
}

# Function to deploy to Kubernetes
deploy_kubernetes() {
    log_info "Deploying to Kubernetes..."
    
    # Apply manifests in order
    kubectl apply -f "$KUBERNETES_MANIFESTS/namespace.yaml"
    kubectl apply -f "$KUBERNETES_MANIFESTS/configmap.yaml"
    kubectl apply -f "$KUBERNETES_MANIFESTS/rbac.yaml"
    kubectl apply -f "$KUBERNETES_MANIFESTS/pvc.yaml"
    kubectl apply -f "$KUBERNETES_MANIFESTS/deployment.yaml"
    kubectl apply -f "$KUBERNETES_MANIFESTS/service.yaml"
    
    # Wait for deployments to be ready
    log_info "Waiting for deployments to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/claude-code-intelligence -n claude-code-intelligence
    kubectl wait --for=condition=available --timeout=300s deployment/claude-ollama -n claude-code-intelligence
    
    # Get service information
    log_success "Service deployed successfully to Kubernetes"
    kubectl get services -n claude-code-intelligence
}

# Function to deploy with Docker
deploy_docker() {
    log_info "Deploying with Docker..."
    
    # Stop existing container if running
    docker stop claude-code-intelligence 2>/dev/null || true
    docker rm claude-code-intelligence 2>/dev/null || true
    
    # Run new container
    docker run -d \
        --name claude-code-intelligence \
        --restart unless-stopped \
        -p 8080:8080 \
        -v "$(pwd)/../../data:/app/data" \
        -v "$(pwd)/../../logs:/app/logs" \
        -e LOG_LEVEL=info \
        -e SERVER_PORT=8080 \
        "$DOCKER_IMAGE"
    
    # Wait for container to be ready
    log_info "Waiting for container to be ready..."
    sleep 10
    
    # Check container health
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        log_success "Service deployed successfully"
        log_info "Service available at: http://localhost:8080"
    else
        log_error "Service health check failed"
        docker logs claude-code-intelligence
        exit 1
    fi
}

# Function to deploy based on type
deploy() {
    case $DEPLOYMENT_TYPE in
        docker-compose)
            deploy_docker_compose
            ;;
        kubernetes)
            deploy_kubernetes
            ;;
        docker)
            deploy_docker
            ;;
    esac
}

# Function to show logs
show_logs() {
    log_info "Showing service logs..."
    
    case $DEPLOYMENT_TYPE in
        docker-compose)
            docker-compose -f "$DOCKER_COMPOSE_FILE" logs -f claude-code-intelligence
            ;;
        kubernetes)
            kubectl logs -f deployment/claude-code-intelligence -n claude-code-intelligence
            ;;
        docker)
            docker logs -f claude-code-intelligence
            ;;
    esac
}

# Function to show status
show_status() {
    log_info "Showing service status..."
    
    case $DEPLOYMENT_TYPE in
        docker-compose)
            docker-compose -f "$DOCKER_COMPOSE_FILE" ps
            ;;
        kubernetes)
            kubectl get pods,services -n claude-code-intelligence
            ;;
        docker)
            docker ps --filter name=claude-code-intelligence
            ;;
    esac
}

# Function to destroy/stop services
destroy() {
    if [[ "$FORCE" != "true" ]]; then
        read -p "Are you sure you want to destroy the deployment? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Aborted"
            exit 0
        fi
    fi
    
    log_info "Destroying deployment..."
    
    case $DEPLOYMENT_TYPE in
        docker-compose)
            docker-compose -f "$DOCKER_COMPOSE_FILE" down -v
            ;;
        kubernetes)
            kubectl delete -f "$KUBERNETES_MANIFESTS/" || true
            ;;
        docker)
            docker stop claude-code-intelligence || true
            docker rm claude-code-intelligence || true
            ;;
    esac
    
    log_success "Deployment destroyed"
}

# Function to create backup
create_backup() {
    log_info "Creating database backup..."
    
    case $DEPLOYMENT_TYPE in
        docker-compose)
            docker-compose -f "$DOCKER_COMPOSE_FILE" exec claude-code-intelligence curl -X POST http://localhost:8080/api/backup
            ;;
        kubernetes)
            POD=$(kubectl get pods -n claude-code-intelligence -l app.kubernetes.io/name=claude-code-intelligence -o jsonpath='{.items[0].metadata.name}')
            kubectl exec -n claude-code-intelligence "$POD" -- curl -X POST http://localhost:8080/api/backup
            ;;
        docker)
            docker exec claude-code-intelligence curl -X POST http://localhost:8080/api/backup
            ;;
    esac
}

# Main function
main() {
    parse_args "$@"
    
    if [[ -z "$COMMAND" ]]; then
        show_usage
        exit 1
    fi
    
    validate_deployment_type
    check_prerequisites
    
    case $COMMAND in
        build)
            build_image
            ;;
        deploy)
            build_image
            deploy
            ;;
        destroy)
            destroy
            ;;
        logs)
            show_logs
            ;;
        status)
            show_status
            ;;
        update)
            build_image
            destroy
            deploy
            ;;
        backup)
            create_backup
            ;;
        *)
            log_error "Unknown command: $COMMAND"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"