package gitlab

import (
	"testing"
)

func TestParseUnifiedDiff_BasicAddedLines(t *testing.T) {
	diff := `@@ -10,3 +10,5 @@
 context line 1
+added line 1
+added line 2
 context line 2
 context line 3`

	parsed := ParseUnifiedDiff(diff)

	if len(parsed.Lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(parsed.Lines))
	}

	// First context line
	if parsed.Lines[0].Type != LineContext {
		t.Errorf("line 0: expected LineContext, got %d", parsed.Lines[0].Type)
	}
	if parsed.Lines[0].OldLine != 10 || parsed.Lines[0].NewLine != 10 {
		t.Errorf("line 0: expected old=10 new=10, got old=%d new=%d", parsed.Lines[0].OldLine, parsed.Lines[0].NewLine)
	}

	// First added line
	if parsed.Lines[1].Type != LineAdded {
		t.Errorf("line 1: expected LineAdded, got %d", parsed.Lines[1].Type)
	}
	if parsed.Lines[1].OldLine != 0 || parsed.Lines[1].NewLine != 11 {
		t.Errorf("line 1: expected old=0 new=11, got old=%d new=%d", parsed.Lines[1].OldLine, parsed.Lines[1].NewLine)
	}

	// Second added line
	if parsed.Lines[2].Type != LineAdded {
		t.Errorf("line 2: expected LineAdded, got %d", parsed.Lines[2].Type)
	}
	if parsed.Lines[2].OldLine != 0 || parsed.Lines[2].NewLine != 12 {
		t.Errorf("line 2: expected old=0 new=12, got old=%d new=%d", parsed.Lines[2].OldLine, parsed.Lines[2].NewLine)
	}

	// Remaining context lines
	if parsed.Lines[3].OldLine != 11 || parsed.Lines[3].NewLine != 13 {
		t.Errorf("line 3: expected old=11 new=13, got old=%d new=%d", parsed.Lines[3].OldLine, parsed.Lines[3].NewLine)
	}
	if parsed.Lines[4].OldLine != 12 || parsed.Lines[4].NewLine != 14 {
		t.Errorf("line 4: expected old=12 new=14, got old=%d new=%d", parsed.Lines[4].OldLine, parsed.Lines[4].NewLine)
	}
}

func TestParseUnifiedDiff_DeletedLines(t *testing.T) {
	diff := `@@ -5,4 +5,2 @@
 context
-deleted line 1
-deleted line 2
 context`

	parsed := ParseUnifiedDiff(diff)

	if len(parsed.Lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(parsed.Lines))
	}

	// First deleted line
	if parsed.Lines[1].Type != LineDeleted {
		t.Errorf("line 1: expected LineDeleted, got %d", parsed.Lines[1].Type)
	}
	if parsed.Lines[1].OldLine != 6 || parsed.Lines[1].NewLine != 0 {
		t.Errorf("line 1: expected old=6 new=0, got old=%d new=%d", parsed.Lines[1].OldLine, parsed.Lines[1].NewLine)
	}

	// Second deleted line
	if parsed.Lines[2].Type != LineDeleted {
		t.Errorf("line 2: expected LineDeleted, got %d", parsed.Lines[2].Type)
	}
	if parsed.Lines[2].OldLine != 7 || parsed.Lines[2].NewLine != 0 {
		t.Errorf("line 2: expected old=7 new=0, got old=%d new=%d", parsed.Lines[2].OldLine, parsed.Lines[2].NewLine)
	}

	// Context after deletions
	if parsed.Lines[3].OldLine != 8 || parsed.Lines[3].NewLine != 6 {
		t.Errorf("line 3: expected old=8 new=6, got old=%d new=%d", parsed.Lines[3].OldLine, parsed.Lines[3].NewLine)
	}
}

func TestParseUnifiedDiff_MixedChanges(t *testing.T) {
	diff := `@@ -1,5 +1,5 @@
 line 1
-old line 2
+new line 2
 line 3
-old line 4
+new line 4
 line 5`

	parsed := ParseUnifiedDiff(diff)

	if len(parsed.Lines) != 7 {
		t.Fatalf("expected 7 lines, got %d", len(parsed.Lines))
	}

	// Context line 1
	if parsed.Lines[0].OldLine != 1 || parsed.Lines[0].NewLine != 1 {
		t.Errorf("line 0: expected old=1 new=1, got old=%d new=%d", parsed.Lines[0].OldLine, parsed.Lines[0].NewLine)
	}

	// Deleted old line 2
	if parsed.Lines[1].Type != LineDeleted || parsed.Lines[1].OldLine != 2 {
		t.Errorf("line 1: expected deleted old=2, got type=%d old=%d", parsed.Lines[1].Type, parsed.Lines[1].OldLine)
	}

	// Added new line 2
	if parsed.Lines[2].Type != LineAdded || parsed.Lines[2].NewLine != 2 {
		t.Errorf("line 2: expected added new=2, got type=%d new=%d", parsed.Lines[2].Type, parsed.Lines[2].NewLine)
	}

	// Context line 3
	if parsed.Lines[3].OldLine != 3 || parsed.Lines[3].NewLine != 3 {
		t.Errorf("line 3: expected old=3 new=3, got old=%d new=%d", parsed.Lines[3].OldLine, parsed.Lines[3].NewLine)
	}
}

func TestParseUnifiedDiff_MultipleHunks(t *testing.T) {
	diff := `@@ -1,3 +1,4 @@
 line 1
+added in first hunk
 line 2
 line 3
@@ -10,3 +11,4 @@
 line 10
+added in second hunk
 line 11
 line 12`

	parsed := ParseUnifiedDiff(diff)

	// Find the added line in second hunk
	line, found := parsed.FindLineByNew(12)
	if !found {
		t.Fatal("expected to find line at new=12")
	}
	if line.Type != LineAdded {
		t.Errorf("expected LineAdded, got %d", line.Type)
	}
	if line.Content != "added in second hunk" {
		t.Errorf("expected 'added in second hunk', got %q", line.Content)
	}
}

func TestParsedDiff_FindLineByNew(t *testing.T) {
	diff := `@@ -10,3 +10,4 @@
 context
+added
 more context
 even more`

	parsed := ParseUnifiedDiff(diff)

	// Find context line
	line, found := parsed.FindLineByNew(10)
	if !found {
		t.Fatal("expected to find line at new=10")
	}
	if line.Type != LineContext {
		t.Errorf("expected LineContext, got %d", line.Type)
	}
	if line.OldLine != 10 {
		t.Errorf("expected old=10, got %d", line.OldLine)
	}

	// Find added line
	line, found = parsed.FindLineByNew(11)
	if !found {
		t.Fatal("expected to find line at new=11")
	}
	if line.Type != LineAdded {
		t.Errorf("expected LineAdded, got %d", line.Type)
	}
	if line.OldLine != 0 {
		t.Errorf("expected old=0 for added line, got %d", line.OldLine)
	}

	// Line not in diff
	_, found = parsed.FindLineByNew(100)
	if found {
		t.Error("expected to not find line at new=100")
	}
}

func TestParsedDiff_FindLineByOld(t *testing.T) {
	diff := `@@ -5,3 +5,2 @@
 context
-deleted
 more context`

	parsed := ParseUnifiedDiff(diff)

	// Find deleted line
	line, found := parsed.FindLineByOld(6)
	if !found {
		t.Fatal("expected to find line at old=6")
	}
	if line.Type != LineDeleted {
		t.Errorf("expected LineDeleted, got %d", line.Type)
	}
	if line.NewLine != 0 {
		t.Errorf("expected new=0 for deleted line, got %d", line.NewLine)
	}

	// Find context line by old
	line, found = parsed.FindLineByOld(5)
	if !found {
		t.Fatal("expected to find line at old=5")
	}
	if line.Type != LineContext {
		t.Errorf("expected LineContext, got %d", line.Type)
	}
}

func TestParseUnifiedDiff_WithFileHeaders(t *testing.T) {
	diff := `diff --git a/file.go b/file.go
index abc123..def456 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"

 func main() {`

	parsed := ParseUnifiedDiff(diff)

	if len(parsed.Lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(parsed.Lines))
	}

	// Verify the import line was parsed correctly
	line, found := parsed.FindLineByNew(2)
	if !found {
		t.Fatal("expected to find line at new=2")
	}
	if line.Type != LineAdded {
		t.Errorf("expected LineAdded, got %d", line.Type)
	}
	if line.Content != `import "fmt"` {
		t.Errorf("expected 'import \"fmt\"', got %q", line.Content)
	}
}

func TestParseUnifiedDiff_NoNewlineAtEOF(t *testing.T) {
	diff := `@@ -1,2 +1,3 @@
 line 1
+added line
 line 2
\ No newline at end of file`

	parsed := ParseUnifiedDiff(diff)

	if len(parsed.Lines) != 3 {
		t.Fatalf("expected 3 lines (excluding no-newline marker), got %d", len(parsed.Lines))
	}
}

func TestParseUnifiedDiff_EmptyDiff(t *testing.T) {
	diff := ``

	parsed := ParseUnifiedDiff(diff)

	if len(parsed.Lines) != 0 {
		t.Fatalf("expected 0 lines for empty diff, got %d", len(parsed.Lines))
	}
}
