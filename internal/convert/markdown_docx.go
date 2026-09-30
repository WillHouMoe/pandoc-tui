package convert

import (
	"fmt"
	"os"
)

// markdownToDOCX is the conversion pandoc-tui ships with. It doubles as the
// reference implementation for anyone adding the next one: describe the two
// formats, list the flags as Options, build argv in Args.
type markdownToDOCX struct{}

func (markdownToDOCX) ID() string { return "markdown-docx" }

func (markdownToDOCX) Label() string { return "Markdown -> Word (.docx)" }

func (markdownToDOCX) Summary() string {
	return "Convert a Markdown file into a Word document that follows a reference style."
}

func (markdownToDOCX) From() Format { return Markdown }

func (markdownToDOCX) To() Format { return DOCX }

func (markdownToDOCX) WantsReferenceDoc() bool { return true }

func (markdownToDOCX) Options() []Option {
	return []Option{
		{
			Key:     "toc",
			Label:   "Table of contents",
			Help:    "Insert a table of contents that Word can refresh with F9.",
			Kind:    KindToggle,
			Default: "true",
			Flag:    "--toc",
		},
		{
			Key:     "toc-depth",
			Label:   "TOC depth",
			Help:    "Heading levels included in the table of contents.",
			Kind:    KindNumber,
			Default: "3",
			Flag:    "--toc-depth",
			Min:     1,
			Max:     6,
		},
		{
			Key:     "number-sections",
			Label:   "Number sections",
			Help:    "Prefix every heading with an automatic number.",
			Kind:    KindToggle,
			Default: "false",
			Flag:    "--number-sections",
		},
		{
			Key:     "highlight-style",
			Label:   "Code highlight",
			Help:    "Colour scheme applied to code blocks.",
			Kind:    KindChoice,
			Default: "pygments",
			Flag:    "--highlight-style",
			Choices: []Choice{
				{Value: "pygments", Label: "pygments"},
				{Value: "tango", Label: "tango"},
				{Value: "espresso", Label: "espresso"},
				{Value: "zenburn", Label: "zenburn"},
				{Value: "kate", Label: "kate"},
				{Value: "monochrome", Label: "monochrome"},
			},
		},
		{
			Key:     "wrap",
			Label:   "Line wrapping",
			Help:    "How pandoc wraps long lines in the generated document.",
			Kind:    KindChoice,
			Default: "auto",
			Flag:    "--wrap",
			Choices: []Choice{
				{Value: "auto", Label: "auto"},
				{Value: "none", Label: "none"},
				{Value: "preserve", Label: "preserve"},
			},
		},
		{
			Key:     "metadata-title",
			Label:   "Title",
			Help:    "Overrides the title taken from the Markdown metadata block.",
			Kind:    KindText,
			Default: "",
			Flag:    "--metadata",
		},
		{
			Key:     "resource-path",
			Label:   "Image search path",
			Help:    "Extra directory pandoc searches for images referenced by the document.",
			Kind:    KindText,
			Default: "",
			Flag:    "--resource-path",
		},
	}
}

func (markdownToDOCX) Validate(req Request) error {
	if err := CheckInput(req.Input); err != nil {
		return err
	}
	if req.Output == "" {
		return ErrNoOutput
	}
	if req.Input == req.Output {
		return fmt.Errorf("the source and the output are the same file")
	}
	if req.Reference != "" {
		if _, err := os.Stat(req.Reference); err != nil {
			return fmt.Errorf("style file is unusable: %w", err)
		}
	}
	return nil
}

func (c markdownToDOCX) Args(req Request) ([]string, error) {
	args := BaseArgs(req, Markdown, DOCX, "--standalone")
	if req.Reference != "" {
		args = append(args, "--reference-doc="+req.Reference)
	}
	for _, o := range c.Options() {
		value := req.Value(o)
		if o.Key == "metadata-title" && value != "" {
			args = append(args, "--metadata=title="+value)
			continue
		}
		args = append(args, o.Args(value)...)
	}
	return args, nil
}
