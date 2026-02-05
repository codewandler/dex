package gitlab

import (
	"fmt"

	"github.com/xanzy/go-gitlab"
)

type Client struct {
	gl *gitlab.Client
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
