package convert

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestOptionArgs(t *testing.T) {
	toggle := Option{Key: "toc", Kind: KindToggle, Flag: "--toc"}
	if got := toggle.Args("true"); !slices.Equal(got, []string{"--toc"}) {
		t.Errorf("toggled on: got %q", got)
	}
	if got := toggle.Args("false"); got != nil {
		t.Errorf("toggled off should add nothing, got %q", got)
	}

	text := Option{Key: "depth", Kind: KindNumber, Flag: "--toc-depth"}
	if got := text.Args("3"); !slices.Equal(got, []string{"--toc-depth=3"}) {
		t.Errorf("number: got %q", got)
	}
	if got := text.Args("  "); got != nil {
		t.Errorf("empty value should add nothing, got %q", got)
	}
}

func TestOptionCycling(t *testing.T) {
	choice := Option{
		Kind:    KindChoice,
		Default: "b",
		Choices: []Choice{{Value: "a"}, {Value: "b"}, {Value: "c"}},
	}
	if got := choice.Next("b"); got != "c" {
		t.Errorf("next from b: got %q, want c", got)
	}
	if got := choice.Next("c"); got != "a" {
		t.Errorf("next should wrap to a, got %q", got)
	}
	if got := choice.Prev("a"); got != "c" {
		t.Errorf("prev should wrap to c, got %q", got)
	}

	stepper := Option{Kind: KindNumber, Default: "3", Min: 1, Max: 6}
	if got := stepper.Prev("1"); got != "1" {
		t.Errorf("stepper should stop at its floor, got %q", got)
	}
	if got := stepper.Next("6"); got != "6" {
		t.Errorf("stepper should stop at its ceiling, got %q", got)
	}
	if got := stepper.Next(""); got != "4" {
		t.Errorf("empty value should fall back to the default, got %q", got)
	}
}

// TestMarkdownDOCXArgs pins the exact command line the prototype builds. If a
// flag is ever renamed by accident, this is where it shows up.
func TestMarkdownDOCXArgs(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "notes.md")
	reference := filepath.Join(dir, "styled.docx")
	for _, path := range []string{input, reference} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	converter := markdownToDOCX{}
	req := Request{
		Input:     input,
		Output:    filepath.Join(dir, "notes.docx"),
		Reference: reference,
		Values: map[string]string{
			"toc":             "true",
			"toc-depth":       "2",
			"number-sections": "false",
			"highlight-style": "tango",
			"wrap":            "none",
			"metadata-title":  "Quarterly report",
			"resource-path":   dir,
		},
	}

	if err := converter.Validate(req); err != nil {
		t.Fatalf("validate: %v", err)
	}
	args, err := converter.Args(req)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(args, " ")

	for _, want := range []string{
		"--from=markdown",
		"--to=docx",
		"--standalone",
		"--reference-doc=" + reference,
		"--toc",
		"--toc-depth=2",
		"--highlight-style=tango",
		"--wrap=none",
		"--metadata=title=Quarterly report",
		"--resource-path=" + dir,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("args missing %q\nfull: %s", want, line)
		}
	}
	if strings.Contains(line, "--number-sections") {
		t.Errorf("a disabled toggle must not reach the command line: %s", line)
	}
}

func TestMarkdownDOCXValidate(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	converter := markdownToDOCX{}

	cases := map[string]Request{
		"no input":      {Output: filepath.Join(dir, "out.docx")},
		"no output":     {Input: input},
		"same file":     {Input: input, Output: input},
		"missing input": {Input: filepath.Join(dir, "nope.md"), Output: filepath.Join(dir, "out.docx")},
		"bad style": {
			Input:     input,
			Output:    filepath.Join(dir, "out.docx"),
			Reference: filepath.Join(dir, "nope.docx"),
		},
	}
	for name, req := range cases {
		if err := converter.Validate(req); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	registry := NewRegistry()
	registry.Register(markdownToDOCX{})
	registry.Register(markdownToDOCX{})
	if registry.Len() != 1 {
		t.Errorf("duplicate IDs should not be registered twice, got %d", registry.Len())
	}
	if _, ok := registry.Lookup("markdown-docx"); !ok {
		t.Error("the registered converter should be findable by ID")
	}
}

func TestDefaultRegistryIsUsable(t *testing.T) {
	registry := Default()
	converter, ok := registry.First()
	if !ok {
		t.Fatal("the shipped registry should not be empty")
	}
	values := DefaultValues(converter)
	if len(values) != len(converter.Options()) {
		t.Errorf("every option should get a default, got %d of %d", len(values), len(converter.Options()))
	}
}
