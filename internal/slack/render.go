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
			text := messageDisplayText(msg.Text, msg.Attachments)
			text = truncateUnread(text, 80)
			fmt.Fprintf(&b, "  %s  %-20s %s\n", ts, "@"+from, text)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Total: %d unread messages across %d channels\n", r.Total, len(r.Channels))
	return b.String()
}

// ThreadMessageAttachment holds a single attachment's rendered text for a thread message.
type ThreadMessageAttachment struct {
	Text string `json:"text,omitempty"`
}

// ThreadMessage is a single message in a thread.
type ThreadMessage struct {
	Index       int                       `json:"index"`
	Label       string                    `json:"label"` // "parent" or "reply"
	Timestamp   string                    `json:"timestamp"`
	Username    string                    `json:"username"`
	UserID      string                    `json:"user_id"`
	BotID       string                    `json:"bot_id,omitempty"`
	IsMe        bool                      `json:"is_me"`
	Text        string                    `json:"text"`
	Attachments []ThreadMessageAttachment `json:"attachments,omitempty"`
}

// ThreadResult is the output of `dex slack thread`.
type ThreadResult struct {
	ChannelID   string          `json:"channel_id"`
	ChannelName string          `json:"channel_name,omitempty"`
	ThreadTS    string          `json:"thread_ts"`
	Messages    []ThreadMessage `json:"messages"`
	Status      string          `json:"status"` // "pending", "acked", "replied"
	// Debug fields — only populated when --debug is set
	MyUserIDs []string `json:"my_user_ids,omitempty"`
	MyBotIDs  []string `json:"my_bot_ids,omitempty"`
}

const threadCompactWidth = 80

// RenderText implements render.Renderable.
func (r *ThreadResult) RenderText(mode render.Mode) string {
	var b strings.Builder

	if len(r.Messages) == 0 {
		return "No messages found in thread.\n"
	}

	channelLabel := r.ChannelID
	if r.ChannelName != "" {
		channelLabel = "#" + r.ChannelName
	}

	if mode == render.ModeCompact {
		fmt.Fprintf(&b, "Thread %s / %s (%d messages, status: %s)\n",
			channelLabel, r.ThreadTS, len(r.Messages), r.Status)
		for _, msg := range r.Messages {
			label := "reply"
			if msg.Index == 0 {
				label = "parent"
			}
			meTag := ""
			if msg.IsMe {
				meTag = " [me]"
			}
			text := strings.ReplaceAll(msg.Text, "\n", " ")
			if len(text) > threadCompactWidth {
				text = text[:threadCompactWidth-1] + "…"
			}
			fmt.Fprintf(&b, "  [%d] %s @%s%s %s: %s\n",
				msg.Index, label, msg.Username, meTag, msg.Timestamp, text)
		}
		return b.String()
	}

	// Normal: full multi-line output
	fmt.Fprintf(&b, "Channel: %s\n", channelLabel)
	fmt.Fprintf(&b, "Thread:  %s\n", r.ThreadTS)
	if len(r.MyUserIDs) > 0 || len(r.MyBotIDs) > 0 {
		fmt.Fprintf(&b, "My User IDs: %v\n", r.MyUserIDs)
		fmt.Fprintf(&b, "My Bot IDs:  %v\n", r.MyBotIDs)
	}
	fmt.Fprintf(&b, "\nThread has %d messages:\n", len(r.Messages))
	fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 80))

	for _, msg := range r.Messages {
		label := "Reply"
		if msg.Index == 0 {
			label = "Parent"
		}
		meMarker := ""
		if msg.IsMe {
			meMarker = " [ME]"
		}
		botPart := ""
		if msg.BotID != "" {
			botPart = " Bot:" + msg.BotID
		}
		fmt.Fprintf(&b, "\n[%d] %s - %s - @%s (User:%s%s)%s\n",
			msg.Index, label, msg.Timestamp, msg.Username, msg.UserID, botPart, meMarker)
		for _, line := range strings.Split(strings.TrimRight(msg.Text, "\n"), "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
		for _, att := range msg.Attachments {
			if att.Text != "" {
				for _, line := range strings.Split(strings.TrimRight(att.Text, "\n"), "\n") {
					fmt.Fprintf(&b, "    │ %s\n", line)
				}
			}
		}
	}

	fmt.Fprintf(&b, "\n%s\n", strings.Repeat("─", 80))
	fmt.Fprintf(&b, "Status: %s\n", r.Status)

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

// formatUnreadTS formats a Slack timestamp, showing date if not today.
func formatUnreadTS(ts string) string {
	t := parseUnixTS(ts)
	if t.IsZero() {
		return ts
	}
	t = t.Local()
	now := time.Now().Local()
	if t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day() {
		return t.Format("15:04")
	}
	return t.Format("Jan 02 15:04")
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

// MessageDisplayText is the exported counterpart of messageDisplayText.
func MessageDisplayText(text string, attachments []MessageAttachment) string {
	return messageDisplayText(text, attachments)
}

// RenderAttachments is the exported counterpart of renderAttachments.
func RenderAttachments(attachments []MessageAttachment) string {
	return renderAttachments(attachments)
}

func truncateUnread(s string, max int) string {
	// Collapse newlines for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// messageDisplayText returns the best human-readable text for a message.
// It prefers the message text; if empty it falls back to attachments.
func messageDisplayText(text string, attachments []MessageAttachment) string {
	if text != "" {
		return text
	}
	for _, a := range attachments {
		if a.Text != "" {
			return a.Text
		}
		if a.Fallback != "" {
			return a.Fallback
		}
	}
	return ""
}

// renderAttachments produces a multi-line text block for a list of attachments.
func renderAttachments(attachments []MessageAttachment) string {
	if len(attachments) == 0 {
		return ""
	}
	var b strings.Builder
	for _, a := range attachments {
		if a.Pretext != "" {
			fmt.Fprintf(&b, "  %s\n", a.Pretext)
		}
		if a.Title != "" {
			if a.TitleLink != "" {
				fmt.Fprintf(&b, "  %s (%s)\n", a.Title, a.TitleLink)
			} else {
				fmt.Fprintf(&b, "  %s\n", a.Title)
			}
		}
		if a.AuthorName != "" {
			fmt.Fprintf(&b, "  %s\n", a.AuthorName)
		}
		if a.Text != "" {
			for _, line := range strings.Split(strings.TrimRight(a.Text, "\n"), "\n") {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		} else if a.Fallback != "" && a.Title == "" {
			// Only show fallback if we have nothing better
			for _, line := range strings.Split(strings.TrimRight(a.Fallback, "\n"), "\n") {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
		if a.Footer != "" {
			fmt.Fprintf(&b, "  ─ %s\n", a.Footer)
		}
	}
	return b.String()
}
