package state

// ConfirmationOpen reports that the pane is waiting for the reader to answer a
// destructive question.
//
// It reads PendingConfirmation rather than naming the fields, because naming
// them is how this went wrong twice: one caller left out the undo confirmation
// and this function left out the worktree removal, so a row offered its verbs
// under a prompt the other prompts hide them for.
func ConfirmationOpen(s State) bool {
	_, open := PendingConfirmation(s)
	return open
}

// CursorRowVerbsAllowed is false while a confirmation is showing or while a
// selection would hide single-row verbs on the cursor line.
func CursorRowVerbsAllowed(s State) bool {
	if ConfirmationOpen(s) {
		return false
	}
	return len(s.Changes.Selected) == 0
}
