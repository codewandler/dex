package prometheus

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps the Prometheus HTTP API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// promResponse represents the standard Prometheus API envelope
type promResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType string          `json:"errorType,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// queryData wraps the resultType + result for /api/v1/query and /api/v1/query_range
type queryData struct {
	ResultType string          `json:"resultType"`
	Result     json.RawMessage `json:"result"`
}

// VectorSample is a single instant-query result (resultType "vector")
type VectorSample struct {
	Metric map[string]string `json:"metric"`
	Value  [2]interface{}    `json:"value"` // [unix_ts, "string_value"]
}

// MatrixSeries is a single range-query result (resultType "matrix")
type MatrixSeries struct {
	Metric map[string]string `json:"metric"`
	Values [][2]interface{}  `json:"values"`
}

// ActiveTarget represents a Prometheus scrape target
type ActiveTarget struct {
	Labels             map[string]string `json:"labels"`
	DiscoveredLabels   map[string]string `json:"discoveredLabels"`
	ScrapePool         string            `json:"scrapePool"`
	ScrapeURL          string            `json:"scrapeUrl"`
	LastError          string            `json:"lastError"`
	LastScrape         time.Time         `json:"lastScrape"`
	LastScrapeDuration float64           `json:"lastScrapeDuration"`
	Health             string            `json:"health"`
}

// Alert represents a Prometheus alert
type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    time.Time         `json:"activeAt"`
	Value       string            `json:"value"`
}

// NewClient creates a new Prometheus client
func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// NewProbeClient creates a Prometheus client with a short timeout for connectivity checks
func NewProbeClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// doGet performs a GET request and returns the parsed data field from the Prometheus response envelope.
func (c *Client) doGet(endpoint string) (json.RawMessage, error) {
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus error (%s): %s", pr.ErrorType, pr.Error)
	}

	return pr.Data, nil
}

// Query executes an instant PromQL query. evalTime may be zero (defaults to server now).
func (c *Client) Query(query string, evalTime time.Time) ([]VectorSample, error) {
	params := url.Values{}
	params.Set("query", query)
	if !evalTime.IsZero() {
		params.Set("time", fmt.Sprintf("%d", evalTime.Unix()))
	}

	data, err := c.doGet(fmt.Sprintf("%s/api/v1/query?%s", c.baseURL, params.Encode()))
	if err != nil {
		return nil, err
	}

	var qd queryData
	if err := json.Unmarshal(data, &qd); err != nil {
		return nil, fmt.Errorf("failed to parse query data: %w", err)
	}

	if qd.ResultType != "vector" {
		return nil, fmt.Errorf("unexpected result type %q (expected vector)", qd.ResultType)
	}

	var samples []VectorSample
	if err := json.Unmarshal(qd.Result, &samples); err != nil {
		return nil, fmt.Errorf("failed to parse vector result: %w", err)
	}

	return samples, nil
}

// QueryRange executes a range PromQL query.
func (c *Client) QueryRange(query string, start, end time.Time, step time.Duration) ([]MatrixSeries, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", fmt.Sprintf("%g", step.Seconds()))

	data, err := c.doGet(fmt.Sprintf("%s/api/v1/query_range?%s", c.baseURL, params.Encode()))
	if err != nil {
		return nil, err
	}

	var qd queryData
	if err := json.Unmarshal(data, &qd); err != nil {
		return nil, fmt.Errorf("failed to parse query data: %w", err)
	}

	if qd.ResultType != "matrix" {
		return nil, fmt.Errorf("unexpected result type %q (expected matrix)", qd.ResultType)
	}

	var series []MatrixSeries
	if err := json.Unmarshal(qd.Result, &series); err != nil {
		return nil, fmt.Errorf("failed to parse matrix result: %w", err)
	}

	return series, nil
}

// Labels returns all label names. Match selectors are optional (e.g. `up{job="foo"}`).
func (c *Client) Labels(match []string) ([]string, error) {
	params := url.Values{}
	for _, m := range match {
		params.Add("match[]", m)
	}

	endpoint := fmt.Sprintf("%s/api/v1/labels", c.baseURL)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	data, err := c.doGet(endpoint)
	if err != nil {
		return nil, err
	}

	var labels []string
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, fmt.Errorf("failed to parse labels: %w", err)
	}
	return labels, nil
}

// LabelValues returns all values for a given label name.
func (c *Client) LabelValues(label string, match []string) ([]string, error) {
	params := url.Values{}
	for _, m := range match {
		params.Add("match[]", m)
	}

	endpoint := fmt.Sprintf("%s/api/v1/label/%s/values", c.baseURL, url.PathEscape(label))
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	data, err := c.doGet(endpoint)
	if err != nil {
		return nil, err
	}

	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse label values: %w", err)
	}
	return values, nil
}

// targetsData wraps the targets API response shape
type targetsData struct {
	ActiveTargets  []ActiveTarget `json:"activeTargets"`
	DroppedTargets []ActiveTarget `json:"droppedTargets"`
}

// Targets returns scrape targets. State filter can be "active", "dropped", or "any".
func (c *Client) Targets(state string) ([]ActiveTarget, error) {
	params := url.Values{}
	if state != "" && state != "any" {
		params.Set("state", state)
	}

	endpoint := fmt.Sprintf("%s/api/v1/targets", c.baseURL)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	data, err := c.doGet(endpoint)
	if err != nil {
		return nil, err
	}

	var td targetsData
	if err := json.Unmarshal(data, &td); err != nil {
		return nil, fmt.Errorf("failed to parse targets: %w", err)
	}

	if state == "dropped" {
		return td.DroppedTargets, nil
	}
	if state == "any" {
		return append(td.ActiveTargets, td.DroppedTargets...), nil
	}
	return td.ActiveTargets, nil
}

// alertsData wraps the alerts API response shape
type alertsData struct {
	Alerts []Alert `json:"alerts"`
}

// Alerts returns active alerts.
func (c *Client) Alerts() ([]Alert, error) {
	data, err := c.doGet(fmt.Sprintf("%s/api/v1/alerts", c.baseURL))
	if err != nil {
		return nil, err
	}

	var ad alertsData
	if err := json.Unmarshal(data, &ad); err != nil {
		return nil, fmt.Errorf("failed to parse alerts: %w", err)
	}

	return ad.Alerts, nil
}

// TestConnection verifies the Prometheus instance is ready.
func (c *Client) TestConnection() error {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/-/ready", c.baseURL))
	if err != nil {
		return fmt.Errorf("failed to connect to Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus not ready, status: %d", resp.StatusCode)
	}
	return nil
}
