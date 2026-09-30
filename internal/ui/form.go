package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/WillHouMoe/pandoc-tui/internal/convert"
	"github.com/WillHouMoe/pandoc-tui/internal/pandoc"
	"github.com/WillHouMoe/pandoc-tui/internal/style"
)

// fieldKind distinguishes the two file slots and the style slot from the
// options a converter declares for itself.
type fieldKind int

const (
	fieldInput fieldKind = iota
	fieldOutput
	fieldStyle
	fieldOption
)

type field struct {
	kind   fieldKind
	option convert.Option
}

// formModel is the convert screen. It is deliberately built from whatever the
// converter declares, so a new conversion gets its files, its style picker and
// its options form for free.
type formModel struct {
	conv   convert.Converter
	client *pandoc.Client
	fields []field

	cursor int

	input  string
	output string
	style  style.Style

	// outputAuto tracks whether the output path is still derived from the
	// input, so picking a new input can follow along without clobbering a path
	// the user typed by hand.
	outputAuto bool

	values map[string]string

	editor    textinput.Model
	editorFor int

	problem string
}

func newForm(conv convert.Converter, client *pandoc.Client, values map[string]string) formModel {
	m := formModel{
		conv:      conv,
		client:    client,
		values:    values,
		editor:    newEditor(),
		editorFor: -1,
		style:     style.Style{Name: "None", Builtin: true},
	}
	m.fields = []field{{kind: fieldInput}, {kind: fieldOutput}}
	if conv.WantsReferenceDoc() {
		m.fields = append(m.fields, field{kind: fieldStyle})
	}
	for _, o := range conv.Options() {
		m.fields = append(m.fields, field{kind: fieldOption, option: o})
	}
	m.syncEditor()
	return m
}

func newEditor() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "…"
	ti.PlaceholderStyle = stFaint
	ti.TextStyle = stValue
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colAccent)
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.CharLimit = 1024
	return ti
}

// ---------------------------------------------------------------- accessors

func (m formModel) request() convert.Request {
	ref := ""
	if !m.style.Builtin {
		ref = m.style.Path
	}
	return convert.Request{
		Input:     m.input,
		Output:    m.output,
		Reference: ref,
		Values:    m.values,
	}
}

func (m formModel) focused() field {
	if m.cursor < 0 || m.cursor >= len(m.fields) {
		return field{}
	}
	return m.fields[m.cursor]
}

func (m formModel) textFocused() (convert.Option, bool) {
	f := m.focused()
	if f.kind == fieldOption && f.option.Kind == convert.KindText {
		return f.option, true
	}
	return convert.Option{}, false
}

// syncEditor points the shared text input at whichever free-text field has
// focus. One widget is enough because only one field can be focused.
func (m *formModel) syncEditor() {
	o, ok := m.textFocused()
	if !ok {
		m.editor.Blur()
		m.editorFor = -1
		return
	}
	if m.editorFor != m.cursor {
		m.editor.SetValue(m.values[o.Key])
		m.editor.CursorEnd()
		m.editorFor = m.cursor
	}
	m.editor.Focus()
}

func (m *formModel) commitEditor() {
	if o, ok := m.textFocused(); ok {
		m.values[o.Key] = m.editor.Value()
	}
}

// ------------------------------------------------------------------ actions

func (m *formModel) move(delta int) {
	m.commitEditor()
	if len(m.fields) == 0 {
		return
	}
	n := len(m.fields)
	m.cursor = ((m.cursor+delta)%n + n) % n
	m.syncEditor()
}

func (m *formModel) setInput(path string) {
	m.input = path
	if m.output == "" || m.outputAuto {
		m.output = deriveOutput(path, m.conv.To())
		m.outputAuto = true
	}
	m.problem = ""
}

func (m *formModel) setOutput(path string) {
	m.output = path
	m.outputAuto = false
	m.problem = ""
}

func (m *formModel) setStyle(s style.Style) {
	m.style = s
	m.problem = ""
}

// deriveOutput turns notes.md into notes.docx beside the original file.
func deriveOutput(input string, to convert.Format) string {
	if input == "" {
		return ""
	}
	dir := filepath.Dir(input)
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	if base == "" {
		base = "output"
	}
	return filepath.Join(dir, base+to.Ext())
}

// clearProblem is called whenever the user touches the form, so a stale error
// does not linger after it has been addressed.
func (m *formModel) clearProblem() { m.problem = "" }

// ------------------------------------------------------------------- update

func (m formModel) Update(msg tea.Msg, width int) (formModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		m.editor.Width = m.editorWidth(width)
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	// Free-text fields swallow ordinary typing, so navigation is the only
	// thing the form handles itself while one is focused.
	if _, text := m.textFocused(); text {
		switch key.String() {
		case "up", "shift+tab":
			m.move(-1)
			return m, nil
		case "down", "tab", "esc":
			m.move(1)
			return m, nil
		case "ctrl+r", "ctrl+s":
			return m, func() tea.Msg { return startRunMsg{} }
		case "ctrl+c":
			return m, tea.Quit
		}
		m.editor.Width = m.editorWidth(width)
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		m.commitEditor()
		return m, cmd
	}

	switch {
	case keyMatches(key, keys.Up):
		m.move(-1)
	case keyMatches(key, keys.Down):
		m.move(1)
	case key.String() == "tab":
		m.move(1)
	case key.String() == "shift+tab":
		m.move(-1)
	case keyMatches(key, keys.Confirm), keyMatches(key, keys.Toggle), key.String() == "l":
		return m.activate()
	case keyMatches(key, keys.Left):
		m.cycle(-1)
	case keyMatches(key, keys.Right):
		m.cycle(1)
	case keyMatches(key, keys.Run):
		return m, func() tea.Msg { return startRunMsg{} }
	case keyMatches(key, keys.Browse):
		return m, m.browse()
	}
	return m, nil
}

// activate runs whatever "enter" means for the focused row.
func (m formModel) activate() (formModel, tea.Cmd) {
	switch f := m.focused(); f.kind {
	case fieldInput:
		return m, m.browse()
	case fieldOutput:
		return m, m.browse()
	case fieldStyle:
		return m, func() tea.Msg { return openStylesMsg{} }
	case fieldOption:
		if f.option.Kind == convert.KindText {
			// Focus already lives in the editor; nothing to do.
			return m, nil
		}
		m.cycle(1)
	}
	return m, nil
}

func (m *formModel) cycle(delta int) {
	f := m.focused()
	if f.kind != fieldOption {
		return
	}
	current := m.values[f.option.Key]
	if delta > 0 {
		m.values[f.option.Key] = f.option.Next(current)
	} else {
		m.values[f.option.Key] = f.option.Prev(current)
	}
	m.problem = ""
}

// browse asks the shell to open the file picker for the focused row.
func (m formModel) browse() tea.Cmd {
	switch f := m.focused(); f.kind {
	case fieldInput:
		start := m.input
		if start == "" {
			start = lastDir(m.output)
		}
		return func() tea.Msg {
			return openPickerMsg{mode: pickOpen, target: targetInput, start: start}
		}
	case fieldOutput:
		start := lastDir(m.output)
		return func() tea.Msg {
			return openPickerMsg{mode: pickSave, target: targetOutput, start: start}
		}
	}
	return nil
}

func (m formModel) editorWidth(width int) int {
	room := width - 4 - m.labelWidth() - 2
	if room < 8 {
		room = 8
	}
	return room
}

// --------------------------------------------------------------------- view

func (m formModel) render(width int) frame {
	if width < 20 {
		width = 20
	}
	contentWidth := width - 4
	labelWidth := m.labelWidth()

	var b blockBuilder
	marks := make([]int, len(m.fields))

	// Conversion summary: what this build can do, and what it cannot yet.
	body := wrap(m.conv.Summary(), contentWidth)
	roadmap := convert.Roadmap()
	if len(roadmap) > 0 {
		body = append(body, stFaint.Render(plural(len(roadmap), "conversion", "conversions")+
			" are planned for later — press ? to see the roadmap"))
	}
	b.panel(width, "Conversion  "+m.conv.Label(), body)
	b.blank()

	// Source files.
	srcBody := []string{
		labelled(contentWidth, labelWidth, "Input", m.inputCell(contentWidth, labelWidth), m.cursor == 0),
		labelled(contentWidth, labelWidth, "Output", m.outputCell(contentWidth, labelWidth), m.cursor == 1),
	}
	start := b.panel(width, "Source", srcBody)
	marks[0] = start + 1
	marks[1] = start + 2
	b.blank()

	// Style.
	if m.conv.WantsReferenceDoc() {
		idx := m.styleIndex()
		styleBody := []string{
			labelled(contentWidth, labelWidth, "Reference", m.styleCell(contentWidth, labelWidth), m.cursor == idx),
		}
		if !m.style.Builtin && m.style.Path != "" {
			styleBody = append(styleBody, stFaint.Render(strings.Repeat(" ", labelWidth+4)+shortenPath(m.style.Path)))
		}
		st := b.panel(width, "Style", styleBody)
		marks[idx] = st + 1
		b.blank()
	}

	// Options: generated one row per declared option.
	if options := m.conv.Options(); len(options) > 0 {
		rows := make([]string, 0, len(options))
		first := m.optionIndex(0)
		for i, o := range options {
			rows = append(rows, labelled(contentWidth, labelWidth, o.Label, m.optionCell(o), m.cursor == first+i))
		}
		st := b.panel(width, "Options", rows)
		for i := range options {
			marks[first+i] = st + 1 + i
		}
		b.blank()
	}

	// Command preview. This is the whole point of the app: the pandoc
	// incantation is written for you, and you can still read it.
	cmdBody := m.commandPreview(contentWidth)
	b.panel(width, "pandoc command", cmdBody)

	cursorLine := -1
	if m.cursor >= 0 && m.cursor < len(marks) {
		cursorLine = marks[m.cursor]
	}

	return frame{
		lines:      b.lines,
		cursorLine: cursorLine,
		footer: []keyHint{
			{keys: "↑↓", desc: "move"},
			{keys: "←→", desc: "change"},
			{keys: "enter", desc: "choose"},
			{keys: "ctrl+r", desc: "convert"},
			{keys: "?", desc: "help"},
			{keys: "q", desc: "quit"},
		},
	}
}

func (m formModel) inputCell(width, labelWidth int) string {
	if m.input == "" {
		return stFaint.Render("no file selected — press enter to browse")
	}
	return shortenPath(m.input)
}

func (m formModel) outputCell(width, labelWidth int) string {
	if m.output == "" {
		return stFaint.Render("no destination yet")
	}
	return shortenPath(m.output)
}

func (m formModel) styleCell(width, labelWidth int) string {
	if m.style.Builtin {
		return stFaint.Render("None") + stFaint.Render("  (pandoc's stock look)")
	}
	name := stValue.Render(m.style.Name)
	if m.style.ModTime.IsZero() {
		return name
	}
	return name + stFaint.Render("  "+m.style.Describe())
}

func (m formModel) optionCell(o convert.Option) string {
	if o.Kind == convert.KindText && m.editorFor == m.cursor {
		return m.editor.View()
	}
	value := m.values[o.Key]

	switch o.Kind {
	case convert.KindToggle:
		if isTruthy(value) {
			return stOK.Render("on") + stFaint.Render("  space to toggle")
		}
		return stFaint.Render("off") + stFaint.Render("  space to toggle")
	case convert.KindChoice, convert.KindNumber:
		return stValue.Render(o.Display(value)) + stFaint.Render("  ‹ ›")
	}
	if strings.TrimSpace(value) == "" {
		return stFaint.Render("not set")
	}
	return stValue.Render(value)
}

func (m formModel) commandPreview(width int) []string {
	args, err := m.conv.Args(m.request())
	if err != nil {
		return []string{stErr.Render(err.Error())}
	}
	text := m.client.CommandLine(args)

	lines := wrap(text, width)
	if m.problem != "" {
		lines = append([]string{stErr.Render("✗ " + m.problem), ""}, lines...)
	}
	return lines
}

func (m formModel) labelWidth() int {
	w := lipgloss.Width("Output")
	for _, f := range m.fields {
		if f.kind == fieldOption {
			w = max(w, lipgloss.Width(f.option.Label))
		}
	}
	return w
}

// styleIndex is the field index of the style row.
func (m formModel) styleIndex() int {
	for i, f := range m.fields {
		if f.kind == fieldStyle {
			return i
		}
	}
	return -1
}

// optionIndex maps an option position to its field index.
func (m formModel) optionIndex(position int) int {
	seen := 0
	for i, f := range m.fields {
		if f.kind != fieldOption {
			continue
		}
		if seen == position {
			return i
		}
		seen++
	}
	return -1
}

// ------------------------------------------------------------------ helpers

// shortenPath replaces the home directory with a tilde so file paths stay
// readable in a narrow terminal.
func shortenPath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+string(os.PathSeparator)) {
			return "~" + strings.TrimPrefix(path, home)
		}
	}
	return path
}

func lastDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
