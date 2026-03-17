package cli

import (
	"fmt"
	"os"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/gitlab"
	"github.com/codewandler/dex/internal/render"

	"github.com/spf13/cobra"
)

// ── gl file ───────────────────────────────────────────────────────────────────

var gitlabFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Repository file commands",
	Long:  `Commands for reading files and blame from GitLab repositories.`,
}

var gitlabFileShowCmd = &cobra.Command{
	Use:   "show <project> <path>",
	Short: "Show a file's content",
	Long: `Fetch and display the raw content of a file in a GitLab repository.

By default, the project's default branch (HEAD) is used.
Use --ref to specify a branch, tag, or commit SHA.

Examples:
  dex gl file show my-group/my-project go.mod
  dex gl file show my-group/my-project go.mod --ref main
  dex gl file show my-group/my-project src/main.go --ref feature/my-branch
  dex gl file show my-group/my-project Makefile --ref abc1234`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		project := args[0]
		path := args[1]
		ref, _ := cmd.Flags().GetString("ref")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireGitLab(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		result, err := client.GetFile(project, path, ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		compact, _ := cmd.Flags().GetBool("compact")
		mode := render.ModeNormal
		if compact {
			mode = render.ModeCompact
		}
		RenderWithMode(result, mode)
	},
}

var gitlabFileMetaCmd = &cobra.Command{
	Use:   "meta <project> <path>",
	Short: "Show file metadata without downloading content",
	Long: `Fetch file metadata (size, last commit, SHA256) without downloading the content.

Examples:
  dex gl file meta my-group/my-project go.mod
  dex gl file meta my-group/my-project go.mod --ref develop`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		project := args[0]
		path := args[1]
		ref, _ := cmd.Flags().GetString("ref")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireGitLab(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		result, err := client.GetFileMetadata(project, path, ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		compact, _ := cmd.Flags().GetBool("compact")
		mode := render.ModeNormal
		if compact {
			mode = render.ModeCompact
		}
		RenderWithMode(result, mode)
	},
}

var gitlabFileBlameCmd = &cobra.Command{
	Use:   "blame <project> <path>",
	Short: "Show git blame for a file",
	Long: `Show git blame output for a file, listing each line with its commit and author.

Examples:
  dex gl file blame my-group/my-project src/server.go
  dex gl file blame my-group/my-project src/server.go --ref main`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		project := args[0]
		path := args[1]
		ref, _ := cmd.Flags().GetString("ref")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireGitLab(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		result, err := client.GetFileBlame(project, path, ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		compact, _ := cmd.Flags().GetBool("compact")
		mode := render.ModeNormal
		if compact {
			mode = render.ModeCompact
		}
		RenderWithMode(result, mode)
	},
}

// ── gl tree ───────────────────────────────────────────────────────────────────

var gitlabTreeCmd = &cobra.Command{
	Use:   "tree <project> [path]",
	Short: "List repository tree",
	Long: `List files and directories in a GitLab repository at a given path and ref.

Without --path, lists the root directory. Use --recursive to walk the full tree.
In compact mode (-o compact), outputs one path per line, suitable for piping.

Examples:
  dex gl tree my-group/my-project
  dex gl tree my-group/my-project --path src/
  dex gl tree my-group/my-project --ref feature/my-branch
  dex gl tree my-group/my-project --recursive
  dex gl tree my-group/my-project --recursive --path internal/ -o compact`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		project := args[0]
		path := ""
		if len(args) == 2 {
			path = args[1]
		}
		if p, _ := cmd.Flags().GetString("path"); p != "" {
			path = p
		}
		ref, _ := cmd.Flags().GetString("ref")
		recursive, _ := cmd.Flags().GetBool("recursive")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireGitLab(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		result, err := client.ListTree(project, path, ref, recursive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		Render(result)
	},
}

// ── gl diff ───────────────────────────────────────────────────────────────────

var gitlabDiffCmd = &cobra.Command{
	Use:   "diff <project> <from> <to>",
	Short: "Compare two refs (branches, tags, commits)",
	Long: `Compare two refs in a GitLab repository.

Always shows a summary: commit list (capped at 20) and changed files.
Diff content is only shown when --path is provided to scope the output
to a specific file or directory — diffing an entire project is too noisy.

<from> and <to> can be branch names, tag names, or commit SHAs.
By default uses three-dot diff semantics (A...B, merge-base comparison).
Use --straight for two-dot semantics (A..B, direct comparison).

Examples:
  dex gl diff my-group/my-project main feature/my-branch
  dex gl diff my-group/my-project main feature/my-branch --path go.mod
  dex gl diff my-group/my-project main feature/my-branch --path internal/server/
  dex gl diff my-group/my-project v1.2.0 v1.3.0 --path src/
  dex gl diff my-group/my-project abc1234 def5678 --path src/main.go
  dex gl diff my-group/my-project main feature/my-branch --path go.mod --compact
  dex gl diff my-group/my-project main feature/my-branch -o json`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		project := args[0]
		from := args[1]
		to := args[2]
		straight, _ := cmd.Flags().GetBool("straight")
		path, _ := cmd.Flags().GetString("path")

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireGitLab(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		result, err := client.CompareRefs(project, from, to, straight, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		compact, _ := cmd.Flags().GetBool("compact")
		mode := render.ModeNormal
		if compact {
			mode = render.ModeCompact
		}
		RenderWithMode(result, mode)
	},
}

// ── gl search blobs ───────────────────────────────────────────────────────────

var gitlabSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search commands",
	Long:  `Commands for searching across GitLab projects.`,
}

var gitlabSearchBlobsCmd = &cobra.Command{
	Use:   "blobs <query>",
	Short: "Search file contents across a project",
	Long: `Search file contents (blobs) across a GitLab project using full-text search.

Requires --project to scope the search. Results include the file path, ref,
line number, and a snippet of the matching content.

Examples:
  dex gl search blobs "TODO: remove" --project my-group/my-project
  dex gl search blobs "DATABASE_URL" --project my-group/my-project --compact
  dex gl search blobs "SomeClassName" --project my-group/my-project -o json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]
		project, _ := cmd.Flags().GetString("project")

		if project == "" {
			fmt.Fprintf(os.Stderr, "Error: --project is required\n")
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := cfg.RequireGitLab(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		client, err := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
			os.Exit(1)
		}

		result, err := client.SearchBlobs(project, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		compact, _ := cmd.Flags().GetBool("compact")
		mode := render.ModeNormal
		if compact {
			mode = render.ModeCompact
		}
		RenderWithMode(result, mode)
	},
}


