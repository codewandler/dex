package confluence

import (
	"github.com/codewandler/dex/internal/atlassian"
	"github.com/codewandler/dex/internal/config"
)

// SaveToken saves the Confluence token to the config file
func SaveToken(token *atlassian.Token) error {
	cfg, err := config.LoadFromFile()
	if err != nil {
		cfg = &config.Config{}
	}

	cfg.Confluence.Token = token

	return config.Save(cfg)
}

// LoadToken loads the Confluence token from the config file
func LoadToken() (*atlassian.Token, error) {
	cfg, err := config.LoadFromFile()
	if err != nil {
		return nil, err
	}
	if cfg.Confluence.Token == nil {
		return nil, nil
	}
	return cfg.Confluence.Token, nil
}
