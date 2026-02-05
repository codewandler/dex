package skills

import (
	"embed"
	"io/fs"
)

//go:embed dex/*
var content embed.FS

// DexSkill returns the embedded dex skill content (main SKILL.md)
func DexSkill() ([]byte, error) {
	return content.ReadFile("dex/SKILL.md")
}

// DexSkillFS returns the embedded filesystem rooted at dex/
func DexSkillFS() (fs.FS, error) {
	return fs.Sub(content, "dex")
}
