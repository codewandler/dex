package jira

import (
	"github.com/codewandler/dex/internal/config"
)

// SaveToken saves the Jira token to the config file
func SaveToken(token *config.JiraToken) error {
	cfg, err := config.LoadFromFile()
	if err != nil {
		cfg = &config.Config{}
	}

	cfg.Jira.Token = token
	if token != nil && token.CloudID != "" {
		cfg.Jira.CloudID = token.CloudID
	}

	return config.Save(cfg)
}

// LoadToken loads the Jira token from the config file
func LoadToken() (*config.JiraToken, error) {
	cfg, err := config.LoadFromFile()
	if err != nil {
		return nil, err
	}
	if cfg.Jira.Token == nil {
		return nil, nil
	}
	return cfg.Jira.Token, nil
}
