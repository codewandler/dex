package slack

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// Client wraps the Slack API client
type Client struct {
	api       *slack.Client
	userAPI   *slack.Client // For search (requires user token)
	appToken  string        // For future Socket Mode support
	userToken string
}

// NewClient creates a new Slack client with the given bot token
func NewClient(botToken string) (*Client, error) {
	if botToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	api := slack.New(botToken)

	return &Client{
		api: api,
	}, nil
}

// NewClientWithUserToken creates a client with both bot and user tokens
// The user token enables search API access
func NewClientWithUserToken(botToken, userToken string) (*Client, error) {
	client, err := NewClient(botToken)
	if err != nil {
		return nil, err
	}
	if userToken != "" {
		client.userToken = userToken
		client.userAPI = slack.New(userToken)
	}
	return client, nil
}

// NewClientWithAppToken creates a client that can also use Socket Mode
func NewClientWithAppToken(botToken, appToken string) (*Client, error) {
	client, err := NewClient(botToken)
	if err != nil {
		return nil, err
	}
	client.appToken = appToken
	return client, nil
}

// HasUserToken returns true if a user token is configured for search
func (c *Client) HasUserToken() bool {
	return c.userAPI != nil
}

// PostMessage sends a message to a channel
func (c *Client) PostMessage(channelID, text string) (string, error) {
	_, timestamp, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		return "", fmt.Errorf("failed to post message: %w", err)
	}
	return timestamp, nil
}

// PostMessageWithBlocks sends a message with Block Kit blocks
func (c *Client) PostMessageWithBlocks(channelID, fallbackText string, blocks []slack.Block) (string, error) {
	_, timestamp, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(fallbackText, false),
		slack.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		return "", fmt.Errorf("failed to post message: %w", err)
	}
	return timestamp, nil
}

// ReplyToThread sends a reply to a thread
func (c *Client) ReplyToThread(channelID, threadTS, text string) (string, error) {
	_, timestamp, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		return "", fmt.Errorf("failed to reply to thread: %w", err)
	}
	return timestamp, nil
}

// TestAuth tests the authentication and returns bot info
func (c *Client) TestAuth() (*slack.AuthTestResponse, error) {
	resp, err := c.api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("auth test failed: %w", err)
	}
	return resp, nil
}

// TestUserAuth tests the user token authentication and returns user info
func (c *Client) TestUserAuth() (*slack.AuthTestResponse, error) {
	if c.userAPI == nil {
		return nil, fmt.Errorf("user token not configured")
	}
	resp, err := c.userAPI.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("user auth test failed: %w", err)
	}
	return resp, nil
}

// GetUserPresence gets the presence status of a user (requires user token)
func (c *Client) GetUserPresence(userID string) (*slack.UserPresence, error) {
	if c.userAPI == nil {
		return nil, fmt.Errorf("user token not configured")
	}
	presence, err := c.userAPI.GetUserPresence(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user presence: %w", err)
	}
	return presence, nil
}

// SetUserPresence sets the user's presence status (requires user token)
// presence must be "auto" or "away"
func (c *Client) SetUserPresence(presence string) error {
	if c.userAPI == nil {
		return fmt.Errorf("user token not configured")
	}
	if presence != "auto" && presence != "away" {
		return fmt.Errorf("presence must be 'auto' or 'away'")
	}
	if err := c.userAPI.SetUserPresence(presence); err != nil {
		return fmt.Errorf("failed to set user presence: %w", err)
	}
	return nil
}

// GetChannelInfo gets information about a channel
func (c *Client) GetChannelInfo(channelID string) (*slack.Channel, error) {
	channel, err := c.api.GetConversationInfo(&slack.GetConversationInfoInput{
		ChannelID: channelID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}
	return channel, nil
}

// ListChannels lists channels the bot is a member of
func (c *Client) ListChannels() ([]slack.Channel, error) {
	var allChannels []slack.Channel
	cursor := ""

	for {
		params := &slack.GetConversationsParameters{
			Cursor:          cursor,
			Limit:           200,
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel"},
		}

		channels, nextCursor, err := c.api.GetConversations(params)
		if err != nil {
			return nil, fmt.Errorf("failed to list channels: %w", err)
		}

		allChannels = append(allChannels, channels...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allChannels, nil
}

// GetChannelMembers returns all member user IDs for a channel, handling pagination and rate limits
func (c *Client) GetChannelMembers(channelID string) ([]string, error) {
	var allMembers []string
	cursor := ""

	for {
		params := &slack.GetUsersInConversationParameters{
			ChannelID: channelID,
			Cursor:    cursor,
			Limit:     200,
		}

		members, nextCursor, err := c.api.GetUsersInConversation(params)
		if err != nil {
			// Handle rate limiting
			if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
				time.Sleep(rateLimitErr.RetryAfter)
				continue
			}
			return nil, fmt.Errorf("failed to get channel members: %w", err)
		}

		allMembers = append(allMembers, members...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allMembers, nil
}

// ListUserGroups lists all user groups in the workspace
func (c *Client) ListUserGroups() ([]slack.UserGroup, error) {
	groups, err := c.api.GetUserGroups(slack.GetUserGroupsOptionIncludeCount(true))
	if err != nil {
		return nil, fmt.Errorf("failed to list user groups: %w", err)
	}
	return groups, nil
}

// ListUsers lists all users in the workspace
func (c *Client) ListUsers() ([]slack.User, error) {
	users, err := c.api.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}

// OpenConversation opens a DM conversation with a user and returns the channel ID
func (c *Client) OpenConversation(userID string) (string, error) {
	params := &slack.OpenConversationParameters{
		Users: []string{userID},
	}
	channel, _, _, err := c.api.OpenConversation(params)
	if err != nil {
		return "", fmt.Errorf("failed to open conversation: %w", err)
	}
	return channel.ID, nil
}

// SearchMessages searches for messages matching a query (requires user token with search:read scope)
func (c *Client) SearchMessages(query string, count int) ([]slack.SearchMessage, int, error) {
	if c.userAPI == nil {
		return nil, 0, fmt.Errorf("user token required for search")
	}

	params := slack.SearchParameters{
		Sort:          "timestamp",
		SortDirection: "desc",
		Count:         count,
		Page:          1,
	}

	result, err := c.userAPI.SearchMessages(query, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search messages: %w", err)
	}

	return result.Matches, result.Total, nil
}

// SearchMentions searches for mentions of a user using the search API
// Returns mentions sorted by timestamp descending. Requires user token.
func (c *Client) SearchMentions(userID string, limit int, since int64) ([]Mention, int, error) {
	if c.userAPI == nil {
		return nil, 0, fmt.Errorf("user token required for search")
	}

	query := fmt.Sprintf("<@%s>", userID)
	if since > 0 {
		// Slack search after: is exclusive, so subtract a day to include target date
		// Client-side filtering below handles exact timestamp precision
		sinceTime := time.Unix(since, 0).AddDate(0, 0, -1)
		query += fmt.Sprintf(" after:%s", sinceTime.Format("2006-01-02"))
	}

	params := slack.SearchParameters{
		Sort:          "timestamp",
		SortDirection: "desc",
		Count:         limit,
		Page:          1,
	}

	result, err := c.userAPI.SearchMessages(query, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search mentions: %w", err)
	}

	var mentions []Mention
	for _, msg := range result.Matches {
		// Filter by exact timestamp since search API is date-based
		if since > 0 {
			msgTime := parseTimestamp(msg.Timestamp)
			if msgTime < since {
				continue
			}
		}
		mentions = append(mentions, Mention{
			ChannelID:   msg.Channel.ID,
			ChannelName: msg.Channel.Name,
			UserID:      msg.User,
			Username:    msg.Username,
			Timestamp:   msg.Timestamp,
			ThreadTS:    extractThreadTS(msg.Permalink),
			Text:        msg.Text,
			Permalink:   msg.Permalink,
		})
	}

	return mentions, len(mentions), nil
}

// MentionStatus indicates whether a mention has been handled
type MentionStatus string

const (
	MentionStatusPending MentionStatus = "Pending" // No reaction or reply
	MentionStatusAcked   MentionStatus = "Acked"   // Has reaction but no reply
	MentionStatusReplied MentionStatus = "Replied" // Has reply from bot or user
)

// Mention represents a message that mentions a user
type Mention struct {
	ChannelID   string
	ChannelName string
	UserID      string
	Username    string
	Timestamp   string
	ThreadTS    string // Parent thread timestamp (if this is a reply)
	Text        string
	Permalink   string
	Status      MentionStatus
}

// extractThreadTS extracts thread_ts from a Slack permalink if present
// Permalink format: https://...slack.com/archives/CHANNEL/pTIMESTAMP?thread_ts=PARENT_TS
func extractThreadTS(permalink string) string {
	if idx := strings.Index(permalink, "thread_ts="); idx != -1 {
		ts := permalink[idx+10:]
		// Remove any trailing query params
		if end := strings.Index(ts, "&"); end != -1 {
			ts = ts[:end]
		}
		return ts
	}
	return ""
}

// GetMentionsInChannels scans channel history for mentions of a user (works with bot tokens)
// If since is non-zero, only returns mentions from after that time
func (c *Client) GetMentionsInChannels(userID string, channels []string, limit int, since int64) ([]Mention, error) {
	mentionPattern := fmt.Sprintf("<@%s>", userID)
	var mentions []Mention

	for _, channelID := range channels {
		params := &slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Limit:     100, // Fetch last 100 messages per channel
		}

		// Filter by time if specified
		if since > 0 {
			params.Oldest = fmt.Sprintf("%d.000000", since)
		}

		history, err := c.api.GetConversationHistory(params)
		if err != nil {
			// Skip channels we can't access
			continue
		}

		for _, msg := range history.Messages {
			if contains(msg.Text, mentionPattern) {
				permalink, _ := c.api.GetPermalink(&slack.PermalinkParameters{
					Channel: channelID,
					Ts:      msg.Timestamp,
				})
				mentions = append(mentions, Mention{
					ChannelID: channelID,
					UserID:    msg.User,
					Timestamp: msg.Timestamp,
					Text:      msg.Text,
					Permalink: permalink,
				})

				if len(mentions) >= limit {
					return mentions, nil
				}
			}
		}
	}

	return mentions, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetBotUserID returns the user ID of the authenticated bot
func (c *Client) GetBotUserID() (string, error) {
	resp, err := c.api.AuthTest()
	if err != nil {
		return "", fmt.Errorf("failed to get bot user ID: %w", err)
	}
	return resp.UserID, nil
}

// GetBotID returns the bot ID (B...) of the authenticated bot
func (c *Client) GetBotID() (string, error) {
	resp, err := c.api.AuthTest()
	if err != nil {
		return "", fmt.Errorf("failed to get bot ID: %w", err)
	}
	return resp.BotID, nil
}

// GetReactions returns reactions on a message
// Uses user token if available (for channels bot isn't a member of), falls back to bot token
func (c *Client) GetReactions(channelID, timestamp string) ([]slack.ItemReaction, error) {
	item := slack.NewRefToMessage(channelID, timestamp)

	// Try user API first if available
	if c.userAPI != nil {
		reactions, err := c.userAPI.GetReactions(item, slack.NewGetReactionsParameters())
		if err == nil {
			return reactions, nil
		}
	}

	// Fall back to bot API
	reactions, err := c.api.GetReactions(item, slack.NewGetReactionsParameters())
	if err != nil {
		return nil, fmt.Errorf("failed to get reactions: %w", err)
	}
	return reactions, nil
}

// GetThreadReplies returns replies in a thread
// Uses user token if available (for channels bot isn't a member of), falls back to bot token
func (c *Client) GetThreadReplies(channelID, threadTS string) ([]slack.Message, error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
		Limit:     100,
	}

	// Try user API first if available (has access to more channels)
	var userAPIErr error
	if c.userAPI != nil {
		msgs, _, _, err := c.userAPI.GetConversationReplies(params)
		if err == nil {
			return msgs, nil
		}
		userAPIErr = err
	}

	// Fall back to bot API
	msgs, _, _, err := c.api.GetConversationReplies(params)
	if err != nil {
		if userAPIErr != nil {
			return nil, fmt.Errorf("failed to get thread replies: user API: %v, bot API: %w", userAPIErr, err)
		}
		return nil, fmt.Errorf("failed to get thread replies: %w", err)
	}
	return msgs, nil
}

// ClassifyMentionStatus determines the status of a mention based on reactions and replies
// myUserIDs should include user IDs (U...) for the bot and authenticated user
// myBotIDs should include bot IDs (B...) to check against message BotID field
func (c *Client) ClassifyMentionStatus(channelID, timestamp string, myUserIDs, myBotIDs []string) MentionStatus {
	// Check for thread replies first (takes precedence over reactions)
	replies, err := c.GetThreadReplies(channelID, timestamp)
	if err == nil && len(replies) > 1 { // First message is the parent, replies start from index 1
		for _, reply := range replies[1:] {
			// Check if reply is from one of our user IDs
			for _, myID := range myUserIDs {
				if reply.User == myID {
					return MentionStatusReplied
				}
			}
			// Check if reply is from one of our bot IDs
			for _, botID := range myBotIDs {
				if reply.BotID == botID {
					return MentionStatusReplied
				}
			}
		}
	}

	// Check for reactions
	reactions, err := c.GetReactions(channelID, timestamp)
	if err == nil && len(reactions) > 0 {
		// Check if any of our users reacted
		for _, reaction := range reactions {
			for _, reactorID := range reaction.Users {
				for _, myID := range myUserIDs {
					if reactorID == myID {
						return MentionStatusAcked
					}
				}
			}
		}
	}

	return MentionStatusPending
}

// SearchResult holds a search result with metadata
type SearchResult struct {
	ChannelID   string
	ChannelName string
	UserID      string
	Username    string
	Timestamp   string
	Text        string
	Permalink   string
}

// Search performs a general search with the given query (requires user token)
func (c *Client) Search(query string, count int, since int64) ([]SearchResult, int, error) {
	if c.userAPI == nil {
		return nil, 0, fmt.Errorf("user token required for search")
	}

	if since > 0 {
		// Slack search uses after:YYYY-MM-DD format (exclusive, so subtract a day)
		sinceTime := time.Unix(since, 0).AddDate(0, 0, -1)
		query += fmt.Sprintf(" after:%s", sinceTime.Format("2006-01-02"))
	}

	params := slack.SearchParameters{
		Sort:          "timestamp",
		SortDirection: "desc",
		Count:         count,
		Page:          1,
	}

	result, err := c.userAPI.SearchMessages(query, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search messages: %w", err)
	}

	var results []SearchResult
	for _, msg := range result.Matches {
		results = append(results, SearchResult{
			ChannelID:   msg.Channel.ID,
			ChannelName: msg.Channel.Name,
			UserID:      msg.User,
			Username:    msg.Username,
			Timestamp:   msg.Timestamp,
			Text:        msg.Text,
			Permalink:   msg.Permalink,
		})
	}

	return results, result.Total, nil
}

// parseTimestamp extracts Unix seconds from a Slack timestamp (e.g., "1612345678.123456")
func parseTimestamp(ts string) int64 {
	var sec int64
	fmt.Sscanf(ts, "%d", &sec)
	return sec
}

// ExtractTickets extracts ticket references from text given a list of project keys.
// For example, with projectKeys=["DEV", "TEL"], it will find "DEV-123", "TEL-456", etc.
func ExtractTickets(text string, projectKeys []string) []string {
	if len(projectKeys) == 0 {
		return nil
	}

	// Build pattern: (DEV|TEL|...)-\d+
	pattern := "(?i)\\b("
	for i, key := range projectKeys {
		if i > 0 {
			pattern += "|"
		}
		pattern += regexp.QuoteMeta(key)
	}
	pattern += ")-\\d+\\b"

	re := regexp.MustCompile(pattern)
	matches := re.FindAllString(text, -1)

	// Deduplicate and uppercase
	seen := make(map[string]bool)
	var tickets []string
	for _, m := range matches {
		// Normalize to uppercase
		ticket := regexp.MustCompile(`(?i)([a-z]+)-(\d+)`).ReplaceAllStringFunc(m, func(s string) string {
			parts := regexp.MustCompile(`(?i)([a-z]+)-(\d+)`).FindStringSubmatch(s)
			if len(parts) == 3 {
				return fmt.Sprintf("%s-%s", toUpper(parts[1]), parts[2])
			}
			return s
		})
		if !seen[ticket] {
			seen[ticket] = true
			tickets = append(tickets, ticket)
		}
	}

	return tickets
}

func toUpper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
