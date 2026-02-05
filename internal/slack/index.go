package slack

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/codewandler/dex/internal/models"
)

func indexDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".dex", "slack")
	return dir, os.MkdirAll(dir, 0700)
}

func indexFilePath() (string, error) {
	dir, err := indexDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "index.json"), nil
}

// LoadIndex loads the Slack index from disk
func LoadIndex() (*models.SlackIndex, error) {
	path, err := indexFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return models.NewSlackIndex("", ""), nil
		}
		return nil, err
	}

	var idx models.SlackIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	idx.BuildLookupMaps()
	return &idx, nil
}

// SaveIndex saves the Slack index to disk
func SaveIndex(idx *models.SlackIndex) error {
	path, err := indexFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ProgressFunc is called during indexing with progress updates
type ProgressFunc func(completed, total int)

// IndexChannels fetches all channels and builds the index
func (c *Client) IndexChannels(progressFn ProgressFunc) (*models.SlackIndex, error) {
	// Get team info
	auth, err := c.TestAuth()
	if err != nil {
		return nil, err
	}

	idx := models.NewSlackIndex(auth.TeamID, auth.Team)
	idx.LastFullIndexAt = time.Now()

	// Fetch all channels
	channels, err := c.ListChannels()
	if err != nil {
		return nil, err
	}

	total := len(channels)
	for i, ch := range channels {
		slackCh := models.SlackChannel{
			ID:         ch.ID,
			Name:       ch.Name,
			IsPrivate:  ch.IsPrivate,
			IsArchived: ch.IsArchived,
			IsMember:   ch.IsMember,
			NumMembers: ch.NumMembers,
			Topic:      ch.Topic.Value,
			Purpose:    ch.Purpose.Value,
			IndexedAt:  time.Now(),
		}
		idx.UpsertChannel(slackCh)

		if progressFn != nil {
			progressFn(i+1, total)
		}
	}

	// Sort channels by name
	sort.Slice(idx.Channels, func(i, j int) bool {
		return idx.Channels[i].Name < idx.Channels[j].Name
	})
	idx.BuildLookupMaps()

	return idx, nil
}

// ResolveChannel resolves a channel name or ID to a channel ID
// Uses the index if available, falls back to returning the input as-is
func ResolveChannel(idOrName string) string {
	idx, err := LoadIndex()
	if err != nil {
		return idOrName
	}
	return idx.ResolveChannelID(idOrName)
}
