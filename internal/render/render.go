package render

// Mode controls the verbosity of text rendering.
type Mode int

const (
	ModeNormal  Mode = iota // full detail (default)
	ModeCompact             // condensed single-line or minimal text
)

// Renderable is implemented by all result structs that support multi-format output.
type Renderable interface {
	RenderText(mode Mode) string
}
