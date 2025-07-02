#!/bin/bash

# Grafana InfluxDB Exporter Project Setup Script
# This script creates the directory structure and empty files for the project

set -e

PROJECT_NAME="grafana-influx-exporter"

echo "Setting up $PROJECT_NAME project structure..."

# Create main directories
mkdir -p cmd/exporter
mkdir -p internal/config
mkdir -p internal/exporter
mkdir -p internal/grafana
mkdir -p systemd
mkdir -p logs

echo "✓ Created directories"

# Create Go source files
touch cmd/exporter/main.go
touch internal/config/config.go
touch internal/exporter/exporter.go
touch internal/exporter/exporter_test.go
touch internal/grafana/client.go

echo "✓ Created Go source files"

# Create configuration files
touch config.yaml
touch config.yaml.example
touch .env.example

echo "✓ Created configuration files"

# Create Docker files
touch Dockerfile
touch docker-compose.yml
touch docker-entrypoint.sh
touch prometheus.yml

echo "✓ Created Docker files"

# Create systemd service file
touch systemd/grafana-influx-exporter.service

echo "✓ Created systemd service file"

# Create development files
touch Makefile
touch .air.toml
touch .golangci.yml

echo "✓ Created development files"

# Create project files
touch README.md
touch .gitignore
touch go.sum

echo "✓ Created project files"

# Make scripts executable
chmod +x docker-entrypoint.sh

echo "✓ Set executable permissions"

# Display the structure
echo ""
echo "📁 Project structure created:"
echo ""
tree -a . 2>/dev/null || find . -type f | sed 's|^\./||' | sort

echo ""
echo "🎉 Project structure setup complete!"
echo ""
echo "Next steps:"
echo "1. Paste the file contents from the provided artifacts"
echo "2. Edit config.yaml with your Grafana details"
echo "3. Set environment variables for authentication"
echo "4. Run 'make setup' to install development tools"
echo "5. Run 'make run' or 'docker-compose up' to start"
echo ""
echo "Files to edit with your content:"
echo "├── cmd/exporter/main.go"
echo "├── internal/config/config.go"
echo "├── internal/exporter/exporter.go"
echo "├── internal/exporter/exporter_test.go"
echo "├── internal/grafana/client.go"
echo "├── config.yaml"
echo "├── config.yaml.example"
echo "├── .env.example"
echo "├── Dockerfile"
echo "├── docker-compose.yml"
echo "├── docker-entrypoint.sh"
echo "├── prometheus.yml"
echo "├── systemd/grafana-influx-exporter.service"
echo "├── Makefile"
echo "├── .air.toml"
echo "├── .golangci.yml"
echo "├── README.md"
echo "├── .gitignore"
echo "└── go.sum"