package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"grafana-influx-exporter/internal/config"
	"grafana-influx-exporter/internal/exporter"
	"grafana-influx-exporter/internal/grafana"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func main() {
	// Setup logging
	logger := logrus.New()
	
	// Load configuration
	configPath := getEnvOrDefault("CONFIG_PATH", "/app/config.yaml")
	
	// First, detect the mode by reading the config file
	mode := detectMode(configPath)
	
	if mode == "monitor" {
		runMonitorMode(configPath, logger)
	} else {
		runExportMode(configPath, logger)
	}
}

// detectMode reads the config file to determine the mode
func detectMode(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "export" // Default to export mode
	}
	
	var modeCheck struct {
		Mode string `yaml:"mode"`
	}
	
	if err := yaml.Unmarshal(data, &modeCheck); err != nil {
		return "export" // Default to export mode
	}
	
	if modeCheck.Mode == "monitor" {
		return "monitor"
	}
	
	return "export"
}

// runMonitorMode runs the exporter in monitoring mode
func runMonitorMode(configPath string, logger *logrus.Logger) {
	cfg, err := config.LoadMonitorConfig(configPath)
	if err != nil {
		logger.Fatalf("Failed to load monitor config: %v", err)
	}
	
	// Configure logging based on config
	setupLogging(logger, cfg.Logging)
	
	// Override with environment variable if set
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		if level, err := logrus.ParseLevel(envLevel); err == nil {
			logger.SetLevel(level)
			logger.Infof("Log level set to %s from environment", envLevel)
		}
	}
	
	logger.Info("Starting Grafana InfluxDB Monitor (Monitoring Mode)")
	logger.Infof("Config loaded from: %s", configPath)
	logger.Infof("Monitoring %d Grafana instances", len(cfg.Targets))
	
	// Create multi-monitor
	monitor, err := exporter.NewMultiMonitor(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to create monitor: %v", err)
	}
	
	// Register with Prometheus
	prometheus.MustRegister(monitor)
	
	// Start monitoring
	monitor.Start()
	
	// Setup HTTP server
	mux := http.NewServeMux()
	
	// Metrics endpoint
	mux.Handle(cfg.Exporter.Path, promhttp.Handler())
	
	// Health check endpoint
	mux.HandleFunc(cfg.Exporter.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","mode":"monitor","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})
	
	// Info endpoint
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"name":"grafana-influx-exporter",
			"version":"1.0.0",
			"mode":"monitor",
			"targets":%d,
			"test_queries":%d
		}`, len(cfg.Targets), len(cfg.TestQueries))
	})
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Exporter.Port),
		Handler: mux,
	}
	
	// Start server
	go func() {
		logger.Infof("Starting HTTP server on port %d", cfg.Exporter.Port)
		logger.Infof("Metrics available at: http://localhost:%d%s", cfg.Exporter.Port, cfg.Exporter.Path)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("HTTP server failed: %v", err)
		}
	}()
	
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	logger.Info("Shutting down...")
	
	// Stop monitoring
	monitor.Stop()
	
	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	} else {
		logger.Info("Server gracefully stopped")
	}
}

// runExportMode runs the exporter in traditional export mode
func runExportMode(configPath string, logger *logrus.Logger) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Configure logging based on config
	setupLogging(logger, cfg.Logging)

	logger.Info("Starting Grafana InfluxDB Exporter (Export Mode)")
	logger.Infof("Config loaded from: %s", configPath)

	// Create Grafana client
	grafanaClient, err := grafana.NewClient(cfg.Grafana, logger)
	if err != nil {
		logger.Fatalf("Failed to create Grafana client: %v", err)
	}

	// Test Grafana connection
	if err := grafanaClient.TestConnection(); err != nil {
		logger.Fatalf("Failed to connect to Grafana: %v", err)
	}
	logger.Info("Successfully connected to Grafana")

	// Create the exporter
	exp, err := exporter.New(cfg, grafanaClient, logger)
	if err != nil {
		logger.Fatalf("Failed to create exporter: %v", err)
	}

	// Register the exporter with Prometheus
	prometheus.MustRegister(exp)

	// Setup HTTP server
	mux := http.NewServeMux()
	
	// Metrics endpoint
	mux.Handle(cfg.Exporter.Path, promhttp.Handler())
	
	// Health check endpoint
	mux.HandleFunc(cfg.Exporter.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","mode":"export","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Info endpoint
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"name":"grafana-influx-exporter",
			"version":"1.0.0",
			"mode":"export",
			"grafana_url":"%s",
			"datasource_uid":"%s",
			"queries_configured":%d
		}`, cfg.Grafana.URL, cfg.Grafana.Datasource.UID, len(cfg.Queries))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Exporter.Port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		logger.Infof("Starting HTTP server on port %d", cfg.Exporter.Port)
		logger.Infof("Metrics available at: http://localhost:%d%s", cfg.Exporter.Port, cfg.Exporter.Path)
		logger.Infof("Health check available at: http://localhost:%d%s", cfg.Exporter.Port, cfg.Exporter.HealthPath)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	} else {
		logger.Info("Server gracefully stopped")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func setupLogging(logger *logrus.Logger, cfg config.LoggingConfig) {
	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		logger.Warnf("Invalid log level '%s', using info", cfg.Level)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	switch cfg.Format {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	default:
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	}

	// Set output file if specified
	if cfg.File != "" {
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logger.Warnf("Failed to open log file '%s': %v", cfg.File, err)
		} else {
			logger.SetOutput(file)
		}
	}
}