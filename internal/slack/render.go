package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/render"
)

// UnreadResult is the output of `dex slack unreads` — a list of channels
// with their unread messages, ready for multi-format rendering.
type UnreadResult struct {
	Channels []UnreadChannelMessages `json:"channels"`
	Total    int                     `json:"total_messages"`
}

// UnreadChannelMessages groups unread messages by channel.
type UnreadChannelMessages struct {
	Channel  UnreadChannel   `json:"channel"`
	Messages []UnreadMessage `json:"messages"`
}

// RenderText implements render.Renderable.
func (r *UnreadResult) RenderText(mode render.Mode) string {
	if len(r.Channels) == 0 {
		return "No unread messages.\n"
	}

	var b strings.Builder

	if mode == render.ModeCompact {
		// Compact: one line per channel
		for _, ch := range r.Channels {
			name := channelDisplayName(ch.Channel)
			fmt.Fprintf(&b, "%-30s %d unread\n", name, ch.Channel.UnreadCount)
		}
		fmt.Fprintf(&b, "\n%d messages across %d channels\n", r.Total, len(r.Channels))
		return b.String()
	}

	// Normal: grouped messages per channel
	for _, ch := range r.Channels {
		name := channelDisplayName(ch.Channel)
		fmt.Fprintf(&b, "%s  (%d unread)\n", name, ch.Channel.UnreadCount)
		fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 60))

		for _, msg := range ch.Messages {
			ts := formatUnreadTS(msg.Timestamp)
			from := msg.Username
			if from == "" {
				from = msg.UserID
			}
			text := truncateUnread(msg.Text, 80)
			fmt.Fprintf(&b, "  %s  %-20s %s\n", ts, "@"+from, text)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Total: %d unread messages across %d channels\n", r.Total, len(r.Channels))
	return b.String()
}

// MarkReadResult is the output of `dex slack mark-read`.
type MarkReadResult struct {
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Timestamp   string `json:"timestamp"`
}

// RenderText implements render.Renderable.
func (m *MarkReadResult) RenderText(mode render.Mode) string {
	name := m.ChannelName
	if name == "" {
		name = m.ChannelID
	} else {
		name = "#" + name
	}
	return fmt.Sprintf("Marked %s as read up to %s\n", name, m.Timestamp)
}

// channelDisplayName returns a human-readable channel name.
func channelDisplayName(ch UnreadChannel) string {
	if ch.IsDM {
		return "DM:" + ch.ID
	}
	if ch.Name != "" {
		return "#" + ch.Name
	}
	return ch.ID
}

// formatUnreadTS formats a Slack timestamp into HH:MM.
func formatUnreadTS(ts string) string {
	t := parseUnixTS(ts)
	if t.IsZero() {
		return ts
	}
	return t.Local().Format("15:04")
}

func parseUnixTS(ts string) time.Time {
	// Slack timestamps are "unix.microseconds" e.g. "1512085950.000216"
	var sec, _ int64
	fmt.Sscanf(ts, "%d", &sec)
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

func truncateUnread(s string, max int) string {
	// Collapse newlines for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
