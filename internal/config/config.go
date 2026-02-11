package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/codewandler/dex/internal/atlassian"
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
	GitLab     GitLabConfig     `json:"gitlab,omitempty"`
	Jira       JiraConfig       `json:"jira,omitempty"`
	Confluence ConfluenceConfig `json:"confluence,omitempty"`
	Slack      SlackConfig      `json:"slack,omitempty"`
	Loki       LokiConfig       `json:"loki,omitempty"`
	Homer      HomerConfig      `json:"homer,omitempty"`
	Prometheus PrometheusConfig `json:"prometheus,omitempty"`
	SQL        SQLConfig        `json:"sql,omitempty"`
	StatusLine StatusLineConfig `json:"status_line,omitempty"`
}

// SQLConfig holds SQL datasource configuration
type SQLConfig struct {
	Datasources map[string]SQLDatasource `json:"datasources,omitempty"`
}

// SQLDatasource holds connection info for a single datasource
type SQLDatasource struct {
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"` // Default: 3306 for MySQL
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// StatusLineConfig holds status line configuration for Claude Code
type StatusLineConfig struct {
	Format   string                   `json:"format,omitempty"`
	Segments map[string]SegmentConfig `json:"segments,omitempty"`
}

// SegmentConfig holds configuration for a single status line segment
type SegmentConfig struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	Format   string `json:"format,omitempty"`
	CacheTTL string `json:"cache_ttl,omitempty"`
}

// LokiConfig holds Loki-specific configuration
type LokiConfig struct {
	URL string `json:"url,omitempty" envconfig:"LOKI_URL"`
}

// PrometheusConfig holds Prometheus-specific configuration
type PrometheusConfig struct {
	URL string `json:"url,omitempty" envconfig:"PROMETHEUS_URL"`
}

// HomerConfig holds Homer SIP tracing configuration
type HomerConfig struct {
	URL       string                   `json:"url,omitempty" envconfig:"HOMER_URL"`
	Username  string                   `json:"username,omitempty" envconfig:"HOMER_USERNAME"`
	Password  string                   `json:"password,omitempty" envconfig:"HOMER_PASSWORD"`
	Endpoints map[string]HomerEndpoint `json:"endpoints,omitempty"`
}

// HomerEndpoint holds credentials for a specific Homer endpoint
type HomerEndpoint struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// GitLabConfig holds GitLab-specific configuration
type GitLabConfig struct {
	URL   string `json:"url,omitempty" envconfig:"GITLAB_URL"`
	Token string `json:"token,omitempty" envconfig:"GITLAB_PERSONAL_TOKEN"`
}

// JiraConfig holds Jira-specific configuration
type JiraConfig struct {
	ClientID     string           `json:"client_id,omitempty" envconfig:"JIRA_CLIENT_ID"`
	ClientSecret string           `json:"client_secret,omitempty" envconfig:"JIRA_CLIENT_SECRET"`
	BaseURL      string           `json:"base_url,omitempty" envconfig:"JIRA_BASE_URL"`
	CloudID      string           `json:"cloud_id,omitempty"`
	Token        *atlassian.Token `json:"token,omitempty"`
}

// ConfluenceConfig holds Confluence-specific configuration
type ConfluenceConfig struct {
	ClientID     string           `json:"client_id,omitempty" envconfig:"CONFLUENCE_CLIENT_ID"`
	ClientSecret string           `json:"client_secret,omitempty" envconfig:"CONFLUENCE_CLIENT_SECRET"`
	Token        *atlassian.Token `json:"token,omitempty"`
}

// SlackConfig holds Slack-specific configuration
type SlackConfig struct {
	// OAuth credentials (for `dex slack auth`)
	ClientID     string      `json:"client_id,omitempty" envconfig:"SLACK_CLIENT_ID"`
	ClientSecret string      `json:"client_secret,omitempty" envconfig:"SLACK_CLIENT_SECRET"`
	Token        *SlackToken `json:"token,omitempty"`

	// Manual tokens (legacy, can be set directly or via OAuth)
	BotToken  string `json:"bot_token,omitempty" envconfig:"SLACK_BOT_TOKEN"`
	AppToken  string `json:"app_token,omitempty" envconfig:"SLACK_APP_TOKEN"`   // For Socket Mode
	UserToken string `json:"user_token,omitempty" envconfig:"SLACK_USER_TOKEN"` // For search API
}

// SlackToken holds Slack OAuth tokens
type SlackToken struct {
	AccessToken  string `json:"access_token"`            // Bot token (xoxb-...)
	UserToken    string `json:"user_token,omitempty"`    // User token (xoxp-...) if user scopes requested
	RefreshToken string `json:"refresh_token,omitempty"` // For token rotation
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
	BotUserID    string `json:"bot_user_id"`
}

// JiraToken is an alias for atlassian.Token for backward compatibility.
type JiraToken = atlassian.Token

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

// RequireLoki validates that Loki URL is configured
func (c *Config) RequireLoki() error {
	if c.Loki.URL == "" {
		return errors.New("Loki URL not configured. Set LOKI_URL or add to ~/.dex/config.json")
	}
	return nil
}

// RequireConfluence validates that Confluence OAuth config is present
func (c *Config) RequireConfluence() error {
	if c.Confluence.ClientID == "" || c.Confluence.ClientSecret == "" {
		return errors.New("Confluence OAuth not configured. Set CONFLUENCE_CLIENT_ID and CONFLUENCE_CLIENT_SECRET or add to ~/.dex/config.json")
	}
	return nil
}

// RequirePrometheus validates that Prometheus URL is configured
func (c *Config) RequirePrometheus() error {
	if c.Prometheus.URL == "" {
		return errors.New("Prometheus URL not configured. Set PROMETHEUS_URL or add to ~/.dex/config.json")
	}
	return nil
}
