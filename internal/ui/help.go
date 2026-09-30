package ui

import (
	"github.com/WillHouMoe/pandoc-tui/internal/config"
	"github.com/WillHouMoe/pandoc-tui/internal/convert"
)

// helpFrame spells out every key in the app plus the roadmap, so the help
// screen doubles as the place to look when wondering what to add next.
func helpFrame(width int, deps Deps) frame {
	var b blockBuilder
	contentWidth := width - 4

	b.panel(width, "What this is", wrap(
		"pandoc-tui writes pandoc commands for you. Pick a conversion, pick the files, "+
			"pick a style, and read the command it is about to run before it runs it.", contentWidth))
	b.blank()

	b.panel(width, "Convert screen", []string{
		row("↑ ↓  /  k j", "move between rows"),
		row("← →  /  h l", "change a value, toggle a switch"),
		row("enter", "choose a file, open the style library, toggle an option"),
		row("b", "browse for a file from any row"),
		row("ctrl+r", "run the conversion"),
		row("tab", "jump to the next row"),
	})
	b.blank()

	b.panel(width, "Style library", []string{
		row("enter", "use the highlighted style for the next conversion"),
		row("n", "create a style from pandoc's reference.docx"),
		row("i", "import an existing .docx as a style"),
		row("r", "rename,  d  delete,  o  open in your editor"),
		row("f", "reveal the file in your file manager"),
	})
	b.blank()

	b.panel(width, "File picker", []string{
		row("enter", "open a folder or choose a file"),
		row("← / esc", "go up a level, or cancel"),
		row(".", "show every file type, not just the ones that fit"),
	})
	b.blank()

	b.panel(width, "Where things live", []string{
		row("config", truncate(shortenPath(configPath()), contentWidth-12)),
		row("styles", truncate(shortenPath(config.StyleDirOrEmpty()), contentWidth-12)),
		row("pandoc", truncate(shortenPath(deps.Pandoc.Path), contentWidth-12)),
	})
	b.blank()

	// The roadmap is the honest version of "extensible": these are the
	// conversions the architecture is shaped for, none of them wired up yet.
	roadmap := convert.Roadmap()
	rows := make([]string, 0, len(roadmap))
	for _, p := range roadmap {
		rows = append(rows, row(p.Label, p.Because))
	}
	b.panel(width, "Roadmap", rows)

	return frame{
		lines:      b.lines,
		cursorLine: -1,
		footer: []keyHint{
			{keys: "esc", desc: "back"},
			{keys: "q", desc: "quit"},
		},
	}
}

func row(left, right string) string {
	return stValue.Render(padRight(left, 16)) + stFaint.Render(right)
}

func configPath() string {
	path, err := config.Path()
	if err != nil {
		return "unavailable"
	}
	return path
}
