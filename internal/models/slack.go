package models

import "time"

// SlackUser represents a Slack user in the index
type SlackUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`     // e.g., "john.doe"
	DisplayName string    `json:"display_name"` // e.g., "John Doe"
	RealName    string    `json:"real_name"`
	Email       string    `json:"email,omitempty"`
	IsBot       bool      `json:"is_bot"`
	IsAdmin     bool      `json:"is_admin"`
	IsDeleted   bool      `json:"is_deleted"`
	IndexedAt   time.Time `json:"indexed_at"`
}

// SlackChannel represents a Slack channel in the index
type SlackChannel struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsPrivate  bool      `json:"is_private"`
	IsArchived bool      `json:"is_archived"`
	IsMember   bool      `json:"is_member"` // Bot is a member and can post
	NumMembers int       `json:"num_members"`
	MemberIDs  []string  `json:"member_ids,omitempty"`
	Topic      string    `json:"topic,omitempty"`
	Purpose    string    `json:"purpose,omitempty"`
	IndexedAt  time.Time `json:"indexed_at"`
}

// SlackUserGroup represents a Slack user group in the index
type SlackUserGroup struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`       // e.g., "sre-team"
	Name        string    `json:"name"`          // e.g., "SRE Team"
	Description string    `json:"description,omitempty"`
	UserCount   int       `json:"user_count"`
	IndexedAt   time.Time `json:"indexed_at"`
}

// SlackIndex holds the cached Slack data (channels, users, and user groups)
type SlackIndex struct {
	Version         int               `json:"version"`
	TeamID          string            `json:"team_id"`
	TeamName        string            `json:"team_name"`
	LastFullIndexAt time.Time         `json:"last_full_index_at"`
	Channels        []SlackChannel    `json:"channels"`
	Users           []SlackUser       `json:"users"`
	UserGroups      []SlackUserGroup  `json:"user_groups,omitempty"`
	// Lookup maps (not persisted)
	ChannelsByID       map[string]int `json:"-"`
	ChannelsByName     map[string]int `json:"-"`
	UsersByID          map[string]int `json:"-"`
	UsersByUsername     map[string]int `json:"-"`
	UserGroupsByID     map[string]int `json:"-"`
	UserGroupsByHandle map[string]int `json:"-"`
}

// NewSlackIndex creates a new empty Slack index
func NewSlackIndex(teamID, teamName string) *SlackIndex {
	return &SlackIndex{
		Version:            1,
		TeamID:             teamID,
		TeamName:           teamName,
		Channels:           []SlackChannel{},
		Users:              []SlackUser{},
		UserGroups:         []SlackUserGroup{},
		ChannelsByID:       make(map[string]int),
		ChannelsByName:     make(map[string]int),
		UsersByID:          make(map[string]int),
		UsersByUsername:     make(map[string]int),
		UserGroupsByID:     make(map[string]int),
		UserGroupsByHandle: make(map[string]int),
	}
}

// BuildLookupMaps rebuilds all lookup maps
func (idx *SlackIndex) BuildLookupMaps() {
	idx.ChannelsByID = make(map[string]int)
	idx.ChannelsByName = make(map[string]int)
	idx.UsersByID = make(map[string]int)
	idx.UsersByUsername = make(map[string]int)
	idx.UserGroupsByID = make(map[string]int)
	idx.UserGroupsByHandle = make(map[string]int)

	for i, ch := range idx.Channels {
		idx.ChannelsByID[ch.ID] = i
		idx.ChannelsByName[ch.Name] = i
	}

	for i, u := range idx.Users {
		idx.UsersByID[u.ID] = i
		if u.Username != "" {
			idx.UsersByUsername[u.Username] = i
		}
	}

	for i, ug := range idx.UserGroups {
		idx.UserGroupsByID[ug.ID] = i
		if ug.Handle != "" {
			idx.UserGroupsByHandle[ug.Handle] = i
		}
	}
}

// Channel methods

// FindChannel looks up a channel by ID or name
func (idx *SlackIndex) FindChannel(idOrName string) *SlackChannel {
	if idx.ChannelsByID == nil || idx.ChannelsByName == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.ChannelsByID[idOrName]; ok {
		return &idx.Channels[i]
	}
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
	return idOrName
}

// UpsertChannel adds or updates a channel in the index
func (idx *SlackIndex) UpsertChannel(ch SlackChannel) {
	if idx.ChannelsByID == nil || idx.ChannelsByName == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.ChannelsByID[ch.ID]; ok {
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

// User methods

// FindUser looks up a user by ID or username
func (idx *SlackIndex) FindUser(idOrUsername string) *SlackUser {
	if idx.UsersByID == nil || idx.UsersByUsername == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.UsersByID[idOrUsername]; ok {
		return &idx.Users[i]
	}
	if i, ok := idx.UsersByUsername[idOrUsername]; ok {
		return &idx.Users[i]
	}
	return nil
}

// ResolveUserID returns the user ID for a given ID or username
func (idx *SlackIndex) ResolveUserID(idOrUsername string) string {
	u := idx.FindUser(idOrUsername)
	if u != nil {
		return u.ID
	}
	return idOrUsername
}

// UpsertUser adds or updates a user in the index
func (idx *SlackIndex) UpsertUser(u SlackUser) {
	if idx.UsersByID == nil || idx.UsersByUsername == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.UsersByID[u.ID]; ok {
		oldUsername := idx.Users[i].Username
		if oldUsername != u.Username {
			delete(idx.UsersByUsername, oldUsername)
		}
		idx.Users[i] = u
		if u.Username != "" {
			idx.UsersByUsername[u.Username] = i
		}
	} else {
		i := len(idx.Users)
		idx.Users = append(idx.Users, u)
		idx.UsersByID[u.ID] = i
		if u.Username != "" {
			idx.UsersByUsername[u.Username] = i
		}
	}
}

// User group methods

// FindUserGroup looks up a user group by ID or handle
func (idx *SlackIndex) FindUserGroup(idOrHandle string) *SlackUserGroup {
	if idx.UserGroupsByID == nil || idx.UserGroupsByHandle == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.UserGroupsByID[idOrHandle]; ok {
		return &idx.UserGroups[i]
	}
	if i, ok := idx.UserGroupsByHandle[idOrHandle]; ok {
		return &idx.UserGroups[i]
	}
	return nil
}

// UpsertUserGroup adds or updates a user group in the index
func (idx *SlackIndex) UpsertUserGroup(ug SlackUserGroup) {
	if idx.UserGroupsByID == nil || idx.UserGroupsByHandle == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.UserGroupsByID[ug.ID]; ok {
		oldHandle := idx.UserGroups[i].Handle
		if oldHandle != ug.Handle {
			delete(idx.UserGroupsByHandle, oldHandle)
		}
		idx.UserGroups[i] = ug
		if ug.Handle != "" {
			idx.UserGroupsByHandle[ug.Handle] = i
		}
	} else {
		i := len(idx.UserGroups)
		idx.UserGroups = append(idx.UserGroups, ug)
		idx.UserGroupsByID[ug.ID] = i
		if ug.Handle != "" {
			idx.UserGroupsByHandle[ug.Handle] = i
		}
	}
}

// ChannelsForUser returns all channels that contain the given user ID
func (idx *SlackIndex) ChannelsForUser(userID string) []SlackChannel {
	var channels []SlackChannel
	for _, ch := range idx.Channels {
		for _, mid := range ch.MemberIDs {
			if mid == userID {
				channels = append(channels, ch)
				break
			}
		}
	}
	return channels
}
