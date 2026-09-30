// Package pandoc locates and drives the pandoc binary.
package pandoc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// ErrNotFound is returned when no pandoc binary can be located.
var ErrNotFound = errors.New("pandoc not found")

// Client runs one specific pandoc executable.
type Client struct {
	Path string
}

// New wraps an explicit path.
func New(path string) *Client { return &Client{Path: path} }

// Detect locates a pandoc binary.
//
// A path the user spelled out -- on the command line or in an environment
// variable -- is honoured or reported as an error, never quietly swapped for a
// different binary. Once nobody has an opinion, detection falls back to $PATH
// and then to the usual install prefixes, because a terminal opened from the
// macOS dock often has a slimmer PATH than a login shell does.
func Detect(override string) (string, error) {
	for _, explicit := range []string{
		override,
		os.Getenv("PANDOC_TUI_PANDOC"),
		os.Getenv("PANDOC"),
	} {
		if strings.TrimSpace(explicit) == "" {
			continue
		}
		if isExecutable(explicit) {
			return explicit, nil
		}
		return "", fmt.Errorf("%w: %s is not an executable file", ErrNotFound, explicit)
	}

	if found, err := exec.LookPath("pandoc"); err == nil && isExecutable(found) {
		return found, nil
	}
	for _, candidate := range wellKnown() {
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", ErrNotFound
}

func wellKnown() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			`C:\Program Files\Pandoc\pandoc.exe`,
			`C:\Program Files (x86)\Pandoc\pandoc.exe`,
		}
	case "darwin":
		return []string{
			"/opt/homebrew/bin/pandoc",
			"/usr/local/bin/pandoc",
			"/opt/local/bin/pandoc",
			"/usr/bin/pandoc",
		}
	default:
		return []string{
			"/usr/local/bin/pandoc",
			"/usr/bin/pandoc",
			"/snap/bin/pandoc",
			"/var/lib/flatpak/exports/bin/pandoc",
			filepath.Join(os.Getenv("HOME"), ".local/bin/pandoc"),
		}
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

// Version returns the first line of `pandoc --version`, e.g. "pandoc 3.10.2".
func (c *Client) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, c.Path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("running %s --version: %w", c.Path, err)
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line), nil
}

// SupportedFormats asks pandoc which writers it has, which is how the app
// knows whether a roadmap conversion could run on this machine right now.
func (c *Client) SupportedFormats(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, c.Path, "--list-output-formats").Output()
	if err != nil {
		return nil, fmt.Errorf("running %s --list-output-formats: %w", c.Path, err)
	}
	return strings.Fields(string(out)), nil
}

// Stream marks which file descriptor a line arrived on.
type Stream int

const (
	Stdout Stream = iota
	Stderr
)

// Run executes pandoc, handing every output line to onLine. A nil onLine
// discards the output. The returned error is the process exit status, so a
// failing conversion is reported to the caller rather than swallowed.
func (c *Client) Run(ctx context.Context, args []string, onLine func(Stream, string)) error {
	cmd := exec.CommandContext(ctx, c.Path, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	pump := func(r *bufio.Scanner, s Stream) {
		defer wg.Done()
		r.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for r.Scan() {
			if onLine != nil {
				onLine(s, r.Text())
			}
		}
	}
	wg.Add(2)
	go pump(bufio.NewScanner(stdout), Stdout)
	go pump(bufio.NewScanner(stderr), Stderr)
	wg.Wait()

	return cmd.Wait()
}

// ReferenceDoc writes pandoc's stock reference.docx to dest. This is how the
// app seeds a new style: pandoc emits the template it would have used anyway,
// the user edits it in Word or LibreOffice, and the result comes back as a
// style they can pick from.
func (c *Client) ReferenceDoc(ctx context.Context, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.Path, "--print-default-data-file", "reference.docx")
	cmd.Stdout = f
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	closeErr := f.Close()

	if runErr != nil {
		os.Remove(dest)
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("pandoc --print-default-data-file reference.docx: %w: %s", runErr, detail)
		}
		return fmt.Errorf("pandoc --print-default-data-file reference.docx: %w", runErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

// CommandLine renders an argv as something the user can paste into a shell.
func (c *Client) CommandLine(args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quote(c.Path))
	for _, a := range args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s == "" {
		return "''"
	}
	// A tilde only matters to the shell at the start of a word, and quoting it
	// would stop the expansion the reader expects.
	if !strings.ContainsAny(s, " \t\n\"'\\$`&|;<>()*?[]{}#!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
