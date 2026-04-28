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

// ThreadMessageFile holds metadata about a file attached to a thread message.
type ThreadMessageFile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Mimetype   string `json:"mimetype"`
	Size       int    `json:"size"`
	Permalink  string `json:"permalink"`
	URLPrivate string `json:"url_private"`
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
	Files       []ThreadMessageFile       `json:"files,omitempty"`
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
			filesSuffix := renderFilesCompact(msg.Files)
			maxText := threadCompactWidth - len(filesSuffix)
			if maxText < 20 {
				maxText = 20
			}
			if len(text) > maxText {
				text = text[:maxText-1] + "…"
			}
			fmt.Fprintf(&b, "  [%d] %s @%s%s %s: %s%s\n",
				msg.Index, label, msg.Username, meTag, msg.Timestamp, text, filesSuffix)
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
		if filesText := renderFiles(msg.Files); filesText != "" {
			b.WriteString(filesText)
		}
	}

	fmt.Fprintf(&b, "\n%s\n", strings.Repeat("─", 80))
	fmt.Fprintf(&b, "Status: %s\n", r.Status)

	return b.String()
}

// MentionItem is a single resolved mention entry for rendering.
type MentionItem struct {
	ChannelID   string              `json:"channel_id"`
	ChannelName string              `json:"channel_name"`
	UserID      string              `json:"user_id"`
	Username    string              `json:"username"`
	Timestamp   string              `json:"timestamp"`
	ThreadTS    string              `json:"thread_ts,omitempty"`
	Text        string              `json:"text"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	Files       []ThreadMessageFile `json:"files,omitempty"`
	Permalink   string              `json:"permalink,omitempty"`
	Status      string              `json:"status"`
}

// MentionsResult is the output of `dex slack mentions`.
type MentionsResult struct {
	Target    string        `json:"target"`
	Mentions  []MentionItem `json:"mentions"`
	Total     int           `json:"total"`
	Shown     int           `json:"shown"`
	Unhandled bool          `json:"unhandled_only"`
}

// RenderText implements render.Renderable.
func (r *MentionsResult) RenderText(mode render.Mode) string {
	var b strings.Builder

	if len(r.Mentions) == 0 {
		if r.Unhandled {
			fmt.Fprintf(&b, "No pending mentions found for %s\n", r.Target)
		} else {
			fmt.Fprintf(&b, "No mentions found for %s\n", r.Target)
		}
		return b.String()
	}

	if mode == render.ModeCompact {
		fmt.Fprintf(&b, "%-19s %-20s %-15s %-8s %s\n", "TIME", "CHANNEL", "FROM", "STATUS", "MESSAGE")
		fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 100))
		for _, m := range r.Mentions {
			filesSuffix := renderFilesCompact(m.Files)
			maxText := 50 - len(filesSuffix)
			if maxText < 20 {
				maxText = 20
			}
			text := mentionTruncate(MessageDisplayText(m.Text, m.Attachments), maxText)
			fmt.Fprintf(&b, "%-19s %-20s %-15s %-8s %s%s\n",
				m.Timestamp,
				mentionTruncate("#"+m.ChannelName, 20),
				mentionTruncate("@"+m.Username, 15),
				m.Status,
				text,
				filesSuffix,
			)
		}
	} else {
		for i, m := range r.Mentions {
			fmt.Fprintf(&b, "── %d ──────────────────────────────────────────────────────────────────────────────\n", i+1)
			fmt.Fprintf(&b, "#%s  •  %s  •  @%s  •  [%s]\n", m.ChannelName, m.Timestamp, m.Username, m.Status)
			if m.Permalink != "" {
				fmt.Fprintf(&b, "%s\n", m.Permalink)
			}
			b.WriteString("\n")
			b.WriteString(m.Text)
			b.WriteString("\n")
			if attText := renderAttachments(m.Attachments); attText != "" {
				b.WriteString(attText)
			}
			if filesText := renderFiles(m.Files); filesText != "" {
				b.WriteString(filesText)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if r.Unhandled {
		fmt.Fprintf(&b, "Found %d pending mentions\n", len(r.Mentions))
	} else if r.Total > r.Shown {
		fmt.Fprintf(&b, "Showing %d of %d total mentions\n", r.Shown, r.Total)
	} else {
		fmt.Fprintf(&b, "Found %d mentions\n", len(r.Mentions))
	}
	return b.String()
}

func mentionTruncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// TicketMention tracks where a Jira ticket was mentioned.
type TicketMention struct {
	Key        string   `json:"key"`
	Mentions   int      `json:"mentions"`
	Permalinks []string `json:"permalinks"`
}

// SearchItem is a single resolved search result for rendering.
type SearchItem struct {
	ChannelID   string              `json:"channel_id"`
	ChannelName string              `json:"channel_name"`
	UserID      string              `json:"user_id"`
	Username    string              `json:"username"`
	Timestamp   string              `json:"timestamp"`
	Text        string              `json:"text"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	Files       []ThreadMessageFile `json:"files,omitempty"`
	Permalink   string              `json:"permalink,omitempty"`
}

// SearchResultOutput is the output of `dex slack search`.
type SearchResultOutput struct {
	Query   string          `json:"query"`
	Results []SearchItem    `json:"results"`
	Tickets []TicketMention `json:"tickets,omitempty"`
	Total   int             `json:"total"`
	Shown   int             `json:"shown"`
}

// RenderText implements render.Renderable.
func (r *SearchResultOutput) RenderText(mode render.Mode) string {
	var b strings.Builder

	if len(r.Tickets) > 0 {
		fmt.Fprintf(&b, "Found %d tickets in %d messages:\n\n", len(r.Tickets), len(r.Results))
		for _, t := range r.Tickets {
			fmt.Fprintf(&b, "  %-12s (%d mention", t.Key, t.Mentions)
			if t.Mentions != 1 {
				b.WriteString("s")
			}
			b.WriteString(")\n")
			if mode == render.ModeNormal {
				for _, link := range t.Permalinks {
					fmt.Fprintf(&b, "    %s\n", link)
				}
			}
		}
		if r.Total > r.Shown {
			fmt.Fprintf(&b, "\nSearched %d of %d total matches\n", r.Shown, r.Total)
		}
		return b.String()
	}

	if len(r.Results) == 0 {
		return "No results found.\n"
	}

	if mode == render.ModeCompact {
		fmt.Fprintf(&b, "%-19s %-20s %-15s %s\n", "TIME", "CHANNEL", "FROM", "MESSAGE")
		fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 100))
		for _, res := range r.Results {
			filesSuffix := renderFilesCompact(res.Files)
			maxText := 60 - len(filesSuffix)
			if maxText < 20 {
				maxText = 20
			}
			text := mentionTruncate(MessageDisplayText(res.Text, res.Attachments), maxText)
			fmt.Fprintf(&b, "%-19s %-20s %-15s %s%s\n",
				res.Timestamp,
				mentionTruncate("#"+res.ChannelName, 20),
				mentionTruncate("@"+res.Username, 15),
				text,
				filesSuffix,
			)
		}
	} else {
		for i, res := range r.Results {
			fmt.Fprintf(&b, "── %d ──────────────────────────────────────────────────────────────────────────────\n", i+1)
			fmt.Fprintf(&b, "#%s  •  %s  •  @%s\n", res.ChannelName, res.Timestamp, res.Username)
			if res.Permalink != "" {
				fmt.Fprintf(&b, "%s\n", res.Permalink)
			}
			b.WriteString("\n")
			b.WriteString(MessageDisplayText(res.Text, res.Attachments))
			b.WriteString("\n")
			if attText := renderAttachments(res.Attachments); attText != "" {
				b.WriteString(attText)
			}
			if filesText := renderFiles(res.Files); filesText != "" {
				b.WriteString(filesText)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if r.Total > r.Shown {
		fmt.Fprintf(&b, "Showing %d of %d total results\n", r.Shown, r.Total)
	} else {
		fmt.Fprintf(&b, "Found %d results\n", len(r.Results))
	}
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

// renderFiles produces a text block for a list of file attachments.
func renderFiles(files []ThreadMessageFile) string {
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range files {
		link := f.Permalink
		if link == "" {
			link = f.URLPrivate
		}
		fmt.Fprintf(&b, "  📎 %s (%s) %s\n", f.Name, formatFileSize(f.Size), link)
	}
	return b.String()
}

// renderFilesCompact produces a compact one-line summary of files.
func renderFilesCompact(files []ThreadMessageFile) string {
	if len(files) == 0 {
		return ""
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	return fmt.Sprintf(" [📎 %s]", strings.Join(names, ", "))
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

// FileListResult is the output of `dex slack file list`.
type FileListResult struct {
	Files []FileInfo `json:"files"`
}

func (r *FileListResult) RenderText(mode render.Mode) string {
	if len(r.Files) == 0 {
		return "No files found.\n"
	}
	var b strings.Builder
	if mode == render.ModeCompact {
		fmt.Fprintf(&b, "%-20s  %-30s  %8s  %s\n", "ID", "NAME", "SIZE", "CREATED")
		fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 80))
		for _, f := range r.Files {
			name := f.Name
			if len(name) > 30 {
				name = name[:27] + "..."
			}
			fmt.Fprintf(&b, "%-20s  %-30s  %8s  %s\n",
				f.ID, name, formatFileSize(f.Size),
				time.Unix(f.Created, 0).Format("2006-01-02 15:04"))
		}
	} else {
		for _, f := range r.Files {
			fmt.Fprintf(&b, "%s  %s  (%s, %s)\n",
				f.ID, f.Name, formatFileSize(f.Size),
				time.Unix(f.Created, 0).Format("2006-01-02 15:04"))
			if f.Permalink != "" {
				fmt.Fprintf(&b, "  %s\n", f.Permalink)
			}
		}
	}
	return b.String()
}

// FileInfoResult is the output of `dex slack file info`.
type FileInfoResult struct {
	File FileInfo `json:"file"`
}

func (r *FileInfoResult) RenderText(_ render.Mode) string {
	f := r.File
	var b strings.Builder
	fmt.Fprintf(&b, "ID:        %s\n", f.ID)
	fmt.Fprintf(&b, "Name:      %s\n", f.Name)
	if f.Title != "" && f.Title != f.Name {
		fmt.Fprintf(&b, "Title:     %s\n", f.Title)
	}
	fmt.Fprintf(&b, "Type:      %s (%s)\n", f.Filetype, f.Mimetype)
	fmt.Fprintf(&b, "Size:      %s\n", formatFileSize(f.Size))
	fmt.Fprintf(&b, "Uploaded:  %s\n", time.Unix(f.Created, 0).Format("2006-01-02 15:04:05"))
	if f.Username != "" {
		fmt.Fprintf(&b, "By:        %s\n", f.Username)
	}
	if f.Shares > 0 {
		fmt.Fprintf(&b, "Shares:    %d channel(s)\n", f.Shares)
	}
	if f.Permalink != "" {
		fmt.Fprintf(&b, "Link:      %s\n", f.Permalink)
	}
	return b.String()
}

func formatFileSize(bytes int) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(bytes)/1024/1024)
	case bytes >= 1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// BookmarksResult is the output of `dex slack bookmarks`.
type BookmarksResult struct {
	ChannelID   string     `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	Bookmarks   []Bookmark `json:"bookmarks"`
}

func (r *BookmarksResult) RenderText(mode render.Mode) string {
	if len(r.Bookmarks) == 0 {
		return fmt.Sprintf("No bookmarks in %s.\n", r.ChannelName)
	}

	var b strings.Builder
	ch := r.ChannelName
	if ch == "" {
		ch = r.ChannelID
	}
	fmt.Fprintf(&b, "Bookmarks in #%s (%d)\n", ch, len(r.Bookmarks))
	fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 60))

	for _, bm := range r.Bookmarks {
		title := bm.Title
		if title == "" {
			title = "(untitled)"
		}
		if mode == render.ModeCompact {
			fmt.Fprintf(&b, "%-40s %s\n", title, bm.Link)
		} else {
			fmt.Fprintf(&b, "%s\n", title)
			if bm.Link != "" {
				fmt.Fprintf(&b, "  %s\n", bm.Link)
			}
			if bm.Type != "" && bm.Type != "link" {
				fmt.Fprintf(&b, "  type: %s\n", bm.Type)
			}
		}
	}
	return b.String()
}
