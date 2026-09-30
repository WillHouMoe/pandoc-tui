package pandoc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requirePandoc skips the test when the machine has no pandoc, so the pure
// unit tests still run on a bare checkout.
func requirePandoc(t *testing.T) *Client {
	t.Helper()
	path, err := Detect("")
	if err != nil {
		t.Skip("pandoc is not installed")
	}
	return New(path)
}

func TestVersionReportsTheBinary(t *testing.T) {
	client := requirePandoc(t)
	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "pandoc ") {
		t.Errorf("unexpected version line %q", version)
	}
}

func TestReferenceDocIsARealDocx(t *testing.T) {
	client := requirePandoc(t)
	dest := filepath.Join(t.TempDir(), "nested", "reference.docx")

	if err := client.ReferenceDoc(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	// Every .docx is a zip archive, so it starts with the local file header
	// "PK". Anything else means pandoc wrote an error message instead.
	if len(raw) < 2 || raw[0] != 'P' || raw[1] != 'K' {
		t.Fatalf("expected a zip archive, got %d bytes starting with %q", len(raw), string(raw[:min(2, len(raw))]))
	}
	if int64(len(raw)) < 4096 {
		t.Errorf("reference document looks too small: %d bytes", len(raw))
	}
}

func TestRunStreamsAndProducesOutput(t *testing.T) {
	client := requirePandoc(t)
	dir := t.TempDir()
	input := filepath.Join(dir, "notes.md")
	output := filepath.Join(dir, "notes.docx")
	if err := os.WriteFile(input, []byte("# Title\n\nSome *text*.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var seen []string
	err := client.Run(context.Background(), []string{
		"--from=markdown", "--to=docx", "--standalone",
		input, "--output=" + output,
	}, func(_ Stream, line string) {
		seen = append(seen, line)
	})
	if err != nil {
		t.Fatalf("run: %v (output: %v)", err, seen)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("pandoc reported success but wrote an empty file")
	}
}

func TestRunReportsFailures(t *testing.T) {
	client := requirePandoc(t)
	err := client.Run(context.Background(), []string{
		"--from=markdown", "--to=not-a-real-writer", "in.md",
	}, nil)
	if err == nil {
		t.Error("an unknown writer should make pandoc fail")
	}
}

func TestCommandLineQuotesAwkwardPaths(t *testing.T) {
	client := New("/opt/homebrew/bin/pandoc")
	got := client.CommandLine([]string{"--output=My Documents/notes.docx", "plain.md"})
	want := "/opt/homebrew/bin/pandoc '--output=My Documents/notes.docx' plain.md"
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestDetectRejectsDirectories(t *testing.T) {
	if _, err := Detect("/tmp"); err == nil {
		t.Error("a directory must not be treated as the pandoc binary")
	}
}
