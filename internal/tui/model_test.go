package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelHandsKeysToState(t *testing.T) {
	m := newModel(newTestState(t, twoFiles).Files)

	if cmd := m.Init(); cmd != nil {
		t.Errorf("Init() returned a command, want nil")
	}

	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); cmd != nil {
		t.Errorf("a resize returned a command, want nil")
	}
	if m.state.Width != 80 || m.state.Height != 24 {
		t.Errorf("size = %dx%d, want 80x24", m.state.Width, m.state.Height)
	}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}); cmd != nil {
		t.Errorf(`"j" returned a command, want nil`)
	}
	if m.state.Cursor != 1 {
		t.Errorf(`after "j" cursor = %d, want 1`, m.state.Cursor)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf(`"q" returned no command, want a quit`)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf(`"q" returned %T, want tea.QuitMsg`, cmd())
	}
}

// The view only paints the frame: whatever the styles do, the text of the
// frame is still in there, line for line.
func TestViewKeepsFrameLines(t *testing.T) {
	m := newModel(newTestState(t, twoFiles).Files)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

	view := strings.Split(m.View(), "\n")
	frame := Frame(m.state, 80, 12)
	if len(view) != len(frame) {
		t.Fatalf("view has %d lines, frame has %d", len(view), len(frame))
	}
	for i, line := range frame {
		if !strings.Contains(view[i], strings.TrimRight(line, " ")) &&
			!strings.Contains(stripStyles(view[i]), strings.TrimRight(line, " ")) {
			t.Errorf("view line %d = %q, want the frame line %q in it", i, view[i], line)
		}
	}
}

// stripStyles drops escape sequences, so that a styled line can be compared
// with the plain one it was made from.
func stripStyles(line string) string {
	var b strings.Builder
	for {
		start := strings.IndexByte(line, 0x1b)
		if start < 0 {
			b.WriteString(line)
			return b.String()
		}
		b.WriteString(line[:start])
		end := strings.IndexByte(line[start:], 'm')
		if end < 0 {
			return b.String()
		}
		line = line[start+end+1:]
	}
}
