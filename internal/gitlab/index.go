package gitlab

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/codewandler/dex/internal/models"

	"github.com/xanzy/go-gitlab"
)

const maxConcurrentFetches = 10

func indexConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".dex", "gitlab")
	return dir, os.MkdirAll(dir, 0700)
}

func indexFilePath() (string, error) {
	dir, err := indexConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "index.json"), nil
}

func LoadIndex() (*models.GitLabIndex, error) {
	path, err := indexFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return models.NewGitLabIndex(""), nil
		}
		return nil, err
	}

	var idx models.GitLabIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	idx.BuildLookupMaps()
	return &idx, nil
}

func SaveIndex(idx *models.GitLabIndex) error {
	path, err := indexFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (c *Client) getAllProjects() ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project

	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
		Membership: gitlab.Ptr(true),
		OrderBy:    gitlab.Ptr("last_activity_at"),
		Sort:       gitlab.Ptr("desc"),
	}

	for {
		projects, resp, err := c.gl.Projects.ListProjects(opts)
		if err != nil {
			return nil, err
		}

		allProjects = append(allProjects, projects...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allProjects, nil
}

func (c *Client) fetchProjectMetadata(p *gitlab.Project) models.ProjectMetadata {
	pm := models.ProjectMetadata{
		ID:             p.ID,
		Name:           p.Name,
		PathWithNS:     p.PathWithNamespace,
		Description:    p.Description,
		WebURL:         p.WebURL,
		DefaultBranch:  p.DefaultBranch,
		Visibility:     string(p.Visibility),
		Topics:         p.Topics,
		StarCount:      p.StarCount,
		ForksCount:     p.ForksCount,
		LastActivityAt: *p.LastActivityAt,
		IndexedAt:      time.Now(),
	}

	// Fetch languages
	langs, _, err := c.gl.Projects.GetProjectLanguages(p.ID)
	if err == nil && langs != nil {
		pm.Languages = make(map[string]float32)
		for lang, pct := range *langs {
			pm.Languages[lang] = pct
		}
	}

	// Fetch contributors and get top 5
	contributors, _, err := c.gl.Repositories.Contributors(p.ID, &gitlab.ListContributorsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 20},
		OrderBy:     gitlab.Ptr("commits"),
		Sort:        gitlab.Ptr("desc"),
	})
	if err == nil && len(contributors) > 0 {
		limit := 5
		if len(contributors) < limit {
			limit = len(contributors)
		}
		pm.TopContributors = make([]models.Contributor, limit)
		for i := 0; i < limit; i++ {
			pm.TopContributors[i] = models.Contributor{
				Name:      contributors[i].Name,
				Email:     contributors[i].Email,
				Commits:   contributors[i].Commits,
				Additions: contributors[i].Additions,
				Deletions: contributors[i].Deletions,
			}
		}
	}

	return pm
}

type ProgressFunc func(completed, total int)

func (c *Client) IndexAllProjects(gitlabURL string, progressFn ProgressFunc) (*models.GitLabIndex, error) {
	projects, err := c.getAllProjects()
	if err != nil {
		return nil, err
	}

	idx := models.NewGitLabIndex(gitlabURL)
	idx.LastFullIndexAt = time.Now()

	results := make(chan models.ProjectMetadata, len(projects))
	semaphore := make(chan struct{}, maxConcurrentFetches)

	var wg sync.WaitGroup

	for _, p := range projects {
		wg.Add(1)
		go func(proj *gitlab.Project) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			pm := c.fetchProjectMetadata(proj)
			results <- pm
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	completed := 0
	total := len(projects)

	for pm := range results {
		completed++
		idx.UpsertProject(pm)
		if progressFn != nil {
			progressFn(completed, total)
		}
	}

	// Sort projects by last activity (most recent first)
	sort.Slice(idx.Projects, func(i, j int) bool {
		return idx.Projects[i].LastActivityAt.After(idx.Projects[j].LastActivityAt)
	})
	idx.BuildLookupMaps()

	return idx, nil
}

func (c *Client) GetProjectMetadata(idOrPath string) (*models.ProjectMetadata, error) {
	var project *gitlab.Project
	var err error

	// Try as ID first
	if id, parseErr := strconv.Atoi(idOrPath); parseErr == nil {
		project, _, err = c.gl.Projects.GetProject(id, nil)
	} else {
		project, _, err = c.gl.Projects.GetProject(idOrPath, nil)
	}

	if err != nil {
		return nil, err
	}

	pm := c.fetchProjectMetadata(project)
	return &pm, nil
}
