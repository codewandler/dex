package models

import (
	"strconv"
	"time"
)

type Contributor struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Commits   int    `json:"commits"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type ProjectMetadata struct {
	ID              int                `json:"id"`
	Name            string             `json:"name"`
	PathWithNS      string             `json:"path_with_namespace"`
	Description     string             `json:"description"`
	WebURL          string             `json:"web_url"`
	DefaultBranch   string             `json:"default_branch"`
	Visibility      string             `json:"visibility"`
	Topics          []string           `json:"topics"`
	StarCount       int                `json:"star_count"`
	ForksCount      int                `json:"forks_count"`
	Languages       map[string]float32 `json:"languages"`
	TopContributors []Contributor      `json:"top_contributors"`
	LastActivityAt  time.Time          `json:"last_activity_at"`
	IndexedAt       time.Time          `json:"indexed_at"`
}

type GitLabIndex struct {
	Version         int               `json:"version"`
	GitLabURL       string            `json:"gitlab_url"`
	LastFullIndexAt time.Time         `json:"last_full_index_at"`
	Projects        []ProjectMetadata `json:"projects"`
	ProjectsByID    map[int]int       `json:"-"`
	ProjectsByPath  map[string]int    `json:"-"`
}

func NewGitLabIndex(gitlabURL string) *GitLabIndex {
	return &GitLabIndex{
		Version:        1,
		GitLabURL:      gitlabURL,
		Projects:       []ProjectMetadata{},
		ProjectsByID:   make(map[int]int),
		ProjectsByPath: make(map[string]int),
	}
}

func (idx *GitLabIndex) BuildLookupMaps() {
	idx.ProjectsByID = make(map[int]int)
	idx.ProjectsByPath = make(map[string]int)

	for i, p := range idx.Projects {
		idx.ProjectsByID[p.ID] = i
		idx.ProjectsByPath[p.PathWithNS] = i
	}
}

func (idx *GitLabIndex) FindProject(idOrPath string) *ProjectMetadata {
	if idx.ProjectsByID == nil || idx.ProjectsByPath == nil {
		idx.BuildLookupMaps()
	}

	// Try as ID first
	if id, err := strconv.Atoi(idOrPath); err == nil {
		if i, ok := idx.ProjectsByID[id]; ok {
			return &idx.Projects[i]
		}
	}

	// Try as path
	if i, ok := idx.ProjectsByPath[idOrPath]; ok {
		return &idx.Projects[i]
	}

	return nil
}

func (idx *GitLabIndex) UpsertProject(p ProjectMetadata) {
	if idx.ProjectsByID == nil || idx.ProjectsByPath == nil {
		idx.BuildLookupMaps()
	}

	if i, ok := idx.ProjectsByID[p.ID]; ok {
		idx.Projects[i] = p
		idx.ProjectsByPath[p.PathWithNS] = i
	} else {
		i := len(idx.Projects)
		idx.Projects = append(idx.Projects, p)
		idx.ProjectsByID[p.ID] = i
		idx.ProjectsByPath[p.PathWithNS] = i
	}
}
