package providers

import (
	"context"

	"github.com/codewandler/dex/internal/config"
)

// Provider defines the interface for status line data providers
type Provider interface {
	// Name returns the provider name (e.g., "k8s", "gitlab")
	Name() string

	// Fetch retrieves data for the status line segment
	Fetch(ctx context.Context) (map[string]any, error)

	// IsConfigured returns true if the provider has valid configuration
	IsConfigured(cfg *config.Config) bool
}
