package gitlab

import "time"

// ── File types ────────────────────────────────────────────────────────────────

// FileResult holds the content and metadata of a repository file.
type FileResult struct {
	FilePath     string `json:"file_path"`
	FileName     string `json:"file_name"`
	Ref          string `json:"ref"`
	Size         int    `json:"size"`
	Encoding     string `json:"encoding"`
	Content      string `json:"content"`
	BlobID       string `json:"blob_id"`
	CommitID     string `json:"commit_id"`
	LastCommitID string `json:"last_commit_id"`
	SHA256       string `json:"sha256"`
}

// FileMetaResult holds only metadata for a repository file (no content).
type FileMetaResult struct {
	FilePath     string `json:"file_path"`
	FileName     string `json:"file_name"`
	Ref          string `json:"ref"`
	Size         int    `json:"size"`
	Encoding     string `json:"encoding"`
	BlobID       string `json:"blob_id"`
	CommitID     string `json:"commit_id"`
	LastCommitID string `json:"last_commit_id"`
	SHA256       string `json:"sha256"`
}

// BlameRange represents a range of lines with the same git blame commit.
type BlameRange struct {
	CommitID      string    `json:"commit_id"`
	CommitShortID string    `json:"commit_short_id"`
	CommitMessage string    `json:"commit_message"`
	AuthorName    string    `json:"author_name"`
	AuthorEmail   string    `json:"author_email"`
	AuthoredDate  time.Time `json:"authored_date"`
	Lines         []string  `json:"lines"`
}

// FileBlameResult holds git blame output for a file.
type FileBlameResult struct {
	FilePath string       `json:"file_path"`
	Ref      string       `json:"ref"`
	Ranges   []BlameRange `json:"ranges"`
}

// ── Tree types ────────────────────────────────────────────────────────────────

// TreeNode represents a file or directory in a repository tree.
type TreeNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "blob" (file) or "tree" (directory)
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// TreeResult holds the listing of a repository tree.
type TreeResult struct {
	ProjectID int        `json:"project_id"`
	Path      string     `json:"path"`
	Ref       string     `json:"ref"`
	Recursive bool       `json:"recursive"`
	Nodes     []TreeNode `json:"nodes"`
	Total     int        `json:"total"`
}

// ── Compare / diff types ──────────────────────────────────────────────────────

// RepoCommit is a simplified commit in a compare result.
type RepoCommit struct {
	ID         string    `json:"id"`
	ShortID    string    `json:"short_id"`
	Title      string    `json:"title"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// RepoDiff represents a single file diff in a compare result.
type RepoDiff struct {
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	IsNew     bool   `json:"is_new"`
	IsDeleted bool   `json:"is_deleted"`
	IsRenamed bool   `json:"is_renamed"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Diff      string `json:"diff"`
}

// CompareResult holds the output of comparing two refs.
type CompareResult struct {
	From       string       `json:"from"`
	To         string       `json:"to"`
	Straight   bool         `json:"straight"`
	Path       string       `json:"path,omitempty"` // non-empty when scoped to a file/dir
	HeadCommit string       `json:"head_commit,omitempty"`
	Timeout    bool         `json:"timeout"`
	SameRef    bool         `json:"same_ref"`
	Commits    []RepoCommit `json:"commits"`
	Diffs      []RepoDiff   `json:"diffs"`
}

// ── Blob search types ─────────────────────────────────────────────────────────

// BlobMatch represents a single search hit in a file.
type BlobMatch struct {
	Basename  string `json:"basename"`
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
	StartLine int    `json:"start_line"`
	Data      string `json:"data"` // matching snippet
}

// BlobSearchResult holds the results of a blob content search.
type BlobSearchResult struct {
	Query   string      `json:"query"`
	Matches []BlobMatch `json:"matches"`
	Total   int         `json:"total"`
}
