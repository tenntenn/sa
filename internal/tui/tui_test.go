package tui

import (
	"strings"
	"testing"
)

func TestDumpWritesOneLinePerRow(t *testing.T) {
	s := newTestState(t, twoFiles)

	var b strings.Builder
	if err := Dump(&b, s.Files, 60, 12, Session{}); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	out := b.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output does not end with a newline: %q", out)
	}
	if got := strings.Count(out, "\n"); got != 12 {
		t.Errorf("wrote %d lines, want 12", got)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("output carries an escape: %q", out)
	}
	if !strings.Contains(out, "sbnn tui") || !strings.Contains(out, "q: quit") {
		t.Errorf("output = %q, want the header and the footer in it", out)
	}

	// No size means the default one, which is what --dump promises.
	b.Reset()
	if err := Dump(&b, s.Files, 0, 0, Session{}); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if got := strings.Count(b.String(), "\n"); got != DefaultHeight {
		t.Errorf("wrote %d lines, want %d", got, DefaultHeight)
	}
}
