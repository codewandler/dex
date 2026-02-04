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
