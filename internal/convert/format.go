// Package convert is the extension point of pandoc-tui.
//
// Every conversion the app can perform is a Converter registered in a
// Registry. The user interface builds itself from whatever the registry
// exposes, so growing pandoc-tui means adding one file here and one line in
// Registry -- no screen ever has to change.
package convert

import "strings"

// Format describes one side of a conversion: a document format pandoc knows
// how to read or write. ID doubles as pandoc's own name for the format, so it
// can be handed straight to --from / --to.
type Format struct {
	ID     string   // pandoc name, e.g. "markdown"
	Name   string   // human readable, e.g. "Markdown"
	Exts   []string // file extensions, dots included
	Binary bool     // true when the output is not plain text
	Note   string   // one line hint shown in the UI
}

func (f Format) String() string { return f.Name }

// Matches reports whether path looks like this format, judged by extension.
func (f Format) Matches(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range f.Exts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// Ext is the extension appended when pandoc-tui derives an output file name.
func (f Format) Ext() string {
	if len(f.Exts) == 0 {
		return ""
	}
	return f.Exts[0]
}

// The catalogue of formats pandoc-tui knows about. Listing a format here only
// gives it a name and an extension; it becomes reachable in the UI once a
// Converter is registered for it.
var (
	Markdown = Format{
		ID:   "markdown",
		Name: "Markdown",
		Exts: []string{".md", ".markdown", ".mdown", ".mkd"},
		Note: "pandoc markdown; CommonMark and GFM are read through the same reader",
	}
	DOCX = Format{
		ID:     "docx",
		Name:   "Word document",
		Exts:   []string{".docx"},
		Binary: true,
		Note:   "Office Open XML, styleable with a reference document",
	}
	HTML = Format{
		ID:   "html",
		Name: "HTML",
		Exts: []string{".html", ".htm"},
		Note: "HTML5",
	}
	EPUB = Format{
		ID:     "epub",
		Name:   "EPUB",
		Exts:   []string{".epub"},
		Binary: true,
		Note:   "EPUB 3 e-book",
	}
	LaTeX = Format{
		ID:   "latex",
		Name: "LaTeX",
		Exts: []string{".tex"},
		Note: "LaTeX source, styleable with a template",
	}
	PDF = Format{
		ID:     "pdf",
		Name:   "PDF",
		Exts:   []string{".pdf"},
		Binary: true,
		Note:   "requires a LaTeX engine or wkhtmltopdf",
	}
	ODT = Format{
		ID:     "odt",
		Name:   "OpenDocument",
		Exts:   []string{".odt"},
		Binary: true,
		Note:   "LibreOffice / OpenOffice text",
	}
	RST = Format{
		ID:   "rst",
		Name: "reStructuredText",
		Exts: []string{".rst"},
	}
	Org = Format{
		ID:   "org",
		Name: "Org mode",
		Exts: []string{".org"},
	}
	Plain = Format{
		ID:   "plain",
		Name: "Plain text",
		Exts: []string{".txt"},
	}
)
