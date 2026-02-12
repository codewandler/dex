# GitLab Commands (`dex gl`)

Aliases: `gitlab`

## Activity
```bash
dex gl activity                   # Show activity from last 14 days
dex gl activity --since 7d        # Activity from last 7 days
dex gl activity --since 4h        # Activity from last 4 hours
```

## Project Index
```bash
dex gl index                      # Index all accessible projects (cached 24h)
dex gl index --force              # Force re-index
```

Index stored at `~/.dex/gitlab/index.json`.

## Projects
```bash
dex gl proj ls                    # List projects (from index)
dex gl proj ls -n 50              # List 50 projects
dex gl proj ls --sort name        # Sort by name (also: created, activity, path)
dex gl proj ls --sort created -d  # Sort descending (default for dates, ascending for names)
dex gl proj ls --no-cache         # Fetch from API instead of index
dex gl proj show <id|path>        # Show project details
dex gl proj show <id> --no-cache  # Always fetch from API, bypass cache
```

## Commits

### List Commits
```bash
dex gl commit ls <project>                  # List recent commits (default: 20, last 14d)
dex gl commit ls group/proj --since 7d      # Commits from last 7 days
dex gl commit ls group/proj --branch main   # Filter by branch
dex gl commit ls group/proj -b develop      # Short flag
dex gl commit ls group/proj -n 50           # Show 50 commits
dex gl commit ls 742 --since 3d -n 10       # By project ID, combined flags
```

### Show Commit
```bash
dex gl commit show <project> <sha>   # Show full commit details (message body, stats)
dex gl commit show 742 95a1e625      # By project ID
dex gl commit show group/proj abc123 # By project path
```

## Merge Requests

### List MRs
```bash
dex gl mr ls                         # List open MRs (excludes WIP/drafts)
dex gl mr ls -n 50                   # List 50 MRs
dex gl mr ls --state merged          # List merged MRs
dex gl mr ls --state closed          # List closed MRs
dex gl mr ls --state all             # All MRs regardless of state
dex gl mr ls --scope created_by_me   # MRs you created
dex gl mr ls --scope assigned_to_me  # MRs assigned to you
dex gl mr ls --include-wip           # Include WIP/draft MRs
dex gl mr ls --conflicts-only        # Only show MRs with merge conflicts
```

### Show MR Details
```bash
dex gl mr show <project!iid>         # Show full MR details with discussion IDs
dex gl mr show my-group/my-project!123         # Example
dex gl mr show my-group/my-project!123 --show-diff  # Include file diffs in output
```

### View File Diff
```bash
dex gl mr diff <project!iid>              # List all changed files in MR
dex gl mr diff proj!123 --file path       # Show raw diff for specific file
dex gl mr diff proj!123 -f src/main.go    # Short flag
dex gl mr diff proj!123 -f path --parsed  # Show with line number columns
dex gl mr diff proj!123 -f path -p        # Short flag for --parsed
```

The `--parsed` flag shows a table with explicit line numbers:
```
    new    old  type  content
    ---    ---  ----  --------
    606    606  ctx       createWidget: 'Create Widget',
    607    607  ctx       selectWidget: 'Select existing widget',
    608      -  add       removeWidget: 'Remove widget from board',
    609    608  ctx       widgetManagement: 'Manage widgets',
```

Use this to inspect diff contents before adding inline comments. The `new` column shows the line number to use with `--line`.

### Inspect Specific Lines
```bash
dex gl mr diff proj!123 -f path --line 42     # Inspect line 42 with context
dex gl mr diff proj!123 -f path -l 42         # Short flag
dex gl mr diff proj!123 -f path -l 42 -C 5    # Show 5 context lines (default: 3)
```

Shows the target line highlighted with surrounding context, and explains:
- Line type (add/del/ctx)
- Old and new line numbers
- How to use this line for inline comments

### Search Lines in Diff
```bash
dex gl mr diff proj!123 -f path --search "TODO"      # Find lines containing "TODO"
dex gl mr diff proj!123 -f path -s "error|warn"      # Regex pattern support
dex gl mr diff proj!123 -f path -s "function.*init"  # Complex patterns
```

Returns all matching lines with their line numbers and types, useful for finding where to add comments.

### Open in Browser
```bash
dex gl mr open <project!iid>         # Open MR in default browser
dex gl mr open sbf/services!2483     # Example
```

### Comments
```bash
dex gl mr comment <project!iid> "message"  # Add a comment
dex gl mr comment sbf/services!2483 "LGTM"  # Example
echo "Long comment" | dex gl mr comment sbf/services!2483 -  # From stdin

# Reply to discussion thread (use discussion ID from mr show output)
dex gl mr comment <project!iid> "reply" --reply-to <discussion-id>
dex gl mr comment proj!123 "Fixed!" --reply-to abc123def456...

# Add inline comment on file/line (requires line to be in the diff)
dex gl mr comment <project!iid> "comment" --file <path> --line <n>
dex gl mr comment proj!123 "Use a constant" --file src/main.go --line 42

# Preview inline comment location (dry run)
dex gl mr comment proj!123 "test" --file src/main.go --line 42 --dry-run
```

The `--dry-run` flag validates and previews where an inline comment will land:
- Shows the target line with context
- Validates the line is in the diff
- Warns on deleted lines or empty lines
- Shows the exact parameters that will be used

Use `--dry-run` before posting to avoid errors from invalid line numbers.

**Inline comment tips:**
- The `--line` number is the NEW file line number (right side of diff)
- You can only comment on lines that appear in the diff hunks
- Use `dex gl mr diff <ref> -f <path> -l <n>` to inspect specific lines
- Use `dex gl mr diff <ref> -f <path> -s "pattern"` to find lines by content
- Errors now include helpful diagnostics showing available line ranges
```

### Reactions
```bash
dex gl mr react <project!iid> <emoji>           # React to MR
dex gl mr react proj!123 thumbsup               # Example
dex gl mr react proj!123 :heart:                # Colons optional
dex gl mr react proj!123 rocket --note <id>     # React to specific note/comment
```

### MR Lifecycle
```bash
dex gl mr close <project!iid>                   # Close a merge request
dex gl mr close proj!123 --reason "No longer needed"  # Close with comment
dex gl mr reopen <project!iid>                  # Reopen a closed merge request
dex gl mr reopen proj!123 --reason "Re-opening for further work"  # Reopen with comment
dex gl mr approve <project!iid>                 # Approve a merge request
dex gl mr merge <project!iid>                   # Merge a merge request
dex gl mr merge proj!123 --squash               # Squash commits
dex gl mr merge proj!123 --remove-source-branch # Delete branch after merge
dex gl mr merge proj!123 --when-pipeline-succeeds  # Merge when CI passes
dex gl mr merge proj!123 -m "Custom message"    # Custom merge commit message
```

### Create MR
```bash
dex gl mr create "<title>"                      # Create MR from current branch to main
dex gl mr create "Fix bug" --target develop     # Specify target branch
dex gl mr create "WIP" --draft                  # Create as draft
dex gl mr create "Feature" -d "Description"     # With description
dex gl mr create "Feature" --squash             # Squash on merge
dex gl mr create "Feature" --remove-source-branch  # Delete branch after merge
dex gl mr create "Feature" -p group/proj -s branch # Explicit project and source
```

## Pipelines

### List Pipelines
```bash
dex gl pipeline ls <project>                        # List recent pipelines (default 20)
dex gl pipeline ls group/proj -n 50                 # Show 50 pipelines
dex gl pipeline ls group/proj --status failed       # Only failed pipelines
dex gl pipeline ls group/proj --ref main            # Only pipelines on main branch
dex gl pipeline ls group/proj --source schedule     # Only scheduled pipelines
```

Aliases: `pipe`, `pl` (e.g., `dex gl pipe ls group/proj`)

### Show Pipeline
```bash
dex gl pipeline show <project> <pipeline-id>        # Show pipeline details with jobs
dex gl pipeline show group/proj 12345 --no-jobs     # Details without jobs
```

### List Pipeline Jobs
```bash
dex gl pipeline jobs <project> <pipeline-id>        # List all jobs grouped by stage
dex gl pipeline jobs group/proj 12345 --scope failed  # Only failed jobs
```

### Retry Pipeline
```bash
dex gl pipeline retry <project> <pipeline-id>       # Retry failed jobs
```

### Cancel Pipeline
```bash
dex gl pipeline cancel <project> <pipeline-id>      # Cancel running jobs
```

### Create Pipeline
```bash
dex gl pipeline create <project> --ref <branch>     # Trigger pipeline on branch/tag
dex gl pipeline create group/proj --ref main         # Run on main
dex gl pipeline create group/proj --ref main --var DEPLOY_ENV=staging  # With variables
dex gl pipeline create group/proj --ref v1.0 --var K1=v1 --var K2=v2  # Multiple variables
```

### View Job Logs
```bash
dex gl pipeline logs <project> <pipeline-id> <job-name>  # Show job console output
dex gl pipeline logs group/proj 12345 "build with buildkit"  # Job with spaces
dex gl pipeline logs group/proj 12345 test               # Simple job name
```

Use `dex gl pipeline jobs <project> <pipeline-id>` to see available job names first.

## Tips

- GitLab project names autocomplete from local index
- Command aliases: `gl`=`gitlab`, `mr`=`merge-request`, `pipeline`=`pipe`=`pl`
- Use `project!iid` format for MR references (e.g., `my-group/my-project!123`)
