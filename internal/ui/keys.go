package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMap is the whole vocabulary of the app. Screens advertise a subset in
// their footer, so a binding carries its own help text instead of the footer
// being hand written per screen.
type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Top       key.Binding
	Bottom    key.Binding
	Confirm   key.Binding
	Back      key.Binding
	Toggle    key.Binding
	New       key.Binding
	Import    key.Binding
	Rename    key.Binding
	Delete    key.Binding
	Open      key.Binding
	Reveal    key.Binding
	Run       key.Binding
	Cancel    key.Binding
	Help      key.Binding
	Quit      key.Binding
	Browse    key.Binding
	ShowAll   key.Binding
	EditValue key.Binding
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	Left:      key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "previous value")),
	Right:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next value")),
	Top:       key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "first")),
	Bottom:    key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "last")),
	Confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Back:      key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	Toggle:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	New:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new style")),
	Import:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "import .docx")),
	Rename:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
	Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Open:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in editor")),
	Reveal:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "reveal in file manager")),
	Run:       key.NewBinding(key.WithKeys("ctrl+r", "ctrl+s"), key.WithHelp("ctrl+r", "convert")),
	Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Help:      key.NewBinding(key.WithKeys("?", "F1"), key.WithHelp("?", "help")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Browse:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "browse")),
	ShowAll:   key.NewBinding(key.WithKeys("."), key.WithHelp(".", "all file types")),
	EditValue: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
}

// keyHint is one entry of the footer bar.
type keyHint struct {
	keys string
	desc string
}

func hint(b key.Binding) keyHint {
	h := b.Help()
	return keyHint{keys: h.Key, desc: h.Desc}
}

func hints(bindings ...key.Binding) []keyHint {
	out := make([]keyHint, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, hint(b))
	}
	return out
}

func keyMatches(msg tea.KeyMsg, binding key.Binding) bool {
	return key.Matches(msg, binding)
}
