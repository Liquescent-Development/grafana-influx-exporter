package exporter

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"grafana-influx-exporter/internal/config"
	"grafana-influx-exporter/internal/grafana"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Exporter collects metrics from InfluxDB via Grafana and exports them for Prometheus
type Exporter struct {
	config        *config.Config
	grafanaClient *grafana.Client
	logger        *logrus.Logger
	
	// Prometheus metrics
	up            prometheus.Gauge
	totalScrapes  prometheus.Counter
	scrapeDuration prometheus.Histogram
	scrapeErrors  prometheus.Counter
	
	// Dynamic metrics storage
	metricsMutex sync.RWMutex
	metrics      map[string]prometheus.Collector
	
	// Last scrape information
	lastScrape time.Time
}

// New creates a new Exporter
func New(cfg *config.Config, client *grafana.Client, logger *logrus.Logger) (*Exporter, error) {
	e := &Exporter{
		config:        cfg,
		grafanaClient: client,
		logger:        logger,
		metrics:       make(map[string]prometheus.Collector),
	}

	// Initialize static metrics
	e.up = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "grafana_influx_exporter_up",
		Help: "Whether the last scrape of InfluxDB via Grafana was successful",
	})

	e.totalScrapes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "grafana_influx_exporter_total_scrapes",
		Help: "Total number of scrapes of InfluxDB via Grafana",
	})

	e.scrapeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "grafana_influx_exporter_scrape_duration_seconds",
		Help:    "Duration of scrapes of InfluxDB via Grafana",
		Buckets: prometheus.DefBuckets,
	})

	e.scrapeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "grafana_influx_exporter_scrape_errors_total",
		Help: "Total number of scrape errors",
	})

	return e, nil
}

// Describe implements prometheus.Collector
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.up.Describe(ch)
	e.totalScrapes.Describe(ch)
	e.scrapeDuration.Describe(ch)
	e.scrapeErrors.Describe(ch)

	e.metricsMutex.RLock()
	defer e.metricsMutex.RUnlock()
	
	for _, metric := range e.metrics {
		metric.Describe(ch)
	}
}

// Collect implements prometheus.Collector
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	e.totalScrapes.Inc()

	// Perform the scrape
	if err := e.scrape(); err != nil {
		e.logger.Errorf("Scrape failed: %v", err)
		e.up.Set(0)
		e.scrapeErrors.Inc()
	} else {
		e.up.Set(1)
		e.lastScrape = time.Now()
	}

	// Record scrape duration
	e.scrapeDuration.Observe(time.Since(start).Seconds())

	// Collect static metrics
	e.up.Collect(ch)
	e.totalScrapes.Collect(ch)
	e.scrapeDuration.Collect(ch)
	e.scrapeErrors.Collect(ch)

	// Collect dynamic metrics
	e.metricsMutex.RLock()
	defer e.metricsMutex.RUnlock()
	
	for _, metric := range e.metrics {
		metric.Collect(ch)
	}
}

// scrape executes all configured queries and updates metrics
func (e *Exporter) scrape() error {
	e.logger.Debug("Starting scrape")

	for _, queryConfig := range e.config.Queries {
		if err := e.executeQuery(queryConfig); err != nil {
			e.logger.Errorf("Query '%s' failed: %v", queryConfig.Name, err)
			return err
		}
	}

	e.logger.Debug("Scrape completed successfully")
	return nil
}

// executeQuery executes a single query and updates its associated metrics
func (e *Exporter) executeQuery(queryConfig config.QueryConfig) error {
	e.logger.Debugf("Executing query: %s", queryConfig.Name)

	// Execute the query via Grafana
	resp, err := e.grafanaClient.ExecuteQuery(queryConfig.Query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Parse the results
	results, err := grafana.ParseQueryResults(resp)
	if err != nil {
		return fmt.Errorf("failed to parse query results: %w", err)
	}

	e.logger.Debugf("Query '%s' returned %d rows", queryConfig.Name, len(results))

	// Process results for each configured metric
	for _, metricConfig := range queryConfig.Metrics {
		if err := e.updateMetric(metricConfig, results); err != nil {
			e.logger.Errorf("Failed to update metric '%s': %v", metricConfig.Name, err)
			continue
		}
	}

	return nil
}

// updateMetric updates a Prometheus metric based on query results
func (e *Exporter) updateMetric(metricConfig config.MetricConfig, results []map[string]interface{}) error {
	// Get or create the metric
	metric, err := e.getOrCreateMetric(metricConfig)
	if err != nil {
		return err
	}

	// Clear previous values for gauge metrics
	if metricConfig.Type == "gauge" {
		if gaugeVec, ok := metric.(*prometheus.GaugeVec); ok {
			gaugeVec.Reset()
		}
	}

	// Process each result row
	for _, row := range results {
		// Extract the value
		value, err := e.extractValue(row, metricConfig.ValueColumn)
		if err != nil {
			e.logger.Warnf("Failed to extract value for metric '%s': %v", metricConfig.Name, err)
			continue
		}

		// Extract labels
		labels := make(prometheus.Labels)
		
		// Add global labels
		for key, value := range e.config.GlobalLabels {
			labels[key] = value
		}
		
		// Add configured labels
		for labelName, sourceColumn := range metricConfig.Labels {
			if labelValue, ok := row[sourceColumn]; ok {
				labels[labelName] = fmt.Sprintf("%v", labelValue)
			}
		}

		// Update the metric based on its type
		switch metricConfig.Type {
		case "gauge":
			if gaugeVec, ok := metric.(*prometheus.GaugeVec); ok {
				gaugeVec.With(labels).Set(value)
			} else if gauge, ok := metric.(prometheus.Gauge); ok {
				gauge.Set(value)
			}
		case "counter":
			if counterVec, ok := metric.(*prometheus.CounterVec); ok {
				counterVec.With(labels).Add(value)
			} else if counter, ok := metric.(prometheus.Counter); ok {
				// Note: Counters can only be incremented, not set to arbitrary values
				// This might not be suitable for all use cases
				counter.Add(value)
			}
		default:
			return fmt.Errorf("unsupported metric type: %s", metricConfig.Type)
		}
	}

	return nil
}

// getOrCreateMetric gets an existing metric or creates a new one
func (e *Exporter) getOrCreateMetric(metricConfig config.MetricConfig) (prometheus.Collector, error) {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()

	// Check if metric already exists
	if metric, exists := e.metrics[metricConfig.Name]; exists {
		return metric, nil
	}

	// Create new metric based on type
	var metric prometheus.Collector
	
	// Determine if we need a vector (multiple label dimensions) or simple metric
	hasLabels := len(metricConfig.Labels) > 0 || len(e.config.GlobalLabels) > 0
	
	switch metricConfig.Type {
	case "gauge":
		if hasLabels {
			// Collect all possible label names
			labelNames := make([]string, 0, len(metricConfig.Labels)+len(e.config.GlobalLabels))
			for labelName := range metricConfig.Labels {
				labelNames = append(labelNames, labelName)
			}
			for labelName := range e.config.GlobalLabels {
				labelNames = append(labelNames, labelName)
			}
			
			metric = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: metricConfig.Name,
					Help: metricConfig.Description,
				},
				labelNames,
			)
		} else {
			metric = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: metricConfig.Name,
					Help: metricConfig.Description,
				},
			)
		}
	case "counter":
		if hasLabels {
			labelNames := make([]string, 0, len(metricConfig.Labels)+len(e.config.GlobalLabels))
			for labelName := range metricConfig.Labels {
				labelNames = append(labelNames, labelName)
			}
			for labelName := range e.config.GlobalLabels {
				labelNames = append(labelNames, labelName)
			}
			
			metric = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: metricConfig.Name,
					Help: metricConfig.Description,
				},
				labelNames,
			)
		} else {
			metric = prometheus.NewCounter(
				prometheus.CounterOpts{
					Name: metricConfig.Name,
					Help: metricConfig.Description,
				},
			)
		}
	default:
		return nil, fmt.Errorf("unsupported metric type: %s", metricConfig.Type)
	}

	// Store the metric
	e.metrics[metricConfig.Name] = metric
	
	return metric, nil
}

// extractValue extracts a numeric value from a result row
func (e *Exporter) extractValue(row map[string]interface{}, column string) (float64, error) {
	rawValue, exists := row[column]
	if !exists {
		return 0, fmt.Errorf("column '%s' not found in result", column)
	}

	if rawValue == nil {
		return 0, fmt.Errorf("column '%s' is null", column)
	}

	// Try to convert to float64
	switch v := rawValue.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		// Try to parse string as float
		if val, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return val, nil
		}
		return 0, fmt.Errorf("cannot convert string '%s' to float64", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}