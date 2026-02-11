package atlassian

import "time"

// Token holds Atlassian OAuth tokens (shared by Jira, Confluence, etc.)
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CloudID      string    `json:"cloud_id,omitempty"`
	SiteURL      string    `json:"site_url,omitempty"`
}

// IsExpired checks if the token is expired (with 1 min buffer)
func (t *Token) IsExpired() bool {
	if t == nil {
		return true
	}
	return time.Now().After(t.ExpiresAt.Add(-time.Minute))
}

// SiteInfo contains cloud ID and browsable site URL from the accessible-resources API
type SiteInfo struct {
	CloudID string
	SiteURL string
}
