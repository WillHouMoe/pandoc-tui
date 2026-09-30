package ui

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WillHouMoe/pandoc-tui/internal/style"
)

type stylesMode int

const (
	stylesBrowse stylesMode = iota
	stylesNaming
	stylesConfirmDelete
)

// Messages the style screen consumes.
type (
	stylesLoadedMsg struct {
		styles []style.Style
		err    error
	}
	styleCreatedMsg struct {
		style style.Style
		err   error
		open  bool
	}
	styleDeletedMsg struct{ err error }
	styleRenamedMsg struct {
		style style.Style
		err   error
	}
	appOpenedMsg struct {
		what string
		err  error
	}
)

// styleChosenMsg is sent when the user picks a style to use.
type styleChosenMsg struct{ style style.Style }

// stylesScreen is the style library: the reference documents a conversion can
// be dressed with, plus the actions that manage them.
type stylesScreen struct {
	lib    *style.Library
	rows   []style.Style
	cursor int

	mode      stylesMode
	nameInput textinput.Model
	target    style.Style
	err       string
	notice    string
	busy      bool
}

func newStylesScreen(lib *style.Library) stylesScreen {
	ti := newEditor()
	ti.Placeholder = "style name"
	return stylesScreen{lib: lib, nameInput: ti}
}

func (s *stylesScreen) reload() tea.Cmd {
	s.busy = true
	lib := s.lib
	return func() tea.Msg {
		list, err := lib.List()
		return stylesLoadedMsg{styles: list, err: err}
	}
}

// selectByPath keeps the highlight on a style across reloads.
func (s *stylesScreen) selectByPath(path string) {
	if path == "" {
		return
	}
	for i, row := range s.rows {
		if row.Path == path {
			s.cursor = i
			return
		}
	}
}

func (s stylesScreen) current() (style.Style, bool) {
	if s.cursor < 0 || s.cursor >= len(s.rows) {
		return style.Style{}, false
	}
	return s.rows[s.cursor], true
}

func (s stylesScreen) Update(msg tea.Msg, width int) (stylesScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case stylesLoadedMsg:
		s.busy = false
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		keep := ""
		if cur, ok := s.current(); ok {
			keep = cur.Path
		}
		s.rows = msg.styles
		s.selectByPath(keep)
		if s.cursor >= len(s.rows) && len(s.rows) > 0 {
			s.cursor = len(s.rows) - 1
		}
		return s, nil

	case styleCreatedMsg:
		s.busy = false
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		s.mode = stylesBrowse
		s.notice = "created " + msg.style.Name + " and opened it in your editor"
		s.selectByPath(msg.style.Path)
		cmds := []tea.Cmd{s.reload()}
		if msg.open {
			cmds = append(cmds, openStyleCmd(msg.style))
		}
		return s, tea.Batch(cmds...)

	case styleDeletedMsg:
		s.busy = false
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		s.mode = stylesBrowse
		s.notice = "deleted"
		return s, s.reload()

	case styleRenamedMsg:
		s.busy = false
		if msg.err != nil {
			s.err = msg.err.Error()
			return s, nil
		}
		s.mode = stylesBrowse
		s.notice = "renamed to " + msg.style.Name
		s.selectByPath(msg.style.Path)
		return s, s.reload()

	case appOpenedMsg:
		if msg.err != nil {
			s.err = msg.err.Error()
		} else {
			s.err = ""
			s.notice = "opened " + msg.what
		}
		return s, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}

	switch s.mode {
	case stylesNaming:
		switch key.String() {
		case "esc":
			s.mode = stylesBrowse
			s.target = style.Style{}
			return s, nil
		case "enter":
			return s.submitName()
		}
		s.nameInput.Width = max(20, width-20)
		var cmd tea.Cmd
		s.nameInput, cmd = s.nameInput.Update(msg)
		return s, cmd

	case stylesConfirmDelete:
		switch strings.ToLower(key.String()) {
		case "y", "enter":
			return s.deleteCurrent()
		case "n", "esc":
			s.mode = stylesBrowse
		}
		return s, nil
	}

	switch {
	case keyMatches(key, keys.Up):
		s.move(-1)
	case keyMatches(key, keys.Down):
		s.move(1)
	case keyMatches(key, keys.Top):
		s.cursor = 0
	case keyMatches(key, keys.Bottom):
		s.cursor = max(0, len(s.rows)-1)
	case keyMatches(key, keys.Confirm):
		if cur, ok := s.current(); ok {
			return s, func() tea.Msg { return styleChosenMsg{style: cur} }
		}
	case keyMatches(key, keys.New):
		s.mode = stylesNaming
		s.target = style.Style{}
		s.notice = ""
		s.err = ""
		s.nameInput.SetValue("")
		s.nameInput.Focus()
		return s, textinput.Blink
	case keyMatches(key, keys.Rename):
		if cur, ok := s.current(); ok && !cur.Builtin {
			s.mode = stylesNaming
			s.target = cur
			s.nameInput.SetValue(cur.Name)
			s.nameInput.CursorEnd()
			s.nameInput.Focus()
			return s, textinput.Blink
		}
	case keyMatches(key, keys.Import):
		dir := s.lib.Dir
		return s, func() tea.Msg {
			return openPickerMsg{mode: pickImport, target: targetImport, start: dir, exts: []string{style.Ext}}
		}
	case keyMatches(key, keys.Open):
		if cur, ok := s.current(); ok && !cur.Builtin {
			return s, openStyleCmd(cur)
		}
	case keyMatches(key, keys.Reveal):
		if cur, ok := s.current(); ok && !cur.Builtin {
			return s, revealCmd(cur.Path)
		}
	case keyMatches(key, keys.Delete):
		if cur, ok := s.current(); ok && !cur.Builtin {
			s.mode = stylesConfirmDelete
			s.target = cur
			s.err = ""
		}
	}
	return s, nil
}

func (s *stylesScreen) move(delta int) {
	if len(s.rows) == 0 {
		return
	}
	s.cursor = ((s.cursor+delta)%len(s.rows) + len(s.rows)) % len(s.rows)
}

// submitName handles both "new" and "rename", which share one prompt.
func (s stylesScreen) submitName() (stylesScreen, tea.Cmd) {
	name := strings.TrimSpace(s.nameInput.Value())
	if name == "" {
		s.err = "a style needs a name"
		return s, nil
	}
	s.busy = true
	s.err = ""

	lib := s.lib
	target := s.target
	renaming := !target.Builtin && target.Path != ""
	return s, func() tea.Msg {
		if renaming {
			updated, err := lib.Rename(target, name)
			return styleRenamedMsg{style: updated, err: err}
		}
		created, err := lib.Create(context.Background(), name)
		return styleCreatedMsg{style: created, err: err, open: true}
	}
}

func (s stylesScreen) deleteCurrent() (stylesScreen, tea.Cmd) {
	target := s.target
	if target.Path == "" {
		s.mode = stylesBrowse
		return s, nil
	}
	s.busy = true
	lib := s.lib
	return s, func() tea.Msg {
		return styleDeletedMsg{err: lib.Delete(target)}
	}
}

func (s stylesScreen) render(width, height int) frame {
	contentWidth := width - 4
	nameWidth := 0
	for _, row := range s.rows {
		nameWidth = max(nameWidth, lipgloss.Width(row.Name))
	}
	nameWidth = min(nameWidth, 28)

	body := make([]string, 0, len(s.rows))
	for i, row := range s.rows {
		body = append(body, rowLabel(row, i == s.cursor, contentWidth, nameWidth))
	}
	if len(s.rows) == 0 {
		body = append(body, stFaint.Render("no styles yet — press n to create one"))
	}

	var b blockBuilder
	first := b.panel(width, "Styles", body) + 1
	b.blank()

	if cur, ok := s.current(); ok {
		detail := []string{}
		if cur.Builtin {
			detail = append(detail, stFaint.Render("Let pandoc use its own reference document."))
			detail = append(detail, stFaint.Render("Style the output by creating one instead."))
		} else {
			detail = append(detail, truncate(shortenPath(cur.Path), contentWidth))
			detail = append(detail, stFaint.Render(cur.Describe()))
		}
		b.panel(width, "Details", detail)
		b.blank()
	}

	switch s.mode {
	case stylesNaming:
		title := "New style"
		detail := []string{
			stLabelHot.Render("Name") + "  " + s.nameInput.View(),
			"",
		}
		if s.target.Path != "" {
			title = "Rename style"
			detail = append(detail, stFaint.Render("The file keeps its contents, only the name changes."))
		} else {
			detail = append(detail, stFaint.Render("A fresh copy of pandoc's reference.docx is created,"))
			detail = append(detail, stFaint.Render("then opened in your editor so you can restyle it."))
		}
		b.panel(width, title, detail)
		b.blank()

	case stylesConfirmDelete:
		b.panel(width, "Delete style", []string{
			"Delete " + stValueHot.Render(truncate(s.target.Name, max(8, contentWidth-20))) + "?",
			"",
			stWarn.Render("y") + stFaint.Render("  delete it     ") + stWarn.Render("n") + stFaint.Render("  keep it"),
		})
		b.blank()
	}

	if s.err != "" {
		for _, line := range wrap("✗ "+s.err, contentWidth) {
			b.add(stErr.Render(line))
		}
		b.blank()
	} else if s.notice != "" {
		b.add(stOK.Render("✓ " + truncate(s.notice, contentWidth-2)))
		b.blank()
	}

	b.add(stFaint.Render("Library  " + truncate(shortenPath(s.lib.Dir), contentWidth-9)))

	cursorLine := -1
	if len(s.rows) > 0 {
		cursorLine = first + s.cursor
	}

	footer := []keyHint{
		{keys: "↑↓", desc: "move"},
		{keys: "enter", desc: "use"},
		{keys: "n", desc: "new"},
		{keys: "i", desc: "import"},
		{keys: "r", desc: "rename"},
		{keys: "d", desc: "delete"},
		{keys: "o", desc: "open"},
		{keys: "f", desc: "reveal"},
		{keys: "esc", desc: "back"},
	}
	switch s.mode {
	case stylesNaming:
		footer = []keyHint{{keys: "enter", desc: "save"}, {keys: "esc", desc: "cancel"}}
	case stylesConfirmDelete:
		footer = []keyHint{{keys: "y", desc: "delete"}, {keys: "n", desc: "cancel"}}
	}

	return frame{lines: b.lines, cursorLine: cursorLine, footer: footer}
}

// rowLabel renders one style line: marker, name, note, size.
func rowLabel(row style.Style, focused bool, width, nameWidth int) string {
	mark := " "
	if focused {
		mark = stMarker.Render("▸")
	}

	name := truncate(row.Name, nameWidth)
	switch {
	case focused:
		name = stSelected.Render(name)
	case row.Builtin:
		name = stFaint.Render(name)
	}
	nameCell := name + strings.Repeat(" ", max(0, nameWidth-lipgloss.Width(row.Name)))

	size := ""
	if !row.Builtin {
		size = humanSize(row.Size)
	}
	room := width - nameWidth - 4 - lipgloss.Width(size)
	if room < 10 {
		room = 10
	}
	return mark + " " + nameCell + "  " + stFaint.Render(truncate(row.Describe(), room)) + "  " + stFaint.Render(size)
}

func openStyleCmd(s style.Style) tea.Cmd {
	return func() tea.Msg {
		return appOpenedMsg{what: s.Name, err: openFile(s.Path)}
	}
}

func revealCmd(path string) tea.Cmd {
	return func() tea.Msg {
		return appOpenedMsg{what: filepath.Base(path), err: revealInFileManager(path)}
	}
}
