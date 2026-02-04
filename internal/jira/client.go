package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	config *Config
	token  *Token
	oauth  *OAuthFlow
}

type Issue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields struct {
		Summary     string `json:"summary"`
		Description any    `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *struct {
			DisplayName  string `json:"displayName"`
			EmailAddress string `json:"emailAddress"`
		} `json:"assignee"`
		Reporter *struct {
			DisplayName  string `json:"displayName"`
			EmailAddress string `json:"emailAddress"`
		} `json:"reporter"`
		Created string `json:"created"`
		Updated string `json:"updated"`
		Labels  []string `json:"labels"`
	} `json:"fields"`
}

type SearchResult struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Issues     []Issue `json:"issues"`
}

func NewClient() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	client := &Client{
		config: cfg,
		oauth:  NewOAuthFlow(cfg),
	}

	// Try to load existing token
	token, err := LoadToken()
	if err == nil {
		client.token = token
	}

	return client, nil
}

// EnsureAuth ensures we have a valid token, refreshing or re-authenticating as needed
func (c *Client) EnsureAuth(ctx context.Context) error {
	if c.token == nil {
		// No token, need to authenticate
		token, err := c.oauth.StartAuthServer(ctx)
		if err != nil {
			return err
		}
		c.token = token
		return nil
	}

	if c.token.IsExpired() {
		// Token expired, try to refresh
		token, err := c.oauth.RefreshToken(ctx, c.token.RefreshToken)
		if err != nil {
			// Refresh failed, re-authenticate
			token, err = c.oauth.StartAuthServer(ctx)
			if err != nil {
				return err
			}
		}
		c.token = token
		if err := SaveToken(token); err != nil {
			return fmt.Errorf("failed to save refreshed token: %w", err)
		}
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	if err := c.EnsureAuth(ctx); err != nil {
		return nil, err
	}

	baseURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3", c.token.CloudID)
	fullURL := baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token.AccessToken)
	req.Header.Set("Accept", "application/json")

	return http.DefaultClient.Do(req)
}

// GetIssue fetches a single issue by key (e.g., "TEL-117")
func (c *Client) GetIssue(ctx context.Context, issueKey string) (*Issue, error) {
	resp, err := c.doRequest(ctx, "GET", "/issue/"+issueKey, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("issue %s not found", issueKey)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error %d: %v", resp.StatusCode, errResp)
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// SearchIssues searches for issues using JQL
func (c *Client) SearchIssues(ctx context.Context, jql string, maxResults int) (*SearchResult, error) {
	query := url.Values{
		"jql":        {jql},
		"maxResults": {fmt.Sprintf("%d", maxResults)},
		"fields":     {"summary,status,assignee,reporter,priority,issuetype,created,updated,labels"},
	}

	resp, err := c.doRequest(ctx, "GET", "/search/jql", query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("search failed %d: %v", resp.StatusCode, errResp)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetMyIssues fetches issues assigned to the current user
func (c *Client) GetMyIssues(ctx context.Context, maxResults int) (*SearchResult, error) {
	return c.SearchIssues(ctx, "assignee = currentUser() AND status != Done ORDER BY updated DESC", maxResults)
}

// GetRecentIssues fetches recently updated issues
func (c *Client) GetRecentIssues(ctx context.Context, maxResults int) (*SearchResult, error) {
	return c.SearchIssues(ctx, "updated >= -7d ORDER BY updated DESC", maxResults)
}

// FormatIssue returns a formatted string representation of an issue
func FormatIssue(issue *Issue) string {
	assignee := "Unassigned"
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}

	return fmt.Sprintf(`%s: %s
  Type:     %s
  Status:   %s
  Priority: %s
  Assignee: %s
  Labels:   %v
  Created:  %s
  Updated:  %s`,
		issue.Key,
		issue.Fields.Summary,
		issue.Fields.IssueType.Name,
		issue.Fields.Status.Name,
		issue.Fields.Priority.Name,
		assignee,
		issue.Fields.Labels,
		issue.Fields.Created,
		issue.Fields.Updated,
	)
}
