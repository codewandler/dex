package statusline

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"
)

// ClaudeInput represents the JSON input from Claude Code
type ClaudeInput struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Model          struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	Version     string `json:"version"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style"`
	Cost struct {
		TotalCostUSD       float64 `json:"total_cost_usd"`
		TotalDurationMS    int64   `json:"total_duration_ms"`
		TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
		TotalLinesAdded    int     `json:"total_lines_added"`
		TotalLinesRemoved  int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		TotalInputTokens    int     `json:"total_input_tokens"`
		TotalOutputTokens   int     `json:"total_output_tokens"`
		ContextWindowSize   int     `json:"context_window_size"`
		UsedPercentage      float64 `json:"used_percentage"`
		RemainingPercentage float64 `json:"remaining_percentage"`
		CurrentUsage        struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
}

// ReadClaudeInput reads and parses Claude's JSON input from stdin
func ReadClaudeInput() (*ClaudeInput, error) {
	// Check if stdin has data (non-blocking check)
	stat, err := os.Stdin.Stat()
	if err != nil {
		return &ClaudeInput{}, nil
	}

	// If stdin is a terminal, return empty input
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return &ClaudeInput{}, nil
	}

	// Check if there's actually data available (size > 0 for pipes)
	if stat.Size() == 0 {
		// For pipes, size might be 0 even with data, so try reading with timeout
		return readWithTimeout()
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return &ClaudeInput{}, nil
	}

	if len(data) == 0 {
		return &ClaudeInput{}, nil
	}

	var input ClaudeInput
	if err := json.Unmarshal(data, &input); err != nil {
		return &ClaudeInput{}, nil
	}

	return &input, nil
}

// readWithTimeout reads stdin with a short timeout to avoid blocking
func readWithTimeout() (*ClaudeInput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dataCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			errCh <- err
			return
		}
		dataCh <- data
	}()

	select {
	case <-ctx.Done():
		// Timeout - no data available
		return &ClaudeInput{}, nil
	case err := <-errCh:
		if err != nil {
			return &ClaudeInput{}, nil
		}
		return &ClaudeInput{}, nil
	case data := <-dataCh:
		if len(data) == 0 {
			return &ClaudeInput{}, nil
		}
		var input ClaudeInput
		if err := json.Unmarshal(data, &input); err != nil {
			return &ClaudeInput{}, nil
		}
		return &input, nil
	}
}

// ToTemplateData converts ClaudeInput to template-friendly map
func (c *ClaudeInput) ToTemplateData() map[string]any {
	return map[string]any{
		"Model":            c.Model.DisplayName,
		"ContextUsed":      int(c.ContextWindow.UsedPercentage),
		"ContextRemaining": int(c.ContextWindow.RemainingPercentage),
		"Cost":             c.Cost.TotalCostUSD,
		"LinesAdded":       c.Cost.TotalLinesAdded,
		"LinesRemoved":     c.Cost.TotalLinesRemoved,
		"SessionID":        c.SessionID,
	}
}
