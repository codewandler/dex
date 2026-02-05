package statusline

import (
	"time"

	"github.com/codewandler/dex/internal/config"
)

// Default format template - includes Claude metrics at the end
const defaultFormat = `{{if .Claude}}{{.Claude}} | {{end}}{{if .K8s}}☸ {{.K8s}}{{end}}{{if .GitLab}}{{if .K8s}} | {{end}}🦊 {{.GitLab}}{{end}}{{if .GitHub}}{{if or .K8s .GitLab}} | {{end}}{{.GitHub}}{{end}}{{if .Jira}}{{if or .K8s .GitLab .GitHub}} | {{end}}📋 {{.Jira}}{{end}}{{if .Slack}}{{if or .K8s .GitLab .GitHub .Jira}} | {{end}}💬 {{.Slack}}{{end}}`

// Default segment configurations
var defaultSegments = map[string]segmentDefaults{
	"claude": {
		format:   `{{.Model}} {{.ContextUsed}}%{{if .Cost}} ${{printf "%.2f" .Cost}}{{end}}`,
		cacheTTL: 0, // No caching needed - data comes from stdin
	},
	"k8s": {
		format:   `{{.Context}}/{{.Namespace}}{{if .Issues}} ({{.Issues}}){{end}}`,
		cacheTTL: 30 * time.Second,
	},
	"gitlab": {
		format:   `{{if .Assigned}}{{.Assigned}} assigned{{end}}{{if and .Assigned .Reviewing}}, {{end}}{{if .Reviewing}}{{.Reviewing}} reviewing{{end}}`,
		cacheTTL: 2 * time.Minute,
	},
	"github": {
		format:   `{{if .PRs}}{{.PRs}} PRs{{end}}{{if .Reviewing}}{{if .PRs}}, {{end}}{{.Reviewing}} review{{end}}{{if .Issues}}{{if or .PRs .Reviewing}}, {{end}}{{.Issues}} issues{{end}}`,
		cacheTTL: 2 * time.Minute,
	},
	"jira": {
		format:   `{{.Open}} open`,
		cacheTTL: 2 * time.Minute,
	},
	"slack": {
		format:   `@{{.Mentions}}`,
		cacheTTL: 1 * time.Minute,
	},
}

type segmentDefaults struct {
	format   string
	cacheTTL time.Duration
}

// ResolvedConfig holds the resolved configuration with defaults applied
type ResolvedConfig struct {
	Format   string
	Segments map[string]ResolvedSegment
}

// ResolvedSegment holds resolved segment configuration
type ResolvedSegment struct {
	Enabled  bool
	Format   string
	CacheTTL time.Duration
}

// ResolveConfig applies defaults to the status line configuration
func ResolveConfig(cfg *config.Config) *ResolvedConfig {
	resolved := &ResolvedConfig{
		Format:   defaultFormat,
		Segments: make(map[string]ResolvedSegment),
	}

	// Override format if specified
	if cfg.StatusLine.Format != "" {
		resolved.Format = cfg.StatusLine.Format
	}

	// Resolve each segment with defaults
	for name, defaults := range defaultSegments {
		segment := ResolvedSegment{
			Enabled:  true, // enabled by default
			Format:   defaults.format,
			CacheTTL: defaults.cacheTTL,
		}

		// Apply user overrides
		if userSeg, ok := cfg.StatusLine.Segments[name]; ok {
			if userSeg.Enabled != nil {
				segment.Enabled = *userSeg.Enabled
			}
			if userSeg.Format != "" {
				segment.Format = userSeg.Format
			}
			if userSeg.CacheTTL != "" {
				if ttl, err := time.ParseDuration(userSeg.CacheTTL); err == nil {
					segment.CacheTTL = ttl
				}
			}
		}

		resolved.Segments[name] = segment
	}

	return resolved
}
