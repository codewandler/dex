package statusline

import (
	"bytes"
	"strings"
	"text/template"
)

// RenderSegment renders a segment template with the given data
func RenderSegment(tmplStr string, data map[string]any) (string, error) {
	tmpl, err := template.New("segment").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}

// RenderOutput renders the final output template with all segment outputs
func RenderOutput(format string, segments map[string]string) (string, error) {
	// Convert segments to template-friendly format
	data := make(map[string]string)
	for k, v := range segments {
		// Capitalize first letter for template access (e.g., .K8s, .GitLab)
		key := capitalizeSegmentName(k)
		data[key] = v
	}

	tmpl, err := template.New("output").Parse(format)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}

// capitalizeSegmentName converts segment names to template variable names
func capitalizeSegmentName(name string) string {
	switch name {
	case "k8s":
		return "K8s"
	case "gitlab":
		return "GitLab"
	case "github":
		return "GitHub"
	case "jira":
		return "Jira"
	case "slack":
		return "Slack"
	case "todo":
		return "Todo"
	default:
		if len(name) > 0 {
			return strings.ToUpper(name[:1]) + name[1:]
		}
		return name
	}
}
