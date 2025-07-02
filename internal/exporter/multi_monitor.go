package exporter

import (
	"fmt"
	"sync"
	"time"

	"grafana-influx-exporter/internal/config"
	"grafana-influx-exporter/internal/grafana"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// MultiMonitor monitors multiple Grafana instances and their InfluxDB datasources
type MultiMonitor struct {
	config   *config.MonitorConfig
	logger   *logrus.Logger
	monitors map[string]*targetMonitor
	
	// Shared metrics across all targets
	queryDuration    *prometheus.HistogramVec
	queryTotal       *prometheus.CounterVec
	queryErrors      *prometheus.CounterVec
	lastQuerySuccess *prometheus.GaugeVec
	datasourceUp     *prometheus.GaugeVec
	
	// Control
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// targetMonitor represents monitoring for a single Grafana instance
type targetMonitor struct {
	name    string
	clients map[string]*grafana.Client // datasource_uid -> client
	target  config.MonitorTarget
	logger  *logrus.Logger
}

// NewMultiMonitor creates a monitor for multiple Grafana instances
func NewMultiMonitor(cfg *config.MonitorConfig, logger *logrus.Logger) (*MultiMonitor, error) {
	m := &MultiMonitor{
		config:   cfg,
		logger:   logger,
		monitors: make(map[string]*targetMonitor),
		stopChan: make(chan struct{}),
	}

	// Initialize shared metrics
	m.queryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "grafana_influx_query_duration_seconds",
			Help: "Duration of InfluxDB queries executed through Grafana",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"grafana_instance", "datasource_uid", "datasource_name", "query_name", "status"},
	)

	m.queryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grafana_influx_query_total",
			Help: "Total number of InfluxDB queries executed through Grafana",
		},
		[]string{"grafana_instance", "datasource_uid", "datasource_name", "query_name", "status"},
	)

	m.queryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grafana_influx_query_errors_total",
			Help: "Total number of InfluxDB query errors by type",
		},
		[]string{"grafana_instance", "datasource_uid", "datasource_name", "query_name", "error_type"},
	)

	m.lastQuerySuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grafana_influx_query_last_success_timestamp",
			Help: "Unix timestamp of the last successful query execution",
		},
		[]string{"grafana_instance", "datasource_uid", "datasource_name", "query_name"},
	)

	m.datasourceUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grafana_influx_datasource_up",
			Help: "Whether the InfluxDB datasource is available (1 = up, 0 = down)",
		},
		[]string{"grafana_instance", "datasource_uid", "datasource_name"},
	)

	// Initialize target monitors
	for _, target := range cfg.Targets {
		tm := &targetMonitor{
			name:    target.Name,
			clients: make(map[string]*grafana.Client),
			target:  target,
			logger:  logger,
		}

		// Create Grafana clients for each datasource
		for i, ds := range target.Datasources {
			clientCfg := config.GrafanaConfig{
				URL: target.URL,
				Datasource: config.DatasourceConfig{
					UID:  ds.UID,
					Name: ds.Name,
				},
				Token:    target.Token,
				Username: target.Username,
				Password: target.Password,
			}

			client, err := grafana.NewClient(clientCfg, logger)
			if err != nil {
				dsIdentifier := ds.UID
				if dsIdentifier == "" {
					dsIdentifier = ds.Name
				}
				return nil, fmt.Errorf("failed to create client for %s/%s: %w", target.Name, dsIdentifier, err)
			}

			// Use the resolved UID as the key
			resolvedUID := client.GetDatasourceUID()
			tm.clients[resolvedUID] = client
			
			// Update the actual datasource config with resolved UID if it was missing
			if target.Datasources[i].UID == "" {
				target.Datasources[i].UID = resolvedUID
			}
		}

		m.monitors[target.Name] = tm
	}

	return m, nil
}

// Start begins monitoring all targets
func (m *MultiMonitor) Start() {
	m.logger.Info("Starting multi-target monitoring")

	// Start a goroutine for each test query interval
	queryIntervals := make(map[int][]config.TestQuery)
	for _, query := range m.config.TestQueries {
		queryIntervals[query.Interval] = append(queryIntervals[query.Interval], query)
	}

	// Start monitoring loops for each unique interval
	for interval, queries := range queryIntervals {
		m.wg.Add(1)
		go m.monitorLoop(interval, queries)
	}
}

// Stop gracefully stops all monitoring
func (m *MultiMonitor) Stop() {
	m.logger.Info("Stopping multi-target monitoring")
	close(m.stopChan)
	m.wg.Wait()
}

// monitorLoop runs queries at the specified interval
func (m *MultiMonitor) monitorLoop(interval int, queries []config.TestQuery) {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Run immediately on start
	m.runQueries(queries)

	for {
		select {
		case <-ticker.C:
			m.runQueries(queries)
		case <-m.stopChan:
			return
		}
	}
}

// runQueries executes queries against all targets
func (m *MultiMonitor) runQueries(queries []config.TestQuery) {
	var wg sync.WaitGroup

	// Run queries for each target in parallel
	for _, tm := range m.monitors {
		for dsUID, client := range tm.clients {
			for _, query := range queries {
				wg.Add(1)
				go func(tm *targetMonitor, dsUID string, client *grafana.Client, query config.TestQuery) {
					defer wg.Done()
					m.executeMonitorQuery(tm, dsUID, client, query)
				}(tm, dsUID, client, query)
			}
		}
	}

	wg.Wait()
}

// executeMonitorQuery executes a single monitoring query
func (m *MultiMonitor) executeMonitorQuery(tm *targetMonitor, dsUID string, client *grafana.Client, query config.TestQuery) {
	start := time.Now()
	status := "success"
	errorType := ""
	
	// Find datasource name
	dsName := dsUID
	for _, ds := range tm.target.Datasources {
		if ds.UID == dsUID {
			dsName = ds.Name
			break
		}
	}

	labels := prometheus.Labels{
		"grafana_instance": tm.name,
		"datasource_uid":   dsUID,
		"datasource_name":  dsName,
		"query_name":       query.Name,
	}

	// Execute the query
	resp, err := client.ExecuteQuery(query.Query)
	duration := time.Since(start).Seconds()
	
	if err != nil {
		status = "error"
		errorType = classifyError(err)
		
		m.logger.WithFields(logrus.Fields{
			"grafana_instance": tm.name,
			"datasource_uid":   dsUID,
			"query_name":       query.Name,
			"error":            err,
			"error_type":       errorType,
		}).Error("Query failed")
		
		// Record error metrics
		errorLabels := prometheus.Labels{
			"grafana_instance": tm.name,
			"datasource_uid":   dsUID,
			"datasource_name":  dsName,
			"query_name":       query.Name,
			"error_type":       errorType,
		}
		m.queryErrors.With(errorLabels).Inc()
		
		// Mark datasource as down
		dsLabels := prometheus.Labels{
			"grafana_instance": tm.name,
			"datasource_uid":   dsUID,
			"datasource_name":  dsName,
		}
		m.datasourceUp.With(dsLabels).Set(0)
	} else {
		// Check if we got data
		results, parseErr := grafana.ParseQueryResults(resp)
		if parseErr != nil {
			status = "error"
			errorType = "parse_error"
			m.logger.WithFields(logrus.Fields{
				"grafana_instance": tm.name,
				"datasource_uid":   dsUID,
				"query_name":       query.Name,
				"error":            parseErr,
			}).Error("Failed to parse query results")
		} else if len(results) == 0 {
			status = "no_data"
			m.logger.WithFields(logrus.Fields{
				"grafana_instance": tm.name,
				"datasource_uid":   dsUID,
				"query_name":       query.Name,
			}).Warn("Query returned no data")
		} else {
			// Success - update metrics
			m.lastQuerySuccess.With(labels).SetToCurrentTime()
			
			// Mark datasource as up
			dsLabels := prometheus.Labels{
				"grafana_instance": tm.name,
				"datasource_uid":   dsUID,
				"datasource_name":  dsName,
			}
			m.datasourceUp.With(dsLabels).Set(1)
			
			m.logger.WithFields(logrus.Fields{
				"grafana_instance": tm.name,
				"datasource_uid":   dsUID,
				"query_name":       query.Name,
				"rows":             len(results),
				"duration_seconds": duration,
			}).Debug("Query successful")
		}
	}

	// Record duration and total metrics
	durationLabels := prometheus.Labels{
		"grafana_instance": tm.name,
		"datasource_uid":   dsUID,
		"datasource_name":  dsName,
		"query_name":       query.Name,
		"status":           status,
	}
	m.queryDuration.With(durationLabels).Observe(duration)
	m.queryTotal.With(durationLabels).Inc()
}

// Describe implements prometheus.Collector
func (m *MultiMonitor) Describe(ch chan<- *prometheus.Desc) {
	m.queryDuration.Describe(ch)
	m.queryTotal.Describe(ch)
	m.queryErrors.Describe(ch)
	m.lastQuerySuccess.Describe(ch)
	m.datasourceUp.Describe(ch)
}

// Collect implements prometheus.Collector
func (m *MultiMonitor) Collect(ch chan<- prometheus.Metric) {
	m.queryDuration.Collect(ch)
	m.queryTotal.Collect(ch)
	m.queryErrors.Collect(ch)
	m.lastQuerySuccess.Collect(ch)
	m.datasourceUp.Collect(ch)
}