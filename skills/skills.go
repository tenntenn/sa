// Package skills embeds the agent skill shipped with sa.
//
// The skill is plain Markdown with YAML front matter and describes the sa
// workflow in terms of its command line only, so it works with any coding
// agent that can read a skill file: copy it wherever your agent looks for
// skills, or point at it from AGENTS.md.
package skills

import (
	"embed"
	"io/fs"
)

//go:embed all:sa
var files embed.FS

// Name is the directory name of the skill.
const Name = "sa"

// FS returns the embedded skill tree, rooted at the directory holding the
// skill (so paths look like "sa/SKILL.md").
func FS() fs.FS { return files }

// Markdown returns the content of SKILL.md.
func Markdown() ([]byte, error) { return files.ReadFile(Name + "/SKILL.md") }
