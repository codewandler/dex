package gitlab

import (
	"regexp"
	"strconv"
	"strings"
)

// LineType represents the type of line in a diff
type LineType int

const (
	LineContext LineType = iota // Unchanged line (space prefix)
	LineAdded                   // Added line (+ prefix)
	LineDeleted                 // Deleted line (- prefix)
)

// DiffLine represents a line in a parsed diff
type DiffLine struct {
	Type    LineType
	OldLine int    // 0 if line doesn't exist in old version (added lines)
	NewLine int    // 0 if line doesn't exist in new version (deleted lines)
	Content string // Line content without the prefix
}

// ParsedDiff contains parsed diff information for a file
type ParsedDiff struct {
	OldPath string
	NewPath string
	Lines   []DiffLine
}

// hunkHeaderRegex matches unified diff hunk headers like "@@ -10,5 +12,8 @@"
var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// ParseUnifiedDiff parses a unified diff string into structured data
func ParseUnifiedDiff(diff string) *ParsedDiff {
	parsed := &ParsedDiff{}
	lines := strings.Split(diff, "\n")

	var oldLine, newLine int

	for _, line := range lines {
		// Parse hunk header to get starting line numbers
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			oldLine, _ = strconv.Atoi(matches[1])
			newLine, _ = strconv.Atoi(matches[2])
			continue
		}

		// Skip file headers and empty lines at the start
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") {
			continue
		}

		// Skip empty lines outside of hunks
		if oldLine == 0 && newLine == 0 {
			continue
		}

		if len(line) == 0 {
			// Empty line in diff is treated as context line
			parsed.Lines = append(parsed.Lines, DiffLine{
				Type:    LineContext,
				OldLine: oldLine,
				NewLine: newLine,
				Content: "",
			})
			oldLine++
			newLine++
			continue
		}

		prefix := line[0]
		content := ""
		if len(line) > 1 {
			content = line[1:]
		}

		switch prefix {
		case '+':
			parsed.Lines = append(parsed.Lines, DiffLine{
				Type:    LineAdded,
				OldLine: 0,
				NewLine: newLine,
				Content: content,
			})
			newLine++
		case '-':
			parsed.Lines = append(parsed.Lines, DiffLine{
				Type:    LineDeleted,
				OldLine: oldLine,
				NewLine: 0,
				Content: content,
			})
			oldLine++
		case ' ':
			parsed.Lines = append(parsed.Lines, DiffLine{
				Type:    LineContext,
				OldLine: oldLine,
				NewLine: newLine,
				Content: content,
			})
			oldLine++
			newLine++
		case '\\':
			// "\ No newline at end of file" - skip
			continue
		default:
			// Treat unknown prefix as context (handles edge cases)
			parsed.Lines = append(parsed.Lines, DiffLine{
				Type:    LineContext,
				OldLine: oldLine,
				NewLine: newLine,
				Content: line,
			})
			oldLine++
			newLine++
		}
	}

	return parsed
}

// FindLineByNew finds a line by its new line number
// Returns the DiffLine and whether it was found
func (p *ParsedDiff) FindLineByNew(newLine int) (*DiffLine, bool) {
	for i := range p.Lines {
		if p.Lines[i].NewLine == newLine {
			return &p.Lines[i], true
		}
	}
	return nil, false
}

// FindLineByOld finds a line by its old line number
// Returns the DiffLine and whether it was found
func (p *ParsedDiff) FindLineByOld(oldLine int) (*DiffLine, bool) {
	for i := range p.Lines {
		if p.Lines[i].OldLine == oldLine {
			return &p.Lines[i], true
		}
	}
	return nil, false
}
