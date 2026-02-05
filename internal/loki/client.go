package loki

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Client wraps Loki HTTP API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// QueryResult represents a single log entry
type QueryResult struct {
	Timestamp time.Time
	Labels    map[string]string
	Line      string
}

// lokiResponse represents the Loki API response structure
type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [timestamp_ns, line]
		} `json:"result"`
	} `json:"data"`
}

// NewClient creates a new Loki client
func NewClient(baseURL string) (*Client, error) {
	// Normalize URL (remove trailing slash)
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Query executes a LogQL query and returns log entries
func (c *Client) Query(query string, since time.Duration, limit int) ([]QueryResult, error) {
	end := time.Now()
	start := end.Add(-since)

	// Build query URL
	endpoint := fmt.Sprintf("%s/loki/api/v1/query_range", c.baseURL)

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	params.Set("end", fmt.Sprintf("%d", end.UnixNano()))
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("direction", "backward") // Most recent first

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	var lokiResp lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lokiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if lokiResp.Status != "success" {
		return nil, fmt.Errorf("loki query failed with status: %s", lokiResp.Status)
	}

	// Convert response to QueryResult slice
	var results []QueryResult
	for _, stream := range lokiResp.Data.Result {
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}

			// Parse nanosecond timestamp
			var tsNano int64
			fmt.Sscanf(value[0], "%d", &tsNano)
			ts := time.Unix(0, tsNano)

			results = append(results, QueryResult{
				Timestamp: ts,
				Labels:    stream.Stream,
				Line:      value[1],
			})
		}
	}

	// Sort by timestamp (most recent first since we used direction=backward)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results, nil
}

// Labels returns all label names from Loki
func (c *Client) Labels() ([]string, error) {
	endpoint := fmt.Sprintf("%s/loki/api/v1/labels", c.baseURL)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	var labelsResp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&labelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return labelsResp.Data, nil
}

// LabelValues returns all values for a given label
func (c *Client) LabelValues(label string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/loki/api/v1/label/%s/values", c.baseURL, url.PathEscape(label))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get label values: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	var valuesResp struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&valuesResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return valuesResp.Data, nil
}

// TestConnection verifies the Loki connection is working
func (c *Client) TestConnection() error {
	endpoint := fmt.Sprintf("%s/ready", c.baseURL)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("failed to connect to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki not ready, status: %d", resp.StatusCode)
	}

	return nil
}
