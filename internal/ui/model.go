package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WillHouMoe/pandoc-tui/internal/config"
	"github.com/WillHouMoe/pandoc-tui/internal/convert"
	"github.com/WillHouMoe/pandoc-tui/internal/pandoc"
	"github.com/WillHouMoe/pandoc-tui/internal/style"
)

// screen names the full screen views the app can show.
type screen int

const (
	screenConvert screen = iota
	screenStyles
	screenPicker
	screenRun
	screenHelp
)

// Deps is everything the UI needs from the outside world. Keeping it in one
// struct is what lets the screens stay independent of how pandoc is located
// and where the config lives.
type Deps struct {
	Converters *convert.Registry
	Pandoc     *pandoc.Client
	Styles     *style.Library
	Config     *config.Config
	Version    string
	// PandocError carries a detection failure into the UI, so the reason the
	// binary is missing can be shown instead of a generic complaint.
	PandocError error
	// StyleError reports a style library that could not be created. It is not
	// fatal: conversions work without one.
	StyleError error
}

// statusKind colours the message under the footer.
type statusKind int

const (
	statusInfo statusKind = iota
	statusOK
	statusErr
)

// Model is the root Bubble Tea model: a small screen stack plus the state that
// outlives any single screen.
type Model struct {
	deps Deps

	width, height int
	offset        int

	stack []screen

	form formModel
	lib  stylesScreen
	fp   picker
	run  runner

	status     string
	statusKind statusKind

	pandocVersion string
	pandocMissing bool
	pandocProblem string
}

// New builds the root model around the converters the registry offers.
func New(deps Deps) *Model {
	m := &Model{
		deps:  deps,
		stack: []screen{screenConvert},
		lib:   newStylesScreen(deps.Styles),
	}
	converter, ok := deps.Converters.First()
	if !ok {
		return m
	}

	// Start from the declared defaults, then let remembered values win. A
	// converter that dropped an option since last run simply loses it.
	values := convert.DefaultValues(converter)
	for key, value := range deps.Config.ValuesFor(converter.ID()) {
		if _, known := values[key]; known {
			values[key] = value
		}
	}
	m.form = newForm(converter, deps.Pandoc, values)
	m.restoreStyle(deps.Config.LastStyle)
	if deps.StyleError != nil {
		m.setStatus(statusErr, deps.StyleError.Error())
	}
	if deps.PandocError != nil {
		m.pandocMissing = true
		m.pandocProblem = deps.PandocError.Error()
		m.setStatus(statusErr, deps.PandocError.Error())
	}
	return m
}

// restoreStyle re-attaches the style used last, if the file is still there.
func (m *Model) restoreStyle(path string) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	m.form.setStyle(style.Style{
		Name:    strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Path:    path,
		ModTime: info.ModTime(),
		Size:    info.Size(),
	})
}

type pandocVersionMsg struct {
	version string
	err     error
}

// openStylesMsg asks the shell to push the style library.
type openStylesMsg struct{}

// startRunMsg asks the shell to validate the form and run it.
type startRunMsg struct{}

// retryRunMsg asks the shell to start the last conversion again.
type retryRunMsg struct{}

func (m *Model) Init() tea.Cmd {
	if m.pandocMissing {
		return nil
	}
	client := m.deps.Pandoc
	return func() tea.Msg {
		version, err := client.Version(context.Background())
		return pandocVersionMsg{version: version, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case pandocVersionMsg:
		if msg.err != nil {
			m.pandocMissing = true
			if m.pandocProblem == "" {
				m.pandocProblem = "pandoc not found — point at it with --pandoc or the PANDOC environment variable"
			}
			m.setStatus(statusErr, m.pandocProblem)
		} else {
			m.pandocVersion = msg.version
		}
		return m, nil

	case openPickerMsg:
		m.fp = newPicker(msg)
		m.stack = append(m.stack, screenPicker)
		return m, nil

	case pickResultMsg:
		m.pop()
		m.applyPick(msg)
		return m, nil

	case openStylesMsg:
		m.stack = append(m.stack, screenStyles)
		return m, m.lib.reload()

	case styleChosenMsg:
		m.form.setStyle(msg.style)
		m.pop()
		m.persist()
		return m, nil

	case startRunMsg, retryRunMsg:
		return m.beginRun()

	case closeRunMsg:
		m.pop()
		m.persist()
		return m, nil

	case appOpenedMsg:
		// Only the style screen and the runner produce these; both handle
		// their own copy, so all the shell has to do is surface failures.
		if msg.err != nil {
			m.setStatus(statusErr, msg.err.Error())
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if keyMatches(key, keys.Help) && m.current() != screenHelp {
			m.stack = append(m.stack, screenHelp)
			return m, nil
		}
		if key.String() == "q" && !m.typing() {
			return m, tea.Quit
		}
	}

	switch m.current() {
	case screenConvert:
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg, m.width)
		return m, cmd
	case screenStyles:
		var cmd tea.Cmd
		m.lib, cmd = m.lib.Update(msg, m.width)
		return m, cmd
	case screenPicker:
		var cmd tea.Cmd
		m.fp, cmd = m.fp.Update(msg)
		return m, cmd
	case screenRun:
		var cmd tea.Cmd
		m.run, cmd = m.run.Update(msg)
		return m, cmd
	case screenHelp:
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc", "?", "enter", "q":
				m.pop()
			}
		}
		return m, nil
	}
	return m, nil
}

// typing reports whether a text field owns the keyboard, in which case single
// letter shortcuts must not fire.
func (m *Model) typing() bool {
	switch m.current() {
	case screenConvert:
		_, ok := m.form.textFocused()
		return ok
	case screenStyles:
		return m.lib.mode == stylesNaming
	}
	return false
}

func (m *Model) beginRun() (tea.Model, tea.Cmd) {
	if m.pandocMissing {
		m.form.problem = m.pandocProblem
		m.setStatus(statusErr, m.pandocProblem)
		return m, nil
	}
	req := m.form.request()
	if err := m.form.conv.Validate(req); err != nil {
		m.form.problem = err.Error()
		m.setStatus(statusErr, err.Error())
		return m, nil
	}
	m.form.problem = ""
	m.status = ""

	var cmd tea.Cmd
	m.run, cmd = startRunner(m.deps.Pandoc, m.form.conv, req)
	m.stack = append(m.stack, screenRun)
	return m, cmd
}

// applyPick folds a picker result back into whichever screen asked for it.
func (m *Model) applyPick(msg pickResultMsg) {
	if msg.cancelled {
		return
	}
	switch msg.target {
	case targetInput:
		m.form.setInput(msg.path)
		m.deps.Config.LastInputDir = filepath.Dir(msg.path)
	case targetOutput:
		m.form.setOutput(filepath.Join(msg.path, m.outputName()))
		m.deps.Config.LastOutputDir = msg.path
	case targetImport:
		m.importStyle(msg.path)
	}
	m.persist()
}

// outputName keeps the file name the user already had, so choosing a folder
// only moves the file rather than renaming it.
func (m *Model) outputName() string {
	if base := filepath.Base(m.form.output); base != "" && base != "." && base != "/" {
		return base
	}
	base := strings.TrimSuffix(filepath.Base(m.form.input), filepath.Ext(m.form.input))
	if base == "" {
		base = "output"
	}
	return base + m.form.conv.To().Ext()
}

func (m *Model) importStyle(path string) {
	imported, err := m.deps.Styles.Import(path)
	if err != nil {
		m.lib.err = err.Error()
		return
	}
	list, err := m.deps.Styles.List()
	if err != nil {
		m.lib.err = err.Error()
		return
	}
	m.lib.rows = list
	m.lib.err = ""
	m.lib.notice = "imported " + imported.Name
	m.lib.selectByPath(imported.Path)
}

// persist writes the choices worth remembering. Failures are not worth
// interrupting the user for: the app works fine without a config file.
func (m *Model) persist() {
	cfg := m.deps.Config
	cfg.LastStyle = m.form.style.Path
	cfg.SetValues(m.form.conv.ID(), m.form.values)
	_ = cfg.Save()
}

func (m *Model) setStatus(kind statusKind, text string) {
	m.status = text
	m.statusKind = kind
}

func (m *Model) current() screen {
	if len(m.stack) == 0 {
		return screenConvert
	}
	return m.stack[len(m.stack)-1]
}

func (m *Model) pop() {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

func (m *Model) View() string {
	// A terminal that has not reported its size yet -- or a pty with no window
	// at all -- should still get a usable frame rather than a blank screen.
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	// One row for the banner, one blank, one for the footer, and one more when
	// there is a status line to show.
	bodyHeight := height - 3
	status := m.statusLine()
	if status != "" {
		bodyHeight--
	}
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	f := m.screenFrame(width, bodyHeight)
	visible, offset := scroll(f.lines, bodyHeight, m.offset, f.cursorLine)
	m.offset = offset
	for len(visible) < bodyHeight {
		visible = append(visible, "")
	}

	footer := f.footer
	if len(f.lines) > bodyHeight {
		footer = append([]keyHint{{keys: "↑↓", desc: "more below"}}, footer...)
	}

	out := make([]string, 0, height)
	out = append(out, banner(width, m.headerLeft(), m.headerRight()))
	out = append(out, "")
	out = append(out, visible...)
	out = append(out, renderFooter(width, footer))
	if status != "" {
		out = append(out, status)
	}
	return strings.Join(out, "\n")
}

func (m *Model) screenFrame(width, bodyHeight int) frame {
	switch m.current() {
	case screenStyles:
		return m.lib.render(width, bodyHeight)
	case screenPicker:
		return m.fp.render(width)
	case screenRun:
		return m.run.render(width, bodyHeight)
	case screenHelp:
		return helpFrame(width, m.deps)
	}
	return m.form.render(width)
}

func (m *Model) headerLeft() string {
	name := "pandoc-tui"
	switch m.current() {
	case screenStyles:
		name += " · styles"
	case screenPicker:
		name += " · files"
	case screenRun:
		name += " · converting"
	case screenHelp:
		name += " · help"
	default:
		if m.deps.Version != "" && m.deps.Version != "dev" {
			name += " " + m.deps.Version
		}
	}
	return name
}

func (m *Model) headerRight() string {
	if m.pandocMissing {
		return "pandoc not found"
	}
	if m.pandocVersion == "" {
		return "looking for pandoc…"
	}
	return m.pandocVersion
}

func (m *Model) statusLine() string {
	if m.status == "" {
		return ""
	}
	style := stSubtitle
	switch m.statusKind {
	case statusOK:
		style = stOK
	case statusErr:
		style = stErr
	}
	return style.Render(truncate(m.status, m.width))
}
