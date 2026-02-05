package skills

import (
	"embed"
)

//go:embed dex/SKILL.md
var content embed.FS

// DexSkill returns the embedded dex skill content
func DexSkill() ([]byte, error) {
	return content.ReadFile("dex/SKILL.md")
}
