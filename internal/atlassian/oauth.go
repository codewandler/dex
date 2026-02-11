package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authURL     = "https://auth.atlassian.com/authorize"
	tokenURL    = "https://auth.atlassian.com/oauth/token"
	redirectURI = "http://localhost:8089/callback"
)

// OAuthConfig holds the credentials and scopes for an Atlassian OAuth flow.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string
}

// OAuthFlow handles the Atlassian OAuth 2.0 flow.
type OAuthFlow struct {
	cfg OAuthConfig
}

// NewOAuthFlow creates a new OAuthFlow with the given config.
func NewOAuthFlow(cfg OAuthConfig) *OAuthFlow {
	return &OAuthFlow{cfg: cfg}
}

// GetAuthURL returns the URL to start the OAuth flow.
func (o *OAuthFlow) GetAuthURL(state string) string {
	params := url.Values{
		"audience":      {"api.atlassian.com"},
		"client_id":     {o.cfg.ClientID},
		"scope":         {o.cfg.Scopes},
		"redirect_uri":  {redirectURI},
		"state":         {state},
		"response_type": {"code"},
		"prompt":        {"consent"},
	}
	return authURL + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
func (o *OAuthFlow) ExchangeCode(ctx context.Context, code string) (*Token, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {o.cfg.ClientID},
		"client_secret": {o.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("token exchange failed: %v", errResp)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	token := &Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	siteInfo, err := o.GetSiteInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get site info: %w", err)
	}
	token.CloudID = siteInfo.CloudID
	token.SiteURL = siteInfo.SiteURL

	return token, nil
}

// RefreshToken refreshes an expired access token.
// existingToken is used to preserve CloudID and SiteURL when available.
func (o *OAuthFlow) RefreshToken(ctx context.Context, refreshToken string, existingToken *Token) (*Token, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {o.cfg.ClientID},
		"client_secret": {o.cfg.ClientSecret},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("token refresh failed: %v", errResp)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	cloudID := ""
	siteURL := ""
	if existingToken != nil {
		cloudID = existingToken.CloudID
		siteURL = existingToken.SiteURL
	}

	token := &Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		CloudID:      cloudID,
		SiteURL:      siteURL,
	}

	if token.CloudID == "" || token.SiteURL == "" {
		siteInfo, err := o.GetSiteInfo(ctx, token.AccessToken)
		if err == nil {
			token.CloudID = siteInfo.CloudID
			token.SiteURL = siteInfo.SiteURL
		}
	}

	return token, nil
}

// GetSiteInfo fetches site info (CloudID and SiteURL) from the accessible-resources API.
func (o *OAuthFlow) GetSiteInfo(ctx context.Context, accessToken string) (*SiteInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.atlassian.com/oauth/token/accessible-resources", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var resources []struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}

	if len(resources) == 0 {
		return nil, fmt.Errorf("no accessible Atlassian sites found")
	}

	return &SiteInfo{
		CloudID: resources[0].ID,
		SiteURL: resources[0].URL,
	}, nil
}

// StartAuthServer starts a local server to handle the OAuth callback.
// It returns the token for the caller to persist.
func (o *OAuthFlow) StartAuthServer(ctx context.Context) (*Token, error) {
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errChan <- fmt.Errorf("auth error: %s - %s", errMsg, r.URL.Query().Get("error_description"))
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code received")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h1>Authorization successful!</h1><p>You can close this window.</p><script>window.close()</script></body></html>`)
		codeChan <- code
	})

	server := &http.Server{Addr: ":8089", Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	authURL := o.GetAuthURL(state)
	fmt.Printf("\nOpen this URL in your browser to authorize:\n\n%s\n\nWaiting for authorization...\n", authURL)

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return nil, ctx.Err()
	}

	server.Shutdown(ctx)

	return o.ExchangeCode(ctx, code)
}
