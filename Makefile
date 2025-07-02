# Grafana InfluxDB Exporter Makefile

# Variables
APP_NAME = grafana-influx-exporter
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go variables
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

# Docker variables
DOCKER_IMAGE ?= $(APP_NAME)
DOCKER_TAG ?= $(VERSION)

# Build flags
LDFLAGS = -X main.version=$(VERSION) \
          -X main.commit=$(COMMIT) \
          -X main.buildTime=$(BUILD_TIME) \
          -w -s

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/ dist/ vendor/
	go clean -cache

.PHONY: deps
deps: ## Download dependencies
	go mod download
	go mod tidy

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: lint
lint: ## Run linters
	golangci-lint run ./...

.PHONY: test
test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-coverage
test-coverage: test ## Run tests and show coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: build
build: deps ## Build the binary
	mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/exporter

.PHONY: build-all
build-all: ## Build binaries for multiple platforms
	@echo "Building for multiple platforms..."
	mkdir -p dist
	
	# Linux AMD64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-linux-amd64 ./cmd/exporter
	
	# Linux ARM64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
	go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-linux-arm64 ./cmd/exporter
	
	# Darwin AMD64 (Intel Mac)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-amd64 ./cmd/exporter
	
	# Darwin ARM64 (Apple Silicon)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
	go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-darwin-arm64 ./cmd/exporter
	
	# Windows AMD64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/exporter

.PHONY: run
run: build ## Build and run the application
	./bin/$(APP_NAME)

.PHONY: dev
dev: ## Run in development mode with hot reload (auto-installs air if needed)
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

.PHONY: docker-run
docker-run: ## Run Docker container
	docker run --rm -it \
		-p 8080:8080 \
		-v $(PWD)/config.yaml:/app/config.yaml:ro \
		-e GRAFANA_TOKEN="${GRAFANA_TOKEN}" \
		-e GRAFANA_USERNAME="${GRAFANA_USERNAME}" \
		-e GRAFANA_PASSWORD="${GRAFANA_PASSWORD}" \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: compose-up
compose-up: ## Start with docker-compose
	docker-compose up -d

.PHONY: compose-down
compose-down: ## Stop docker-compose
	docker-compose down

.PHONY: compose-logs
compose-logs: ## Show docker-compose logs
	docker-compose logs -f

.PHONY: validate-config
validate-config: build ## Validate configuration file
	./bin/$(APP_NAME) -validate-config

.PHONY: install-tools
install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest

.PHONY: setup-config
setup-config: ## Copy example configuration files
	@echo "Setting up configuration files..."
	@if [ ! -f config.yaml ]; then cp config.yaml.example config.yaml && echo "Created config.yaml from example"; fi
	@if [ ! -f monitor-config.yaml ]; then cp monitor-config.yaml.example monitor-config.yaml && echo "Created monitor-config.yaml from example"; fi
	@echo "Configuration files ready for editing"

.PHONY: health-check
health-check: ## Check if the service is healthy
	@curl -f http://localhost:8080/health || (echo "Health check failed" && exit 1)

.PHONY: metrics-check
metrics-check: ## Fetch metrics from the exporter
	@curl -s http://localhost:8080/metrics

.PHONY: release
release: clean test build-all ## Prepare a release
	@echo "Release $(VERSION) prepared in dist/"
	@ls -la dist/

.PHONY: list-datasources
list-datasources: ## List datasources from Grafana (Usage: make list-datasources GRAFANA_URL=https://your-grafana.com)
	@echo "Usage: make list-datasources GRAFANA_URL=https://your-grafana.com"
	@echo "Make sure to set GRAFANA_TOKEN or GRAFANA_USERNAME+GRAFANA_PASSWORD"
	@if [ -z "$(GRAFANA_URL)" ]; then echo "Error: GRAFANA_URL is required"; exit 1; fi
	@go run cmd/list-datasources/main.go $(GRAFANA_URL)

# Development helpers
.PHONY: setup
setup: install-tools deps setup-config ## Set up development environment
	@echo "Development environment set up complete!"
	@echo "1. Edit config.yaml with your Grafana details"
	@echo "2. Set GRAFANA_TOKEN or GRAFANA_USERNAME/GRAFANA_PASSWORD environment variables"
	@echo "3. Run 'make run' to start the exporter"

.PHONY: check
check: fmt lint test ## Run all checks (format, lint, test)