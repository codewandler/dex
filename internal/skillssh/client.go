package skillssh

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://skills.sh/api"

// Client wraps the skills.sh API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Skill represents a skill from the search results
type Skill struct {
	ID       string `json:"id"`
	SkillID  string `json:"skillId"`
	Name     string `json:"name"`
	Installs int    `json:"installs"`
	Source   string `json:"source"`
}

// SearchResponse represents the API response from /search
type SearchResponse struct {
	Query      string  `json:"query"`
	SearchType string  `json:"searchType"`
	Skills     []Skill `json:"skills"`
	Count      int     `json:"count"`
	DurationMs int     `json:"duration_ms"`
}

// NewClient creates a new skills.sh client
func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search queries the skills.sh API for skills matching the query
func (c *Client) Search(query string, limit int) (*SearchResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/search?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to search skills: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("skills.sh returned status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
