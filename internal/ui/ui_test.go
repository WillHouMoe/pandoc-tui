package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/WillHouMoe/pandoc-tui/internal/config"
	"github.com/WillHouMoe/pandoc-tui/internal/convert"
	"github.com/WillHouMoe/pandoc-tui/internal/pandoc"
	"github.com/WillHouMoe/pandoc-tui/internal/style"
)

func TestMain(m *testing.M) {
	// Rendered output is compared as plain text; colour escapes would only
	// make the assertions harder to read.
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("PANDOC_TUI_HOME", t.TempDir())

	dir, err := config.StyleDir()
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		Converters: convert.Default(),
		Pandoc:     pandoc.New("pandoc"),
		Styles:     style.New(dir, nil),
		Config:     &config.Config{Values: map[string]map[string]string{}},
		Version:    "test",
	})
	m.pandocVersion = "pandoc 3.10.2"
	return m
}

func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	}
}

// send drives one keystroke through the model and returns whatever command the
// screen wants to run.
func send(t *testing.T, m *Model, name string) tea.Cmd {
	t.Helper()
	model, cmd := m.Update(keyMsg(name))
	next, ok := model.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *Model", model)
	}
	*m = *next
	return cmd
}

func resize(t *testing.T, m *Model, width, height int) {
	t.Helper()
	model, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	*m = *(model.(*Model))
}

// checkFrame asserts the invariants every screen has to keep: the view fills
// the window exactly and nothing spills past the right edge.
func checkFrame(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Errorf("view has %d lines, want %d", len(lines), height)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %d is %d columns wide, want at most %d: %q", i, w, width, line)
		}
	}
}

func TestConvertScreenFillsTheWindow(t *testing.T) {
	m := newTestModel(t)
	for _, size := range [][2]int{{80, 24}, {100, 40}, {60, 18}, {46, 12}} {
		resize(t, m, size[0], size[1])
		checkFrame(t, m.View(), size[0], size[1])
	}
}

func TestConvertScreenShowsTheGeneratedCommand(t *testing.T) {
	m := newTestModel(t)
	resize(t, m, 110, 40)

	dir := t.TempDir()
	input := filepath.Join(dir, "report.md")
	write(t, input, "# hi")
	m.form.setInput(input)

	if want := filepath.Join(dir, "report.docx"); m.form.output != want {
		t.Errorf("output = %q, want %q (derived from the input file)", m.form.output, want)
	}

	// Assert against the whole form rather than the clipped view: the command
	// panel sits below the fold on a short window, which is what scrolling is
	// for.
	whole := strings.Join(m.form.render(110).lines, "\n")
	for _, want := range []string{
		"Markdown -> Word (.docx)",
		"pandoc --from=markdown",
		"--to=docx",
	} {
		if !strings.Contains(whole, want) {
			t.Errorf("form is missing %q\n%s", want, whole)
		}
	}
	if strings.Contains(whole, "--reference-doc") {
		t.Error("no style is selected, so no reference document should appear")
	}
}

func TestPickerFiltersByExtension(t *testing.T) {
	m := newTestModel(t)
	dir := t.TempDir()
	write(t, filepath.Join(dir, "keep.md"), "# x")
	write(t, filepath.Join(dir, "skip.pdf"), "%PDF")

	m.fp = newPicker(openPickerMsg{mode: pickOpen, target: targetInput, start: dir, exts: []string{".md"}})
	resize(t, m, 80, 24)

	labels := make([]string, 0, len(m.fp.rows))
	for _, row := range m.fp.rows {
		labels = append(labels, row.label)
	}
	joined := strings.Join(labels, ",")
	if !strings.Contains(joined, "keep.md") {
		t.Errorf("markdown files should be listed, got %v", labels)
	}
	if strings.Contains(joined, "skip.pdf") {
		t.Errorf("filtered-out files should not be listed, got %v", labels)
	}

	// Toggling the filter has to bring the other file back.
	m.fp.showAll = true
	m.fp.reload("")
	found := false
	for _, row := range m.fp.rows {
		found = found || row.label == "skip.pdf"
	}
	if !found {
		t.Error("showing all files should reveal skip.pdf")
	}
}

func TestPickerOutputDefaultsToTheSameName(t *testing.T) {
	m := newTestModel(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "notes.md")
	write(t, input, "# x")
	m.form.setInput(input)

	target := t.TempDir()
	m.applyPick(pickResultMsg{target: targetOutput, path: target})

	want := filepath.Join(target, "notes.docx")
	if m.form.output != want {
		t.Errorf("output = %q, want %q", m.form.output, want)
	}
}

func TestStyleSelectionUpdatesTheForm(t *testing.T) {
	m := newTestModel(t)
	resize(t, m, 90, 30)

	chosen := style.Style{Name: "House style", Path: "/tmp/house.docx"}
	model, _ := m.Update(styleChosenMsg{style: chosen})
	m = model.(*Model)

	if m.form.style.Name != "House style" {
		t.Errorf("form style = %q, want House style", m.form.style.Name)
	}
	if got := m.form.request().Reference; got != "/tmp/house.docx" {
		t.Errorf("request reference = %q", got)
	}
	whole := strings.Join(m.form.render(90).lines, "\n")
	if !strings.Contains(whole, "--reference-doc=/tmp/house.docx") {
		t.Errorf("the command preview should mention the style\n%s", whole)
	}
}

func TestScrollingKeepsEverythingReachable(t *testing.T) {
	m := newTestModel(t)
	resize(t, m, 90, 20)
	m.form.setInput("/tmp/report.md")

	// Walk to the last row and make sure the cursor stays inside the window.
	for range m.form.fields {
		model, _ := m.Update(keyMsg("down"))
		m = model.(*Model)
		checkFrame(t, m.View(), 90, 20)
		if m.offset < 0 {
			t.Fatalf("scroll offset went negative: %d", m.offset)
		}
	}

	f := m.form.render(90)
	if f.cursorLine < 0 || f.cursorLine >= len(f.lines) {
		t.Fatalf("cursor line %d is outside the form (%d lines)", f.cursorLine, len(f.lines))
	}
	visible, _ := scroll(f.lines, 17, 0, f.cursorLine)
	if len(visible) != 17 {
		t.Errorf("scrolling returned %d lines, want 17", len(visible))
	}
}

func TestRunningWithoutPandocExplainsItself(t *testing.T) {
	m := newTestModel(t)
	resize(t, m, 90, 30)
	m.pandocMissing = true
	m.form.setInput("/tmp/whatever.md")

	model, _ := m.Update(startRunMsg{})
	m = model.(*Model)

	if m.current() != screenConvert {
		t.Error("a failed start should stay on the convert screen")
	}
	if !strings.Contains(m.View(), "pandoc") {
		t.Error("the user should be told pandoc is missing")
	}
}

func TestEveryScreenRendersWithinBounds(t *testing.T) {
	setups := map[string]func(*Model){
		"convert": func(*Model) {},
		"help": func(m *Model) {
			m.stack = append(m.stack, screenHelp)
		},
		"styles": func(m *Model) {
			m.stack = append(m.stack, screenStyles)
			m.lib.rows = []style.Style{
				{Name: "None", Builtin: true},
				{Name: "Academic Paper", Path: "/tmp/academic.docx", Size: 11264},
			}
		},
		"styles-naming": func(m *Model) {
			m.stack = append(m.stack, screenStyles)
			m.lib.rows = []style.Style{{Name: "None", Builtin: true}}
			m.lib.mode = stylesNaming
			m.lib.nameInput.Focus()
		},
		"picker": func(m *Model) {
			m.fp = newPicker(openPickerMsg{mode: pickSave, target: targetOutput, start: t.TempDir()})
			m.stack = append(m.stack, screenPicker)
		},
		"run": func(m *Model) {
			m.run = runner{
				command: "pandoc --from=markdown notes.md --to=docx --output=notes.docx",
				state:   runSucceeded,
				lines:   []string{"[WARNING] something", "[INFO] done"},
				output:  "/tmp/notes.docx",
			}
			m.stack = append(m.stack, screenRun)
		},
	}

	for _, size := range [][2]int{{100, 34}, {72, 20}, {120, 50}} {
		for name, setup := range setups {
			t.Run(fmt.Sprintf("%s@%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m := newTestModel(t)
				resize(t, m, size[0], size[1])
				setup(m)
				checkFrame(t, m.View(), size[0], size[1])
			})
		}
	}
}

func TestHelpListsTheRoadmap(t *testing.T) {
	m := newTestModel(t)
	resize(t, m, 100, 40)
	m.stack = append(m.stack, screenHelp)

	view := m.View()
	for _, want := range []string{"Roadmap", "Markdown -> HTML", "Where things live"} {
		if !strings.Contains(view, want) {
			t.Errorf("help is missing %q\n%s", want, view)
		}
	}
}

func TestChineseTextIsMeasuredByDisplayWidth(t *testing.T) {
	if got := lipgloss.Width(truncate("中文字符串测试", 6)); got > 6 {
		t.Errorf("truncate returned %d columns, want at most 6", got)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
