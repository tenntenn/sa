package tui

import (
	"reflect"
	"testing"
)

func TestDecodeKeys(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []byte
		want []string
	}{
		{"nothing", nil, nil},
		{"letters", []byte("jkqgG"), []string{"j", "k", "q", "g", "G"}},
		{"tab", []byte{0x09}, []string{"tab"}},
		{"ctrl+c", []byte{0x03}, []string{"ctrl+c"}},
		{"ctrl+d", []byte{0x04}, []string{"ctrl+d"}},
		{"ctrl+u", []byte{0x15}, []string{"ctrl+u"}},
		{"escape on its own", []byte{0x1b}, []string{"esc"}},
		{"arrow up", []byte{0x1b, '[', 'A'}, []string{"up"}},
		{"arrow down", []byte{0x1b, '[', 'B'}, []string{"down"}},
		{"arrow up in application mode", []byte{0x1b, 'O', 'A'}, []string{"up"}},
		{"arrow down in application mode", []byte{0x1b, 'O', 'B'}, []string{"down"}},
		{"arrow right is dropped", []byte{0x1b, '[', 'C'}, nil},
		{"arrow left is dropped", []byte{0x1b, '[', 'D'}, nil},
		{"a report is dropped whole", []byte{0x1b, '[', '2', '4', ';', '8', '0', 'R'}, nil},
		{"a mouse report is dropped whole", []byte{0x1b, '[', '<', '0', ';', '1', ';', '1', 'M'}, nil},
		{"a cut sequence is dropped", []byte{0x1b, '['}, nil},
		// A key held down arrives as a burst, and so does a paste.
		{"a held arrow", []byte{0x1b, '[', 'B', 0x1b, '[', 'B', 0x1b, '[', 'B'},
			[]string{"down", "down", "down"}},
		{"a burst of keys", []byte{'j', 0x1b, '[', 'B', 0x09, 'k', 'q'},
			[]string{"j", "down", "tab", "k", "q"}},
		// An escape with a letter behind it is Alt+letter, which means nothing
		// here: the escape counts, and the letter is read on its own.
		{"escape then a letter", []byte{0x1b, 'q'}, []string{"esc", "q"}},
		// A dropped sequence must not leave its letters behind as key presses.
		{"a dropped sequence leaves nothing", []byte{'j', 0x1b, '[', 'C', 'k'},
			[]string{"j", "k"}},
		{"characters outside ascii are dropped", []byte("あ"), nil},
		{"other control bytes are dropped", []byte{0x00, 0x01, 0x1f, 0x7f}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeKeys(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeKeys(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The names are not the decoder's to choose: State has to know them, or a key
// press decodes into something and then does nothing.
func TestDecodeKeysNamesReachState(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []byte
		quit bool
		then func(t *testing.T, s *State)
	}{
		{name: "q quits", in: []byte("q"), quit: true},
		{name: "ctrl+c quits", in: []byte{0x03}, quit: true},
		{name: "escape quits", in: []byte{0x1b}, quit: true},
		{
			name: "the down arrow moves the cursor",
			in:   []byte{0x1b, '[', 'B'},
			then: func(t *testing.T, s *State) {
				if s.Cursor != 1 {
					t.Errorf("cursor = %d, want 1", s.Cursor)
				}
			},
		},
		{
			name: "the up arrow moves it back",
			in:   []byte{0x1b, '[', 'B', 0x1b, '[', 'A'},
			then: func(t *testing.T, s *State) {
				if s.Cursor != 0 {
					t.Errorf("cursor = %d, want 0", s.Cursor)
				}
			},
		},
		{
			name: "tab switches pane",
			in:   []byte{0x09},
			then: func(t *testing.T, s *State) {
				if s.Focus != PaneDiff {
					t.Errorf("focus = %v, want PaneDiff", s.Focus)
				}
			},
		},
		{
			name: "ctrl+d scrolls the diff",
			in:   []byte{0x09, 0x04},
			then: func(t *testing.T, s *State) {
				if s.Top <= 0 {
					t.Errorf("top = %d, want the diff scrolled", s.Top)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestState(t, longFirstDiff())
			keys := decodeKeys(tt.in)
			if len(keys) == 0 {
				t.Fatalf("decodeKeys(%q) returned nothing", tt.in)
			}
			quit := false
			for _, key := range keys {
				if s.Key(key) {
					quit = true
					break
				}
			}
			if quit != tt.quit {
				t.Fatalf("decodeKeys(%q) = %q, quit = %v, want %v", tt.in, keys, quit, tt.quit)
			}
			if tt.then != nil {
				tt.then(t, s)
			}
		})
	}
}
