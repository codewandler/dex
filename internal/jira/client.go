package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
)

type Client struct {
	config *config.Config
	token  *config.JiraToken
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
		Created    string   `json:"created"`
		Updated    string   `json:"updated"`
		Labels     []string `json:"labels"`
		Parent     *Issue   `json:"parent"`
		Subtasks   []Issue  `json:"subtasks"`
		IssueLinks []struct {
			ID   string `json:"id"`
			Type struct {
				Name    string `json:"name"`
				Inward  string `json:"inward"`
				Outward string `json:"outward"`
			} `json:"type"`
			InwardIssue *struct {
				Key    string `json:"key"`
				Fields struct {
					Summary string `json:"summary"`
					Status  struct {
						Name string `json:"name"`
					} `json:"status"`
				} `json:"fields"`
			} `json:"inwardIssue"`
			OutwardIssue *struct {
				Key    string `json:"key"`
				Fields struct {
					Summary string `json:"summary"`
					Status  struct {
						Name string `json:"name"`
					} `json:"status"`
				} `json:"fields"`
			} `json:"outwardIssue"`
		} `json:"issuelinks"`
		Comment *struct {
			Comments []Comment `json:"comments"`
			Total    int       `json:"total"`
		} `json:"comment"`
	} `json:"fields"`
}

type Comment struct {
	ID      string `json:"id"`
	Author  *struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Body    any    `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type SearchResult struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Issues     []Issue `json:"issues"`
}

// Project represents a Jira project
type Project struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Name       string `json:"name"`
	ProjectType string `json:"projectTypeKey"`
	Self       string `json:"self"`
}

func NewClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err := cfg.RequireJira(); err != nil {
		return nil, err
	}

	client := &Client{
		config: cfg,
		oauth:  NewOAuthFlow(cfg),
	}

	// Try to load existing token
	client.token = cfg.Jira.Token

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
		if err := SaveToken(c.token); err != nil {
			return fmt.Errorf("failed to save refreshed token: %w", err)
		}
	}

	// Backfill SiteURL if missing (for tokens created before this field existed)
	if c.token.SiteURL == "" {
		siteInfo, err := c.oauth.GetSiteInfo(ctx, c.token.AccessToken)
		if err == nil {
			c.token.SiteURL = siteInfo.SiteURL
			if c.token.CloudID == "" {
				c.token.CloudID = siteInfo.CloudID
			}
			_ = SaveToken(c.token) // Best effort save
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

func (c *Client) doRequestWithBody(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	if err := c.EnsureAuth(ctx); err != nil {
		return nil, err
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	baseURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3", c.token.CloudID)
	fullURL := baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

// GetIssue fetches a single issue by key (e.g., "TEL-117")
func (c *Client) GetIssue(ctx context.Context, issueKey string) (*Issue, error) {
	query := url.Values{
		"expand": {"renderedFields"},
	}
	resp, err := c.doRequest(ctx, "GET", "/issue/"+issueKey, query)
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

// ListProjects fetches all accessible Jira projects
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	resp, err := c.doRequest(ctx, "GET", "/project", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("failed to list projects %d: %v", resp.StatusCode, errResp)
	}

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}

	return projects, nil
}

// GetProjectKeys returns just the project keys (e.g., DEV, TEL)
func (c *Client) GetProjectKeys(ctx context.Context) ([]string, error) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(projects))
	for i, p := range projects {
		keys[i] = p.Key
	}
	return keys, nil
}

// GetSiteURL returns the browsable Jira site URL (e.g., https://company.atlassian.net)
func (c *Client) GetSiteURL() string {
	if c.token != nil {
		return c.token.SiteURL
	}
	return ""
}

// CreateIssueRequest contains the parameters for creating a new issue
type CreateIssueRequest struct {
	ProjectKey  string   // Required: project key (e.g., "DEV")
	IssueType   string   // Required: issue type (e.g., "Task", "Bug", "Story", "Sub-task")
	Summary     string   // Required: issue summary/title
	Description string   // Optional: plain text description
	Labels      []string // Optional: labels to apply
	Assignee    string   // Optional: account ID or email
	Priority    string   // Optional: "Lowest", "Low", "Medium", "High", "Highest"
	Parent      string   // Optional: parent issue key for subtasks (e.g., "DEV-123")
}

// CreateIssue creates a new Jira issue
func (c *Client) CreateIssue(ctx context.Context, req CreateIssueRequest) (*Issue, error) {
	// Build the issue fields
	fields := map[string]interface{}{
		"project": map[string]string{
			"key": req.ProjectKey,
		},
		"issuetype": map[string]string{
			"name": req.IssueType,
		},
		"summary": req.Summary,
	}

	// Add description in ADF format if provided
	if req.Description != "" {
		fields["description"] = buildADF(req.Description)
	}

	// Add labels if provided
	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}

	// Add priority if provided
	if req.Priority != "" {
		fields["priority"] = map[string]string{
			"name": req.Priority,
		}
	}

	// Add parent for subtasks
	if req.Parent != "" {
		fields["parent"] = map[string]string{
			"key": req.Parent,
		}
	}

	// Resolve assignee (could be email or account ID)
	if req.Assignee != "" {
		accountID := req.Assignee
		// If it looks like an email, look up the user
		if strings.Contains(req.Assignee, "@") {
			users, err := c.FindUser(ctx, req.Assignee)
			if err != nil {
				return nil, fmt.Errorf("failed to find user %q: %w", req.Assignee, err)
			}
			if len(users) == 0 {
				return nil, fmt.Errorf("no user found with email %q", req.Assignee)
			}
			accountID = users[0].AccountID
		}
		fields["assignee"] = map[string]string{
			"id": accountID,
		}
	}

	body := map[string]interface{}{
		"fields": fields,
	}

	resp, err := c.doRequestWithBody(ctx, "POST", "/issue", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create issue (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse the response to get the created issue key
	var createResp struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("failed to parse create response: %w", err)
	}

	// Fetch the full issue to return
	return c.GetIssue(ctx, createResp.Key)
}

// LinkIssuesRequest contains the parameters for linking two issues
type LinkIssuesRequest struct {
	InwardIssue  string // The issue that is the source of the link (e.g., "DEV-123")
	OutwardIssue string // The issue that is the target of the link (e.g., "DEV-456")
	LinkType     string // The type of link (e.g., "Relates", "Blocks", "Duplicates")
}

// LinkIssues creates a link between two issues
func (c *Client) LinkIssues(ctx context.Context, req LinkIssuesRequest) error {
	body := map[string]interface{}{
		"type": map[string]string{
			"name": req.LinkType,
		},
		"inwardIssue": map[string]string{
			"key": req.InwardIssue,
		},
		"outwardIssue": map[string]string{
			"key": req.OutwardIssue,
		},
	}

	resp, err := c.doRequestWithBody(ctx, "POST", "/issueLink", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to link issues (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// IssueLinkType represents a type of link between issues
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// ListLinkTypes returns all available issue link types
func (c *Client) ListLinkTypes(ctx context.Context) ([]IssueLinkType, error) {
	resp, err := c.doRequest(ctx, "GET", "/issueLinkType", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list link types (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse link types: %w", err)
	}

	return result.IssueLinkTypes, nil
}

// DeleteIssue deletes an issue by key. If deleteSubtasks is true, subtasks are also deleted.
func (c *Client) DeleteIssue(ctx context.Context, issueKey string, deleteSubtasks bool) error {
	query := url.Values{}
	if deleteSubtasks {
		query.Set("deleteSubtasks", "true")
	}
	resp, err := c.doRequest(ctx, "DELETE", "/issue/"+issueKey, query)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete issue (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// FindUser searches for users by query (email, name, etc.)
func (c *Client) FindUser(ctx context.Context, query string) ([]User, error) {
	params := url.Values{
		"query": {query},
	}
	resp, err := c.doRequest(ctx, "GET", "/user/search", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("user search failed %d: %v", resp.StatusCode, errResp)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}

// buildADF creates a minimal Atlassian Document Format structure from plain text
func buildADF(text string) map[string]interface{} {
	// Split text into paragraphs
	paragraphs := strings.Split(text, "\n\n")
	content := make([]interface{}, 0, len(paragraphs))

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// Handle single newlines within a paragraph as hard breaks
		lines := strings.Split(para, "\n")
		paraContent := make([]interface{}, 0)
		for i, line := range lines {
			if i > 0 {
				paraContent = append(paraContent, map[string]string{"type": "hardBreak"})
			}
			paraContent = append(paraContent, map[string]interface{}{
				"type": "text",
				"text": line,
			})
		}
		content = append(content, map[string]interface{}{
			"type":    "paragraph",
			"content": paraContent,
		})
	}

	// If no content, create an empty paragraph
	if len(content) == 0 {
		content = append(content, map[string]interface{}{
			"type": "paragraph",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": text,
				},
			},
		})
	}

	return map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": content,
	}
}

// User represents the authenticated Jira user
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// GetCurrentUser fetches the authenticated user's info
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	resp, err := c.doRequest(ctx, "GET", "/myself", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("failed to get current user %d: %v", resp.StatusCode, errResp)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// FormatIssue returns a formatted string representation of an issue
func FormatIssue(issue *Issue) string {
	var result strings.Builder

	assignee := "Unassigned"
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.DisplayName
	}

	reporter := "Unknown"
	if issue.Fields.Reporter != nil {
		reporter = issue.Fields.Reporter.DisplayName
	}

	description := parseADF(issue.Fields.Description)
	if description == "" {
		description = "(no description)"
	}

	// Basic info
	result.WriteString(fmt.Sprintf("%s: %s\n", issue.Key, issue.Fields.Summary))
	result.WriteString(fmt.Sprintf("  Type:     %s\n", issue.Fields.IssueType.Name))
	result.WriteString(fmt.Sprintf("  Status:   %s\n", issue.Fields.Status.Name))
	result.WriteString(fmt.Sprintf("  Priority: %s\n", issue.Fields.Priority.Name))
	result.WriteString(fmt.Sprintf("  Assignee: %s\n", assignee))
	result.WriteString(fmt.Sprintf("  Reporter: %s\n", reporter))
	result.WriteString(fmt.Sprintf("  Labels:   %v\n", issue.Fields.Labels))
	result.WriteString(fmt.Sprintf("  Created:  %s\n", issue.Fields.Created))
	result.WriteString(fmt.Sprintf("  Updated:  %s\n", issue.Fields.Updated))

	// Parent issue
	if issue.Fields.Parent != nil {
		result.WriteString(fmt.Sprintf("  Parent:   %s - %s\n", issue.Fields.Parent.Key, issue.Fields.Parent.Fields.Summary))
	}

	// Subtasks
	if len(issue.Fields.Subtasks) > 0 {
		result.WriteString("\nSubtasks:\n")
		for _, subtask := range issue.Fields.Subtasks {
			result.WriteString(fmt.Sprintf("  • %s [%s] %s\n",
				subtask.Key,
				subtask.Fields.Status.Name,
				subtask.Fields.Summary,
			))
		}
	}

	// Linked issues
	if len(issue.Fields.IssueLinks) > 0 {
		result.WriteString("\nLinked Issues:\n")
		for _, link := range issue.Fields.IssueLinks {
			if link.OutwardIssue != nil {
				result.WriteString(fmt.Sprintf("  • %s %s [%s] %s\n",
					link.Type.Outward,
					link.OutwardIssue.Key,
					link.OutwardIssue.Fields.Status.Name,
					link.OutwardIssue.Fields.Summary,
				))
			}
			if link.InwardIssue != nil {
				result.WriteString(fmt.Sprintf("  • %s %s [%s] %s\n",
					link.Type.Inward,
					link.InwardIssue.Key,
					link.InwardIssue.Fields.Status.Name,
					link.InwardIssue.Fields.Summary,
				))
			}
		}
	}

	// Description
	result.WriteString("\nDescription:\n")
	result.WriteString(indentText(description, "  "))

	// Comments
	if issue.Fields.Comment != nil && len(issue.Fields.Comment.Comments) > 0 {
		result.WriteString(fmt.Sprintf("\n\nComments (%d):\n", issue.Fields.Comment.Total))
		for _, comment := range issue.Fields.Comment.Comments {
			author := "Unknown"
			if comment.Author != nil {
				author = comment.Author.DisplayName
			}
			commentBody := parseADF(comment.Body)
			result.WriteString(fmt.Sprintf("\n  ── %s (%s) ──\n", author, formatJiraTime(comment.Created)))
			result.WriteString(indentText(commentBody, "  "))
			result.WriteString("\n")
		}
	}

	return result.String()
}

// formatJiraTime formats a Jira timestamp to a more readable format
func formatJiraTime(timestamp string) string {
	// Jira format: 2025-11-11T11:03:29.626+0100
	t, err := parseJiraTime(timestamp)
	if err != nil {
		return timestamp
	}
	return t.Format("2006-01-02 15:04")
}

// parseJiraTime parses a Jira timestamp
func parseJiraTime(timestamp string) (time.Time, error) {
	// Try different formats Jira might use
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, timestamp); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timestamp)
}

// parseADF converts Atlassian Document Format to plain text
func parseADF(doc any) string {
	if doc == nil {
		return ""
	}

	docMap, ok := doc.(map[string]any)
	if !ok {
		// Maybe it's already a string (old format)
		if s, ok := doc.(string); ok {
			return s
		}
		return ""
	}

	content, ok := docMap["content"].([]any)
	if !ok {
		return ""
	}

	var result strings.Builder
	for i, node := range content {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(parseADFNode(node))
	}
	return strings.TrimSpace(result.String())
}

// parseADFNode recursively parses an ADF node
func parseADFNode(node any) string {
	nodeMap, ok := node.(map[string]any)
	if !ok {
		return ""
	}

	nodeType, _ := nodeMap["type"].(string)

	switch nodeType {
	case "text":
		text, _ := nodeMap["text"].(string)
		return text

	case "paragraph":
		return parseADFContent(nodeMap) + "\n"

	case "heading":
		level, _ := nodeMap["attrs"].(map[string]any)["level"].(float64)
		prefix := strings.Repeat("#", int(level)) + " "
		return prefix + parseADFContent(nodeMap) + "\n"

	case "bulletList":
		return parseADFList(nodeMap, "• ")

	case "orderedList":
		return parseADFOrderedList(nodeMap)

	case "listItem":
		return parseADFContent(nodeMap)

	case "codeBlock":
		return "```\n" + parseADFContent(nodeMap) + "\n```\n"

	case "blockquote":
		lines := strings.Split(parseADFContent(nodeMap), "\n")
		var quoted []string
		for _, line := range lines {
			quoted = append(quoted, "> "+line)
		}
		return strings.Join(quoted, "\n") + "\n"

	case "hardBreak":
		return "\n"

	case "mention":
		if attrs, ok := nodeMap["attrs"].(map[string]any); ok {
			if text, ok := attrs["text"].(string); ok {
				return text
			}
		}
		return "@mention"

	case "inlineCard", "link":
		if attrs, ok := nodeMap["attrs"].(map[string]any); ok {
			if url, ok := attrs["url"].(string); ok {
				return url
			}
		}
		return parseADFContent(nodeMap)

	case "mediaSingle", "media":
		return "[media]"

	case "table":
		return parseADFTable(nodeMap)

	case "tableRow", "tableCell", "tableHeader":
		return parseADFContent(nodeMap)

	default:
		// For unknown types, try to extract content
		return parseADFContent(nodeMap)
	}
}

// parseADFContent extracts text from a node's content array
func parseADFContent(nodeMap map[string]any) string {
	content, ok := nodeMap["content"].([]any)
	if !ok {
		return ""
	}

	var result strings.Builder
	for _, child := range content {
		result.WriteString(parseADFNode(child))
	}
	return result.String()
}

// parseADFList parses a bullet list
func parseADFList(nodeMap map[string]any, bullet string) string {
	content, ok := nodeMap["content"].([]any)
	if !ok {
		return ""
	}

	var result strings.Builder
	for _, item := range content {
		itemText := strings.TrimSpace(parseADFNode(item))
		result.WriteString(bullet + itemText + "\n")
	}
	return result.String()
}

// parseADFOrderedList parses a numbered list
func parseADFOrderedList(nodeMap map[string]any) string {
	content, ok := nodeMap["content"].([]any)
	if !ok {
		return ""
	}

	var result strings.Builder
	for i, item := range content {
		itemText := strings.TrimSpace(parseADFNode(item))
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, itemText))
	}
	return result.String()
}

// parseADFTable parses a table into simple text format
func parseADFTable(nodeMap map[string]any) string {
	content, ok := nodeMap["content"].([]any)
	if !ok {
		return ""
	}

	var result strings.Builder
	for _, row := range content {
		rowMap, ok := row.(map[string]any)
		if !ok {
			continue
		}
		cells, ok := rowMap["content"].([]any)
		if !ok {
			continue
		}
		var cellTexts []string
		for _, cell := range cells {
			cellText := strings.TrimSpace(parseADFNode(cell))
			cellTexts = append(cellTexts, cellText)
		}
		result.WriteString("| " + strings.Join(cellTexts, " | ") + " |\n")
	}
	return result.String()
}

// indentText adds a prefix to each line of text
func indentText(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
