package convert

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// OptionKind selects which widget the TUI draws for an option.
type OptionKind int

const (
	// KindToggle is an on/off switch that maps to a bare pandoc flag.
	KindToggle OptionKind = iota
	// KindText is a free text value, e.g. --metadata=title=...
	KindText
	// KindNumber is a bounded integer, e.g. --toc-depth=3
	KindNumber
	// KindChoice is one of a fixed set of values.
	KindChoice
)

// Choice is a single entry of a KindChoice option.
type Choice struct {
	Value string
	Label string
}

// Option is one knob a converter exposes. The UI builds its form from this
// list, which is why a converter can grow new flags without touching any
// screen.
type Option struct {
	Key     string
	Label   string
	Help    string
	Kind    OptionKind
	Default string
	Choices []Choice
	// Flag is the pandoc flag this option maps to, e.g. "--toc". An option
	// without a flag is still remembered between runs, it simply never
	// reaches the command line.
	Flag string
	// Min and Max bound KindNumber options. Zero means "use the built-in
	// 1..9 range".
	Min int
	Max int
}

// Args renders the option into pandoc arguments for the given value.
func (o Option) Args(value string) []string {
	value = strings.TrimSpace(value)
	if o.Flag == "" {
		return nil
	}
	if o.Kind == KindToggle {
		if isOn(value) {
			return []string{o.Flag}
		}
		return nil
	}
	if value == "" {
		return nil
	}
	return []string{o.Flag + "=" + value}
}

// Next returns the value a forward cycle lands on. It is what the UI uses for
// toggles, choices and number steppers.
func (o Option) Next(current string) string {
	switch o.Kind {
	case KindToggle:
		if isOn(current) {
			return "false"
		}
		return "true"
	case KindChoice:
		if len(o.Choices) == 0 {
			return current
		}
		return o.Choices[(o.indexOf(current)+1)%len(o.Choices)].Value
	case KindNumber:
		lo, hi := o.limits()
		return strconv.Itoa(clamp(o.number(current)+1, lo, hi))
	}
	return current
}

// Prev returns the value a backward cycle lands on.
func (o Option) Prev(current string) string {
	switch o.Kind {
	case KindToggle:
		return o.Next(current)
	case KindChoice:
		if len(o.Choices) == 0 {
			return current
		}
		i := o.indexOf(current)
		return o.Choices[(i+len(o.Choices)-1)%len(o.Choices)].Value
	case KindNumber:
		lo, hi := o.limits()
		return strconv.Itoa(clamp(o.number(current)-1, lo, hi))
	}
	return current
}

// Label returns the human readable text for a raw option value.
func (o Option) Display(value string) string {
	switch o.Kind {
	case KindToggle:
		return map[bool]string{true: "on", false: "off"}[isOn(value)]
	case KindChoice:
		for _, c := range o.Choices {
			if c.Value == value {
				if c.Label != "" {
					return c.Label
				}
				return c.Value
			}
		}
	}
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func (o Option) indexOf(value string) int {
	for i, c := range o.Choices {
		if c.Value == value {
			return i
		}
	}
	return 0
}

func (o Option) number(value string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return n
	}
	if n, err := strconv.Atoi(o.Default); err == nil {
		return n
	}
	return 1
}

func (o Option) limits() (int, int) {
	lo, hi := o.Min, o.Max
	if lo == 0 {
		lo = 1
	}
	if hi == 0 {
		hi = 9
	}
	return lo, hi
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func isOn(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// Request is a conversion job with every field already resolved by the UI.
type Request struct {
	Input     string // absolute path of the source document
	Output    string // absolute path to write
	Reference string // optional style file (reference.docx), empty for none
	Values    map[string]string
}

// Value returns the resolved value of an option, falling back to its default.
func (r Request) Value(o Option) string {
	if r.Values == nil {
		return o.Default
	}
	if v, ok := r.Values[o.Key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return o.Default
}

// Converter is one source -> target conversion pandoc-tui can perform.
//
// Implement this interface, register the result in Default, and the new
// conversion shows up in the UI with its own options form.
type Converter interface {
	// ID is stable and unique across the registry, e.g. "markdown-docx".
	ID() string
	// Label is the one line title in the conversion list.
	Label() string
	// Summary explains what the conversion produces.
	Summary() string
	From() Format
	To() Format
	// WantsReferenceDoc reports whether a style file can be attached.
	WantsReferenceDoc() bool
	// Options lists the adjustable flags for this conversion.
	Options() []Option
	// Validate reports problems before pandoc is spawned.
	Validate(Request) error
	// Args builds the pandoc arguments, input and output included. The pandoc
	// binary itself is added by the runner.
	Args(Request) ([]string, error)
}

// BaseArgs is the argument prefix almost every converter wants.
func BaseArgs(req Request, from, to Format, extra ...string) []string {
	args := []string{
		"--from=" + from.ID,
		"--to=" + to.ID,
	}
	args = append(args, extra...)
	return append(args, req.Input, "--output="+req.Output)
}

// DefaultValues returns a fresh map holding every option at its default, ready
// to be handed to a converter.
func DefaultValues(c Converter) map[string]string {
	values := make(map[string]string)
	for _, o := range c.Options() {
		values[o.Key] = o.Default
	}
	return values
}

// ErrNoInput and friends are returned by Validate implementations so the UI
// can point at the field that needs attention.
var (
	ErrNoInput  = errors.New("no source file selected")
	ErrNoOutput = errors.New("no output file selected")
)

// CheckInput is a small helper for Validate implementations.
func CheckInput(path string) error {
	if strings.TrimSpace(path) == "" {
		return ErrNoInput
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot read source file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}
