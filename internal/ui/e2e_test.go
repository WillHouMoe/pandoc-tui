package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WillHouMoe/pandoc-tui/internal/config"
	"github.com/WillHouMoe/pandoc-tui/internal/convert"
	"github.com/WillHouMoe/pandoc-tui/internal/pandoc"
	"github.com/WillHouMoe/pandoc-tui/internal/style"
)

// drain runs a command and everything it spawns, feeding each message back
// into the model. It is the test-side equivalent of the Bubble Tea event loop,
// so a test can drive the app exactly the way a keyboard would.
func drain(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 10000 {
			t.Fatal("the command queue never settled")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		model, follow := m.Update(msg)
		updated, ok := model.(*Model)
		if !ok {
			t.Fatalf("Update returned %T, want *Model", model)
		}
		*m = *updated
		if follow != nil {
			queue = append(queue, follow)
		}
	}
}

// TestEndToEndStyleThenConversion drives the whole product the way a user
// does: make a style with the installed pandoc, pick it, convert a Markdown
// file, then confirm the .docx that came out is a real Word file.
func TestEndToEndStyleThenConversion(t *testing.T) {
	binary, err := pandoc.Detect("")
	if err != nil {
		t.Skip("pandoc is not installed")
	}

	// Never let a test launch Word.
	openFile = func(string) error { return nil }
	t.Cleanup(func() { openFile = openInApp })

	home := t.TempDir()
	t.Setenv("PANDOC_TUI_HOME", home)
	styleDir, err := config.StyleDir()
	if err != nil {
		t.Fatal(err)
	}

	client := pandoc.New(binary)
	m := New(Deps{
		Converters: convert.Default(),
		Pandoc:     client,
		Styles:     style.New(styleDir, client.ReferenceDoc),
		Config:     &config.Config{Values: map[string]map[string]string{}},
		Version:    "test",
	})
	resize(t, m, 100, 34)
	drain(t, m, m.Init())
	if m.pandocMissing {
		t.Fatalf("pandoc was detected at %s but the model still reports it missing", binary)
	}

	// --- create a style ---------------------------------------------------

	m.stack = append(m.stack, screenStyles)
	drain(t, m, m.lib.reload())
	if len(m.lib.rows) != 1 {
		t.Fatalf("a fresh library should hold only the built-in entry, got %d", len(m.lib.rows))
	}

	m.lib.mode = stylesNaming
	m.lib.nameInput.SetValue("House style")
	next, cmd := m.lib.submitName()
	m.lib = next
	drain(t, m, cmd)

	if len(m.lib.rows) != 2 {
		t.Fatalf("the new style should be listed, got %d rows: %+v", len(m.lib.rows), m.lib.rows)
	}
	created := m.lib.rows[1]
	if created.Name != "House style" {
		t.Errorf("style name = %q, want House style", created.Name)
	}
	info, err := os.Stat(created.Path)
	if err != nil {
		t.Fatalf("the style file should exist: %v", err)
	}
	if info.Size() < 4096 {
		t.Errorf("the style should be a real reference document, got %d bytes", info.Size())
	}

	// --- convert with it --------------------------------------------------

	docs := filepath.Join(home, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(docs, "report.md")
	body := "# Quarterly report\n\nSome *text*, a list:\n\n- one\n- two\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	if err := os.WriteFile(input, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	m.form.setStyle(created)
	m.form.setInput(input)
	m.pop() // leave the style screen
	if got, want := m.form.output, filepath.Join(docs, "report.docx"); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	model, cmd := m.Update(startRunMsg{})
	m = model.(*Model)
	drain(t, m, cmd)

	if m.run.state != runSucceeded {
		t.Fatalf("conversion did not succeed (state %d, err %v)\n%s",
			m.run.state, m.run.err, strings.Join(m.run.lines, "\n"))
	}
	if !strings.Contains(m.run.command, "--reference-doc="+created.Path) {
		t.Errorf("the chosen style should be on the command line:\n%s", m.run.command)
	}

	raw, err := os.ReadFile(m.form.output)
	if err != nil {
		t.Fatalf("reading the converted file: %v", err)
	}
	if len(raw) < 4096 || raw[0] != 'P' || raw[1] != 'K' {
		t.Errorf("the conversion did not produce a Word file (%d bytes)", len(raw))
	}
}

// TestEndToEndWithoutAConfigKeepsGoing checks the app survives a first run on
// a machine where nothing has been configured yet.
func TestEndToEndWithoutAConfigKeepsGoing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PANDOC_TUI_HOME", home)

	dir, err := config.StyleDir()
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		Converters: convert.Default(),
		Pandoc:     pandoc.New("pandoc-does-not-exist-here"),
		Styles:     style.New(dir, nil),
		Config:     &config.Config{},
		Version:    "test",
	})
	resize(t, m, 80, 24)

	if view := m.View(); !strings.Contains(view, "Input") {
		t.Errorf("the convert screen should still render\n%s", view)
	}
}
