package statusline

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/statusline/providers"
)

// Run executes the status line generation
func Run(ctx context.Context, cfg *config.Config) (string, error) {
	// Read Claude's input from stdin
	claudeInput, err := ReadClaudeInput()
	if err != nil {
		claudeInput = &ClaudeInput{}
	}

	resolvedCfg := ResolveConfig(cfg)

	// Create file-based cache using session ID
	cache, err := NewFileCache(claudeInput.SessionID)
	if err != nil {
		// Fall back to no caching
		return runWithoutCache(ctx, cfg, resolvedCfg, claudeInput)
	}

	// Load existing cache
	cacheData, err := cache.Load()
	if err != nil {
		cacheData = &CacheData{Segments: make(map[string]SegmentCache)}
	}

	// Initialize providers
	allProviders := []providers.Provider{
		providers.NewK8sProvider(),
		providers.NewGitLabProvider(cfg),
		providers.NewGitHubProvider(),
		providers.NewJiraProvider(),
		providers.NewSlackProvider(cfg),
	}

	// Filter to enabled and configured providers
	var activeProviders []providers.Provider
	for _, p := range allProviders {
		seg, ok := resolvedCfg.Segments[p.Name()]
		if !ok || !seg.Enabled {
			continue
		}
		if !p.IsConfigured(cfg) {
			continue
		}
		activeProviders = append(activeProviders, p)
	}

	// Collect segment data (from cache or fresh)
	segmentData := make(map[string]map[string]any)
	var staleSegments []string

	for _, p := range activeProviders {
		seg := resolvedCfg.Segments[p.Name()]

		// Check cache
		if cached, ok, stale := cache.Get(cacheData, p.Name()); ok {
			segmentData[p.Name()] = cached
			if stale {
				staleSegments = append(staleSegments, p.Name())
			}
			continue
		}

		// No cache - need to fetch
		staleSegments = append(staleSegments, p.Name())
		// Use empty data for now, background refresh will populate
		segmentData[p.Name()] = make(map[string]any)

		// Try quick fetch with short timeout for first run
		fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		data, err := p.Fetch(fetchCtx)
		cancel()
		if err == nil {
			segmentData[p.Name()] = data
			cache.Set(cacheData, p.Name(), data, seg.CacheTTL)
		}
	}

	// Save cache
	_ = cache.Save(cacheData)

	// Trigger background refresh for stale segments
	if len(staleSegments) > 0 {
		go refreshStaleSegments(cfg, resolvedCfg, claudeInput.SessionID, staleSegments, activeProviders)
	}

	// Render each segment
	segmentOutputs := make(map[string]string)
	for name, data := range segmentData {
		seg := resolvedCfg.Segments[name]
		output, err := RenderSegment(seg.Format, data)
		if err != nil {
			continue
		}
		if output != "" {
			segmentOutputs[name] = output
		}
	}

	// Add Claude metrics as a segment
	if claudeInput.SessionID != "" {
		claudeData := claudeInput.ToTemplateData()
		claudeSeg := resolvedCfg.Segments["claude"]
		if claudeSeg.Enabled {
			output, err := RenderSegment(claudeSeg.Format, claudeData)
			if err == nil && output != "" {
				segmentOutputs["claude"] = output
			}
		}
	}

	// Render final output
	return RenderOutput(resolvedCfg.Format, segmentOutputs)
}

// runWithoutCache runs without caching (fallback)
func runWithoutCache(ctx context.Context, cfg *config.Config, resolvedCfg *ResolvedConfig, claudeInput *ClaudeInput) (string, error) {
	allProviders := []providers.Provider{
		providers.NewK8sProvider(),
		providers.NewGitLabProvider(cfg),
		providers.NewGitHubProvider(),
		providers.NewJiraProvider(),
		providers.NewSlackProvider(cfg),
	}

	segmentOutputs := make(map[string]string)
	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	for _, p := range allProviders {
		seg, ok := resolvedCfg.Segments[p.Name()]
		if !ok || !seg.Enabled || !p.IsConfigured(cfg) {
			continue
		}

		data, err := p.Fetch(fetchCtx)
		if err != nil {
			continue
		}

		output, err := RenderSegment(seg.Format, data)
		if err != nil || output == "" {
			continue
		}
		segmentOutputs[p.Name()] = output
	}

	// Add Claude metrics
	if claudeInput.SessionID != "" {
		claudeData := claudeInput.ToTemplateData()
		claudeSeg := resolvedCfg.Segments["claude"]
		if claudeSeg.Enabled {
			output, err := RenderSegment(claudeSeg.Format, claudeData)
			if err == nil && output != "" {
				segmentOutputs["claude"] = output
			}
		}
	}

	return RenderOutput(resolvedCfg.Format, segmentOutputs)
}

// refreshStaleSegments refreshes stale segments in background
func refreshStaleSegments(cfg *config.Config, resolvedCfg *ResolvedConfig, sessionID string, segments []string, allProviders []providers.Provider) {
	// Run as detached process so it doesn't block
	// We'll just do inline refresh here since Go's goroutines will die with the process
	// Instead, spawn a background dex process

	// For now, skip background refresh - the next call will pick up fresh data
	// A proper daemon would be needed for true background refresh

	// Alternative: spawn a quick background process
	for _, name := range segments {
		for _, p := range allProviders {
			if p.Name() != name {
				continue
			}

			seg := resolvedCfg.Segments[name]
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			data, err := p.Fetch(ctx)
			cancel()

			if err != nil {
				continue
			}

			// Update cache
			cache, err := NewFileCache(sessionID)
			if err != nil {
				continue
			}
			cacheData, _ := cache.Load()
			cache.Set(cacheData, name, data, seg.CacheTTL)
			_ = cache.Save(cacheData)
		}
	}
}

// RefreshCache is called by background refresh command
func RefreshCache(ctx context.Context, cfg *config.Config, sessionID string, segment string) error {
	resolvedCfg := ResolveConfig(cfg)

	allProviders := []providers.Provider{
		providers.NewK8sProvider(),
		providers.NewGitLabProvider(cfg),
		providers.NewGitHubProvider(),
		providers.NewJiraProvider(),
		providers.NewSlackProvider(cfg),
	}

	for _, p := range allProviders {
		if p.Name() != segment {
			continue
		}

		seg := resolvedCfg.Segments[segment]
		if !seg.Enabled || !p.IsConfigured(cfg) {
			return fmt.Errorf("segment %s not enabled or configured", segment)
		}

		data, err := p.Fetch(ctx)
		if err != nil {
			return err
		}

		cache, err := NewFileCache(sessionID)
		if err != nil {
			return err
		}

		cacheData, _ := cache.Load()
		cache.Set(cacheData, segment, data, seg.CacheTTL)
		return cache.Save(cacheData)
	}

	return fmt.Errorf("unknown segment: %s", segment)
}

// SpawnBackgroundRefresh spawns a background process to refresh a segment
func SpawnBackgroundRefresh(sessionID, segment string) {
	cmd := exec.Command("dex", "claude", "refresh", "--session", sessionID, "--segment", segment)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
	// Don't wait - let it run in background
}
