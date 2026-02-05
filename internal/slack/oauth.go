package slack

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/config"
)

// silentLogger discards TLS handshake errors from self-signed certs
type silentLogger struct{}

func (s silentLogger) Write(p []byte) (n int, err error) {
	// Silently discard TLS handshake errors
	return len(p), nil
}

const (
	slackAuthURL  = "https://slack.com/oauth/v2/authorize"
	slackTokenURL = "https://slack.com/api/oauth.v2.access"
	redirectURI   = "https://localhost:8089/callback"
)

// Bot scopes (what the bot can do)
var botScopes = []string{
	"app_mentions:read",
	"assistant:write",
	"channels:history",
	"channels:read",
	"chat:write",
	"chat:write.public",
	"groups:history",
	"groups:read",
	"im:read",
	"im:write",
	"reactions:read",
	"reactions:write",
	"users.profile:read",
	"users:read",
}

// User scopes (what actions can be performed as the user)
var userScopes = []string{
	"bookmarks:read",
	"channels:history",
	"groups:history",
	"im:history",
	"mpim:history",
	"search:read",
	"users:write",
}

// OAuthFlow handles Slack OAuth authentication
type OAuthFlow struct {
	config *config.Config
}

// NewOAuthFlow creates a new OAuth flow handler
func NewOAuthFlow(cfg *config.Config) *OAuthFlow {
	return &OAuthFlow{config: cfg}
}

// GetAuthURL returns the URL to start the OAuth flow
func (o *OAuthFlow) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":    {o.config.Slack.ClientID},
		"scope":        {strings.Join(botScopes, ",")},
		"user_scope":   {strings.Join(userScopes, ",")},
		"redirect_uri": {redirectURI},
		"state":        {state},
	}
	return slackAuthURL + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens
func (o *OAuthFlow) ExchangeCode(ctx context.Context, code string) (*config.SlackToken, error) {
	data := url.Values{
		"client_id":     {o.config.Slack.ClientID},
		"client_secret": {o.config.Slack.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", slackTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error,omitempty"`
		AccessToken string `json:"access_token"` // Bot token
		TokenType   string `json:"token_type"`
		Team        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"team"`
		BotUserID  string `json:"bot_user_id"`
		AuthedUser struct {
			ID          string `json:"id"`
			Scope       string `json:"scope"`
			AccessToken string `json:"access_token"` // User token
			TokenType   string `json:"token_type"`
		} `json:"authed_user"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if !tokenResp.OK {
		return nil, fmt.Errorf("token exchange failed: %s", tokenResp.Error)
	}

	token := &config.SlackToken{
		AccessToken:  tokenResp.AccessToken,
		UserToken:    tokenResp.AuthedUser.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TeamID:       tokenResp.Team.ID,
		TeamName:     tokenResp.Team.Name,
		BotUserID:    tokenResp.BotUserID,
	}

	return token, nil
}

// StartAuthServer starts a local HTTPS server to handle the OAuth callback
func (o *OAuthFlow) StartAuthServer(ctx context.Context) (*config.SlackToken, error) {
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()

	// Generate self-signed certificate for localhost
	tlsCert, err := generateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %w", err)
	}

	server := &http.Server{
		Addr:    ":8089",
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		},
		ErrorLog: log.New(silentLogger{}, "", 0), // Suppress TLS handshake errors
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errChan <- fmt.Errorf("auth error: %s", errMsg)
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
		fmt.Fprint(w, `<html><body><h1>Slack authorization successful!</h1><p>You can close this window.</p><script>window.close()</script></body></html>`)
		codeChan <- code
	})

	go func() {
		// Use ListenAndServeTLS with empty cert/key files since we set TLSConfig
		if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	authURL := o.GetAuthURL(state)
	fmt.Printf("\nOpen this URL in your browser to authorize:\n\n%s\n\nWaiting for authorization...\n", authURL)
	fmt.Println("\nNote: Your browser may warn about the self-signed certificate. Accept it to continue.")

	// Try to open browser automatically
	openBrowser(authURL)

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

	token, err := o.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	if err := SaveToken(token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}

// generateSelfSignedCert creates a self-signed certificate for localhost
func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"dex OAuth"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}

// openBrowser tries to open the URL in the default browser
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	// Best effort, ignore errors
	_ = cmd.Start()
}

// SaveToken saves the Slack token to the config file
func SaveToken(token *config.SlackToken) error {
	cfg, err := config.LoadFromFile()
	if err != nil {
		cfg = &config.Config{}
	}

	cfg.Slack.Token = token
	// Also set the convenience fields for backward compatibility
	if token != nil {
		cfg.Slack.BotToken = token.AccessToken
		cfg.Slack.UserToken = token.UserToken
	}

	return config.Save(cfg)
}

// LoadToken loads the Slack token from the config file
func LoadToken() (*config.SlackToken, error) {
	cfg, err := config.LoadFromFile()
	if err != nil {
		return nil, err
	}
	return cfg.Slack.Token, nil
}
