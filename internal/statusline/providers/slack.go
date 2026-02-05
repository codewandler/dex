package providers

import (
	"context"
	"time"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/slack"
)

// SlackProvider fetches Slack mention status
type SlackProvider struct {
	cfg *config.Config
}

func NewSlackProvider(cfg *config.Config) *SlackProvider {
	return &SlackProvider{cfg: cfg}
}

func (p *SlackProvider) Name() string {
	return "slack"
}

func (p *SlackProvider) IsConfigured(cfg *config.Config) bool {
	return cfg.RequireSlack() == nil
}

func (p *SlackProvider) Fetch(ctx context.Context) (map[string]any, error) {
	data := map[string]any{
		"Mentions": 0,
	}

	// Get bot token
	botToken := p.cfg.Slack.BotToken
	if p.cfg.Slack.Token != nil && p.cfg.Slack.Token.AccessToken != "" {
		botToken = p.cfg.Slack.Token.AccessToken
	}

	// Need user token for search
	userToken := p.cfg.Slack.UserToken
	if p.cfg.Slack.Token != nil && p.cfg.Slack.Token.UserToken != "" {
		userToken = p.cfg.Slack.Token.UserToken
	}

	if userToken == "" {
		// Can't search without user token, skip silently
		return data, nil
	}

	client, err := slack.NewClientWithUserToken(botToken, userToken)
	if err != nil {
		return nil, err
	}

	// Get current user ID via user auth test
	authResp, err := client.TestUserAuth()
	if err != nil {
		return nil, err
	}

	userID := authResp.UserID

	// Search mentions from last 24 hours
	since := time.Now().Add(-24 * time.Hour).Unix()
	mentions, _, err := client.SearchMentions(userID, 100, since)
	if err != nil {
		return nil, err
	}

	// Count pending mentions (not replied/acked)
	var pending int
	for _, m := range mentions {
		if m.Status == slack.MentionStatusPending {
			pending++
		}
	}

	data["Mentions"] = pending

	return data, nil
}
