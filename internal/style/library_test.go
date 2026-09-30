package style

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// seed writes a stand-in for pandoc's reference document. It does not have to
// be a real .docx: the library only moves bytes around.
func seed(t *testing.T) DefaultDocFunc {
	t.Helper()
	return func(_ context.Context, dest string) error {
		return os.WriteFile(dest, []byte("fake reference doc"), 0o644)
	}
}

func TestCreateUniquifiesNames(t *testing.T) {
	lib := New(t.TempDir(), seed(t))

	first, err := lib.Create(context.Background(), "Report")
	if err != nil {
		t.Fatal(err)
	}
	second, err := lib.Create(context.Background(), "Report")
	if err != nil {
		t.Fatal(err)
	}
	if first.Path == second.Path {
		t.Fatalf("second create should not overwrite the first: %s", first.Path)
	}
	if second.Name != "Report 2" {
		t.Errorf("got name %q, want %q", second.Name, "Report 2")
	}
}

func TestListAlwaysOffersTheBuiltin(t *testing.T) {
	lib := New(filepath.Join(t.TempDir(), "not-created-yet"), seed(t))

	styles, err := lib.List()
	if err != nil {
		t.Fatalf("listing a missing library should not fail: %v", err)
	}
	if len(styles) != 1 || !styles[0].Builtin {
		t.Fatalf("expected exactly the built-in entry, got %+v", styles)
	}
}

func TestRenameAndDelete(t *testing.T) {
	lib := New(t.TempDir(), seed(t))
	created, err := lib.Create(context.Background(), "Draft")
	if err != nil {
		t.Fatal(err)
	}

	renamed, err := lib.Rename(created, "Final")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Final" {
		t.Errorf("got %q, want Final", renamed.Name)
	}
	if _, err := os.Stat(created.Path); !os.IsNotExist(err) {
		t.Error("the old file should be gone after a rename")
	}

	if err := lib.Delete(renamed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(renamed.Path); !os.IsNotExist(err) {
		t.Error("delete should remove the file")
	}
	if err := lib.Delete(renamed); err == nil {
		t.Error("deleting twice should report the style as missing")
	}
}

func TestBuiltinCannotBeChanged(t *testing.T) {
	lib := New(t.TempDir(), seed(t))
	builtin := Style{Name: "None", Builtin: true}
	if err := lib.Delete(builtin); err == nil {
		t.Error("the built-in entry must not be deletable")
	}
	if _, err := lib.Rename(builtin, "Something"); err == nil {
		t.Error("the built-in entry must not be renamable")
	}
}

func TestImport(t *testing.T) {
	dir := t.TempDir()
	lib := New(filepath.Join(dir, "library"), seed(t))

	source := filepath.Join(dir, "Corporate.docx")
	if err := os.WriteFile(source, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := lib.Import(source)
	if err != nil {
		t.Fatal(err)
	}
	if imported.Name != "Corporate" {
		t.Errorf("got %q, want Corporate", imported.Name)
	}
	// The original must survive: importing copies, it does not move.
	if _, err := os.Stat(source); err != nil {
		t.Errorf("import should leave the original alone: %v", err)
	}

	if _, err := lib.Import(filepath.Join(dir, "notes.md")); err == nil {
		t.Error("importing a non-.docx should fail")
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"  Report  ":   "Report",
		"a/b:c":        "a-b-c",
		"中文文档":         "中文文档",
		"../../escape": "-..-escape",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
