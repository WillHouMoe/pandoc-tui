package ui

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// openInApp hands a file to whatever application the desktop associates with
// it. This is how a style gets edited: pandoc-tui creates the reference.docx,
// the user restyles it in Word or LibreOffice, and the file they save is the
// style from then on.
func openInApp(path string) error {
	name, args := openerCommand(path)
	return exec.Command(name, args...).Start()
}

// openFile is indirected through a variable so tests can keep the desktop out
// of the loop; launching Word from a unit test would be rude.
var openFile = openInApp

// revealInFileManager opens the folder holding path, with the file selected
// where the platform supports it.
func revealInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,"+path).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

func openerCommand(path string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{path}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", path}
	default:
		return "xdg-open", []string{path}
	}
}
