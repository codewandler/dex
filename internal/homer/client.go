package homer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client wraps the Homer 7.x REST API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	Debug      bool
}

// SearchParams holds search query parameters for Homer API calls
type SearchParams struct {
	From       time.Time
	To         time.Time
	SmartInput string
	CallID     string
	Limit      int
}

// SearchResult holds the response from a call search
type SearchResult struct {
	Data []CallRecord `json:"data"`
}

// CallRecord represents a raw call record from the Homer API
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
	UserAgent  string  `json:"user_agent"`
	CSeq       string  `json:"cseq"`
	Status     float64 `json:"status"`
	AliasSrc   string  `json:"aliasSrc"`
	AliasDst   string  `json:"aliasDst"`
	Table      string  `json:"table"`
}

// SearchRecord is the clean output type for search results.
// Used by both table and JSON/JSONL output.
type SearchRecord struct {
	Date      time.Time `json:"date"`
	SrcIP     string    `json:"src_ip"`
	SrcPort   int       `json:"src_port"`
	DstIP     string    `json:"dst_ip"`
	DstPort   int       `json:"dst_port"`
	Method    string    `json:"method"`
	FromUser  string    `json:"from_user"`
	ToUser    string    `json:"to_user"`
	CallID    string    `json:"call_id"`
	SessionID string    `json:"session_id"`
	UserAgent string    `json:"user_agent"`
	CSeq      string    `json:"cseq"`
}

// ToSearchRecords converts raw API records to clean SearchRecord values.
func ToSearchRecords(records []CallRecord) []SearchRecord {
	out := make([]SearchRecord, len(records))
	for i, r := range records {
		method := r.Method
		if method == "" {
			method = r.MethodText
		}
		toUser := r.ToUser
		if toUser == "" {
			toUser = r.RuriUser
		}
		out[i] = SearchRecord{
			Date:      time.UnixMilli(r.Date),
			SrcIP:     r.SourceIP,
			SrcPort:   int(r.SourcePort),
			DstIP:     r.DestIP,
			DstPort:   int(r.DestPort),
			Method:    method,
			FromUser:  r.FromUser,
			ToUser:    toUser,
			CallID:    r.CallID,
			SessionID: r.CallID,
			UserAgent: FormatUserAgent(r.UserAgent),
			CSeq:      r.CSeq,
		}
	}
	return out
}

// FormatUserAgent transforms raw SIP User-Agent strings into compact display forms.
// "Asterisk PBX 11.13.1~dfsg-2+deb8u4" → "* 11.13.1"
// "FPBX-15.0.16.75(16.13.0)" → "FPBX 16.13.0"
func FormatUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return ""
	}

	// Asterisk: extract version number (first dotted version after "Asterisk")
	if strings.Contains(strings.ToLower(ua), "asterisk") {
		// Find first digit sequence that looks like a version (x.y.z)
		for i := 0; i < len(ua); i++ {
			if ua[i] >= '0' && ua[i] <= '9' {
				// Read until non-version character
				end := i
				for end < len(ua) && (ua[end] >= '0' && ua[end] <= '9' || ua[end] == '.') {
					end++
				}
				ver := strings.TrimRight(ua[i:end], ".")
				if strings.Contains(ver, ".") {
					return "Asterisk " + ver
				}
			}
		}
	}

	// FPBX: extract version from parentheses
	if strings.HasPrefix(ua, "FPBX") {
		if open := strings.Index(ua, "("); open >= 0 {
			if close := strings.Index(ua[open:], ")"); close >= 0 {
				return "FPBX " + ua[open+1:open+close]
			}
		}
	}

	return ua
}

// FormatSessionID formats a SIP Call-ID into a compact session identifier.
// e.g. "824e9152-ab4f-453a-9293-45827b4c96d1" → "824e..96d1"
// e.g. "1a675a3e795a2f23@172.31.1.2:5060" → "1a67..2f23@172.31.1.2:5060"
func FormatSessionID(callID string) string {
	if callID == "" {
		return ""
	}

	local := callID
	host := ""
	if idx := strings.Index(callID, "@"); idx >= 0 {
		local = callID[:idx]
		host = callID[idx:]
	}

	stripped := strings.ReplaceAll(local, "-", "")
	if len(stripped) > 8 {
		local = stripped[:4] + ".." + stripped[len(stripped)-4:]
	} else {
		local = stripped
	}

	return local + host
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
		limit = 200
	}

	// Build search filters as array items (hepid=1 for SIP, profile=call)
	filters := []map[string]any{
		{"name": "limit", "value": fmt.Sprintf("%d", limit), "type": "string", "hepid": 1},
	}
	if params.SmartInput != "" {
		filters = append(filters, map[string]any{
			"name": "smartinput", "value": params.SmartInput, "type": "string", "hepid": 1,
		})
	}
	if params.CallID != "" {
		filters = append(filters, map[string]any{
			"name": "sid", "value": params.CallID, "type": "string", "hepid": 1,
		})
	}

	// Compute local timezone offset in minutes (Homer convention: negative of UTC offset)
	_, offsetSec := time.Now().Zone()
	tzMinutes := -(offsetSec / 60)

	return map[string]any{
		"config": map[string]any{
			"protocol_id":      map[string]any{"name": "SIP", "value": 1},
			"protocol_profile": map[string]any{"name": "call", "value": "call"},
		},
		"param": map[string]any{
			"transaction": map[string]any{},
			"limit":       limit,
			"search": map[string]any{
				"1_call": filters,
			},
			"location": map[string]any{},
			"timezone": map[string]any{
				"name":  "Local",
				"value": tzMinutes,
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
		if c.Debug {
			var pretty bytes.Buffer
			json.Indent(&pretty, data, "", "  ")
			fmt.Fprintf(os.Stderr, "\n[DEBUG] %s %s%s\n%s\n\n", method, c.baseURL, path, pretty.String())
		}
		bodyReader = bytes.NewReader(data)
	} else if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] %s %s%s\n\n", method, c.baseURL, path)
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
