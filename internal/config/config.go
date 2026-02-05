package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dex", "config.json"), nil
}

// ConfigDir returns the config directory path
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dex"), nil
}

// Config is the root configuration struct
type Config struct {
	// Global settings (non-integration specific)
	ActivityDays int `json:"activity_days,omitempty" envconfig:"ACTIVITY_DAYS" default:"14"`

	// Integration configs (embedded)
	GitLab GitLabConfig `json:"gitlab,omitempty"`
	Jira   JiraConfig   `json:"jira,omitempty"`
	Slack  SlackConfig  `json:"slack,omitempty"`
}

// GitLabConfig holds GitLab-specific configuration
type GitLabConfig struct {
	URL   string `json:"url,omitempty" envconfig:"GITLAB_URL"`
	Token string `json:"token,omitempty" envconfig:"GITLAB_PERSONAL_TOKEN"`
}

// JiraConfig holds Jira-specific configuration
type JiraConfig struct {
	ClientID     string     `json:"client_id,omitempty" envconfig:"JIRA_CLIENT_ID"`
	ClientSecret string     `json:"client_secret,omitempty" envconfig:"JIRA_CLIENT_SECRET"`
	BaseURL      string     `json:"base_url,omitempty" envconfig:"JIRA_BASE_URL"`
	CloudID      string     `json:"cloud_id,omitempty"`
	Token        *JiraToken `json:"token,omitempty"`
}

// SlackConfig holds Slack-specific configuration
type SlackConfig struct {
	BotToken string `json:"bot_token,omitempty" envconfig:"SLACK_BOT_TOKEN"`
	AppToken string `json:"app_token,omitempty" envconfig:"SLACK_APP_TOKEN"` // For Socket Mode
}

// JiraToken holds Jira OAuth tokens
type JiraToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CloudID      string    `json:"cloud_id,omitempty"`
}

// IsExpired checks if the token is expired (with 1 min buffer)
func (t *JiraToken) IsExpired() bool {
	if t == nil {
		return true
	}
	return time.Now().After(t.ExpiresAt.Add(-time.Minute))
}

// Load reads config from file and applies environment variable overrides
func Load() (*Config, error) {
	cfg, err := LoadFromFile()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if cfg == nil {
		cfg = &Config{}
	}

	// Apply defaults and env overrides
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.Jira.BaseURL == "" {
		cfg.Jira.BaseURL = "https://api.atlassian.com"
	}

	return cfg, nil
}

// LoadFromFile reads config from file only (no env overrides)
// Used when we want to modify and write back without losing env-only values
func LoadFromFile() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the config to file
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// RequireGitLab validates that GitLab config is present
func (c *Config) RequireGitLab() error {
	if c.GitLab.URL == "" {
		return errors.New("GitLab URL not configured. Set GITLAB_URL or add to ~/.dex/config.json")
	}
	if c.GitLab.Token == "" {
		return errors.New("GitLab token not configured. Set GITLAB_PERSONAL_TOKEN or add to ~/.dex/config.json")
	}
	return nil
}

// RequireJira validates that Jira OAuth config is present
func (c *Config) RequireJira() error {
	if c.Jira.ClientID == "" || c.Jira.ClientSecret == "" {
		return errors.New("Jira OAuth not configured. Set JIRA_CLIENT_ID and JIRA_CLIENT_SECRET or add to ~/.dex/config.json")
	}
	return nil
}

// RequireSlack validates that Slack bot token is present
func (c *Config) RequireSlack() error {
	if c.Slack.BotToken == "" {
		return errors.New("Slack bot token not configured. Set SLACK_BOT_TOKEN or add to ~/.dex/config.json")
	}
	return nil
}
