package state

// FileWord returns "file" for one path and "files" otherwise.
func FileWord(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}
