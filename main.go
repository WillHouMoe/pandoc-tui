// Command pandoc-tui is a terminal user interface for pandoc.
//
// It exists because pandoc's command line is a toolbox, not a workflow: the
// flags you need for "markdown to a styled Word document" are easy to forget
// and easy to get subtly wrong. This app makes that workflow visible, shows
// you the command it built, and stays out of the way afterwards.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WillHouMoe/pandoc-tui/internal/config"
	"github.com/WillHouMoe/pandoc-tui/internal/convert"
	"github.com/WillHouMoe/pandoc-tui/internal/pandoc"
	"github.com/WillHouMoe/pandoc-tui/internal/style"
	"github.com/WillHouMoe/pandoc-tui/internal/ui"
)

// version is stamped at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

// init falls back to the version Go records in the binary. Without this,
// `go install ...@v0.1.0` would report "dev", which tells the user nothing
// about what they installed.
func init() {
	if version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	// Local `go build` reports "(devel)"; only a real module version helps.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		version = v
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pandoc-tui:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		showVersion = flag.Bool("version", false, "print the version and exit")
		pandocPath  = flag.String("pandoc", "", "path to the pandoc binary (default: auto-detect)")
		styleDir    = flag.String("styles", "", "directory holding style files (default: the user config directory)")
		printPaths  = flag.Bool("paths", false, "print the config and style directories, then exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("pandoc-tui", version)
		return nil
	}

	dir, err := config.StyleDir()
	if err != nil {
		return err
	}
	if *styleDir != "" {
		dir = *styleDir
	}

	if *printPaths {
		cfg, _ := config.Path()
		fmt.Println("config:", cfg)
		fmt.Println("styles:", dir)
		return nil
	}

	cfg := config.Load()

	// An explicit flag beats a remembered path; the remembered path beats
	// whatever happens to be first on $PATH.
	if *pandocPath == "" && cfg.PandocPath != "" {
		*pandocPath = cfg.PandocPath
	}
	binary, detectErr := pandoc.Detect(*pandocPath)
	if detectErr != nil {
		if !errors.Is(detectErr, pandoc.ErrNotFound) {
			return detectErr
		}
		// Keep going: the UI explains what is missing and how to fix it
		// instead of dumping the user back into the shell.
		binary = "pandoc"
	}
	if *pandocPath != "" && binary != cfg.PandocPath {
		cfg.PandocPath = binary
		_ = cfg.Save()
	}

	client := pandoc.New(binary)
	// A style library that cannot be created is not fatal: converting still
	// works, only the list of styles will be empty. Say so instead of refusing
	// to start.
	var styleErr error
	if err := os.MkdirAll(dir, 0o755); err != nil {
		styleErr = fmt.Errorf("cannot create a style library at %s (use --styles to pick another place): %w", dir, err)
	}

	model := ui.New(ui.Deps{
		Converters:  convert.Default(),
		Pandoc:      client,
		Styles:      style.New(dir, client.ReferenceDoc),
		Config:      cfg,
		Version:     version,
		PandocError: detectErr,
		StyleError:  styleErr,
	})

	// No mouse support on purpose: the terminal's own text selection has to
	// keep working so the generated pandoc command can be copied out.
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
