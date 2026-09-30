# pandoc-tui

A terminal UI for the pandoc commands you actually use.

[中文说明](README.zh-CN.md)

pandoc is a phenomenal tool with a command line that reads like a toolbox.
Turning a Markdown file into a properly styled Word document means remembering
`--reference-doc`, `--toc-depth`, `--highlight-style` and half a dozen other
flags you touch twice a year. pandoc-tui puts that workflow on one screen: pick
the conversion, pick the files, pick a style, read the exact command before it
runs, then get out of the way.

```
  pandoc-tui                                                          pandoc 3.10.2

  ╭─ Conversion  Markdown -> Word (.docx) ──────────────────────────────────────╮
  │ Convert a Markdown file into a Word document that follows a reference style.│
  │ 5 conversions are planned for later — press ? to see the roadmap           │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ Source ────────────────────────────────────────────────────────────────────╮
  │ ▸ Input              ~/notes/quarterly-report.md                            │
  │   Output             ~/notes/quarterly-report.docx                          │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ Style ─────────────────────────────────────────────────────────────────────╮
  │   Reference          House style                                            │
  │                      ~/.config/pandoc-tui/styles/House style.docx           │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ Options ───────────────────────────────────────────────────────────────────╮
  │   Table of contents  on  space to toggle                                    │
  │   TOC depth          3  ‹ ›                                                 │
  │   Number sections    on  space to toggle                                    │
  │   Code highlight     pygments  ‹ ›                                          │
  │   Title              Quarterly report                                       │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ╭─ pandoc command ────────────────────────────────────────────────────────────╮
  │ pandoc --from=markdown --to=docx --standalone ~/notes/quarterly-report.md    │
  │ --output=~/notes/quarterly-report.docx --reference-doc=~/.config/.../House   │
  │ style.docx --toc --toc-depth=3 --number-sections                            │
  ╰────────────────────────────────────────────────────────────────────────────╯

  ↑↓ move   ←→ change   enter choose   ctrl+r convert   ? help   q quit
```

## Status

Early prototype, but a working one. Exactly one conversion is wired up:

| From     | To                      | Notes                                  |
| -------- | ----------------------- | -------------------------------------- |
| Markdown | Word (`.docx`)          | with reference styles, TOC, highlighting |

The architecture is built for more. Adding a conversion means writing one file
that implements an interface; no screen has to change. See
[Adding a conversion](#adding-a-conversion).

## Requirements

- pandoc 3.x on your `PATH` (the app finds it, and tells you how to fix it if
  it cannot)
- A UTF-8 terminal
- macOS, Linux or Windows

## Install

```sh
go install github.com/WillHouMoe/pandoc-tui@latest
```

Or from a checkout:

```sh
git clone https://github.com/WillHouMoe/pandoc-tui
cd pandoc-tui
make build      # writes bin/pandoc-tui
```

## Usage

```sh
pandoc-tui
```

| Key            | What it does                                          |
| -------------- | ----------------------------------------------------- |
| `↑` `↓` / `k` `j` | move between rows                                  |
| `←` `→` / `h` `l` | change a value, flip a switch                      |
| `enter`        | choose a file, open the style library, toggle an option |
| `b`            | browse for a file                                     |
| `ctrl+r`       | convert                                               |
| `?`            | help, including the roadmap                           |
| `q`            | quit                                                  |

Useful flags:

```sh
pandoc-tui --pandoc /opt/homebrew/bin/pandoc   # pin the binary
pandoc-tui --styles ~/Documents/docx-styles    # keep styles in a project folder
pandoc-tui --paths                             # print where config and styles live
```

## Styles

This is the part worth the trouble. A "style" is a Word reference document:
pandoc's own `reference.docx`, restyled by you in Word, Pages or LibreOffice.
Everything about the output that is not the text — fonts, heading sizes,
spacing, the look of code blocks — comes from that file.

In the app:

1. On the convert screen, `enter` on the **Reference** row opens the library.
2. `n` creates a style. pandoc-tui copies the reference document pandoc would
   have used anyway and opens it in your editor.
3. Restyle it, save, come back. Pick it with `enter`.
4. `i` imports a `.docx` you already have; `r` renames, `d` deletes, `o` opens,
   `f` reveals it in your file manager.

Styles live in the user config directory (`~/.config/pandoc-tui/styles` on
Linux, `~/Library/Application Support/pandoc-tui/styles` on macOS). Move the
folder, or pass `--styles`, if you would rather keep them next to a project.

## Adding a conversion

Everything the UI draws comes from the registry in `internal/convert`. Write a
type that implements `Converter`, register it, and the new conversion appears
with its own options form, its own file filters and its own command preview.

```go
// internal/convert/markdown_html.go
package convert

type markdownToHTML struct{}

func (markdownToHTML) ID() string              { return "markdown-html" }
func (markdownToHTML) Label() string           { return "Markdown -> HTML" }
func (markdownToHTML) Summary() string         { return "A standalone HTML5 page." }
func (markdownToHTML) From() Format            { return Markdown }
func (markdownToHTML) To() Format              { return HTML }
func (markdownToHTML) WantsReferenceDoc() bool { return false }

func (markdownToHTML) Options() []Option {
	return []Option{
		{Key: "css", Label: "Stylesheet", Help: "Path to a CSS file.",
			Kind: KindText, Flag: "--css"},
		{Key: "embed", Label: "Embed resources", Kind: KindToggle,
			Default: "true", Flag: "--embed-resources"},
	}
}

func (markdownToHTML) Validate(req Request) error {
	if err := CheckInput(req.Input); err != nil {
		return err
	}
	if req.Output == "" {
		return ErrNoOutput
	}
	return nil
}

func (c markdownToHTML) Args(req Request) ([]string, error) {
	args := BaseArgs(req, Markdown, HTML, "--standalone")
	for _, o := range c.Options() {
		args = append(args, o.Args(req.Value(o))...)
	}
	return args, nil
}
```

Then add it to the default registry:

```go
func Default() *Registry {
	return NewRegistry(
		markdownToDOCX{},
		markdownToHTML{},   // <- one line
	)
}
```

The UI needs no changes: `Options()` becomes the form, `From()`/`To()` drive
the file picker filters and the derived output name, and `Args()` feeds the
command preview.

## How it is put together

```
main.go                 wiring: find pandoc, load config, start the program
internal/convert/       the extension point: formats, options, converters
internal/pandoc/        locating pandoc, streaming a run, exporting reference.docx
internal/style/         the style library, which is really just a folder
internal/config/        the handful of choices worth remembering
internal/ui/            Bubble Tea screens, one file each
```

The UI never talks to pandoc directly. It renders whatever the registry
exposes, which is what keeps a new conversion from turning into a UI project.

## Roadmap

The conversions the architecture is already shaped for, in rough order:

- Markdown -> HTML (needs the HTML writer options)
- Markdown -> EPUB (needs cover and metadata handling)
- Markdown -> PDF (needs a LaTeX engine probe)
- Word -> Markdown (needs media extraction)
- A format matrix, replacing the single fixed source/target pair

Press `?` inside the app to see the same list.

## Development

```sh
make check      # gofmt, go vet, go test
make preview    # print every screen as plain text, no terminal needed
```

The test suite drives the real app: it creates a style with the installed
pandoc, converts a Markdown file through the same code path the UI uses, and
checks the resulting `.docx`. Tests that need pandoc skip themselves when it is
not installed, so a bare checkout still tests.

## License

MIT. See [LICENSE](LICENSE).
