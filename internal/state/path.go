package state

import "strings"

// ParentDir returns the path before the last slash, or "" when p has none.
func ParentDir(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

func IsBelowDirectory(path, directory string) bool {
	return directory == "" || strings.HasPrefix(path, directory+"/")
}
