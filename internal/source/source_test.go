package source_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tenntenn/sbnn/internal/model"
	"github.com/tenntenn/sbnn/internal/source"
)

// A diff is text sbnn did not write. A path that climbs out of the directory
// the diff was sent from is refused, because whatever is read here is shown
// in the preview and baked into exported pages.
func TestAbsPathRefusesToLeaveTheBaseDir(t *testing.T) {
	base := t.TempDir()
	inside := filepath.Join(base, "docs", "guide.md")

	cases := []struct {
		name string
		rel  string
		want string
	}{
		{"an ordinary path", "docs/guide.md", inside},
		{"a path that stays inside after a detour", "docs/../docs/guide.md", inside},
		{"a path that climbs out", "../../../../etc/passwd", ""},
		{"a path that climbs out and back into a sibling", "../other/secret.md", ""},
		{"an absolute path elsewhere", "/etc/passwd", ""},
		{"the base itself", ".", base},
		{"nothing", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && tc.rel == "/etc/passwd" {
				t.Skip("absolute paths are spelled differently here")
			}
			if got := source.AbsPath(base, tc.rel); got != tc.want {
				t.Errorf("AbsPath(%q) = %q, want %q", tc.rel, got, tc.want)
			}
		})
	}
}

// Without a base directory nothing is read from disk at all.
func TestAbsPathNeedsABaseDir(t *testing.T) {
	if got := source.AbsPath("", "/etc/passwd"); got != "" {
		t.Errorf("AbsPath = %q, want nothing", got)
	}
}

// A refused path is not an error the reader sees: the new side is rebuilt
// from the diff, which is what happens for any file that is not on disk.
func TestNewSideFallsBackWhenThePathEscapes(t *testing.T) {
	base := t.TempDir()
	secret := filepath.Join(filepath.Dir(base), "secret.txt")
	if err := os.WriteFile(secret, []byte("token=hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(secret) })

	got := source.NewSide(base, &model.File{NewPath: "../" + filepath.Base(secret)})
	if got.Kind != source.FromDiff {
		t.Errorf("kind = %q, want the diff to be the source", got.Kind)
	}
	if got.Content == "token=hunter2\n" {
		t.Error("the file outside the base directory was read anyway")
	}
}
