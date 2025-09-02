#!/bin/sh

# Entrypoint script for claude-code-intelligence
set -e

# Function to log messages
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to check if a service is ready
wait_for_service() {
    local host=$1
    local port=$2
    local service=$3
    local max_attempts=30
    local attempt=0

    log "Waiting for $service to be ready at $host:$port..."

    while [ $attempt -lt $max_attempts ]; do
        if nc -z "$host" "$port" 2>/dev/null; then
            log "$service is ready!"
            return 0
        fi
        
        attempt=$((attempt + 1))
        log "Attempt $attempt/$max_attempts: $service not ready yet, waiting..."
        sleep 2
    done

    log "ERROR: $service failed to become ready after $max_attempts attempts"
    return 1
}

# Function to setup directories
setup_directories() {
    log "Setting up directories..."
    
    # Ensure data directories exist
    mkdir -p /app/data/backups
    mkdir -p /app/data/cache
    mkdir -p /app/logs
    
    log "Directories setup completed"
}

# Function to validate configuration
validate_config() {
    log "Validating configuration..."
    
    # Check if configuration file exists
    if [ ! -f "/app/configs/config.yaml" ]; then
        log "WARNING: No config.yaml found, will use environment variables and defaults"
    else
        log "Configuration file found"
    fi
    
    # Validate required environment variables
    local required_vars=""
    local missing_vars=""
    
    # Check for critical environment variables
    if [ -z "$DATABASE_PATH" ]; then
        export DATABASE_PATH="/app/data/intelligence.db"
        log "Set default DATABASE_PATH=$DATABASE_PATH"
    fi
    
    if [ -z "$LOG_LEVEL" ]; then
        export LOG_LEVEL="info"
        log "Set default LOG_LEVEL=$LOG_LEVEL"
    fi
    
    if [ -z "$SERVER_PORT" ]; then
        export SERVER_PORT="8080"
        log "Set default SERVER_PORT=$SERVER_PORT"
    fi
    
    log "Configuration validation completed"
}

# Function to run pre-flight checks
pre_flight_checks() {
    log "Running pre-flight checks..."
    
    # Check if binary exists and is executable
    if [ ! -x "/app/claude-code-intelligence" ]; then
        log "ERROR: Binary not found or not executable"
        exit 1
    fi
    
    # Check disk space
    local available_space=$(df /app/data | tail -1 | awk '{print $4}')
    if [ "$available_space" -lt 1048576 ]; then  # Less than 1GB in KB
        log "WARNING: Low disk space available: ${available_space}KB"
    else
        log "Disk space check passed: ${available_space}KB available"
    fi
    
    # Check memory
    local available_memory=$(free | grep '^Mem:' | awk '{print $7}')
    if [ "$available_memory" -lt 524288 ]; then  # Less than 512MB in KB
        log "WARNING: Low memory available: ${available_memory}KB"
    else
        log "Memory check passed: ${available_memory}KB available"
    fi
    
    log "Pre-flight checks completed"
}

# Function to wait for dependencies
wait_for_dependencies() {
    log "Checking for dependencies..."
    
    # Wait for Ollama if configured
    if [ -n "$OLLAMA_HOST" ] && [ -n "$OLLAMA_PORT" ]; then
        wait_for_service "$OLLAMA_HOST" "$OLLAMA_PORT" "Ollama"
    else
        log "Ollama dependency check skipped (not configured)"
    fi
    
    # Wait for database if external
    if [ -n "$POSTGRES_HOST" ] && [ -n "$POSTGRES_PORT" ]; then
        wait_for_service "$POSTGRES_HOST" "$POSTGRES_PORT" "PostgreSQL"
    else
        log "PostgreSQL dependency check skipped (using SQLite or not configured)"
    fi
    
    log "Dependency checks completed"
}

# Function to run database migrations
run_migrations() {
    log "Running database migrations..."
    
    # This would typically run database migration commands
    # For now, we'll let the application handle initialization
    log "Database initialization will be handled by the application"
}

# Function to start the application with proper signal handling
start_application() {
    log "Starting claude-code-intelligence service..."
    log "Version: ${APP_VERSION:-unknown}"
    log "Build: ${BUILD_DATE:-unknown}"
    log "Git Commit: ${GIT_COMMIT:-unknown}"
    
    # Set up signal handlers for graceful shutdown
    trap 'log "Received SIGTERM, shutting down gracefully..."; kill -TERM $PID' TERM
    trap 'log "Received SIGINT, shutting down gracefully..."; kill -INT $PID' INT
    
    # Start the application in the background
    exec "$@" &
    PID=$!
    
    # Wait for the process to finish
    wait $PID
    EXIT_CODE=$?
    
    log "Application exited with code $EXIT_CODE"
    exit $EXIT_CODE
}

# Main execution
main() {
    log "=== Claude Code Intelligence Service Starting ==="
    log "Container started at $(date)"
    log "Running as user: $(id)"
    log "Working directory: $(pwd)"
    
    # Print environment info
    log "Environment variables:"
    log "  SERVER_PORT=${SERVER_PORT:-8080}"
    log "  LOG_LEVEL=${LOG_LEVEL:-info}"
    log "  DATABASE_PATH=${DATABASE_PATH:-/app/data/intelligence.db}"
    log "  OLLAMA_HOST=${OLLAMA_HOST:-localhost}"
    log "  OLLAMA_PORT=${OLLAMA_PORT:-11434}"
    
    # Run initialization steps
    setup_directories
    validate_config
    pre_flight_checks
    wait_for_dependencies
    run_migrations
    
    log "=== Initialization Complete ==="
    
    # Start the application
    start_application "$@"
}

# Run main function with all arguments
main "$@"