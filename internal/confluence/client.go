package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/codewandler/dex/internal/atlassian"
	"github.com/codewandler/dex/internal/config"
)

const confluenceScopes = "read:confluence-content.all read:confluence-space.summary search:confluence offline_access"

// Space represents a Confluence space
type Space struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description struct {
		Plain struct {
			Value string `json:"value"`
		} `json:"plain"`
	} `json:"description"`
	HomepageID string `json:"homepageId"`
}

// Page represents a Confluence page
type Page struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Title   string `json:"title"`
	SpaceID string `json:"spaceId"`
	Body    struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number  int    `json:"number"`
		Message string `json:"message"`
		CreatedAt string `json:"createdAt"`
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// SearchResult represents a Confluence search result
type SearchResult struct {
	Results []struct {
		Content struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"content"`
		Excerpt      string `json:"excerpt"`
		URL          string `json:"url"`
		LastModified string `json:"lastModified"`
		FriendlyLastModified string `json:"friendlyLastModified"`
	} `json:"results"`
	Start      int `json:"start"`
	Limit      int `json:"limit"`
	Size       int `json:"size"`
	TotalSize  int `json:"totalSize"`
}

type Client struct {
	config *config.Config
	token  *atlassian.Token
	oauth  *atlassian.OAuthFlow
}

func NewClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err := cfg.RequireConfluence(); err != nil {
		return nil, err
	}

	client := &Client{
		config: cfg,
		oauth: atlassian.NewOAuthFlow(atlassian.OAuthConfig{
			ClientID:     cfg.Confluence.ClientID,
			ClientSecret: cfg.Confluence.ClientSecret,
			Scopes:       confluenceScopes,
		}),
	}

	client.token = cfg.Confluence.Token

	return client, nil
}

// EnsureAuth ensures we have a valid token, refreshing or re-authenticating as needed
func (c *Client) EnsureAuth(ctx context.Context) error {
	if c.token == nil {
		token, err := c.oauth.StartAuthServer(ctx)
		if err != nil {
			return err
		}
		if err := SaveToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}
		c.token = token
		return nil
	}

	if c.token.IsExpired() {
		token, err := c.oauth.RefreshToken(ctx, c.token.RefreshToken, c.token)
		if err != nil {
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

	if c.token.SiteURL == "" {
		siteInfo, err := c.oauth.GetSiteInfo(ctx, c.token.AccessToken)
		if err == nil {
			c.token.SiteURL = siteInfo.SiteURL
			if c.token.CloudID == "" {
				c.token.CloudID = siteInfo.CloudID
			}
			_ = SaveToken(c.token)
		}
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	if err := c.EnsureAuth(ctx); err != nil {
		return nil, err
	}

	baseURL := fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s", c.token.CloudID)
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

// ListSpaces lists Confluence spaces
func (c *Client) ListSpaces(ctx context.Context, limit int) ([]Space, error) {
	query := url.Values{
		"limit": {fmt.Sprintf("%d", limit)},
	}

	resp, err := c.doRequest(ctx, "GET", "/wiki/api/v2/spaces", query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("failed to list spaces %d: %v", resp.StatusCode, errResp)
	}

	var result struct {
		Results []Space `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// GetPage fetches a single page by ID
func (c *Client) GetPage(ctx context.Context, pageID string) (*Page, error) {
	query := url.Values{
		"body-format": {"storage"},
	}

	resp, err := c.doRequest(ctx, "GET", "/wiki/api/v2/pages/"+pageID, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("page %s not found", pageID)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error %d: %v", resp.StatusCode, errResp)
	}

	var page Page
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}

	return &page, nil
}

// Search searches Confluence using CQL
func (c *Client) Search(ctx context.Context, cql string, limit int) (*SearchResult, error) {
	query := url.Values{
		"cql":   {cql},
		"limit": {fmt.Sprintf("%d", limit)},
	}

	resp, err := c.doRequest(ctx, "GET", "/wiki/rest/api/search", query)
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

// GetSiteURL returns the browsable Confluence site URL
func (c *Client) GetSiteURL() string {
	if c.token != nil {
		return c.token.SiteURL
	}
	return ""
}

// StripHTML removes HTML tags from a string for plain text display
func StripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(s, "")
	// Collapse multiple whitespace/newlines
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
