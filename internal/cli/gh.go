package cli

import (
	"fmt"
	"strings"

	"github.com/codewandler/dex/internal/gh"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:     "gh",
	Aliases: []string{"github"},
	Short:   "GitHub operations via gh CLI",
	Long:    `Wrapper around the gh CLI for GitHub operations.`,
}

var ghCloneCmd = &cobra.Command{
	Use:   "clone <repo> [dest]",
	Short: "Clone a GitHub repository",
	Long: `Clone a GitHub repository using the gh CLI.

The repo can be specified as:
  - Full URL: https://github.com/owner/repo
  - Short form: owner/repo

If dest is not provided, clones to the repo name in the current directory.

Examples:
  dex gh clone owner/repo
  dex gh clone https://github.com/owner/repo
  dex gh clone owner/repo ./my-local-dir`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'gh auth login' first")
		}

		repoURL := args[0]
		dest := ""
		if len(args) > 1 {
			dest = args[1]
		}

		fmt.Printf("Cloning %s...\n", repoURL)
		if err := client.Clone(repoURL, dest); err != nil {
			return err
		}

		fmt.Println("Clone complete!")
		return nil
	},
}

var ghAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with GitHub",
	Long: `Authenticate with GitHub using the gh CLI.

This runs 'gh auth login' interactively to set up authentication.

Examples:
  dex gh auth`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsInstalled() {
			return fmt.Errorf("gh CLI is not installed. Install from https://cli.github.com/")
		}

		return client.Login()
	},
}

// Issue commands
var ghIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage GitHub issues",
	Long:  `Create, list, and view GitHub issues.`,
}

var ghIssueListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List issues in a repository",
	Long: `List issues in a GitHub repository.

By default, lists open issues in the current repository.

Examples:
  dex gh issue list
  dex gh issue ls
  dex gh issue list --state closed
  dex gh issue list --label bug
  dex gh issue list --no-label
  dex gh issue list --repo owner/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		state, _ := cmd.Flags().GetString("state")
		label, _ := cmd.Flags().GetString("label")
		noLabel, _ := cmd.Flags().GetBool("no-label")
		assignee, _ := cmd.Flags().GetString("assignee")
		limit, _ := cmd.Flags().GetInt("limit")
		repo, _ := cmd.Flags().GetString("repo")

		issues, err := client.IssueList(gh.IssueListOptions{
			State:    state,
			Label:    label,
			NoLabel:  noLabel,
			Assignee: assignee,
			Limit:    limit,
			Repo:     repo,
		})
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			fmt.Println("No issues found")
			return nil
		}

		for _, issue := range issues {
			labels := ""
			if len(issue.Labels) > 0 {
				labels = fmt.Sprintf(" [%s]", issue.Labels[0])
				if len(issue.Labels) > 1 {
					labels = fmt.Sprintf(" [%s +%d]", issue.Labels[0], len(issue.Labels)-1)
				}
			}
			fmt.Printf("#%-4d %s%s\n", issue.Number, issue.Title, labels)
		}

		return nil
	},
}

var ghIssueViewCmd = &cobra.Command{
	Use:   "view <number>",
	Short: "View a specific issue",
	Long: `View details of a GitHub issue.

Examples:
  dex gh issue view 123
  dex gh issue view 123 --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		var number int
		if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		repo, _ := cmd.Flags().GetString("repo")

		issue, err := client.IssueView(number, repo)
		if err != nil {
			return err
		}

		fmt.Printf("#%d %s\n", issue.Number, issue.Title)
		fmt.Printf("State: %s | Author: @%s | Created: %s\n", issue.State, issue.Author, issue.CreatedAt[:10])

		if len(issue.Labels) > 0 {
			fmt.Printf("Labels: %s\n", joinStrings(issue.Labels))
		}
		if len(issue.Assignees) > 0 {
			fmt.Printf("Assignees: %s\n", joinStrings(issue.Assignees))
		}

		fmt.Printf("URL: %s\n", issue.URL)

		if issue.Body != "" {
			fmt.Printf("\n%s\n", issue.Body)
		}

		return nil
	},
}

var ghIssueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	Long: `Create a new GitHub issue.

Examples:
  dex gh issue create --title "Bug report" --body "Description here"
  dex gh issue create -t "Feature request" -b "Details" --label enhancement
  dex gh issue create --title "Issue" --repo owner/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		labels, _ := cmd.Flags().GetStringSlice("label")
		assignee, _ := cmd.Flags().GetString("assignee")
		repo, _ := cmd.Flags().GetString("repo")

		if title == "" {
			return fmt.Errorf("--title is required")
		}

		issue, err := client.IssueCreate(gh.IssueCreateOptions{
			Title:    title,
			Body:     body,
			Labels:   labels,
			Assignee: assignee,
			Repo:     repo,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created issue #%d: %s\n", issue.Number, issue.URL)
		return nil
	},
}

var ghIssueCloseCmd = &cobra.Command{
	Use:   "close <number>",
	Short: "Close an issue",
	Long: `Close a GitHub issue.

Examples:
  dex gh issue close 123
  dex gh issue close 123 --comment "Fixed in PR #456"
  dex gh issue close 123 --reason "not planned"
  dex gh issue close 123 --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		var number int
		if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		comment, _ := cmd.Flags().GetString("comment")
		reason, _ := cmd.Flags().GetString("reason")
		repo, _ := cmd.Flags().GetString("repo")

		if err := client.IssueClose(gh.IssueCloseOptions{
			Number:  number,
			Comment: comment,
			Reason:  reason,
			Repo:    repo,
		}); err != nil {
			return err
		}

		fmt.Printf("Closed issue #%d\n", number)
		return nil
	},
}

var ghIssueCommentCmd = &cobra.Command{
	Use:   "comment <number>",
	Short: "Add a comment to an issue",
	Long: `Add a comment to a GitHub issue.

Examples:
  dex gh issue comment 123 --body "This is fixed in the latest release"
  dex gh issue comment 123 -b "Working on this now"
  dex gh issue comment 123 -b "Comment" --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		var number int
		if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		body, _ := cmd.Flags().GetString("body")
		repo, _ := cmd.Flags().GetString("repo")

		if body == "" {
			return fmt.Errorf("--body is required")
		}

		if err := client.IssueComment(gh.IssueCommentOptions{
			Number: number,
			Body:   body,
			Repo:   repo,
		}); err != nil {
			return err
		}

		fmt.Printf("Commented on issue #%d\n", number)
		return nil
	},
}

var ghIssueEditCmd = &cobra.Command{
	Use:   "edit <number>",
	Short: "Edit an issue (add/remove labels)",
	Long: `Edit a GitHub issue to add or remove labels.

Examples:
  dex gh issue edit 123 --add-label bug
  dex gh issue edit 123 --add-label "Feature Request" --add-label "New Integration"
  dex gh issue edit 123 --remove-label bug
  dex gh issue edit 123 --add-label enhancement --remove-label bug
  dex gh issue edit 123 --add-label bug --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		var number int
		if _, err := fmt.Sscanf(args[0], "%d", &number); err != nil {
			return fmt.Errorf("invalid issue number: %s", args[0])
		}

		addLabels, _ := cmd.Flags().GetStringSlice("add-label")
		removeLabels, _ := cmd.Flags().GetStringSlice("remove-label")
		repo, _ := cmd.Flags().GetString("repo")

		if len(addLabels) == 0 && len(removeLabels) == 0 {
			return fmt.Errorf("at least one --add-label or --remove-label is required")
		}

		if err := client.IssueEdit(gh.IssueEditOptions{
			Number:       number,
			AddLabels:    addLabels,
			RemoveLabels: removeLabels,
			Repo:         repo,
		}); err != nil {
			return err
		}

		fmt.Printf("Updated issue #%d\n", number)
		return nil
	},
}

func joinStrings(s []string) string {
	return strings.Join(s, ", ")
}

// Label commands
var ghLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage GitHub labels",
	Long:  `Create, list, and delete GitHub labels.`,
}

var ghLabelListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List labels in a repository",
	Long: `List labels in a GitHub repository.

Examples:
  dex gh label list
  dex gh label ls
  dex gh label list --limit 50
  dex gh label list --repo owner/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		limit, _ := cmd.Flags().GetInt("limit")
		repo, _ := cmd.Flags().GetString("repo")

		labels, err := client.LabelList(gh.LabelListOptions{
			Limit: limit,
			Repo:  repo,
		})
		if err != nil {
			return err
		}

		if len(labels) == 0 {
			fmt.Println("No labels found")
			return nil
		}

		for _, label := range labels {
			desc := ""
			if label.Description != "" {
				desc = fmt.Sprintf(" - %s", label.Description)
			}
			fmt.Printf("%-30s #%s%s\n", label.Name, label.Color, desc)
		}

		return nil
	},
}

var ghLabelCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new label",
	Long: `Create a new label in a GitHub repository.

Examples:
  dex gh label create "bug"
  dex gh label create "Feature Request" --color ff0000
  dex gh label create "New Integration" --color 00ff00 --description "Request for new integration"
  dex gh label create "bug" --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		name := args[0]
		color, _ := cmd.Flags().GetString("color")
		description, _ := cmd.Flags().GetString("description")
		repo, _ := cmd.Flags().GetString("repo")

		label, err := client.LabelCreate(gh.LabelCreateOptions{
			Name:        name,
			Color:       color,
			Description: description,
			Repo:        repo,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created label: %s\n", label.Name)
		return nil
	},
}

var ghLabelDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a label",
	Long: `Delete a label from a GitHub repository.

Examples:
  dex gh label delete "bug"
  dex gh label delete "Feature Request" --repo owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		name := args[0]
		repo, _ := cmd.Flags().GetString("repo")

		if err := client.LabelDelete(gh.LabelDeleteOptions{
			Name: name,
			Repo: repo,
		}); err != nil {
			return err
		}

		fmt.Printf("Deleted label: %s\n", name)
		return nil
	},
}

// Release commands
var ghReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Manage GitHub releases",
	Long:  `Create, list, and view GitHub releases.`,
}

var ghReleaseListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List releases in a repository",
	Long: `List releases in a GitHub repository.

By default, lists the most recent releases.

Examples:
  dex gh release list
  dex gh release ls
  dex gh release list --limit 5
  dex gh release list --exclude-drafts
  dex gh release list --repo owner/repo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		limit, _ := cmd.Flags().GetInt("limit")
		excludeDrafts, _ := cmd.Flags().GetBool("exclude-drafts")
		excludePrereleases, _ := cmd.Flags().GetBool("exclude-pre-releases")
		repo, _ := cmd.Flags().GetString("repo")

		releases, err := client.ReleaseList(gh.ReleaseListOptions{
			Limit:              limit,
			ExcludeDrafts:      excludeDrafts,
			ExcludePrereleases: excludePrereleases,
			Repo:               repo,
		})
		if err != nil {
			return err
		}

		if len(releases) == 0 {
			fmt.Println("No releases found")
			return nil
		}

		for _, release := range releases {
			flags := ""
			if release.IsLatest {
				flags = " (latest)"
			}
			if release.IsDraft {
				flags = " (draft)"
			}
			if release.IsPrerelease {
				flags = " (prerelease)"
			}

			name := release.Name
			if name == "" {
				name = release.TagName
			}

			date := ""
			if release.PublishedAt != "" && len(release.PublishedAt) >= 10 {
				date = release.PublishedAt[:10]
			}

			fmt.Printf("%-12s  %s  %s%s\n", release.TagName, date, name, flags)
		}

		return nil
	},
}

var ghReleaseViewCmd = &cobra.Command{
	Use:   "view [tag]",
	Short: "View a specific release",
	Long: `View details of a GitHub release.

If no tag is specified, shows the latest release.

Examples:
  dex gh release view
  dex gh release view v1.0.0
  dex gh release view v1.0.0 --repo owner/repo`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		tag := ""
		if len(args) > 0 {
			tag = args[0]
		}

		repo, _ := cmd.Flags().GetString("repo")

		release, err := client.ReleaseView(tag, repo)
		if err != nil {
			return err
		}

		name := release.Name
		if name == "" {
			name = release.TagName
		}
		fmt.Printf("%s - %s\n", release.TagName, name)

		status := "published"
		if release.IsDraft {
			status = "draft"
		} else if release.IsPrerelease {
			status = "prerelease"
		}

		date := release.PublishedAt
		if date != "" && len(date) >= 10 {
			date = date[:10]
		}

		fmt.Printf("Status: %s | Author: @%s | Published: %s\n", status, release.Author, date)
		fmt.Printf("URL: %s\n", release.URL)

		if release.Body != "" {
			fmt.Printf("\n%s\n", release.Body)
		}

		return nil
	},
}

var ghReleaseCreateCmd = &cobra.Command{
	Use:   "create <tag>",
	Short: "Create a new release",
	Long: `Create a new GitHub release.

If the tag doesn't exist, it will be created from the default branch.

Examples:
  dex gh release create v1.0.0 --notes "First release"
  dex gh release create v1.0.0 --generate-notes
  dex gh release create v1.0.0 --notes-file CHANGELOG.md
  dex gh release create v1.0.0 --draft
  dex gh release create v1.0.0 --prerelease --title "Beta Release"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		tag := args[0]
		title, _ := cmd.Flags().GetString("title")
		notes, _ := cmd.Flags().GetString("notes")
		notesFile, _ := cmd.Flags().GetString("notes-file")
		generateNotes, _ := cmd.Flags().GetBool("generate-notes")
		draft, _ := cmd.Flags().GetBool("draft")
		prerelease, _ := cmd.Flags().GetBool("prerelease")
		latest, _ := cmd.Flags().GetString("latest")
		target, _ := cmd.Flags().GetString("target")
		repo, _ := cmd.Flags().GetString("repo")

		// Validate: need either notes, notes-file, or generate-notes
		if notes == "" && notesFile == "" && !generateNotes {
			return fmt.Errorf("one of --notes, --notes-file, or --generate-notes is required")
		}

		opts := gh.ReleaseCreateOptions{
			Tag:           tag,
			Title:         title,
			Notes:         notes,
			NotesFile:     notesFile,
			GenerateNotes: generateNotes,
			Draft:         draft,
			Prerelease:    prerelease,
			Target:        target,
			Repo:          repo,
		}

		// Handle --latest flag
		if latest != "" {
			latestBool := latest == "true"
			opts.Latest = &latestBool
		}

		release, err := client.ReleaseCreate(opts)
		if err != nil {
			return err
		}

		fmt.Printf("Created release %s: %s\n", release.TagName, release.URL)
		return nil
	},
}

// Repo commands
var ghRepoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage GitHub repositories",
	Long:  `Create and manage GitHub repositories.`,
}

var ghRepoCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new repository",
	Long: `Create a new GitHub repository.

The name can be just a repo name (creates under your account) or owner/name format.

Examples:
  dex gh repo create my-new-repo --public
  dex gh repo create my-org/my-repo --private
  dex gh repo create my-repo --private --description "My awesome project"
  dex gh repo create my-repo --public --clone
  dex gh repo create my-repo --public --gitignore Go --license MIT
  dex gh repo create my-repo --public --add-readme`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsAvailable() {
			return fmt.Errorf("gh CLI is not available or not authenticated. Run 'dex gh auth' first")
		}

		name := args[0]
		description, _ := cmd.Flags().GetString("description")
		private, _ := cmd.Flags().GetBool("private")
		public, _ := cmd.Flags().GetBool("public")
		internal, _ := cmd.Flags().GetBool("internal")
		clone, _ := cmd.Flags().GetBool("clone")
		source, _ := cmd.Flags().GetString("source")
		gitignore, _ := cmd.Flags().GetString("gitignore")
		license, _ := cmd.Flags().GetString("license")
		addReadme, _ := cmd.Flags().GetBool("add-readme")
		disableWiki, _ := cmd.Flags().GetBool("disable-wiki")
		disableIssues, _ := cmd.Flags().GetBool("disable-issues")

		// Require at least one visibility flag
		if !private && !public && !internal {
			return fmt.Errorf("visibility required: --public, --private, or --internal")
		}

		repo, err := client.RepoCreate(gh.RepoCreateOptions{
			Name:          name,
			Description:   description,
			Private:       private,
			Public:        public,
			Internal:      internal,
			Clone:         clone,
			Source:        source,
			GitIgnore:     gitignore,
			License:       license,
			AddReadme:     addReadme,
			DisableWiki:   disableWiki,
			DisableIssues: disableIssues,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created repository: %s\n", repo.URL)
		if clone {
			fmt.Println("Cloned to local directory")
		}
		return nil
	},
}

var ghTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test GitHub CLI authentication",
	Long: `Test that the gh CLI is installed and authenticated.

Examples:
  dex gh test`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gh.NewClient()

		if !client.IsInstalled() {
			return fmt.Errorf("gh CLI is not installed. Install from https://cli.github.com/")
		}

		status, err := client.GetAuthStatus()
		if err != nil {
			return fmt.Errorf("gh CLI is not authenticated. Run 'gh auth login' to authenticate")
		}

		fmt.Printf("Authenticated as @%s\n", status.Username)
		return nil
	},
}

func init() {
	// Issue list flags
	ghIssueListCmd.Flags().StringP("state", "s", "", "Filter by state: open, closed, all (default: open)")
	ghIssueListCmd.Flags().StringP("label", "l", "", "Filter by label")
	ghIssueListCmd.Flags().Bool("no-label", false, "Filter issues with no labels")
	ghIssueListCmd.Flags().StringP("assignee", "a", "", "Filter by assignee")
	ghIssueListCmd.Flags().IntP("limit", "L", 30, "Maximum number of issues to fetch")
	ghIssueListCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue view flags
	ghIssueViewCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue create flags
	ghIssueCreateCmd.Flags().StringP("title", "t", "", "Issue title (required)")
	ghIssueCreateCmd.Flags().StringP("body", "b", "", "Issue body")
	ghIssueCreateCmd.Flags().StringSliceP("label", "l", nil, "Labels to add")
	ghIssueCreateCmd.Flags().StringP("assignee", "a", "", "Assignee")
	ghIssueCreateCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue close flags
	ghIssueCloseCmd.Flags().StringP("comment", "c", "", "Leave a closing comment")
	ghIssueCloseCmd.Flags().StringP("reason", "r", "", "Reason for closing: completed, not planned")
	ghIssueCloseCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue comment flags
	ghIssueCommentCmd.Flags().StringP("body", "b", "", "Comment body (required)")
	ghIssueCommentCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Issue edit flags
	ghIssueEditCmd.Flags().StringSliceP("add-label", "a", nil, "Labels to add (can be repeated)")
	ghIssueEditCmd.Flags().StringSliceP("remove-label", "r", nil, "Labels to remove (can be repeated)")
	ghIssueEditCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Add issue subcommands
	ghIssueCmd.AddCommand(ghIssueCloseCmd)
	ghIssueCmd.AddCommand(ghIssueCommentCmd)
	ghIssueCmd.AddCommand(ghIssueCreateCmd)
	ghIssueCmd.AddCommand(ghIssueEditCmd)
	ghIssueCmd.AddCommand(ghIssueListCmd)
	ghIssueCmd.AddCommand(ghIssueViewCmd)

	// Release list flags
	ghReleaseListCmd.Flags().IntP("limit", "L", 30, "Maximum number of releases to fetch")
	ghReleaseListCmd.Flags().Bool("exclude-drafts", false, "Exclude draft releases")
	ghReleaseListCmd.Flags().Bool("exclude-pre-releases", false, "Exclude pre-releases")
	ghReleaseListCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Release view flags
	ghReleaseViewCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Release create flags
	ghReleaseCreateCmd.Flags().StringP("title", "t", "", "Release title")
	ghReleaseCreateCmd.Flags().StringP("notes", "n", "", "Release notes")
	ghReleaseCreateCmd.Flags().StringP("notes-file", "F", "", "Read release notes from file")
	ghReleaseCreateCmd.Flags().Bool("generate-notes", false, "Automatically generate release notes")
	ghReleaseCreateCmd.Flags().BoolP("draft", "d", false, "Save as draft instead of publishing")
	ghReleaseCreateCmd.Flags().BoolP("prerelease", "p", false, "Mark as prerelease")
	ghReleaseCreateCmd.Flags().String("latest", "", "Mark as latest release (true/false)")
	ghReleaseCreateCmd.Flags().String("target", "", "Target branch or commit SHA")
	ghReleaseCreateCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Add release subcommands
	ghReleaseCmd.AddCommand(ghReleaseCreateCmd)
	ghReleaseCmd.AddCommand(ghReleaseListCmd)
	ghReleaseCmd.AddCommand(ghReleaseViewCmd)

	// Label list flags
	ghLabelListCmd.Flags().IntP("limit", "L", 30, "Maximum number of labels to fetch")
	ghLabelListCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Label create flags
	ghLabelCreateCmd.Flags().StringP("color", "c", "", "Label color (hex without #)")
	ghLabelCreateCmd.Flags().StringP("description", "d", "", "Label description")
	ghLabelCreateCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Label delete flags
	ghLabelDeleteCmd.Flags().StringP("repo", "R", "", "Repository in owner/repo format")

	// Add label subcommands
	ghLabelCmd.AddCommand(ghLabelListCmd)
	ghLabelCmd.AddCommand(ghLabelCreateCmd)
	ghLabelCmd.AddCommand(ghLabelDeleteCmd)

	// Repo create flags
	ghRepoCreateCmd.Flags().StringP("description", "d", "", "Repository description")
	ghRepoCreateCmd.Flags().Bool("public", false, "Make the repository public")
	ghRepoCreateCmd.Flags().Bool("private", false, "Make the repository private")
	ghRepoCreateCmd.Flags().Bool("internal", false, "Make the repository internal (organization only)")
	ghRepoCreateCmd.Flags().BoolP("clone", "c", false, "Clone the repository locally after creation")
	ghRepoCreateCmd.Flags().StringP("source", "s", "", "Path to local source to push")
	ghRepoCreateCmd.Flags().String("gitignore", "", "Gitignore template (e.g., Go, Node, Python)")
	ghRepoCreateCmd.Flags().StringP("license", "l", "", "License template (e.g., MIT, Apache-2.0)")
	ghRepoCreateCmd.Flags().Bool("add-readme", false, "Add a README file")
	ghRepoCreateCmd.Flags().Bool("disable-wiki", false, "Disable wiki for the repository")
	ghRepoCreateCmd.Flags().Bool("disable-issues", false, "Disable issues for the repository")

	// Add repo subcommands
	ghRepoCmd.AddCommand(ghRepoCreateCmd)

	ghCmd.AddCommand(ghAuthCmd)
	ghCmd.AddCommand(ghCloneCmd)
	ghCmd.AddCommand(ghIssueCmd)
	ghCmd.AddCommand(ghLabelCmd)
	ghCmd.AddCommand(ghReleaseCmd)
	ghCmd.AddCommand(ghRepoCmd)
	ghCmd.AddCommand(ghTestCmd)
	rootCmd.AddCommand(ghCmd)
}
