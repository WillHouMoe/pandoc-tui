package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// frame is what a screen hands back to the shell: everything it wants drawn,
// where the cursor ended up, and which keys are worth advertising.
type frame struct {
	lines      []string
	cursorLine int
	footer     []keyHint
}

// panel draws a rounded box exactly width columns wide, with the title spliced
// into the top border so the header costs no content row. Anything wider than
// the box is truncated rather than allowed to break the frame.
func panel(width int, title string, body []string) []string {
	if width < 10 {
		width = 10
	}
	inner := width - 2
	out := make([]string, 0, len(body)+2)
	out = append(out, panelTop(inner, title))
	for _, line := range body {
		out = append(out, panelLine(inner, line))
	}
	out = append(out, stBorder.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

func panelTop(inner int, title string) string {
	if strings.TrimSpace(title) == "" {
		return stBorder.Render("╭" + strings.Repeat("─", inner) + "╮")
	}
	label := " " + truncate(title, inner-5) + " "
	dashes := inner - lipgloss.Width(label) - 1
	if dashes < 0 {
		dashes = 0
	}
	return stBorder.Render("╭─") +
		stPanelTitle.Render(label) +
		stBorder.Render(strings.Repeat("─", dashes)+"╮")
}

func panelLine(inner int, content string) string {
	room := inner - 2
	if room < 0 {
		room = 0
	}
	pad := room - lipgloss.Width(content)
	if pad < 0 {
		// Stripping first keeps a styled string from being cut in the middle
		// of an escape sequence; losing the colour beats a garbled frame.
		content = truncate(ansi.Strip(content), room)
		pad = room - lipgloss.Width(content)
	}
	if pad < 0 {
		pad = 0
	}
	return stBorder.Render("│") + " " + content + strings.Repeat(" ", pad) + " " + stBorder.Render("│")
}

// truncate shortens plain text to width columns. It operates on runes and
// accounts for double width characters; it deliberately does not understand
// ANSI escapes, so callers truncate before styling.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := runeWidth(r)
		if used+w > width-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteString("…")
	return b.String()
}

func runeWidth(r rune) int {
	return ansi.StringWidth(string(r))
}

// wrap breaks plain text into lines of at most width columns. Words longer
// than the line -- a file path usually is -- are broken rather than allowed to
// overflow the panel, because the command preview has to stay readable.
func wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	for _, paragraph := range strings.Split(s, "\n") {
		paragraph = strings.TrimRight(paragraph, " ")
		if paragraph == "" {
			out = append(out, "")
			continue
		}

		line := ""
		for _, word := range strings.Fields(paragraph) {
			for lipgloss.Width(word) > width {
				head, tail := splitAtWidth(word, width)
				if line != "" {
					out = append(out, line)
					line = ""
				}
				out = append(out, head)
				word = tail
			}
			switch {
			case line == "":
				line = word
			case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		out = append(out, line)
	}
	return out
}

// splitAtWidth cuts a string after the rune that fills width columns.
func splitAtWidth(s string, width int) (string, string) {
	used := 0
	for i, r := range s {
		w := runeWidth(r)
		if used+w > width {
			return s[:i], s[i:]
		}
		used += w
	}
	return s, ""
}

// labelled renders "  Label        value" with a marker in the gutter for the
// focused row. Labels are padded to the same column so a form reads as a
// table.
func labelled(width, labelWidth int, label, value string, focused bool) string {
	mark := " "
	if focused {
		mark = stMarker.Render("▸")
	}

	labelStyle := stLabel
	valueStyle := stValue
	if focused {
		labelStyle = stLabelHot
		valueStyle = stValueHot
	}

	labelCell := padRight(label, labelWidth)
	prefix := mark + " " + labelStyle.Render(labelCell) + "  "
	room := width - lipgloss.Width(prefix)
	if room < 1 {
		room = 1
	}
	if lipgloss.Width(value) > room {
		value = truncate(ansi.Strip(value), room)
		return prefix + valueStyle.Render(value)
	}
	return prefix + valueStyle.Render(value)
}

// blockBuilder accumulates rendered lines while remembering where each block
// started, which is how a screen reports the cursor's row for scrolling.
type blockBuilder struct {
	lines []string
}

func (b *blockBuilder) add(lines ...string) {
	b.lines = append(b.lines, lines...)
}

func (b *blockBuilder) blank() {
	b.lines = append(b.lines, "")
}

// panel appends a box and returns the index of its first line.
func (b *blockBuilder) panel(width int, title string, body []string) int {
	start := len(b.lines)
	b.lines = append(b.lines, panel(width, title, body)...)
	return start
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TB", value/unit)
}

// banner draws the app header: name on the left, context on the right.
func banner(width int, left, right string) string {
	leftText := stTitle.Render(left)
	rightText := stFaint.Render(right)
	gap := width - lipgloss.Width(leftText) - lipgloss.Width(rightText)
	if gap < 1 {
		rightText = ""
		gap = width - lipgloss.Width(leftText)
	}
	if gap < 0 {
		gap = 0
	}
	return leftText + strings.Repeat(" ", gap) + rightText
}

// scroll clips lines to a window of height rows, keeping cursorLine visible.
// It returns the visible lines and the offset it settled on, so callers can
// carry the offset into the next frame instead of snapping back to the top.
func scroll(lines []string, height, offset, cursorLine int) ([]string, int) {
	if height <= 0 || len(lines) <= height {
		return lines, 0
	}
	maxOffset := len(lines) - height
	if cursorLine >= 0 {
		if cursorLine < offset {
			offset = cursorLine
		}
		if cursorLine >= offset+height {
			offset = cursorLine - height + 1
		}
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	return lines[offset : offset+height], offset
}

// footer renders the key hints as a single dim line.
func renderFooter(width int, hints []keyHint) string {
	if len(hints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		if h.keys == "" || h.desc == "" {
			continue
		}
		parts = append(parts, stKeyCap.Render(h.keys)+" "+stKeyDesc.Render(h.desc))
	}
	line := strings.Join(parts, stHint.Render("   "))
	if lipgloss.Width(line) <= width {
		return line
	}
	// Drop hints from the right until the bar fits; the most important keys are
	// listed first by every screen.
	for len(parts) > 1 {
		parts = parts[:len(parts)-1]
		line = strings.Join(parts, stHint.Render("   "))
		if lipgloss.Width(line) <= width {
			return line
		}
	}
	return truncate(ansi.Strip(parts[0]), width)
}
