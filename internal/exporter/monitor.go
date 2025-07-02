package exporter

import (
	"strings"
	"sync"
	"time"

	"grafana-influx-exporter/internal/config"
	"grafana-influx-exporter/internal/grafana"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// MonitorExporter monitors the reliability and performance of InfluxDB datasources via Grafana
type MonitorExporter struct {
	config        *config.Config
	grafanaClient *grafana.Client
	logger        *logrus.Logger
	
	// Prometheus metrics for monitoring
	queryDuration    *prometheus.HistogramVec
	queryTotal       *prometheus.CounterVec
	queryErrors      *prometheus.CounterVec
	lastQuerySuccess *prometheus.GaugeVec
	
	// Instance identification
	grafanaInstance string
	
	// Mutex for thread safety
	mutex sync.RWMutex
}

// NewMonitor creates a new monitoring exporter
func NewMonitor(cfg *config.Config, client *grafana.Client, logger *logrus.Logger) (*MonitorExporter, error) {
	// Extract instance name from Grafana URL
	instance := extractInstanceName(cfg.Grafana.URL)
	
	e := &MonitorExporter{
		config:          cfg,
		grafanaClient:   client,
		logger:          logger,
		grafanaInstance: instance,
	}

	// Initialize monitoring metrics
	e.queryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "grafana_influx_query_duration_seconds",
			Help: "Duration of InfluxDB queries executed through Grafana",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"grafana_instance", "datasource_uid", "query_name", "status"},
	)

	e.queryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grafana_influx_query_total",
			Help: "Total number of InfluxDB queries executed through Grafana",
		},
		[]string{"grafana_instance", "datasource_uid", "query_name", "status"},
	)

	e.queryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grafana_influx_query_errors_total",
			Help: "Total number of InfluxDB query errors by type",
		},
		[]string{"grafana_instance", "datasource_uid", "query_name", "error_type"},
	)

	e.lastQuerySuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grafana_influx_query_last_success_timestamp",
			Help: "Unix timestamp of the last successful query execution",
		},
		[]string{"grafana_instance", "datasource_uid", "query_name"},
	)

	return e, nil
}

// Describe implements prometheus.Collector
func (e *MonitorExporter) Describe(ch chan<- *prometheus.Desc) {
	e.queryDuration.Describe(ch)
	e.queryTotal.Describe(ch)
	e.queryErrors.Describe(ch)
	e.lastQuerySuccess.Describe(ch)
}

// Collect implements prometheus.Collector
func (e *MonitorExporter) Collect(ch chan<- prometheus.Metric) {
	e.logger.Debug("Starting monitoring scrape")

	// Execute all configured queries and monitor their performance
	for _, queryConfig := range e.config.Queries {
		e.monitorQuery(queryConfig)
	}

	// Collect all metrics
	e.queryDuration.Collect(ch)
	e.queryTotal.Collect(ch)
	e.queryErrors.Collect(ch)
	e.lastQuerySuccess.Collect(ch)
}

// monitorQuery executes a query and records monitoring metrics
func (e *MonitorExporter) monitorQuery(queryConfig config.QueryConfig) {
	start := time.Now()
	status := "success"
	errorType := ""
	
	labels := prometheus.Labels{
		"grafana_instance": e.grafanaInstance,
		"datasource_uid":   e.config.Grafana.Datasource.UID,
		"query_name":       queryConfig.Name,
	}

	// Execute the query
	resp, err := e.grafanaClient.ExecuteQuery(queryConfig.Query)
	duration := time.Since(start).Seconds()
	
	if err != nil {
		// Determine error type
		status = "error"
		errorType = classifyError(err)
		
		e.logger.Errorf("Query '%s' failed: %v", queryConfig.Name, err)
		
		// Record error metrics
		errorLabels := prometheus.Labels{
			"grafana_instance": e.grafanaInstance,
			"datasource_uid":   e.config.Grafana.Datasource.UID,
			"query_name":       queryConfig.Name,
			"error_type":       errorType,
		}
		e.queryErrors.With(errorLabels).Inc()
	} else {
		// Check if we got data
		results, parseErr := grafana.ParseQueryResults(resp)
		if parseErr != nil {
			status = "error"
			errorType = "parse_error"
			e.logger.Errorf("Failed to parse results for query '%s': %v", queryConfig.Name, parseErr)
		} else if len(results) == 0 {
			status = "no_data"
			e.logger.Warnf("Query '%s' returned no data", queryConfig.Name)
		} else {
			// Success - update last success timestamp
			e.lastQuerySuccess.With(labels).SetToCurrentTime()
			e.logger.Debugf("Query '%s' successful, returned %d rows", queryConfig.Name, len(results))
		}
	}

	// Record duration and total metrics
	durationLabels := prometheus.Labels{
		"grafana_instance": e.grafanaInstance,
		"datasource_uid":   e.config.Grafana.Datasource.UID,
		"query_name":       queryConfig.Name,
		"status":           status,
	}
	e.queryDuration.With(durationLabels).Observe(duration)
	e.queryTotal.With(durationLabels).Inc()
}

// classifyError determines the type of error for detailed tracking
func classifyError(err error) string {
	errStr := err.Error()
	
	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "403"):
		return "auth_error"
	case strings.Contains(errStr, "404"):
		return "not_found"
	case strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503"):
		return "server_error"
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host"):
		return "connection_error"
	case strings.Contains(errStr, "parse"):
		return "parse_error"
	case strings.Contains(errStr, "query failed"):
		return "query_error"
	default:
		return "unknown"
	}
}

// extractInstanceName extracts a clean instance name from Grafana URL
func extractInstanceName(url string) string {
	// Remove protocol
	instance := strings.TrimPrefix(url, "https://")
	instance = strings.TrimPrefix(instance, "http://")
	
	// Remove path
	if idx := strings.Index(instance, "/"); idx > 0 {
		instance = instance[:idx]
	}
	
	// Remove port if it's default
	instance = strings.TrimSuffix(instance, ":443")
	instance = strings.TrimSuffix(instance, ":80")
	
	return instance
}