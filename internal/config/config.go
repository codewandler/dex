package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	GitLabURL    string
	GitLabToken  string
	ActivityDays int
}

func Load() (*Config, error) {
	cfg := &Config{
		GitLabURL:    os.Getenv("GITLAB_URL"),
		GitLabToken:  os.Getenv("GITLAB_PERSONAL_TOKEN"),
		ActivityDays: 14,
	}

	if cfg.GitLabURL == "" {
		return nil, fmt.Errorf("GITLAB_URL environment variable is required")
	}

	if cfg.GitLabToken == "" {
		return nil, fmt.Errorf("GITLAB_PERSONAL_TOKEN environment variable is required")
	}

	if days := os.Getenv("ACTIVITY_DAYS"); days != "" {
		d, err := strconv.Atoi(days)
		if err != nil {
			return nil, fmt.Errorf("ACTIVITY_DAYS must be a valid integer: %w", err)
		}
		if d < 1 {
			return nil, fmt.Errorf("ACTIVITY_DAYS must be at least 1")
		}
		cfg.ActivityDays = d
	}

	return cfg, nil
}
