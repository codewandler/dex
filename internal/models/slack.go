package models

import "time"

// SlackChannel represents a Slack channel in the index
type SlackChannel struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsPrivate  bool      `json:"is_private"`
	IsArchived bool      `json:"is_archived"`
	IsMember   bool      `json:"is_member"` // Bot is a member and can post
	NumMembers int       `json:"num_members"`
	Topic      string    `json:"topic,omitempty"`
	Purpose    string    `json:"purpose,omitempty"`
	IndexedAt  time.Time `json:"indexed_at"`
}

// SlackIndex holds the cached Slack channel data
type SlackIndex struct {
	Version         int            `json:"version"`
	TeamID          string         `json:"team_id"`
	TeamName        string         `json:"team_name"`
	LastFullIndexAt time.Time      `json:"last_full_index_at"`
	Channels        []SlackChannel `json:"channels"`
	ChannelsByID    map[string]int `json:"-"`
	ChannelsByName  map[string]int `json:"-"`
}

// NewSlackIndex creates a new empty Slack index
func NewSlackIndex(teamID, teamName string) *SlackIndex {
	return &SlackIndex{
		Version:        1,
		TeamID:         teamID,
		TeamName:       teamName,
		Channels:       []SlackChannel{},
		ChannelsByID:   make(map[string]int),
		ChannelsByName: make(map[string]int),
	}
}

// BuildLookupMaps rebuilds the ID and name lookup maps
func (idx *SlackIndex) BuildLookupMaps() {
	idx.ChannelsByID = make(map[string]int)
	idx.ChannelsByName = make(map[string]int)

	for i, ch := range idx.Channels {
		idx.ChannelsByID[ch.ID] = i
		idx.ChannelsByName[ch.Name] = i
	}
}

// FindChannel looks up a channel by ID or name
func (idx *SlackIndex) FindChannel(idOrName string) *SlackChannel {
	if idx.ChannelsByID == nil || idx.ChannelsByName == nil {
		idx.BuildLookupMaps()
	}

	// Try as ID first (IDs start with C or G)
	if i, ok := idx.ChannelsByID[idOrName]; ok {
		return &idx.Channels[i]
	}

	// Try as name
	if i, ok := idx.ChannelsByName[idOrName]; ok {
		return &idx.Channels[i]
	}

	return nil
}

// ResolveChannelID returns the channel ID for a given ID or name
func (idx *SlackIndex) ResolveChannelID(idOrName string) string {
	ch := idx.FindChannel(idOrName)
	if ch != nil {
		return ch.ID
	}
	// Return as-is if not found (might be a valid ID not in index)
	return idOrName
}

// UpsertChannel adds or updates a channel in the index
func (idx *SlackIndex) UpsertChannel(ch SlackChannel) {
	if idx.ChannelsByID == nil || idx.ChannelsByName == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.ChannelsByID[ch.ID]; ok {
		// Update existing - remove old name mapping if name changed
		oldName := idx.Channels[i].Name
		if oldName != ch.Name {
			delete(idx.ChannelsByName, oldName)
		}
		idx.Channels[i] = ch
		idx.ChannelsByName[ch.Name] = i
	} else {
		i := len(idx.Channels)
		idx.Channels = append(idx.Channels, ch)
		idx.ChannelsByID[ch.ID] = i
		idx.ChannelsByName[ch.Name] = i
	}
}
