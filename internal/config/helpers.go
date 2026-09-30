package config

// StyleDirOrEmpty is a convenience for callers that only want to display the
// location and have nothing useful to do with an error.
func StyleDirOrEmpty() string {
	dir, err := StyleDir()
	if err != nil {
		return ""
	}
	return dir
}
