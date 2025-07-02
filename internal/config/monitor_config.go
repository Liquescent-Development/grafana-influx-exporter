package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MonitorConfig represents the configuration for monitoring mode
type MonitorConfig struct {
	Mode         string                  `yaml:"mode"`  // "monitor" or "export"
	Targets      []MonitorTarget         `yaml:"targets"`
	TestQueries  []TestQuery             `yaml:"test_queries"`
	Exporter     ExporterConfig          `yaml:"exporter"`
	Logging      LoggingConfig           `yaml:"logging"`
}

// MonitorTarget represents a Grafana instance to monitor
type MonitorTarget struct {
	Name         string           `yaml:"name"`
	URL          string           `yaml:"url"`
	Datasources  []DatasourceConfig `yaml:"datasources"`
	
	// Authentication - populated from environment variables
	Token        string `yaml:"-"`
	Username     string `yaml:"-"`
	Password     string `yaml:"-"`
}

// TestQuery represents a query to test datasource availability
type TestQuery struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Query       string `yaml:"query"`
	Interval    int    `yaml:"interval"` // How often to run this query in seconds
}

// LoadMonitorConfig loads monitoring configuration from a YAML file
func LoadMonitorConfig(path string) (*MonitorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config MonitorConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load authentication from environment variables for each target
	for i := range config.Targets {
		target := &config.Targets[i]
		envPrefix := fmt.Sprintf("GRAFANA_%s_", target.Name)
		
		// Try target-specific env vars first
		target.Token = os.Getenv(envPrefix + "TOKEN")
		target.Username = os.Getenv(envPrefix + "USERNAME")
		target.Password = os.Getenv(envPrefix + "PASSWORD")
		
		// Fall back to global env vars if target-specific not found
		if target.Token == "" {
			target.Token = os.Getenv("GRAFANA_TOKEN")
		}
		if target.Username == "" {
			target.Username = os.Getenv("GRAFANA_USERNAME")
		}
		if target.Password == "" {
			target.Password = os.Getenv("GRAFANA_PASSWORD")
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Set defaults
	config.SetDefaults()

	return &config, nil
}

// Validate checks if the monitoring configuration is valid
func (c *MonitorConfig) Validate() error {
	// Check mode
	if c.Mode != "monitor" && c.Mode != "export" && c.Mode != "" {
		return fmt.Errorf("mode must be 'monitor' or 'export'")
	}

	// Check targets
	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target must be defined")
	}

	// Validate each target
	for i, target := range c.Targets {
		if target.Name == "" {
			return fmt.Errorf("targets[%d].name is required", i)
		}
		if target.URL == "" {
			return fmt.Errorf("targets[%d].url is required", i)
		}
		
		// Check authentication
		if target.Token == "" && (target.Username == "" || target.Password == "") {
			return fmt.Errorf("authentication required for target '%s': set GRAFANA_%s_TOKEN or GRAFANA_%s_USERNAME+PASSWORD", 
				target.Name, target.Name, target.Name)
		}
		
		// Check datasources
		if len(target.Datasources) == 0 {
			return fmt.Errorf("targets[%d] must have at least one datasource", i)
		}
		
		for j, ds := range target.Datasources {
			if ds.UID == "" && ds.Name == "" {
				return fmt.Errorf("targets[%d].datasources[%d] must have uid or name", i, j)
			}
			// Note: UID will be resolved from name during client creation if needed
		}
	}

	// Check test queries
	if len(c.TestQueries) == 0 {
		return fmt.Errorf("at least one test query must be defined")
	}

	for i, query := range c.TestQueries {
		if query.Name == "" {
			return fmt.Errorf("test_queries[%d].name is required", i)
		}
		if query.Query == "" {
			return fmt.Errorf("test_queries[%d].query is required", i)
		}
	}

	return nil
}

// SetDefaults sets default values for monitoring configuration
func (c *MonitorConfig) SetDefaults() {
	// Default mode
	if c.Mode == "" {
		c.Mode = "monitor"
	}

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

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	// Query interval defaults
	for i := range c.TestQueries {
		if c.TestQueries[i].Interval == 0 {
			c.TestQueries[i].Interval = 60 // Default to 1 minute
		}
	}
}