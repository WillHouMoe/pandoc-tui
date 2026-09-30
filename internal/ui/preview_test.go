package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WillHouMoe/pandoc-tui/internal/style"
)

// TestPreview dumps every screen so the layout can be eyeballed without a
// terminal attached:
//
//	PANDOC_TUI_PREVIEW=1 go test ./internal/ui -run TestPreview -v
//
// It is skipped by default so ordinary test runs stay quiet.
func TestPreview(t *testing.T) {
	if os.Getenv("PANDOC_TUI_PREVIEW") == "" {
		t.Skip("set PANDOC_TUI_PREVIEW=1 to print the screens")
	}

	const width, height = 100, 34
	home := t.TempDir()
	notes := filepath.Join(home, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}

	// Paths are written the way they would look on a real machine, so the
	// preview reads like a screenshot instead of a temp directory listing.
	input := "~/notes/quarterly-report.md"
	output := "~/notes/quarterly-report.docx"
	stylePath := "~/.config/pandoc-tui/styles/House style.docx"

	build := func(t *testing.T) *Model {
		m := newTestModel(t)
		resize(t, m, width, height)
		m.form.setInput(input)
		m.form.setStyle(style.Style{Name: "House style", Path: stylePath, Size: 11264})
		m.form.values["number-sections"] = "true"
		m.form.values["metadata-title"] = "Quarterly report"
		return m
	}

	t.Run("convert", func(t *testing.T) {
		t.Log("\n" + build(t).View())
	})

	t.Run("convert-scrolled", func(t *testing.T) {
		m := build(t)
		for range 5 {
			model, _ := m.Update(keyMsg("down"))
			m = model.(*Model)
		}
		t.Log("\n" + m.View())
	})

	t.Run("styles", func(t *testing.T) {
		m := build(t)
		m.stack = append(m.stack, screenStyles)
		edited := time.Date(2026, 9, 28, 11, 2, 0, 0, time.UTC)
		m.lib.rows = []style.Style{
			{Name: "None", Builtin: true},
			{Name: "Academic Paper", Path: "~/styles/Academic Paper.docx", Size: 11264, ModTime: edited},
			{Name: "House style", Path: stylePath, Size: 12480, ModTime: edited},
			{Name: "论文模板", Path: "~/styles/论文模板.docx", Size: 9200, ModTime: edited},
		}
		m.lib.cursor = 2
		t.Log("\n" + m.View())
	})

	t.Run("styles-new", func(t *testing.T) {
		m := build(t)
		m.stack = append(m.stack, screenStyles)
		m.lib.rows = []style.Style{{Name: "None", Builtin: true}}
		m.lib.mode = stylesNaming
		m.lib.nameInput.SetValue("Conference paper")
		m.lib.nameInput.Focus()
		t.Log("\n" + m.View())
	})

	t.Run("picker", func(t *testing.T) {
		m := build(t)
		for _, name := range []string{"draft.md", "notes.md", "outline.pdf"} {
			if err := os.WriteFile(filepath.Join(notes, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(filepath.Join(notes, "archive"), 0o755); err != nil {
			t.Fatal(err)
		}
		m.fp = newPicker(openPickerMsg{mode: pickOpen, target: targetInput, start: notes, exts: []string{".md"}})
		m.stack = append(m.stack, screenPicker)
		t.Log("\n" + m.View())
	})

	t.Run("run", func(t *testing.T) {
		m := build(t)
		m.run = runner{
			command:    "pandoc --from=markdown --to=docx --standalone " + input + " --output=" + output + " --reference-doc=" + stylePath,
			state:      runSucceeded,
			output:     output,
			outputSize: 24832,
			hasOutput:  true,
		}
		m.stack = append(m.stack, screenRun)
		t.Log("\n" + m.View())
	})

	t.Run("run-failed", func(t *testing.T) {
		m := build(t)
		m.run = runner{
			command: "pandoc --from=markdown --to=docx --standalone " + input +
				" --output=" + output + " --reference-doc=" + stylePath,
			state:  runFailed,
			err:    errors.New("exit status 1"),
			output: output,
			lines: []string{
				`[WARNING] Could not fetch resource chart.png: replacing image with description`,
				"pandoc: " + input + ": openBinaryFile: does not exist (No such file or directory)",
			},
		}
		m.stack = append(m.stack, screenRun)
		t.Log("\n" + m.View())
	})

	t.Run("help", func(t *testing.T) {
		m := build(t)
		m.stack = append(m.stack, screenHelp)
		t.Log("\n" + m.View())
	})
}
