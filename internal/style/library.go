// Package style manages the reference documents -- "styles" in the UI -- that
// a conversion can be dressed with.
//
// A style is nothing more than a .docx: pandoc's stock reference document,
// copied into the library and then tweaked in Word, Pages or LibreOffice. The
// library is deliberately just a folder, so a user can also drop a .docx in
// there with Finder and have it show up.
package style

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Ext is the extension a style file must carry.
const Ext = ".docx"

// ErrNotFound is returned when a style was deleted behind our back.
var ErrNotFound = errors.New("style not found")

// DefaultDocFunc writes pandoc's stock reference.docx to the given path. It is
// injected so the library does not have to know about the pandoc package.
type DefaultDocFunc func(ctx context.Context, dest string) error

// Style is one entry of the library.
type Style struct {
	// Name is what the user sees, derived from the file name.
	Name string
	// Path is empty for the built-in "pandoc default" entry.
	Path    string
	Builtin bool
	ModTime time.Time
	Size    int64
}

// IsBuiltin reports whether the entry means "let pandoc use its own template".
func (s Style) IsBuiltin() bool { return s.Builtin }

// Describe renders a short note for the style list.
func (s Style) Describe() string {
	if s.Builtin {
		return "pandoc's stock look, no file attached"
	}
	if s.ModTime.IsZero() {
		return "custom"
	}
	return "edited " + s.ModTime.Format("2006-01-02 15:04")
}

// Library is a directory of style files.
type Library struct {
	Dir string
	// Default produces a fresh template when the user creates a style.
	Default DefaultDocFunc
}

// New builds a library rooted at dir.
func New(dir string, seed DefaultDocFunc) *Library {
	return &Library{Dir: dir, Default: seed}
}

// EnsureDir creates the library folder if it is missing.
func (l *Library) EnsureDir() error {
	return os.MkdirAll(l.Dir, 0o755)
}

// List returns the built-in entry followed by every .docx in the library,
// sorted by name.
func (l *Library) List() ([]Style, error) {
	styles := []Style{{Name: "None", Builtin: true}}

	entries, err := os.ReadDir(l.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return styles, nil
		}
		return nil, err
	}

	var custom []Style
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), Ext) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		custom = append(custom, Style{
			Name:    strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
			Path:    filepath.Join(l.Dir, e.Name()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}
	sort.Slice(custom, func(i, j int) bool {
		return strings.ToLower(custom[i].Name) < strings.ToLower(custom[j].Name)
	})
	return append(styles, custom...), nil
}

// Create seeds a new style from pandoc's stock reference document. The name is
// uniquified so that creating "Report" twice yields "Report" and "Report 2".
func (l *Library) Create(ctx context.Context, name string) (Style, error) {
	if l.Default == nil {
		return Style{}, errors.New("no pandoc binary available to build a template")
	}
	if err := l.EnsureDir(); err != nil {
		return Style{}, err
	}
	clean := SanitizeName(name)
	if clean == "" {
		return Style{}, errors.New("pick a name for the style")
	}
	path, err := l.uniquePath(clean)
	if err != nil {
		return Style{}, err
	}
	if err := l.Default(ctx, path); err != nil {
		return Style{}, err
	}
	return l.Describe(path)
}

// Import copies an existing .docx into the library so it can be reused.
func (l *Library) Import(src string) (Style, error) {
	if err := l.EnsureDir(); err != nil {
		return Style{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return Style{}, err
	}
	if info.IsDir() {
		return Style{}, fmt.Errorf("%s is a directory", src)
	}
	if !strings.EqualFold(filepath.Ext(src), Ext) {
		return Style{}, fmt.Errorf("a style must be a %s file, got %s", Ext, filepath.Ext(src))
	}

	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	dest, err := l.uniquePath(SanitizeName(base))
	if err != nil {
		return Style{}, err
	}
	if same, err := sameFile(src, dest); err == nil && same {
		return l.Describe(dest)
	}
	if err := copyFile(src, dest); err != nil {
		return Style{}, err
	}
	return l.Describe(dest)
}

// Rename gives an existing style a new display name.
func (l *Library) Rename(s Style, name string) (Style, error) {
	if s.Builtin {
		return Style{}, errors.New("the built-in entry cannot be renamed")
	}
	clean := SanitizeName(name)
	if clean == "" {
		return Style{}, errors.New("the name cannot be empty")
	}
	if clean == s.Name {
		return s, nil
	}
	dest, err := l.uniquePath(clean)
	if err != nil {
		return Style{}, err
	}
	if err := os.Rename(s.Path, dest); err != nil {
		return Style{}, err
	}
	return l.Describe(dest)
}

// Delete removes a style file from the library.
func (l *Library) Delete(s Style) error {
	if s.Builtin {
		return errors.New("the built-in entry cannot be deleted")
	}
	if err := os.Remove(s.Path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Duplicate copies a style so the user can branch off a variant.
func (l *Library) Duplicate(s Style) (Style, error) {
	if s.Builtin {
		return Style{}, errors.New("there is nothing to duplicate")
	}
	dest, err := l.uniquePath(SanitizeName(s.Name + " copy"))
	if err != nil {
		return Style{}, err
	}
	if err := copyFile(s.Path, dest); err != nil {
		return Style{}, err
	}
	return l.Describe(dest)
}

// Describe re-stats a path and turns it into a Style.
func (l *Library) Describe(path string) (Style, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Style{}, err
	}
	return Style{
		Name:    strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Path:    path,
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}, nil
}

func (l *Library) uniquePath(name string) (string, error) {
	for i := 1; i < 1000; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s %d", name, i)
		}
		path := filepath.Join(l.Dir, candidate+Ext)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
	}
	return "", fmt.Errorf("too many styles named %q", name)
}

// SanitizeName turns free text into something safe to use as a file name while
// still allowing non-ASCII scripts.
func SanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', 0:
			return '-'
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	return name
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	return out.Close()
}

func sameFile(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}
