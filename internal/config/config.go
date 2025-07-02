package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration
type Config struct {
	Grafana      GrafanaConfig      `yaml:"grafana"`
	Exporter     ExporterConfig     `yaml:"exporter"`
	Queries      []QueryConfig      `yaml:"queries"`
	Logging      LoggingConfig      `yaml:"logging"`
	GlobalLabels map[string]string  `yaml:"global_labels"`
}

// GrafanaConfig holds Grafana connection settings
type GrafanaConfig struct {
	URL        string           `yaml:"url"`
	Datasource DatasourceConfig `yaml:"datasource"`
	
	// These will be populated from environment variables
	Token    string `yaml:"-"`
	Username string `yaml:"-"`
	Password string `yaml:"-"`
}

// DatasourceConfig holds InfluxDB datasource configuration
type DatasourceConfig struct {
	UID  string `yaml:"uid"`
	Name string `yaml:"name"`
}

// ExporterConfig holds exporter settings
type ExporterConfig struct {
	Port         int    `yaml:"port"`
	Path         string `yaml:"path"`
	HealthPath   string `yaml:"health_path"`
	ScrapeInterval int  `yaml:"scrape_interval"`
}

// QueryConfig defines a query and its associated metrics
type QueryConfig struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Query       string         `yaml:"query"`
	Metrics     []MetricConfig `yaml:"metrics"`
}

// MetricConfig defines how to create a Prometheus metric from query results
type MetricConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Type        string            `yaml:"type"`
	ValueColumn string            `yaml:"value_column"`
	Labels      map[string]string `yaml:"labels"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load authentication from environment variables
	config.Grafana.Token = os.Getenv("GRAFANA_TOKEN")
	config.Grafana.Username = os.Getenv("GRAFANA_USERNAME")
	config.Grafana.Password = os.Getenv("GRAFANA_PASSWORD")

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Set defaults
	config.SetDefaults()

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check Grafana URL
	if c.Grafana.URL == "" {
		return fmt.Errorf("grafana.url is required")
	}

	// Check authentication
	if c.Grafana.Token == "" && (c.Grafana.Username == "" || c.Grafana.Password == "") {
		return fmt.Errorf("either GRAFANA_TOKEN or GRAFANA_USERNAME+GRAFANA_PASSWORD environment variables must be set")
	}

	// Check datasource
	if c.Grafana.Datasource.UID == "" && c.Grafana.Datasource.Name == "" {
		return fmt.Errorf("datasource.uid or datasource.name is required")
	}

	// Check queries
	if len(c.Queries) == 0 {
		return fmt.Errorf("at least one query must be defined")
	}

	// Validate each query
	for i, query := range c.Queries {
		if query.Name == "" {
			return fmt.Errorf("query[%d].name is required", i)
		}
		if query.Query == "" {
			return fmt.Errorf("query[%d].query is required", i)
		}
		if len(query.Metrics) == 0 {
			return fmt.Errorf("query[%d] must have at least one metric defined", i)
		}

		// Validate each metric
		for j, metric := range query.Metrics {
			if metric.Name == "" {
				return fmt.Errorf("query[%d].metrics[%d].name is required", i, j)
			}
			if metric.ValueColumn == "" {
				return fmt.Errorf("query[%d].metrics[%d].value_column is required", i, j)
			}
			if metric.Type != "" && metric.Type != "gauge" && metric.Type != "counter" && metric.Type != "histogram" && metric.Type != "summary" {
				return fmt.Errorf("query[%d].metrics[%d].type must be one of: gauge, counter, histogram, summary", i, j)
			}
		}
	}

	return nil
}

// SetDefaults sets default values for optional configuration fields
func (c *Config) SetDefaults() {
	// Exporter defaults
	if c.Exporter.Port == 0 {
		c.Exporter.Port = 8080
	}
	if c.Exporter.Path == "" {
		c.Exporter.Path = "/metrics"
	}
	if c.Exporter.HealthPath == "" {
		c.Exporter.HealthPath = "/health"
	}
	if c.Exporter.ScrapeInterval == 0 {
		c.Exporter.ScrapeInterval = 30
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	// Metric type defaults
	for i := range c.Queries {
		for j := range c.Queries[i].Metrics {
			if c.Queries[i].Metrics[j].Type == "" {
				c.Queries[i].Metrics[j].Type = "gauge"
			}
		}
	}

	// Initialize global labels if nil
	if c.GlobalLabels == nil {
		c.GlobalLabels = make(map[string]string)
	}
}