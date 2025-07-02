# Advanced Configuration and Usage

This document provides comprehensive guidance for power users, developers, and advanced monitoring scenarios.

## 📊 Comprehensive PromQL Query Reference

### Health & Availability Monitoring

#### Overall System Health
```promql
# Current status of all datasources (1 = up, 0 = down)
grafana_influx_datasource_up

# Overall health score across all instances (0-100%)
avg(grafana_influx_datasource_up) * 100

# Datasources currently down
grafana_influx_datasource_up == 0

# Count of healthy vs unhealthy datasources
count(grafana_influx_datasource_up == 1) / count(grafana_influx_datasource_up) * 100
```

#### Success Rate Analysis
```promql
# Success rate percentage over last 5 minutes
(rate(grafana_influx_query_total{status="success"}[5m]) / rate(grafana_influx_query_total[5m])) * 100

# Success rate by Grafana instance
(rate(grafana_influx_query_total{status="success"}[5m]) / rate(grafana_influx_query_total[5m])) * 100 by (grafana_instance)

# Success rate by datasource
(rate(grafana_influx_query_total{status="success"}[5m]) / rate(grafana_influx_query_total[5m])) * 100 by (datasource_name)

# Queries returning no data (different from errors)
rate(grafana_influx_query_total{status="no_data"}[5m])
```

#### Time-Based Health Tracking
```promql
# Hours since last successful query (for alerting)
(time() - grafana_influx_query_last_success_timestamp) / 3600

# Datasources with no successful queries in last hour
(time() - grafana_influx_query_last_success_timestamp) > 3600

# Uptime percentage over different time periods
avg_over_time(grafana_influx_datasource_up[1h]) * 100  # Last hour
avg_over_time(grafana_influx_datasource_up[24h]) * 100 # Last day
avg_over_time(grafana_influx_datasource_up[7d]) * 100  # Last week
```

### Performance & Responsiveness Analysis

#### Response Time Metrics
```promql
# Average query duration in milliseconds
rate(grafana_influx_query_duration_seconds_sum[5m]) / rate(grafana_influx_query_duration_seconds_count[5m]) * 1000

# 50th, 95th, and 99th percentile response times
histogram_quantile(0.50, rate(grafana_influx_query_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(grafana_influx_query_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(grafana_influx_query_duration_seconds_bucket[5m]))

# Response time by instance and datasource
histogram_quantile(0.95, rate(grafana_influx_query_duration_seconds_bucket[5m])) by (grafana_instance, datasource_name)
```

#### Performance Degradation Detection
```promql
# Current vs historical response time comparison (detect 50% increase)
(
  rate(grafana_influx_query_duration_seconds_sum[5m]) / rate(grafana_influx_query_duration_seconds_count[5m])
) / (
  rate(grafana_influx_query_duration_seconds_sum[1h] offset 1h) / rate(grafana_influx_query_duration_seconds_count[1h] offset 1h)
) > 1.5

# Queries exceeding SLA thresholds
increase(grafana_influx_query_duration_seconds_bucket{le="5"}[5m]) - increase(grafana_influx_query_duration_seconds_bucket{le="2"}[5m])
```

#### Throughput Analysis
```promql
# Queries per second by instance and status
rate(grafana_influx_query_total[1m])

# Total query volume trends
sum(rate(grafana_influx_query_total[5m])) by (grafana_instance)

# Query volume by hour of day (for capacity planning)
sum(rate(grafana_influx_query_total[1h])) by (hour(time()))
```

### Error Analysis & Troubleshooting

#### Error Rate Monitoring
```promql
# Overall error rate percentage
(rate(grafana_influx_query_errors_total[5m]) / rate(grafana_influx_query_total[5m])) * 100

# Error rate by error type
(rate(grafana_influx_query_errors_total[5m]) / rate(grafana_influx_query_total[5m])) * 100 by (error_type)

# Top error types across all instances
topk(5, sum by (error_type) (rate(grafana_influx_query_errors_total[5m])))
```

#### Specific Error Type Analysis
```promql
# Connection-related errors
rate(grafana_influx_query_errors_total{error_type=~"connection_error|timeout"}[5m])

# Authentication and authorization errors
rate(grafana_influx_query_errors_total{error_type="auth_error"}[5m])

# Server-side errors (500, 502, 503)
rate(grafana_influx_query_errors_total{error_type="server_error"}[5m])

# Query parsing errors
rate(grafana_influx_query_errors_total{error_type="query_error"}[5m])

# Network-related issues
rate(grafana_influx_query_errors_total{error_type=~"connection_error|timeout|not_found"}[5m])
```

### Instance & Environment Comparison

#### Cross-Instance Performance
```promql
# Compare average response times across instances
avg by (grafana_instance) (rate(grafana_influx_query_duration_seconds_sum[5m]) / rate(grafana_influx_query_duration_seconds_count[5m]))

# Compare success rates across instances
(rate(grafana_influx_query_total{status="success"}[5m]) / rate(grafana_influx_query_total[5m])) * 100 by (grafana_instance)

# Compare error rates across instances
sum by (grafana_instance) (rate(grafana_influx_query_errors_total[5m])) / sum by (grafana_instance) (rate(grafana_influx_query_total[5m])) * 100
```

#### Datasource Comparison
```promql
# Performance by datasource across all instances
avg by (datasource_name) (rate(grafana_influx_query_duration_seconds_sum[5m]) / rate(grafana_influx_query_duration_seconds_count[5m]))

# Most reliable datasources
topk(10, avg_over_time(grafana_influx_datasource_up[24h]) by (datasource_name))

# Most problematic datasources
bottomk(5, (rate(grafana_influx_query_total{status="success"}[1h]) / rate(grafana_influx_query_total[1h])) by (datasource_name))
```

## 🚨 Advanced Alerting Rules

### Critical Alerts

#### Datasource Outage
```yaml
groups:
- name: datasource.rules
  rules:
  - alert: DatasourceDown
    expr: grafana_influx_datasource_up == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "InfluxDB datasource {{ $labels.datasource_name }} is down"
      description: "Datasource {{ $labels.datasource_name }} on {{ $labels.grafana_instance }} has been down for more than 1 minute"
```

#### High Error Rate
```yaml
  - alert: HighErrorRate
    expr: (rate(grafana_influx_query_errors_total[5m]) / rate(grafana_influx_query_total[5m])) * 100 > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      description: "Error rate is {{ $value }}% on {{ $labels.grafana_instance }}/{{ $labels.datasource_name }}"
```

#### Performance Degradation
```yaml
  - alert: SlowQueryPerformance
    expr: histogram_quantile(0.95, rate(grafana_influx_query_duration_seconds_bucket[5m])) > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Slow query performance detected"
      description: "95th percentile query time is {{ $value }}s on {{ $labels.grafana_instance }}"
```

### Warning Alerts

#### Data Staleness
```yaml
  - alert: StaleData
    expr: (time() - grafana_influx_query_last_success_timestamp) / 3600 > 1
    for: 0m
    labels:
      severity: warning
    annotations:
      summary: "No successful queries for {{ $labels.datasource_name }}"
      description: "Last successful query was {{ $value }} hours ago"
```

#### Success Rate Drop
```yaml
  - alert: LowSuccessRate
    expr: (rate(grafana_influx_query_total{status="success"}[10m]) / rate(grafana_influx_query_total[10m])) * 100 < 95
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Success rate below threshold"
      description: "Success rate is {{ $value }}% for {{ $labels.datasource_name }}"
```

## 🎨 Custom Dashboard Creation

### Dashboard Panel Types

#### Single Stat Panels
```json
{
  "type": "stat",
  "targets": [
    {"expr": "avg(grafana_influx_datasource_up) * 100", "refId": "A"}
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "percent",
      "thresholds": {
        "steps": [
          {"color": "red", "value": 0},
          {"color": "yellow", "value": 95},
          {"color": "green", "value": 99}
        ]
      }
    }
  }
}
```

#### Time Series Panels
```json
{
  "type": "timeseries",
  "targets": [
    {
      "expr": "histogram_quantile(0.95, rate(grafana_influx_query_duration_seconds_bucket[5m]))",
      "legendFormat": "95th percentile - {{grafana_instance}}"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "unit": "s",
      "custom": {"drawStyle": "line", "fillOpacity": 0}
    }
  }
}
```

#### Table Panels
```json
{
  "type": "table",
  "targets": [
    {"expr": "grafana_influx_datasource_up", "format": "table", "instant": true}
  ],
  "transformations": [
    {
      "id": "organize",
      "options": {
        "renameByName": {
          "grafana_instance": "Instance",
          "datasource_name": "Datasource",
          "Value": "Status"
        }
      }
    }
  ]
}
```

### Template Variables

#### Instance Selection
```yaml
- name: grafana_instance
  type: query
  query: label_values(grafana_influx_datasource_up, grafana_instance)
  refresh: 1
  includeAll: true
  allValue: ".*"
```

#### Datasource Selection
```yaml
- name: datasource_name
  type: query
  query: label_values(grafana_influx_datasource_up{grafana_instance=~"$grafana_instance"}, datasource_name)
  refresh: 1
  includeAll: true
  allValue: ".*"
```

## 🏗️ Architecture

### Component Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Grafana A     │    │   Grafana B     │    │   Grafana C     │
│  (Production)   │    │   (Staging)     │    │ (Development)   │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          │ InfluxQL Queries     │ InfluxQL Queries     │ InfluxQL Queries
          │                      │                      │
          └──────────────┬───────────────┬──────────────┘
                         │               │
                         ▼               ▼
                ┌─────────────────────────────────┐
                │   Multi-Target Monitor          │
                │  ┌─────────────────────────────┐ │
                │  │ ┌─────────┐ ┌─────────────┐ │ │
                │  │ │ Target  │ │ Test Queries │ │ │
                │  │ │Monitor  │ │  Scheduler   │ │ │
                │  │ └─────────┘ └─────────────┘ │ │
                │  └─────────────────────────────┘ │
                └─────────────┬───────────────────┘
                              │ Prometheus Metrics
                              ▼
                    ┌─────────────────┐
                    │   Prometheus    │
                    │     Server      │
                    └─────────┬───────┘
                              │ PromQL Queries
                              ▼
                    ┌─────────────────┐
                    │    Grafana      │
                    │   Dashboard     │
                    └─────────────────┘
```

### Data Flow

1. **Configuration Loading**: Multi-target monitor loads configuration with Grafana instances and test queries
2. **Client Creation**: For each datasource, create authenticated Grafana client with UID resolution
3. **Query Scheduling**: Test queries run on configurable intervals per query type
4. **Metric Collection**: Each query execution generates metrics for duration, status, and errors
5. **Prometheus Export**: Metrics exposed on `/metrics` endpoint with comprehensive labels
6. **Dashboard Visualization**: Grafana queries Prometheus for monitoring dashboards

### Metric Labels

All metrics include these labels for multi-dimensional analysis:

- `grafana_instance`: Target name from configuration (e.g., "production", "staging")
- `datasource_uid`: Grafana's internal datasource identifier
- `datasource_name`: Human-readable datasource name
- `query_name`: Test query identifier from configuration

Additional labels by metric type:
- `status`: success/no_data/error (for query_total)
- `error_type`: timeout/auth_error/connection_error/etc (for query_errors_total)

## 🔧 Advanced Configuration

### Complex Multi-Environment Setup

```yaml
mode: monitor

targets:
  # Production cluster
  - name: prod-us-east
    url: https://grafana-prod-use1.company.com
    datasources:
      - name: InfluxDB Production Primary
      - name: InfluxDB Production Secondary
      - name: InfluxDB Logs
  
  - name: prod-eu-west
    url: https://grafana-prod-euw1.company.com
    datasources:
      - name: InfluxDB Production EU
      - name: InfluxDB Compliance
  
  # Non-production environments
  - name: staging
    url: https://grafana-staging.company.com
    datasources:
      - name: InfluxDB Staging
  
  - name: development
    url: https://grafana-dev.company.com
    datasources:
      - name: InfluxDB Development

# Comprehensive test suite
test_queries:
  # Basic connectivity (frequent)
  - name: basic_connectivity
    description: Verify basic database connectivity
    query: "SHOW DATABASES"
    interval: 30
  
  # Data freshness check (regular)
  - name: recent_data_check
    description: Ensure recent data exists
    query: "SELECT * FROM system_metrics WHERE time >= now() - 5m LIMIT 1"
    interval: 120
  
  # Business metric validation (less frequent)
  - name: business_kpi_check
    description: Validate critical business metrics are updating
    query: "SELECT last(revenue_total) FROM business_metrics WHERE time >= now() - 1h"
    interval: 300
  
  # Performance test (periodic)
  - name: performance_test
    description: Test query performance with larger dataset
    query: "SELECT mean(cpu_usage) FROM system_metrics WHERE time >= now() - 1h GROUP BY time(1m)"
    interval: 600

# Exporter configuration
exporter:
  port: 8080
  path: /metrics
  health_path: /health

# Structured logging
logging:
  level: info
  format: json
```

### Environment-Specific Authentication

```bash
# Production environment
export GRAFANA_prod_us_east_TOKEN="prod-use1-service-account-token"
export GRAFANA_prod_eu_west_TOKEN="prod-euw1-service-account-token"

# Non-production environments with basic auth
export GRAFANA_staging_USERNAME="staging-monitor"
export GRAFANA_staging_PASSWORD="staging-password"
export GRAFANA_development_USERNAME="dev-monitor"
export GRAFANA_development_PASSWORD="dev-password"

# Global fallback for any unconfigured targets
export GRAFANA_TOKEN="default-readonly-token"
```

### Docker Deployment with External Config

```yaml
# docker-compose.production.yml
version: '3.8'
services:
  grafana-influx-exporter:
    build: .
    container_name: datasource-monitor
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/app/production-config.yaml
      - LOG_LEVEL=info
      - GRAFANA_prod_us_east_TOKEN_FILE=/run/secrets/prod_token
      - GRAFANA_staging_TOKEN_FILE=/run/secrets/staging_token
    volumes:
      - ./production-monitor-config.yaml:/app/production-config.yaml:ro
      - monitoring_data:/app/logs
    secrets:
      - prod_token
      - staging_token
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

secrets:
  prod_token:
    external: true
  staging_token:
    external: true

volumes:
  monitoring_data:
```

## 🧪 Testing & Validation

### Manual Testing

```bash
# Test datasource discovery
make list-datasources GRAFANA_URL=https://your-grafana.com

# Validate configuration
make validate-config

# Test single query manually
curl -X POST https://your-grafana.com/api/ds/query \
  -H "Authorization: Bearer $GRAFANA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "queries": [{
      "refId": "A",
      "datasource": {"uid": "your-datasource-uid"},
      "query": "SHOW DATABASES",
      "rawQuery": true,
      "resultFormat": "time_series"
    }],
    "from": "now-5m",
    "to": "now"
  }'
```

### Health Check Validation

```bash
# Check exporter health
curl http://localhost:8080/health

# Verify metrics are being generated
curl http://localhost:8080/metrics | grep grafana_influx

# Test specific metric
curl -s http://localhost:8080/metrics | grep grafana_influx_datasource_up
```

## 🚨 Troubleshooting Guide

### Common Issues & Solutions

#### Authentication Problems

**Symptoms**: 401/403 errors, "auth_error" in metrics
```bash
# Debug authentication
export LOG_LEVEL=debug
docker compose up --build

# Test credentials manually
make list-datasources GRAFANA_URL=https://your-grafana.com

# Verify token permissions
curl -H "Authorization: Bearer $GRAFANA_TOKEN" https://your-grafana.com/api/datasources
```

#### Query Parsing Errors

**Symptoms**: "query_error" in metrics, InfluxQL syntax errors
```bash
# Check query syntax in Grafana UI first
# Verify query works manually:
curl -X POST https://your-grafana.com/api/ds/query \
  -H "Authorization: Bearer $GRAFANA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"queries":[{"datasource":{"uid":"your-uid"},"query":"SHOW DATABASES","rawQuery":true}]}'
```

#### Network Connectivity Issues

**Symptoms**: "connection_error", "timeout" errors
```bash
# Test network connectivity
curl -I https://your-grafana.com/api/health

# Check DNS resolution
nslookup your-grafana.com

# Test from container
docker exec -it grafana-influx-exporter curl -I https://your-grafana.com
```

#### Missing Metrics

**Symptoms**: Empty dashboard, no data in Prometheus
```bash
# Verify exporter is running
curl http://localhost:8080/health

# Check if specific datasources are configured
curl http://localhost:8080/metrics | grep 'grafana_instance="your-instance"'

# Validate configuration syntax
make validate-config
```

### Debug Configuration

```yaml
# debug-config.yaml - Minimal config for troubleshooting
mode: monitor

targets:
  - name: debug
    url: https://your-grafana.com
    datasources:
      - name: InfluxDB  # Use exact name from Grafana

test_queries:
  - name: simple_test
    query: "SHOW DATABASES"
    interval: 30

exporter:
  port: 8080

logging:
  level: debug  # Enable detailed logging
  format: json
```

## 🔗 API Reference

### REST Endpoints

#### Health Check
```
GET /health
```
Returns service health status and basic configuration info.

#### Metrics Export
```
GET /metrics
```
Prometheus-formatted metrics for all configured targets.

#### Service Information
```
GET /info
```
Service metadata including version, mode, and target count.

### Metric Definitions

#### `grafana_influx_datasource_up`
- **Type**: Gauge
- **Range**: 0 (down) to 1 (up)
- **Labels**: grafana_instance, datasource_uid, datasource_name
- **Purpose**: Binary availability indicator

#### `grafana_influx_query_duration_seconds`
- **Type**: Histogram
- **Buckets**: [0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30]
- **Labels**: grafana_instance, datasource_uid, datasource_name, query_name, status
- **Purpose**: Query execution time distribution

#### `grafana_influx_query_total`
- **Type**: Counter
- **Labels**: grafana_instance, datasource_uid, datasource_name, query_name, status
- **Values**: status ∈ {success, no_data, error}
- **Purpose**: Total query count by outcome

#### `grafana_influx_query_errors_total`
- **Type**: Counter
- **Labels**: grafana_instance, datasource_uid, datasource_name, query_name, error_type
- **Values**: error_type ∈ {timeout, auth_error, connection_error, server_error, query_error, parse_error, unknown}
- **Purpose**: Error categorization and tracking

#### `grafana_influx_query_last_success_timestamp`
- **Type**: Gauge
- **Range**: Unix timestamp
- **Labels**: grafana_instance, datasource_uid, datasource_name, query_name
- **Purpose**: Data freshness and staleness detection

## 🤝 Contributing

### Development Setup

```bash
# Clone and setup
git clone https://github.com/your-org/grafana-influx-exporter
cd grafana-influx-exporter
make setup

# Development workflow
make dev          # Run with hot reload
make test         # Run test suite
make lint         # Code quality checks
make build        # Build binary
make docker-build # Build container
```

### Code Structure

```
├── cmd/
│   ├── exporter/           # Main application entry point
│   └── list-datasources/   # Utility for datasource discovery
├── internal/
│   ├── config/            # Configuration management
│   │   ├── config.go      # Traditional export mode config
│   │   └── monitor_config.go # Monitoring mode config
│   ├── exporter/          # Core monitoring logic
│   │   ├── exporter.go    # Traditional export mode
│   │   ├── monitor.go     # Single-target monitoring
│   │   └── multi_monitor.go # Multi-target monitoring
│   └── grafana/           # Grafana API client
│       └── client.go      # HTTP client with auth & query execution
├── dashboards/            # Grafana dashboard definitions
├── grafana-config/        # Grafana provisioning configuration
└── docs/                  # Additional documentation
```

### Adding New Features

1. **New Metric Types**: Add to `multi_monitor.go` with appropriate Prometheus metric type
2. **New Error Types**: Extend `classifyError()` function in monitor implementations
3. **New Query Types**: Add query validation and execution logic
4. **New Authentication**: Extend client authentication in `grafana/client.go`

### Testing Guidelines

```bash
# Unit tests
go test ./internal/...

# Integration tests (requires test Grafana instance)
export TEST_GRAFANA_URL="https://test-grafana.com"
export TEST_GRAFANA_TOKEN="test-token"
go test -tags=integration ./...

# Load testing
go test -bench=. ./internal/exporter/
```

---

**Questions or issues?** Open an issue or contribute improvements via pull request!