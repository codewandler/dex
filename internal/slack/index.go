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

// IndexAll fetches all channels and users and builds the index
func (c *Client) IndexAll(channelProgressFn, userProgressFn ProgressFunc) (*models.SlackIndex, error) {
	auth, err := c.TestAuth()
	if err != nil {
		return nil, err
	}

	idx := models.NewSlackIndex(auth.TeamID, auth.Team)
	idx.LastFullIndexAt = time.Now()

	// Index channels
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

		if channelProgressFn != nil {
			channelProgressFn(i+1, total)
		}
	}

	// Sort channels by name
	sort.Slice(idx.Channels, func(i, j int) bool {
		return idx.Channels[i].Name < idx.Channels[j].Name
	})

	// Index users
	users, err := c.ListUsers()
	if err != nil {
		return nil, err
	}

	total = len(users)
	for i, u := range users {
		// Skip deleted users and slackbot
		if u.Deleted || u.ID == "USLACKBOT" {
			if userProgressFn != nil {
				userProgressFn(i+1, total)
			}
			continue
		}

		slackUser := models.SlackUser{
			ID:          u.ID,
			Username:    u.Name,
			DisplayName: u.Profile.DisplayName,
			RealName:    u.RealName,
			Email:       u.Profile.Email,
			IsBot:       u.IsBot,
			IsAdmin:     u.IsAdmin,
			IsDeleted:   u.Deleted,
			IndexedAt:   time.Now(),
		}
		idx.UpsertUser(slackUser)

		if userProgressFn != nil {
			userProgressFn(i+1, total)
		}
	}

	// Sort users by username
	sort.Slice(idx.Users, func(i, j int) bool {
		return idx.Users[i].Username < idx.Users[j].Username
	})

	idx.BuildLookupMaps()
	return idx, nil
}

// ResolveChannel resolves a channel name or ID to a channel ID
func ResolveChannel(idOrName string) string {
	idx, err := LoadIndex()
	if err != nil {
		return idOrName
	}
	return idx.ResolveChannelID(idOrName)
}

// ResolveUser resolves a username or ID to a user ID
func ResolveUser(idOrUsername string) string {
	idx, err := LoadIndex()
	if err != nil {
		return idOrUsername
	}
	return idx.ResolveUserID(idOrUsername)
}

// ResolveMentions converts @username mentions in text to Slack <@USER_ID> format
// Example: "Hey @john.doe check this" -> "Hey <@U0123456789> check this"
func ResolveMentions(text string) string {
	idx, err := LoadIndex()
	if err != nil || len(idx.Users) == 0 {
		return text
	}

	result := text
	i := 0
	for i < len(result) {
		// Find next @
		atIdx := -1
		for j := i; j < len(result); j++ {
			if result[j] == '@' {
				atIdx = j
				break
			}
		}
		if atIdx == -1 {
			break
		}

		// Skip if this @ is already part of a Slack mention format <@...>
		if atIdx > 0 && result[atIdx-1] == '<' {
			i = atIdx + 1
			continue
		}

		// Extract potential username (alphanumeric, dots, underscores, hyphens)
		endIdx := atIdx + 1
		for endIdx < len(result) {
			c := result[endIdx]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
				c == '.' || c == '_' || c == '-' {
				endIdx++
			} else {
				break
			}
		}

		if endIdx > atIdx+1 {
			username := result[atIdx+1 : endIdx]
			user := idx.FindUser(username)
			if user != nil {
				mention := "<@" + user.ID + ">"
				result = result[:atIdx] + mention + result[endIdx:]
				i = atIdx + len(mention)
				continue
			}
		}
		i = atIdx + 1
	}

	return result
}

// MentionStatusCache caches classification results for mentions
// Only "Replied" and "Acked" statuses are cached (they're stable)
// "Pending" is not cached as it may change when user replies
type MentionStatusCache struct {
	// Key format: "channelID:timestamp" -> status
	Statuses map[string]MentionStatus `json:"statuses"`
}

func mentionCacheFilePath() (string, error) {
	dir, err := indexDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mention_status_cache.json"), nil
}

// LoadMentionStatusCache loads the cache from disk
func LoadMentionStatusCache() (*MentionStatusCache, error) {
	path, err := mentionCacheFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &MentionStatusCache{Statuses: make(map[string]MentionStatus)}, nil
		}
		return nil, err
	}

	var cache MentionStatusCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return &MentionStatusCache{Statuses: make(map[string]MentionStatus)}, nil
	}
	if cache.Statuses == nil {
		cache.Statuses = make(map[string]MentionStatus)
	}
	return &cache, nil
}

// SaveMentionStatusCache saves the cache to disk
func SaveMentionStatusCache(cache *MentionStatusCache) error {
	path, err := mentionCacheFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// cacheKey generates a cache key for a mention
func cacheKey(channelID, timestamp string) string {
	return channelID + ":" + timestamp
}

// Get returns the cached status for a mention, or empty string if not cached
func (c *MentionStatusCache) Get(channelID, timestamp string) MentionStatus {
	return c.Statuses[cacheKey(channelID, timestamp)]
}

// Set caches a status (only Replied and Acked are cached)
func (c *MentionStatusCache) Set(channelID, timestamp string, status MentionStatus) {
	if status == MentionStatusReplied || status == MentionStatusAcked {
		c.Statuses[cacheKey(channelID, timestamp)] = status
	}
}
