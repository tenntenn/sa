package source_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tenntenn/sa/internal/diff"
	"github.com/tenntenn/sa/internal/source"
)

const sample = `diff --git a/internal/server/server.go b/internal/server/server.go
--- a/internal/server/server.go
+++ b/internal/server/server.go
@@ -1,2 +1,2 @@
 package server
-var x = 1
+var x = 2
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-old
+new
`

// checkout builds a tiny tree that looks like the diff's project.
func checkout(t *testing.T, root string) {
	t.Helper()
	for _, p := range []string{"internal/server/server.go", "README.md"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRootFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	checkout(t, root)
	files := diff.Parse(sample)

	// A diff sent from deep inside the checkout still belongs to its root.
	from := filepath.Join(root, "internal", "server")
	if got := source.Root(from, files); got != root {
		t.Errorf("Root(%q) = %q, want %q", from, got, root)
	}
	if got := source.Root(root, files); got != root {
		t.Errorf("Root(%q) = %q, want the root itself", root, got)
	}
}

func TestRootTellsTwoCheckoutsApart(t *testing.T) {
	base := t.TempDir()
	one := filepath.Join(base, "main")
	two := filepath.Join(base, "feature")
	checkout(t, one)
	checkout(t, two)
	files := diff.Parse(sample)

	// Two worktrees of the same project: each diff belongs to its own.
	if got := source.Root(filepath.Join(one, "internal"), files); got != one {
		t.Errorf("Root() = %q, want %q", got, one)
	}
	if got := source.Root(two, files); got != two {
		t.Errorf("Root() = %q, want %q", got, two)
	}
}

func TestRootFallsBackToTheDirectory(t *testing.T) {
	dir := t.TempDir()
	files := diff.Parse(sample)
	// Nothing of the diff is on disk - a patch from elsewhere.
	if got := source.Root(dir, files); got != dir {
		t.Errorf("Root() = %q, want the directory it was sent from (%q)", got, dir)
	}
	if got := source.Root("", files); got != "" {
		t.Errorf("Root(\"\") = %q, want empty", got)
	}
}

func TestRootPrefersTheDeeperMatch(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "vendored")
	checkout(t, outer)
	checkout(t, inner)
	files := diff.Parse(sample)
	// Both explain the diff; the one the command was run in wins.
	if got := source.Root(inner, files); got != inner {
		t.Errorf("Root() = %q, want the nearer checkout %q", got, inner)
	}
}
