# Wishlist: GitLab MR Diff Inspection Tools

These features would help AI agents (and humans) debug inline comment issues by inspecting diff contents before posting comments.

## Background

When adding inline comments to MRs, users need to specify the correct file and line number. Currently there's no way to:
1. Inspect what content is at a specific line in the diff
2. Verify a line is within the diff before commenting
3. Search for specific code within a diff
4. Preview where a comment will land

This led to debugging sessions where throwaway Go code had to be written just to inspect diff contents.

---

## Feature 1: `dex gl mr diff` - File-specific diff inspection

### Basic usage
```bash
dex gl mr diff <project!iid> --file <path>
```

Show the diff for a single file instead of all files. Currently `dex gl mr show --show-diff` shows all files which is noisy when you only care about one.

### Implementation notes
- Add new subcommand `dex gl mr diff`
- Reuse existing `GetMergeRequestChanges()` with `includeDiff=true`
- Filter to the specified file path
- Output the raw diff content

### Files to modify
- `internal/cli/gitlab.go` - Add `mr diff` subcommand
- No new API calls needed, reuses existing functionality

---

## Feature 2: `--parsed` flag for diff command

### Usage
```bash
dex gl mr diff <project!iid> --file <path> --parsed
```

### Expected output
```
src/components/Listing/CardWidgetList.vue (190 lines in new version)

@@ -1,95 +1,190 @@
  new   old  type  content
  ---   ---  ----  -------
    1     1  ctx   <template>
    2     -  add     <div data-cy="audit-card" class="audit-card">
    -     2  del       <div data-cy="audit-card" class="audit-card">
    3     -  add       <b-row>
  ...
  155    -  add         const babelDeskLabelList = this.getBabeldesksByWidget(widgetId)
  156   94  ctx
  157    -  add         // If widget is not used in any babelDesk, allow delete
  ...
```

### Implementation notes
- Use the existing `ParseUnifiedDiff()` from `internal/gitlab/diffparse.go`
- Format output as a table showing both line numbers and line type
- Color-code: green for added, red for deleted, white for context

### Files to modify
- `internal/cli/gitlab.go` - Add `--parsed` flag to `mr diff`
- `internal/output/` - Add diff formatting helper

---

## Feature 3: `--line` flag to inspect specific line

### Usage
```bash
dex gl mr diff <project!iid> --file <path> --line 161
```

### Expected output
```
Line 161 in src/components/Listing/CardWidgetList.vue:

  Type:     added (only exists in new version)
  Content:  (empty line)
  Old line: n/a
  New line: 161

Context (lines 158-164):
  158  add  |       if (!babelDeskLabelList.length) {
  159  add  |         return false
  160  add  |       }
  161  add  |
  162  add  |       // If widget is used in more than one babelDesk, disable delete
  163  add  |       if (babelDeskLabelList.length > 1) {
  164  add  |         return true
```

### Implementation notes
- Use `ParsedDiff.FindLineByNew()` to locate the line
- Show 3 lines of context above and below
- Warn if line is empty or outside the diff

### Files to modify
- `internal/cli/gitlab.go` - Add `--line` flag to `mr diff`

---

## Feature 4: `--search` flag to find lines by content

### Usage
```bash
dex gl mr diff <project!iid> --file <path> --search "allWidgets.find"
```

### Expected output
```
Found 2 matches in src/views/app/manager/babeldesk/index.vue:

  Line 737 [add]: const widget = this.allWidgets.find((wi) => wi.id === widgetId)
  Line 745 [add]: const widget = this.allWidgets.find((wi) => wi.id === widgetId)
```

### Implementation notes
- Iterate through `ParsedDiff.Lines` and match content
- Support regex patterns (use Go's `regexp` package)
- Show line number, type, and matching content

### Files to modify
- `internal/cli/gitlab.go` - Add `--search` flag to `mr diff`

---

## Feature 5: `--dry-run` flag for inline comments

### Usage
```bash
dex gl mr comment proj!123 "msg" --file path.vue --line 161 --dry-run
```

### Expected output (success case)
```
Dry run - would add inline comment:

  File:     src/components/Listing/CardWidgetList.vue
  Line:     161 (new) / n/a (old)
  Type:     added
  Content:  (empty line)

  Comment:  "Consider consistent defensive handling..."

Use without --dry-run to post the comment.
```

### Expected output (warning case)
```
Dry run - would add inline comment:

  File:     src/components/Listing/CardWidgetList.vue
  Line:     161 (new) / n/a (old)
  Type:     added
  Content:  (empty line)

  ⚠️  Warning: This is an empty line. Did you mean a different line?

  Nearby non-empty lines:
    160: }
    162: // If widget is used in more than one babelDesk, disable delete
```

### Implementation notes
- Fetch and parse the diff
- Show what the comment would attach to
- Warn on suspicious targets (empty lines, lines outside diff)
- Do NOT call the GitLab API to create the comment

### Files to modify
- `internal/cli/gitlab.go` - Add `--dry-run` flag to `mr comment`

---

## Feature 6: Better error messages for failed inline comments

### Current behavior
```
Failed to add inline comment: POST .../discussions: 400
{message: 400 Bad request - Note {:line_code=>["can't be blank", "must be a valid line code"]}}
```

### Desired behavior
```
Failed to add inline comment:

  File:   src/components/Listing/CardWidgetList.vue
  Line:   250
  Error:  Line 250 is not in the diff

  The diff for this file covers:
    - Lines 1-190 (new file)

  Hint: Use `dex gl mr diff <ref> --file <path> --parsed` to see available lines.
```

### Implementation notes
- Catch the 400 error in `CreateMergeRequestInlineComment`
- Before returning error, fetch the diff and explain why it failed
- Suggest the correct command to inspect available lines

### Files to modify
- `internal/gitlab/mergerequests.go` - Enhance error handling in `CreateMergeRequestInlineComment`
- `internal/cli/gitlab.go` - Format the enhanced error message

---

## Priority order

1. **Feature 1** (`mr diff --file`) - Foundation for other features
2. **Feature 3** (`--line`) - Most useful for debugging "wrong line" issues
3. **Feature 5** (`--dry-run`) - Prevents mistakes before they happen
4. **Feature 6** (Better errors) - Helps diagnose failures after the fact
5. **Feature 2** (`--parsed`) - Nice for full diff inspection
6. **Feature 4** (`--search`) - Convenience feature

---

## Testing

Each feature should include:
1. Unit tests for any new parsing/formatting logic
2. Integration test with a real MR (can use existing test MRs)
3. Update to `internal/skills/dex/references/gitlab.md` documenting new flags
