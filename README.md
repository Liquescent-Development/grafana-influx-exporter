# Grafana InfluxDB Monitoring Exporter

Monitor the reliability and performance of your InfluxDB datasources across multiple Grafana instances. Get comprehensive metrics on query success rates, response times, and datasource availability.

## 🚀 Quick Start (5 minutes)

### 1. Clone and Configure

```bash
git clone https://github.com/your-org/grafana-influx-exporter
cd grafana-influx-exporter
cp monitor-config.yaml.example monitor-config.yaml
```

### 2. Configure Your Grafana Instances

Edit `monitor-config.yaml`:

```yaml
mode: monitor

targets:
  - name: production
    url: https://your-grafana-prod.com
    datasources:
      - name: InfluxDB Production  # Use datasource name, UID auto-resolved
  
  - name: staging
    url: https://your-grafana-staging.com
    datasources:
      - name: InfluxDB Staging

test_queries:
  - name: basic_connectivity
    query: "SHOW DATABASES"
    interval: 60  # Check every minute
```

### 3. Set Authentication

```bash
# Option 1: Service account tokens (recommended)
export GRAFANA_production_TOKEN="your-prod-token"
export GRAFANA_staging_TOKEN="your-staging-token"

# Option 2: Username/password
export GRAFANA_production_USERNAME="username"
export GRAFANA_production_PASSWORD="password"
```

### 4. Start the Complete Stack

```bash
docker compose up -d
```

## 📊 Access Your Monitoring Dashboard

After startup (30-60 seconds):

- **🎯 Grafana Dashboard**: http://localhost:3000
  - Login: `admin` / `admin`
  - Auto-imported dashboard: "InfluxDB Datasource Monitoring"

- **📈 Prometheus**: http://localhost:9090 (raw metrics)
- **🔍 Exporter Health**: http://localhost:8080/health

## 📈 What You Get

### Real-Time Monitoring
- **Datasource Availability** - Which datasources are up/down across instances
- **Query Performance** - Response times, success rates, throughput
- **Error Analysis** - Detailed error tracking by type (timeout, auth, connection)
- **Historical Trends** - Performance over time for SLA tracking

### Key Metrics Dashboard
- **Overall Health Score** - System-wide availability percentage
- **Response Time Trends** - 95th percentile query performance
- **Success Rate Tracking** - Query success percentage over time
- **Error Breakdown** - Errors by type and Grafana instance
- **Instance Comparison** - Performance across different environments

## 🚨 Essential PromQL Queries

### Health Monitoring
```promql
# Overall system health (0-100%)
avg(grafana_influx_datasource_up) * 100

# Success rate percentage
(rate(grafana_influx_query_total{status="success"}[5m]) / rate(grafana_influx_query_total[5m])) * 100

# 95th percentile response time
histogram_quantile(0.95, rate(grafana_influx_query_duration_seconds_bucket[5m]))
```

### Alerting Queries
```promql
# Datasource down alert
grafana_influx_datasource_up == 0

# High error rate alert (>5%)
(rate(grafana_influx_query_errors_total[5m]) / rate(grafana_influx_query_total[5m])) * 100 > 5

# Slow response time alert (>10 seconds)
histogram_quantile(0.95, rate(grafana_influx_query_duration_seconds_bucket[5m])) > 10
```

## 🔧 Configuration Options

### Discover Datasources
If you don't know your datasource names:

```bash
# List all datasources from a Grafana instance
export GRAFANA_TOKEN="your-token"
make list-datasources GRAFANA_URL=https://your-grafana.com
```

### Custom Test Queries
Add application-specific monitoring queries:

```yaml
test_queries:
  - name: recent_data_check
    description: Verify recent data exists
    query: "SELECT * FROM your_measurement WHERE time >= now() - 10m LIMIT 1"
    interval: 300

  - name: specific_metric_check
    description: Check critical business metric
    query: "SELECT last(cpu_usage) FROM system_metrics"
    interval: 120
```

### Multi-Instance Authentication
Set different credentials per environment:

```bash
# Per-instance tokens
export GRAFANA_production_TOKEN="prod-token-here"
export GRAFANA_staging_TOKEN="staging-token-here"
export GRAFANA_development_TOKEN="dev-token-here"

# Global fallback
export GRAFANA_TOKEN="default-token"
```

## 📊 Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `grafana_influx_datasource_up` | Gauge | Datasource availability (1=up, 0=down) |
| `grafana_influx_query_duration_seconds` | Histogram | Query execution time distribution |
| `grafana_influx_query_total` | Counter | Total queries by status (success/error/no_data) |
| `grafana_influx_query_errors_total` | Counter | Errors by type (timeout/auth/connection) |
| `grafana_influx_query_last_success_timestamp` | Gauge | Unix timestamp of last successful query |

All metrics include labels:
- `grafana_instance` - Which Grafana instance
- `datasource_name` - Human-readable datasource name
- `datasource_uid` - Grafana datasource UID
- `query_name` - Test query identifier

## 🛠️ Development & Advanced Usage

### Local Development
```bash
# Setup development environment
make setup

# Run with hot reload
make dev

# Run tests and linting
make check
```

### Traditional Export Mode
For exporting actual InfluxDB data (not monitoring):

```yaml
# config.yaml for export mode
mode: export  # or omit mode field
grafana:
  url: "http://localhost:3000"
  datasource:
    name: "InfluxDB"
    
queries:
  - name: "cpu_metrics"
    query: "SELECT mean(usage_idle) FROM cpu WHERE time >= now() - 5m"
    metrics:
      - name: "cpu_usage_idle_percent"
        type: "gauge"
        value_column: "mean"
```

### Docker Deployment
```bash
# Build and run individual components
make docker-build
make docker-run

# Custom configuration
docker run -v ./config.yaml:/app/config.yaml grafana-influx-exporter
```

## 🚨 Troubleshooting

### Authentication Issues
```bash
# Verify credentials are set
echo $GRAFANA_production_TOKEN

# Test connection manually
make list-datasources GRAFANA_URL=https://your-grafana.com
```

### No Data in Dashboard
```bash
# Check exporter health
curl http://localhost:8080/health

# Verify metrics are generated
curl http://localhost:8080/metrics | grep grafana_influx

# Check logs
docker compose logs grafana-influx-exporter
```

### Enable Debug Logging
```bash
export LOG_LEVEL=debug
docker compose up --build
```

## 📚 Additional Resources

- **[ADVANCED.md](ADVANCED.md)** - Detailed PromQL examples, custom dashboards, and power-user features
- **[Architecture](ADVANCED.md#architecture)** - How the monitoring system works
- **[API Reference](ADVANCED.md#api-reference)** - REST endpoints and metrics details
- **[Contributing](ADVANCED.md#contributing)** - Development guidelines and code structure

## 💡 Use Cases

### DevOps Teams
- Monitor InfluxDB reliability across environments
- Track query performance degradation
- Set up alerts for datasource outages
- Generate SLA reports for stakeholders

### Site Reliability Engineering
- Implement proactive monitoring of data infrastructure
- Detect performance regressions before they impact users
- Track error patterns and connection issues
- Monitor cross-region datasource performance

### Data Teams
- Ensure data availability for analytics workflows
- Monitor query performance for data pipelines
- Track datasource health for compliance reporting
- Detect data freshness issues early

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run `make check` to ensure quality
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

---

**Need help?** Open an issue or check the [troubleshooting guide](ADVANCED.md#troubleshooting) for detailed solutions.