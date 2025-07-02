package exporter

import (
	"testing"

	"grafana-influx-exporter/internal/config"

	"github.com/sirupsen/logrus"
)

func TestExtractValue(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	cfg := &config.Config{}
	exporter := &Exporter{
		config: cfg,
		logger: logger,
	}

	tests := []struct {
		name     string
		row      map[string]interface{}
		column   string
		expected float64
		wantErr  bool
	}{
		{
			name:     "float64 value",
			row:      map[string]interface{}{"value": 42.5},
			column:   "value",
			expected: 42.5,
			wantErr:  false,
		},
		{
			name:     "int value",
			row:      map[string]interface{}{"value": 42},
			column:   "value",
			expected: 42.0,
			wantErr:  false,
		},
		{
			name:     "string number value",
			row:      map[string]interface{}{"value": "42.5"},
			column:   "value",
			expected: 42.5,
			wantErr:  false,
		},
		{
			name:    "missing column",
			row:     map[string]interface{}{"other": 42},
			column:  "value",
			wantErr: true,
		},
		{
			name:    "null value",
			row:     map[string]interface{}{"value": nil},
			column:  "value",
			wantErr: true,
		},
		{
			name:    "invalid string",
			row:     map[string]interface{}{"value": "not-a-number"},
			column:  "value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := exporter.extractValue(tt.row, tt.column)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: config.Config{
				Grafana: config.GrafanaConfig{
					URL:   "http://localhost:3000",
					Token: "test-token",
					Datasource: config.DatasourceConfig{
						UID: "test-uid",
					},
				},
				Queries: []config.QueryConfig{
					{
						Name:  "test-query",
						Query: "SELECT * FROM test",
						Metrics: []config.MetricConfig{
							{
								Name:        "test_metric",
								ValueColumn: "value",
								Type:        "gauge",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing grafana url",
			config: config.Config{
				Grafana: config.GrafanaConfig{
					Token: "test-token",
				},
			},
			wantErr: true,
		},
		{
			name: "missing auth",
			config: config.Config{
				Grafana: config.GrafanaConfig{
					URL: "http://localhost:3000",
				},
			},
			wantErr: true,
		},
		{
			name: "no queries",
			config: config.Config{
				Grafana: config.GrafanaConfig{
					URL:   "http://localhost:3000",
					Token: "test-token",
					Datasource: config.DatasourceConfig{
						UID: "test-uid",
					},
				},
				Queries: []config.QueryConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.wantErr && err == nil {
				t.Error("expected validation error but got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}