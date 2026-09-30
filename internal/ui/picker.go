package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pickMode tells the picker what picking means.
type pickMode int

const (
	// pickOpen selects an existing file.
	pickOpen pickMode = iota
	// pickSave selects a folder; the output file name is decided in the form
	// and rarely needs revisiting.
	pickSave
	// pickImport selects a .docx to copy into the style library.
	pickImport
)

// pickTarget routes the result back to the row that asked for it.
type pickTarget int

const (
	targetInput pickTarget = iota
	targetOutput
	targetImport
)

// openPickerMsg asks the shell to push the picker screen.
type openPickerMsg struct {
	mode   pickMode
	target pickTarget
	start  string
	exts   []string
}

// pickResultMsg is what the picker sends when it closes.
type pickResultMsg struct {
	target    pickTarget
	path      string
	cancelled bool
}

type pickRowKind int

const (
	rowEntry pickRowKind = iota
	rowParent
	rowUseCurrent
)

type pickRow struct {
	label string
	path  string
	isDir bool
	kind  pickRowKind
}

// picker is a single pane file browser. It filters by extension so the convert
// screen only ever offers files that make sense for the conversion at hand.
type picker struct {
	mode   pickMode
	target pickTarget
	exts   []string
	title  string

	dir     string
	rows    []pickRow
	cursor  int
	showAll bool
	err     string
}

func newPicker(msg openPickerMsg) picker {
	p := picker{
		mode:   msg.mode,
		target: msg.target,
		exts:   msg.exts,
		title:  "Choose a source file",
	}
	switch msg.mode {
	case pickSave:
		p.title = "Choose a destination folder"
	case pickImport:
		p.title = "Choose a .docx to import"
	}
	p.open(msg.start)
	return p
}

// open points the picker at a file or folder and settles on the closest
// existing directory, highlighting the file it was given.
func (p *picker) open(start string) {
	dir := strings.TrimSpace(start)
	if dir == "" {
		dir, _ = os.Getwd()
	}

	highlight := ""
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		highlight = filepath.Base(dir)
		dir = filepath.Dir(dir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}

	p.dir = dir
	p.reload(highlight)
}

func (p *picker) reload(highlight string) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		p.err = err.Error()
		entries = nil
	} else {
		p.err = ""
	}

	var dirs, files []pickRow
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !p.showAll {
			continue
		}
		path := filepath.Join(p.dir, name)
		if e.IsDir() {
			dirs = append(dirs, pickRow{label: name + "/", path: path, isDir: true, kind: rowEntry})
			continue
		}
		if !p.accept(name) {
			continue
		}
		files = append(files, pickRow{label: name, path: path, kind: rowEntry})
	}
	byName := func(rows []pickRow) {
		sort.Slice(rows, func(i, j int) bool {
			return strings.ToLower(rows[i].label) < strings.ToLower(rows[j].label)
		})
	}
	byName(dirs)
	byName(files)

	rows := make([]pickRow, 0, len(dirs)+len(files)+2)
	if p.mode == pickSave {
		rows = append(rows, pickRow{label: "Use this folder", path: p.dir, isDir: true, kind: rowUseCurrent})
	}
	if parent := filepath.Dir(p.dir); parent != p.dir {
		rows = append(rows, pickRow{label: "..", path: parent, isDir: true, kind: rowParent})
	}
	rows = append(rows, dirs...)
	rows = append(rows, files...)
	p.rows = rows

	p.cursor = 0
	if highlight != "" {
		for i, r := range rows {
			if r.kind == rowEntry && r.label == highlight {
				p.cursor = i
				break
			}
		}
	}
}

// accept decides whether a file name is worth showing.
func (p picker) accept(name string) bool {
	if p.showAll || len(p.exts) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, want := range p.exts {
		if ext == strings.ToLower(want) {
			return true
		}
	}
	return false
}

func (p picker) Update(msg tea.Msg) (picker, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case keyMatches(key, keys.Up):
		p.move(-1)
	case keyMatches(key, keys.Down):
		p.move(1)
	case keyMatches(key, keys.Top):
		p.cursor = 0
	case keyMatches(key, keys.Bottom):
		p.cursor = max(0, len(p.rows)-1)
	case keyMatches(key, keys.ShowAll):
		p.showAll = !p.showAll
		p.reload("")
	case keyMatches(key, keys.Confirm), keyMatches(key, keys.Right):
		return p.choose()
	case keyMatches(key, keys.Left):
		p.ascend()
	case keyMatches(key, keys.Back):
		target := p.target
		return p, func() tea.Msg { return pickResultMsg{target: target, cancelled: true} }
	}
	return p, nil
}

func (p *picker) move(delta int) {
	if len(p.rows) == 0 {
		return
	}
	p.cursor = ((p.cursor+delta)%len(p.rows) + len(p.rows)) % len(p.rows)
}

func (p *picker) ascend() {
	parent := filepath.Dir(p.dir)
	if parent == p.dir {
		return
	}
	p.dir = parent
	p.reload("")
}

// choose turns the highlighted row into an answer.
func (p picker) choose() (picker, tea.Cmd) {
	if len(p.rows) == 0 {
		return p, nil
	}
	row := p.rows[p.cursor]
	if row.isDir && row.kind != rowUseCurrent {
		p.dir = row.path
		p.reload("")
		return p, nil
	}

	path := row.path
	if row.kind == rowUseCurrent {
		path = p.dir
	}
	target := p.target
	return p, func() tea.Msg {
		return pickResultMsg{target: target, path: path}
	}
}

func (p picker) render(width int) frame {
	contentWidth := width - 4
	body := []string{stFaint.Render(truncate(shortenPath(p.dir), contentWidth)), ""}

	switch {
	case p.err != "":
		body = append(body, stErr.Render(truncate(p.err, contentWidth)))
	case len(p.rows) == 0:
		body = append(body, stFaint.Render("nothing here"))
	}

	first := len(body)
	for i, row := range p.rows {
		mark := " "
		if i == p.cursor {
			mark = stMarker.Render("▸")
		}
		body = append(body, mark+" "+p.rowLabel(row, i == p.cursor, contentWidth))
	}

	cursorLine := -1
	if len(p.rows) > 0 {
		cursorLine = first + p.cursor
	}

	return frame{
		lines:      panel(width, p.title, body),
		cursorLine: cursorLine,
		footer: []keyHint{
			{keys: "↑↓", desc: "move"},
			{keys: "enter", desc: "open"},
			{keys: "esc", desc: "cancel"},
			{keys: ".", desc: "show all files"},
		},
	}
}

func (p picker) rowLabel(row pickRow, focused bool, width int) string {
	label := row.label
	if row.kind == rowUseCurrent {
		label = "✓ " + label
	}
	label = truncate(label, width-2)

	switch {
	case focused:
		return stSelected.Render(label)
	case row.kind == rowUseCurrent:
		return stOK.Render(label)
	case row.isDir:
		return stSubtitle.Render(label)
	default:
		return label
	}
}
