package convert

// Registry holds every conversion the app offers, in registration order.
type Registry struct {
	order []Converter
	byID  map[string]Converter
}

// NewRegistry builds a registry from the given converters.
func NewRegistry(converters ...Converter) *Registry {
	r := &Registry{byID: make(map[string]Converter)}
	for _, c := range converters {
		r.Register(c)
	}
	return r
}

// Register adds a converter. Duplicate IDs are ignored so that a plugin cannot
// silently shadow a built-in conversion.
func (r *Registry) Register(c Converter) {
	if c == nil || c.ID() == "" {
		return
	}
	if _, exists := r.byID[c.ID()]; exists {
		return
	}
	r.byID[c.ID()] = c
	r.order = append(r.order, c)
}

// All returns the converters in registration order.
func (r *Registry) All() []Converter {
	out := make([]Converter, len(r.order))
	copy(out, r.order)
	return out
}

// Len is the number of registered conversions.
func (r *Registry) Len() int { return len(r.order) }

// Lookup finds a converter by ID.
func (r *Registry) Lookup(id string) (Converter, bool) {
	c, ok := r.byID[id]
	return c, ok
}

// First returns the converter the app should open with.
func (r *Registry) First() (Converter, bool) {
	if len(r.order) == 0 {
		return nil, false
	}
	return r.order[0], true
}

// Planned describes a conversion the roadmap promises but this build does not
// ship yet. Showing the roadmap inside the app keeps the extension point
// visible for anyone who wants to write the next converter.
type Planned struct {
	Label   string
	From    Format
	To      Format
	Because string
}

// Roadmap is what pandoc-tui intends to grow into. None of these are wired up
// yet; each one becomes real by implementing Converter.
func Roadmap() []Planned {
	return []Planned{
		{
			Label:   "Markdown -> HTML",
			From:    Markdown,
			To:      HTML,
			Because: "needs the HTML writer options (css, standalone, math)",
		},
		{
			Label:   "Markdown -> EPUB",
			From:    Markdown,
			To:      EPUB,
			Because: "needs cover image and metadata handling",
		},
		{
			Label:   "Markdown -> PDF",
			From:    Markdown,
			To:      PDF,
			Because: "needs a LaTeX engine probe and template handling",
		},
		{
			Label:   "Word -> Markdown",
			From:    DOCX,
			To:      Markdown,
			Because: "needs media extraction (--extract-media)",
		},
		{
			Label:   "Any -> Any",
			From:    Markdown,
			To:      Plain,
			Because: "a format matrix picker replacing the single fixed slot",
		},
	}
}

// Default is the registry the shipped binary uses. This is the one line a new
// converter has to be added to.
func Default() *Registry {
	return NewRegistry(
		markdownToDOCX{},
	)
}
