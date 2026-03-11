package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/codewandler/dex/internal/render"
	"sigs.k8s.io/yaml"
)

// Render writes a Renderable result to stdout in the format requested via -o/--output.
func Render(r render.Renderable) {
	switch outputFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			RenderError(err)
		}
	case "yaml":
		out, err := yaml.Marshal(r)
		if err != nil {
			RenderError(err)
		}
		fmt.Print(string(out))
	case "compact":
		fmt.Print(r.RenderText(render.ModeCompact))
	case "text", "":
		fmt.Print(r.RenderText(render.ModeNormal))
	default:
		RenderError(fmt.Errorf("unsupported output format: %s", outputFormat))
	}
}

// RenderError writes an error in the requested format and exits 1.
// When -o json or -o yaml, the error goes to stdout so it can be piped/parsed.
func RenderError(err error) {
	type errorResult struct {
		Error string `json:"error"`
	}
	switch outputFormat {
	case "json":
		_ = json.NewEncoder(os.Stdout).Encode(errorResult{err.Error()})
	case "yaml":
		out, _ := yaml.Marshal(errorResult{err.Error()})
		fmt.Print(string(out))
	default:
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(1)
}
