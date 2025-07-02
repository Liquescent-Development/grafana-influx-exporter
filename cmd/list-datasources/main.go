package main

import (
	"fmt"
	"os"

	"grafana-influx-exporter/internal/config"
	"grafana-influx-exporter/internal/grafana"

	"github.com/sirupsen/logrus"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/list-datasources/main.go <grafana-url>")
		fmt.Println("Environment variables needed: GRAFANA_TOKEN or GRAFANA_USERNAME+GRAFANA_PASSWORD")
		os.Exit(1)
	}

	grafanaURL := os.Args[1]
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Check authentication
	token := os.Getenv("GRAFANA_TOKEN")
	username := os.Getenv("GRAFANA_USERNAME")
	password := os.Getenv("GRAFANA_PASSWORD")
	
	if token == "" && (username == "" || password == "") {
		fmt.Println("Error: No authentication provided")
		fmt.Println("Set either GRAFANA_TOKEN or both GRAFANA_USERNAME and GRAFANA_PASSWORD")
		os.Exit(1)
	}
	
	if token != "" {
		fmt.Printf("Using token authentication (token length: %d)\n", len(token))
	} else {
		fmt.Printf("Using basic authentication (username: %s)\n", username)
	}

	// Create a temporary config for connecting
	cfg := config.GrafanaConfig{
		URL:      grafanaURL,
		Token:    token,
		Username: username,
		Password: password,
		Datasource: config.DatasourceConfig{
			UID: "temp", // Temporary, just for creating client
		},
	}

	// Create client (but skip datasource validation)
	client, err := grafana.NewClientWithoutDatasource(cfg, logger)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	// List all datasources
	datasources, err := client.ListDatasources()
	if err != nil {
		fmt.Printf("Error listing datasources: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d datasources in %s:\n\n", len(datasources), grafanaURL)
	fmt.Printf("%-40s %-20s %-15s %s\n", "NAME", "UID", "TYPE", "URL")
	fmt.Printf("%-40s %-20s %-15s %s\n", "----", "---", "----", "---")

	for _, ds := range datasources {
		fmt.Printf("%-40s %-20s %-15s %s\n", ds.Name, ds.UID, ds.Type, ds.URL)
	}

	fmt.Printf("\nTo use in monitoring config, reference datasources by name:\n")
	fmt.Printf("datasources:\n")
	for _, ds := range datasources {
		if ds.Type == "influxdb" {
			fmt.Printf("  - name: %s\n", ds.Name)
		}
	}
}