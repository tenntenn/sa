package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tenntenn/sa/skills"
)

var (
	skillDir   string
	skillForce bool
	skillList  bool
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print or install the agent skill for sa",
	Long: `Print or install the agent skill bundled with sa.

The skill is vendor neutral: it is a plain Markdown file with YAML front
matter that describes the sa review workflow in terms of the sa command line,
so any coding agent can use it. Install it where your agent looks for skills,
or point at it from AGENTS.md.

  $ sa skill                              # print SKILL.md
  $ sa skill --install .agents/skills     # write .agents/skills/sa/SKILL.md
  $ sa skill --install ~/.claude/skills   # or wherever your agent reads them`,
	Args:         cobra.NoArgs,
	RunE:         runSkill,
	SilenceUsage: true,
}

func init() {
	f := skillCmd.Flags()
	f.StringVar(&skillDir, "install", "", "Directory to install the skill into")
	f.BoolVar(&skillForce, "force", false, "Overwrite existing files")
	f.BoolVar(&skillList, "list", false, "List the files of the skill")
}

func runSkill(_ *cobra.Command, _ []string) error {
	switch {
	case skillList:
		return fs.WalkDir(skills.FS(), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			fmt.Println(path)
			return nil
		})
	case skillDir != "":
		return installSkill(skillDir)
	default:
		md, err := skills.Markdown()
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(md)
		return err
	}
}

// installSkill copies the embedded skill into dir, keeping its directory
// name so that it lands as <dir>/sa/SKILL.md.
func installSkill(dir string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	written := 0
	err = fs.WalkDir(skills.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(root, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !skillForce {
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("%s already exists (pass --force to overwrite)", dst)
			}
		}
		b, err := fs.ReadFile(skills.FS(), path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "sa: wrote", dst)
		written++
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "sa: installed the sa skill (%d file(s)) into %s\n", written, root)
	return nil
}
