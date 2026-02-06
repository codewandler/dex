package homer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps the Homer 7.x REST API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// SearchParams holds search query parameters for Homer API calls
type SearchParams struct {
	From    time.Time
	To      time.Time
	Caller  string
	Callee  string
	CallID  string
	Limit   int
}

// SearchResult holds the response from a call search
type SearchResult struct {
	Data []CallRecord `json:"data"`
}

// CallRecord represents a single call in search results
type CallRecord struct {
	ID         float64 `json:"id"`
	Date       int64   `json:"create_date"`
	MicroTS    int64   `json:"micro_ts"`
	Protocol   float64 `json:"protocol"`
	SourceIP   string  `json:"srcIp"`
	SourcePort float64 `json:"srcPort"`
	DestIP     string  `json:"dstIp"`
	DestPort   float64 `json:"dstPort"`
	CallID     string  `json:"sid"`
	Method     string  `json:"method"`
	MethodText string  `json:"method_text"`
	FromUser   string  `json:"from_user"`
	ToUser     string  `json:"to_user"`
	RuriUser   string  `json:"ruri_user"`
	Status     float64 `json:"status"`
	AliasSrc   string  `json:"aliasSrc"`
	AliasDst   string  `json:"aliasDst"`
	Table      string  `json:"table"`
}

// Alias represents a Homer IP/port alias
type Alias struct {
	ID       float64 `json:"id"`
	IP       string  `json:"ip"`
	Port     float64 `json:"port"`
	Mask     float64 `json:"mask"`
	Alias    string  `json:"alias"`
	Status   bool    `json:"status"`
	CaptureID string `json:"captureID"`
}

// NewClient creates a new Homer API client
func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Authenticate logs in to Homer and stores the JWT token
func (c *Client) Authenticate(username, password string) error {
	payload := map[string]string{
		"username": username,
		"password": password,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/v3/auth", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var authResp struct {
		Token   string `json:"token"`
		Scope   string `json:"scope"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.Token == "" {
		return fmt.Errorf("authentication returned empty token")
	}

	c.token = authResp.Token
	return nil
}

// SearchCalls searches for SIP calls matching the given parameters
func (c *Client) SearchCalls(params SearchParams) (*SearchResult, error) {
	reqBody := c.buildSearchPayload(params)

	body, err := c.doAuthRequest("POST", "/api/v3/search/call/data", reqBody)
	if err != nil {
		return nil, fmt.Errorf("search calls failed: %w", err)
	}

	var result SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	return &result, nil
}

// ExportPCAP exports call messages as a PCAP file
func (c *Client) ExportPCAP(params SearchParams) ([]byte, error) {
	reqBody := c.buildSearchPayload(params)

	body, err := c.doAuthRequest("POST", "/api/v3/export/call/messages/pcap", reqBody)
	if err != nil {
		return nil, fmt.Errorf("export PCAP failed: %w", err)
	}

	return body, nil
}

// ListAliases returns all configured IP/port aliases
func (c *Client) ListAliases() ([]Alias, error) {
	body, err := c.doAuthRequest("GET", "/api/v3/alias", nil)
	if err != nil {
		return nil, fmt.Errorf("list aliases failed: %w", err)
	}

	var resp struct {
		Data []Alias `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode aliases: %w", err)
	}

	return resp.Data, nil
}

// TestConnection verifies the Homer API is reachable (unauthenticated health check)
func (c *Client) TestConnection() error {
	resp, err := c.httpClient.Get(c.baseURL + "/api/v3/agent/check")
	if err != nil {
		return fmt.Errorf("failed to connect to Homer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("homer returned status %d", resp.StatusCode)
	}

	return nil
}

// buildSearchPayload constructs the Homer search API request body
func (c *Client) buildSearchPayload(params SearchParams) map[string]any {
	fromMS := params.From.UnixMilli()
	toMS := params.To.UnixMilli()

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}

	// Build search filters as array items (hepid=1 for SIP, profile=call)
	var filters []map[string]any
	if params.CallID != "" {
		filters = append(filters, map[string]any{
			"name": "sid", "value": params.CallID, "type": "string", "hepid": 1,
		})
	}
	if params.Caller != "" {
		filters = append(filters, map[string]any{
			"name": "from_user", "value": params.Caller, "type": "string", "hepid": 1,
		})
	}
	if params.Callee != "" {
		filters = append(filters, map[string]any{
			"name": "to_user", "value": params.Callee, "type": "string", "hepid": 1,
		})
	}
	// Limit is passed as a search filter item, not as param.limit
	filters = append(filters, map[string]any{
		"name": "limit", "value": fmt.Sprintf("%d", limit), "type": "string", "hepid": 1,
	})

	return map[string]any{
		"config": map[string]any{},
		"param": map[string]any{
			"transaction": map[string]any{},
			"limit":       limit,
			"search": map[string]any{
				"1_call": filters,
			},
			"location": map[string]any{},
			"timezone": map[string]any{
				"name":  "Local",
				"value": 0,
			},
		},
		"timestamp": map[string]any{
			"from": fromMS,
			"to":   toMS,
		},
	}
}

// doAuthRequest makes an authenticated HTTP request to the Homer API
func (c *Client) doAuthRequest(method, path string, payload any) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("homer returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
