package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tenntenn/sbnn/internal/model"
)

// Colours are the basic ANSI ones on purpose: the reader's own terminal theme
// decides what green and red look like, the way git does it.
var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	footerStyle = lipgloss.NewStyle().Faint(true)
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	addStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	deleteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	plainStyle  = lipgloss.NewStyle()
)

// bubbleModel drives the frame from bubbletea. It keeps no state of its own:
// the state is State, which knows nothing about terminals, and the drawing is
// Frame, which returns plain text. This type only carries messages between
// them and paints what comes back.
type bubbleModel struct {
	state *State
}

func newModel(files []*model.File) *bubbleModel {
	return &bubbleModel{state: NewState(files)}
}

func (m *bubbleModel) Init() tea.Cmd {
	return nil
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.state.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		// What a key means is State's business, so that it can be tested
		// without a terminal to press it on.
		if quit := m.state.Key(msg.String()); quit {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *bubbleModel) View() string {
	lines := Frame(m.state, m.state.Width, m.state.Height)
	if len(lines) == 0 {
		return ""
	}
	listWidth, _ := paneWidths(m.state.Width)
	out := make([]string, len(lines))
	for i, line := range lines {
		switch {
		case i == 0:
			out[i] = headerStyle.Render(line)
		case i == len(lines)-1:
			out[i] = footerStyle.Render(line)
		default:
			out[i] = styleBody(line, listWidth)
		}
	}
	return strings.Join(out, "\n")
}

// styleBody colours one body line without changing a character of it: the
// file column, the rule, and the diff column are styled where Frame put them.
func styleBody(line string, listWidth int) string {
	if listWidth <= 0 {
		return diffStyle(line).Render(line)
	}
	runes := []rune(line)
	if len(runes) <= listWidth {
		return fileStyle(line).Render(line)
	}
	list, rest := string(runes[:listWidth]), string(runes[listWidth:])
	sep, diff := rest, ""
	if strings.HasPrefix(rest, separator) {
		sep, diff = separator, rest[len(separator):]
	}
	return fileStyle(list).Render(list) + sep + diffStyle(diff).Render(diff)
}

// fileStyle picks out the selected file, which FileListLines marks with "> ".
func fileStyle(list string) lipgloss.Style {
	if strings.HasPrefix(list, "> ") {
		return cursorStyle
	}
	return plainStyle
}

// diffStyle colours a diff line by the marker DiffLines drew in front of its
// content.
func diffStyle(diff string) lipgloss.Style {
	switch diffMarker(diff) {
	case '+':
		return addStyle
	case '-':
		return deleteStyle
	default:
		return plainStyle
	}
}

// diffMarker is the first character of a diff line past its number gutter.
func diffMarker(diff string) byte {
	for i := 0; i < len(diff); i++ {
		if c := diff[i]; c != ' ' && (c < '0' || c > '9') {
			return c
		}
	}
	return 0
}
