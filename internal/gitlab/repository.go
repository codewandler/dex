package gitlab

import (
	"strings"
	"encoding/base64"
	"fmt"

	gogitlab "github.com/xanzy/go-gitlab"
)

// GetFile fetches a file's raw content from a repository at a given ref.
// If ref is empty, the project's default branch is used.
func (c *Client) GetFile(projectID any, path, ref string) (*FileResult, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.GetFileOptions{}
	if ref != "" {
		opts.Ref = gogitlab.Ptr(ref)
	} else {
		opts.Ref = gogitlab.Ptr("HEAD")
	}

	f, _, err := c.gl.RepositoryFiles.GetFile(pid, path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %q: %w", path, err)
	}

	content := f.Content
	if f.Encoding == "base64" {
		decoded, decErr := base64.StdEncoding.DecodeString(f.Content)
		if decErr == nil {
			content = string(decoded)
		}
	}

	return &FileResult{
		FilePath:     f.FilePath,
		FileName:     f.FileName,
		Ref:          f.Ref,
		Size:         f.Size,
		Encoding:     f.Encoding,
		Content:      content,
		BlobID:       f.BlobID,
		CommitID:     f.CommitID,
		LastCommitID: f.LastCommitID,
		SHA256:       f.SHA256,
	}, nil
}

// GetFileMetadata fetches only the metadata for a file (no content download).
func (c *Client) GetFileMetadata(projectID any, path, ref string) (*FileMetaResult, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.GetFileMetaDataOptions{}
	if ref != "" {
		opts.Ref = gogitlab.Ptr(ref)
	} else {
		opts.Ref = gogitlab.Ptr("HEAD")
	}

	f, _, err := c.gl.RepositoryFiles.GetFileMetaData(pid, path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata for %q: %w", path, err)
	}

	return &FileMetaResult{
		FilePath:     f.FilePath,
		FileName:     f.FileName,
		Ref:          f.Ref,
		Size:         f.Size,
		Encoding:     f.Encoding,
		BlobID:       f.BlobID,
		CommitID:     f.CommitID,
		LastCommitID: f.LastCommitID,
		SHA256:       f.SHA256,
	}, nil
}

// GetFileBlame returns git blame for a file at the given ref.
func (c *Client) GetFileBlame(projectID any, path, ref string) (*FileBlameResult, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.GetFileBlameOptions{}
	if ref != "" {
		opts.Ref = gogitlab.Ptr(ref)
	} else {
		opts.Ref = gogitlab.Ptr("HEAD")
	}

	ranges, _, err := c.gl.RepositoryFiles.GetFileBlame(pid, path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get blame for %q: %w", path, err)
	}

	result := &FileBlameResult{
		FilePath: path,
		Ref:      ref,
	}
	for _, r := range ranges {
		br := BlameRange{
			Lines: r.Lines,
		}
		if r.Commit.ID != "" {
			br.CommitID = r.Commit.ID
			br.CommitShortID = r.Commit.ID
			if len(r.Commit.ID) > 8 {
				br.CommitShortID = r.Commit.ID[:8]
			}
			br.CommitMessage = r.Commit.Message
			br.AuthorName = r.Commit.AuthorName
			br.AuthorEmail = r.Commit.AuthorEmail
			if r.Commit.AuthoredDate != nil {
				br.AuthoredDate = *r.Commit.AuthoredDate
			}
		}
		result.Ranges = append(result.Ranges, br)
	}

	return result, nil
}

// ListTree lists files and directories in a repository at a given path and ref.
func (c *Client) ListTree(projectID any, path, ref string, recursive bool) (*TreeResult, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.ListTreeOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 100, Page: 1},
		Recursive:   gogitlab.Ptr(recursive),
	}
	if path != "" {
		opts.Path = gogitlab.Ptr(path)
	}
	if ref != "" {
		opts.Ref = gogitlab.Ptr(ref)
	}

	var allNodes []TreeNode
	for {
		nodes, resp, err := c.gl.Repositories.ListTree(pid, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list tree: %w", err)
		}
		for _, n := range nodes {
			allNodes = append(allNodes, TreeNode{
				ID:   n.ID,
				Name: n.Name,
				Type: n.Type,
				Path: n.Path,
				Mode: n.Mode,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return &TreeResult{
		ProjectID: pid,
		Path:      path,
		Ref:       ref,
		Recursive: recursive,
		Nodes:     allNodes,
		Total:     len(allNodes),
	}, nil
}

// CompareRefs compares two refs (branches, tags, commit SHAs) in a project.
// path optionally scopes the diff to a specific file or directory prefix.
func (c *Client) CompareRefs(projectID any, from, to string, straight bool, path string) (*CompareResult, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.CompareOptions{
		From:     gogitlab.Ptr(from),
		To:       gogitlab.Ptr(to),
		Straight: gogitlab.Ptr(straight),
	}

	cmp, _, err := c.gl.Repositories.Compare(pid, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to compare %q..%q: %w", from, to, err)
	}

	result := &CompareResult{
		From:     from,
		To:       to,
		Straight: straight,
		Path:     path,
		Timeout:  cmp.CompareTimeout,
		SameRef:  cmp.CompareSameRef,
	}

	if cmp.Commit != nil {
		result.HeadCommit = cmp.Commit.ID
	}

	for _, commit := range cmp.Commits {
		rc := RepoCommit{
			ShortID:    commit.ShortID,
			Title:      commit.Title,
			AuthorName: commit.AuthorName,
		}
		if commit.ID != "" {
			rc.ID = commit.ID
		}
		if commit.CreatedAt != nil {
			rc.CreatedAt = *commit.CreatedAt
		}
		result.Commits = append(result.Commits, rc)
	}

	for _, d := range cmp.Diffs {
		// If a path filter is set, skip files that don't match
		if path != "" {
			prefix := strings.TrimSuffix(path, "/") + "/"
			newMatch := d.NewPath == path || strings.HasPrefix(d.NewPath, prefix)
			oldMatch := d.OldPath == path || strings.HasPrefix(d.OldPath, prefix)
			if !newMatch && !oldMatch {
				continue
			}
		}
		rd := RepoDiff{
			OldPath:   d.OldPath,
			NewPath:   d.NewPath,
			IsNew:     d.NewFile,
			IsDeleted: d.DeletedFile,
			IsRenamed: d.RenamedFile,
			Diff:      d.Diff,
		}
		for _, line := range splitLines(d.Diff) {
			if len(line) > 0 {
				switch line[0] {
				case '+':
					rd.Additions++
				case '-':
					rd.Deletions++
				}
			}
		}
		result.Diffs = append(result.Diffs, rd)
	}

	return result, nil
}

// SearchBlobs performs full-text search across file contents in a project.
func (c *Client) SearchBlobs(projectID any, query string) (*BlobSearchResult, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gogitlab.SearchOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 100, Page: 1},
	}

	var allBlobs []BlobMatch
	for {
		blobs, resp, err := c.gl.Search.BlobsByProject(pid, query, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to search blobs: %w", err)
		}
		for _, b := range blobs {
			allBlobs = append(allBlobs, BlobMatch{
				Basename:  b.Basename,
				Filename:  b.Filename,
				Path:      b.Path,
				Ref:       b.Ref,
				StartLine: b.Startline,
				Data:      b.Data,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return &BlobSearchResult{
		Query:   query,
		Matches: allBlobs,
		Total:   len(allBlobs),
	}, nil
}

// splitLines splits a string by newlines, used for counting diff lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

