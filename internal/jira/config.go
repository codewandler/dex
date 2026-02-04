package jira

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BaseURL      string `json:"base_url"`
	CloudID      string `json:"cloud_id"`
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CloudID      string    `json:"cloud_id"`
}

func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-time.Minute)) // 1 min buffer
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "jira-oauth")
	return dir, os.MkdirAll(dir, 0700)
}

func LoadConfig() (*Config, error) {
	// Try environment variables first
	clientID := os.Getenv("JIRA_CLIENT_ID")
	clientSecret := os.Getenv("JIRA_CLIENT_SECRET")

	if clientID != "" && clientSecret != "" {
		return &Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			BaseURL:      "https://api.atlassian.com",
		}, nil
	}

	// Fall back to config file
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("jira OAuth not configured. Set JIRA_CLIENT_ID and JIRA_CLIENT_SECRET environment variables")
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.atlassian.com"
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
}

func LoadToken() (*Token, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, "token.json"))
	if err != nil {
		return nil, err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func SaveToken(token *Token) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "token.json"), data, 0600)
}
