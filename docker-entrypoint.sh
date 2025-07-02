#!/bin/sh
set -e

# Docker entrypoint script for Grafana InfluxDB Exporter

# Function to log messages
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

# Validate required environment variables
validate_env() {
    if [ -z "$GRAFANA_TOKEN" ] && ( [ -z "$GRAFANA_USERNAME" ] || [ -z "$GRAFANA_PASSWORD" ] ); then
        log "ERROR: Either GRAFANA_TOKEN or both GRAFANA_USERNAME and GRAFANA_PASSWORD must be set"
        exit 1
    fi

    if [ -n "$GRAFANA_TOKEN" ] && ( [ -n "$GRAFANA_USERNAME" ] || [ -n "$GRAFANA_PASSWORD" ] ); then
        log "WARNING: Both token and basic auth provided. Token will take precedence."
    fi
}

# Check if config file exists
check_config() {
    config_path="${CONFIG_PATH:-/app/config.yaml}"
    
    if [ ! -f "$config_path" ]; then
        log "ERROR: Configuration file not found at $config_path"
        if [ -f "/app/config.yaml.example" ]; then
            log "Example configuration available at /app/config.yaml.example"
        fi
        exit 1
    fi
    
    log "Using configuration file: $config_path"
}

# Create log directory if specified and doesn't exist
setup_logging() {
    if [ -n "$LOG_FILE" ]; then
        log_dir=$(dirname "$LOG_FILE")
        if [ ! -d "$log_dir" ]; then
            mkdir -p "$log_dir"
            log "Created log directory: $log_dir"
        fi
    fi
}

# Handle signals for graceful shutdown
trap_signals() {
    local pid=$1
    
    # Function to handle shutdown
    shutdown() {
        log "Received shutdown signal, stopping gracefully..."
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill -TERM "$pid"
            wait "$pid"
        fi
        exit 0
    }
    
    trap shutdown TERM INT
}

# Main execution
main() {
    log "Starting Grafana InfluxDB Exporter"
    
    # Validate environment
    validate_env
    
    # Check configuration
    check_config
    
    # Setup logging
    setup_logging
    
    # Start the exporter directly
    log "Starting exporter directly"
    exec /app/grafana-influx-exporter "$@"
}

# Execute main if script is run directly
main "$@"