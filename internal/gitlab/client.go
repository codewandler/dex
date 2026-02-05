package gitlab

import (
	"fmt"
	"strconv"

	"github.com/xanzy/go-gitlab"
)

type Client struct {
	gl *gitlab.Client
}

// resolveProjectID converts a project path or ID string to a numeric project ID.
// It checks the local cache first, then falls back to an API lookup.
// Numeric IDs (as int or string) are returned directly.
func (c *Client) resolveProjectID(pid any) (int, error) {
	switch v := pid.(type) {
	case int:
		return v, nil
	case string:
		// Try parsing as numeric ID first
		if id, err := strconv.Atoi(v); err == nil {
			return id, nil
		}

		// Check the local index/cache
		idx, err := LoadIndex()
		if err == nil {
			if pm := idx.FindProject(v); pm != nil {
				return pm.ID, nil
			}
		}

		// Fall back to API lookup
		project, _, err := c.gl.Projects.GetProject(gitlab.PathEscape(v), nil)
		if err != nil {
			return 0, fmt.Errorf("project not found: %s", v)
		}
		return project.ID, nil
	default:
		return 0, fmt.Errorf("unsupported project ID type: %T", pid)
	}
}

func NewClient(url, token string) (*Client, error) {
	gl, err := gitlab.NewClient(token, gitlab.WithBaseURL(url+"/api/v4"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	return &Client{gl: gl}, nil
}

// TestAuth verifies the token works and returns current user info
func (c *Client) TestAuth() (*gitlab.User, error) {
	user, _, err := c.gl.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("auth test failed: %w", err)
	}
	return user, nil
}
