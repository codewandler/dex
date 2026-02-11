package slack

import (
	"testing"

	"github.com/codewandler/dex/internal/models"
)

func TestResolveChannelMentions(t *testing.T) {
	// Create a test index with sample channels
	idx := models.NewSlackIndex("T123", "Test Team")
	idx.UpsertChannel(models.SlackChannel{
		ID:   "C12345",
		Name: "dev-team",
	})
	idx.UpsertChannel(models.SlackChannel{
		ID:   "C67890",
		Name: "general",
	})
	idx.BuildLookupMaps()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single channel mention",
			input:    "Check out #dev-team",
			expected: "Check out <#C12345>",
		},
		{
			name:     "multiple channel mentions",
			input:    "See #dev-team and #general for updates",
			expected: "See <#C12345> and <#C67890> for updates",
		},
		{
			name:     "no channel mentions",
			input:    "No channels here",
			expected: "No channels here",
		},
		{
			name:     "already formatted mention",
			input:    "Already formatted <#C12345>",
			expected: "Already formatted <#C12345>",
		},
		{
			name:     "unknown channel",
			input:    "Unknown #nonexistent channel",
			expected: "Unknown #nonexistent channel",
		},
		{
			name:     "channel with hyphens",
			input:    "Check #dev-team please",
			expected: "Check <#C12345> please",
		},
		{
			name:     "mixed with user mentions",
			input:    "Hey @user check #dev-team",
			expected: "Hey @user check <#C12345>",
		},
	}

	// Save the test index temporarily
	originalIdx := idx
	SaveIndex(originalIdx)
	defer func() {
		// Clean up is not critical for this test
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveChannelMentions(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveChannelMentions(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveMentions(t *testing.T) {
	// Create a test index with sample users
	idx := models.NewSlackIndex("T123", "Test Team")
	idx.UpsertUser(models.SlackUser{
		ID:       "U12345",
		Username: "john.doe",
	})
	idx.UpsertUser(models.SlackUser{
		ID:       "U67890",
		Username: "alice",
	})
	idx.BuildLookupMaps()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single user mention",
			input:    "Hey @john.doe check this",
			expected: "Hey <@U12345> check this",
		},
		{
			name:     "multiple user mentions",
			input:    "@john.doe and @alice please review",
			expected: "<@U12345> and <@U67890> please review",
		},
		{
			name:     "no user mentions",
			input:    "No mentions here",
			expected: "No mentions here",
		},
		{
			name:     "already formatted mention",
			input:    "Already formatted <@U12345>",
			expected: "Already formatted <@U12345>",
		},
		{
			name:     "unknown user",
			input:    "Unknown @bob",
			expected: "Unknown @bob",
		},
	}

	// Save the test index temporarily
	originalIdx := idx
	SaveIndex(originalIdx)
	defer func() {
		// Clean up is not critical for this test
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveMentions(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveMentions(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveGroupMentions(t *testing.T) {
	// Create a test index with sample user groups and users
	idx := models.NewSlackIndex("T123", "Test Team")
	idx.UpsertUserGroup(models.SlackUserGroup{
		ID:     "S12345",
		Handle: "sre-team",
		Name:   "SRE Team",
	})
	idx.UpsertUserGroup(models.SlackUserGroup{
		ID:     "S67890",
		Handle: "backend",
		Name:   "Backend Engineers",
	})
	// Add a user to verify users take precedence (resolved first by ResolveMentions)
	idx.UpsertUser(models.SlackUser{
		ID:       "U11111",
		Username: "john.doe",
	})
	idx.BuildLookupMaps()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single group mention",
			input:    "Hey @sre-team check this",
			expected: "Hey <!subteam^S12345> check this",
		},
		{
			name:     "multiple group mentions",
			input:    "@sre-team and @backend please review",
			expected: "<!subteam^S12345> and <!subteam^S67890> please review",
		},
		{
			name:     "unknown group left as-is",
			input:    "Hey @nonexistent check this",
			expected: "Hey @nonexistent check this",
		},
		{
			name:     "already formatted subteam",
			input:    "Already formatted <!subteam^S12345>",
			expected: "Already formatted <!subteam^S12345>",
		},
		{
			name:     "no mentions",
			input:    "No mentions here",
			expected: "No mentions here",
		},
		{
			name:     "mixed with already-resolved user mentions",
			input:    "<@U11111> and @sre-team please check",
			expected: "<@U11111> and <!subteam^S12345> please check",
		},
	}

	// Save the test index temporarily
	SaveIndex(idx)
	defer func() {
		// Clean up is not critical for this test
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveGroupMentions(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveGroupMentions(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
