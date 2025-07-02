package grafana

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"grafana-influx-exporter/internal/config"

	"github.com/sirupsen/logrus"
)

// Client represents a Grafana API client
type Client struct {
	baseURL      string
	datasourceUID string
	httpClient   *http.Client
	logger       *logrus.Logger
	authHeader   string
	username     string
	password     string
}

// QueryRequest represents a Grafana query request
type QueryRequest struct {
	Queries []Query `json:"queries"`
	From    string  `json:"from"`
	To      string  `json:"to"`
}

// Query represents a single query in the request
type Query struct {
	RefID         string          `json:"refId"`
	Datasource    QueryDatasource `json:"datasource"`
	Query         string          `json:"query"`
	RawQuery      bool            `json:"rawQuery"`
	ResultFormat  string          `json:"resultFormat"`
	Alias         string          `json:"alias"`
	AdhocFilters  []interface{}   `json:"adhocFilters"`
	RawSQL        string          `json:"rawSql"`
	Limit         string          `json:"limit"`
	Measurement   string          `json:"measurement"`
	Policy        string          `json:"policy"`
	SLimit        string          `json:"slimit"`
	Tz            string          `json:"tz"`
	MaxDataPoints int             `json:"maxDataPoints"`
	IntervalMs    int             `json:"intervalMs"`
}

// QueryDatasource represents datasource reference in query
type QueryDatasource struct {
	UID  string `json:"uid"`
	Type string `json:"type"`
}

// QueryResponse represents the response from Grafana query API
type QueryResponse struct {
	Results map[string]QueryResult `json:"results"`
}

// QueryResult represents results for a single query
type QueryResult struct {
	Frames []DataFrame `json:"frames"`
	Error  string      `json:"error,omitempty"`
}

// DataFrame represents a data frame from Grafana
type DataFrame struct {
	Schema DataFrameSchema `json:"schema"`
	Data   DataFrameData   `json:"data"`
}

// DataFrameSchema represents the schema of a data frame
type DataFrameSchema struct {
	RefID  string  `json:"refId"`
	Fields []Field `json:"fields"`
}

// Field represents a field in the data frame
type Field struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	TypeInfo map[string]interface{} `json:"typeInfo,omitempty"`
	Labels   map[string]string      `json:"labels,omitempty"`
}

// DataFrameData represents the actual data in a data frame
type DataFrameData struct {
	Values [][]interface{} `json:"values"`
}

// DatasourceInfo represents a Grafana datasource
type DatasourceInfo struct {
	ID       int    `json:"id"`
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Access   string `json:"access"`
	IsDefault bool  `json:"isDefault"`
}

// NewClient creates a new Grafana client
func NewClient(cfg config.GrafanaConfig, logger *logrus.Logger) (*Client, error) {
	client := &Client{
		baseURL:      cfg.URL,
		logger:       logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Set up authentication
	if cfg.Token != "" {
		client.authHeader = fmt.Sprintf("Bearer %s", cfg.Token)
		logger.Info("Using service account token authentication")
	} else if cfg.Username != "" && cfg.Password != "" {
		client.username = cfg.Username
		client.password = cfg.Password
		logger.Info("Using basic authentication")
	} else {
		return nil, fmt.Errorf("no valid authentication method configured")
	}

	// Resolve datasource UID if only name is provided
	if cfg.Datasource.UID == "" && cfg.Datasource.Name != "" {
		logger.Infof("Resolving datasource name '%s' to UID", cfg.Datasource.Name)
		uid, err := client.ResolveDatasourceUID(cfg.Datasource.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve datasource name '%s': %w", cfg.Datasource.Name, err)
		}
		client.datasourceUID = uid
		logger.Infof("Resolved datasource '%s' to UID: %s", cfg.Datasource.Name, uid)
	} else if cfg.Datasource.UID != "" {
		client.datasourceUID = cfg.Datasource.UID
	} else {
		return nil, fmt.Errorf("either datasource UID or name must be provided")
	}

	return client, nil
}

// GetDatasourceUID returns the current datasource UID
func (c *Client) GetDatasourceUID() string {
	return c.datasourceUID
}

// NewClientWithoutDatasource creates a client without validating a specific datasource
// Useful for listing datasources or general API access
func NewClientWithoutDatasource(cfg config.GrafanaConfig, logger *logrus.Logger) (*Client, error) {
	client := &Client{
		baseURL: cfg.URL,
		logger:  logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Set up authentication
	if cfg.Token != "" {
		client.authHeader = fmt.Sprintf("Bearer %s", cfg.Token)
		logger.Debug("Using service account token authentication")
	} else if cfg.Username != "" && cfg.Password != "" {
		client.username = cfg.Username
		client.password = cfg.Password
		logger.Debug("Using basic authentication")
	} else {
		return nil, fmt.Errorf("no valid authentication method configured")
	}

	return client, nil
}

// TestConnection tests the connection to Grafana
func (c *Client) TestConnection() error {
	url := fmt.Sprintf("%s/api/datasources/uid/%s", c.baseURL, c.datasourceUID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Grafana: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Grafana API returned status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Infof("Successfully validated datasource UID: %s", c.datasourceUID)
	return nil
}

// ResolveDatasourceUID resolves a datasource name to its UID
func (c *Client) ResolveDatasourceUID(name string) (string, error) {
	datasources, err := c.ListDatasources()
	if err != nil {
		return "", fmt.Errorf("failed to list datasources: %w", err)
	}

	for _, ds := range datasources {
		if ds.Name == name {
			c.logger.Debugf("Resolved datasource '%s' to UID: %s", name, ds.UID)
			return ds.UID, nil
		}
	}

	return "", fmt.Errorf("datasource with name '%s' not found", name)
}

// ListDatasources returns all datasources from Grafana
func (c *Client) ListDatasources() ([]DatasourceInfo, error) {
	url := fmt.Sprintf("%s/api/datasources", c.baseURL)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list datasources: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list datasources, status %d: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var datasources []DatasourceInfo
	if err := json.Unmarshal(respBody, &datasources); err != nil {
		return nil, fmt.Errorf("failed to parse datasources response: %w", err)
	}

	c.logger.Debugf("Found %d datasources", len(datasources))
	return datasources, nil
}

// ExecuteQuery executes an InfluxQL query through Grafana
func (c *Client) ExecuteQuery(query string) (*QueryResponse, error) {
	// Create the query request
	queryReq := QueryRequest{
		Queries: []Query{
			{
				RefID: "A",
				Datasource: QueryDatasource{
					UID:  c.datasourceUID,
					Type: "influxdb",
				},
				Query:         query,
				RawQuery:      true,
				ResultFormat:  "time_series",
				Alias:         "",
				AdhocFilters:  []interface{}{},
				RawSQL:        "",
				Limit:         "",
				Measurement:   "",
				Policy:        "",
				SLimit:        "",
				Tz:            "",
				MaxDataPoints: 1000,
				IntervalMs:    1000,
			},
		},
		From: fmt.Sprintf("%d", time.Now().Add(-5*time.Minute).UnixMilli()),
		To:   fmt.Sprintf("%d", time.Now().UnixMilli()),
	}

	// Marshal to JSON
	reqBody, err := json.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %w", err)
	}

	c.logger.Infof("Executing query: %s", query)
	c.logger.Infof("Request body: %s", string(reqBody))
	c.logger.Infof("Query in request: %+v", queryReq.Queries[0])

	// Create HTTP request
	url := fmt.Sprintf("%s/api/ds/query", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorf("Query failed with status %d: %s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	c.logger.Debugf("Query response: %s", string(respBody))

	// Parse response
	var queryResp QueryResponse
	if err := json.Unmarshal(respBody, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse query response: %w", err)
	}

	// Check for errors in the response
	for refID, result := range queryResp.Results {
		if result.Error != "" {
			return nil, fmt.Errorf("query %s failed: %s", refID, result.Error)
		}
	}

	return &queryResp, nil
}

// setAuthHeaders sets the appropriate authentication headers
func (c *Client) setAuthHeaders(req *http.Request) {
	if c.authHeader != "" {
		// Use token authentication
		req.Header.Set("Authorization", c.authHeader)
	} else if c.username != "" && c.password != "" {
		// Use basic authentication
		req.SetBasicAuth(c.username, c.password)
	}
}

// ParseQueryResults converts Grafana query results to a more usable format
func ParseQueryResults(resp *QueryResponse) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, result := range resp.Results {
		for _, frame := range result.Frames {
			// Get field names and types
			fieldNames := make([]string, len(frame.Schema.Fields))
			for i, field := range frame.Schema.Fields {
				fieldNames[i] = field.Name
			}

			// Process each row of data
			if len(frame.Data.Values) > 0 && len(frame.Data.Values[0]) > 0 {
				// Transpose the data (Grafana returns columns, we want rows)
				numRows := len(frame.Data.Values[0])
				for rowIdx := 0; rowIdx < numRows; rowIdx++ {
					row := make(map[string]interface{})
					
					for colIdx, fieldName := range fieldNames {
						if colIdx < len(frame.Data.Values) && rowIdx < len(frame.Data.Values[colIdx]) {
							row[fieldName] = frame.Data.Values[colIdx][rowIdx]
						}
					}
					
					// Add any labels from the schema
					for _, field := range frame.Schema.Fields {
						if field.Labels != nil {
							for labelKey, labelValue := range field.Labels {
								row[labelKey] = labelValue
							}
						}
					}
					
					results = append(results, row)
				}
			}
		}
	}

	return results, nil
}