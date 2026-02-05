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
dex gl mr show sre/helm!2903         # Example
dex gl mr show sre/helm!2903 --show-diff  # Include file diffs in output
```

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

# Add inline comment on file/line
dex gl mr comment <project!iid> "comment" --file <path> --line <n>
dex gl mr comment proj!123 "Use a constant" --file src/main.go --line 42
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

## Tips

- GitLab project names autocomplete from local index
- Command aliases: `gl`=`gitlab`, `mr`=`merge-request`
- Use `project!iid` format for MR references (e.g., `sre/helm!2903`)
