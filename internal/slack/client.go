package slack

import (
	"fmt"

	"github.com/slack-go/slack"
)

// Client wraps the Slack API client
type Client struct {
	api      *slack.Client
	appToken string // For future Socket Mode support
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

// NewClientWithAppToken creates a client that can also use Socket Mode
func NewClientWithAppToken(botToken, appToken string) (*Client, error) {
	client, err := NewClient(botToken)
	if err != nil {
		return nil, err
	}
	client.appToken = appToken
	return client, nil
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
