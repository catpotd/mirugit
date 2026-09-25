package state

// Confirmation is a destructive verb waiting for a yes. Four of them exist and
// every reader of one had to name all four: the footer, three key handlers and
// the click path each listed them, and worktree remove was added to none, so
// the same x key asked first on two tabs out of three.
type Confirmation struct {
	// Verb is the word the footer prints beside the y.
	Verb VerbName
	// Warning is what the reader cannot get back, or empty when they can.
	Warning string
	// Kept is the notice a cancel leaves behind.
	Kept string
}

// confirmations is every destructive prompt, in the order they are asked. A
// table rather than a switch so the count below is the table itself: a count
// kept beside a switch is a second list, and a second list is what the prompts
// drifted apart in to begin with.
var confirmations = [...]struct {
	open func(State) bool
	Confirmation
}{
	{
		func(s State) bool { return s.Changes.DiscardConfirm != nil },
		Confirmation{Verb: VerbNameDiscard, Kept: "canceled · nothing discarded"},
	},
	{
		func(s State) bool { return s.Changes.UndiscardConfirm != nil },
		Confirmation{Verb: VerbNameRestore, Warning: "edits will be lost",
			Kept: "canceled · discard kept"},
	},
	{
		func(s State) bool { return s.Stashed.DropConfirm != nil },
		Confirmation{Verb: VerbNameDrop, Warning: "stash cannot be undone",
			Kept: "canceled · stash kept"},
	},
	{
		func(s State) bool { return s.Worktrees.RemoveConfirm != nil },
		Confirmation{Verb: VerbNameRemove, Kept: "canceled · worktree kept"},
	},
}

// ConfirmationCount is how many destructive prompts this program can show. A
// test outside this package counts its own list against it, so a prompt added
// here and checked nowhere fails the count it was measured against.
const ConfirmationCount = len(confirmations)

// PendingConfirmation is the one that is open, and false when none is.
func PendingConfirmation(s State) (Confirmation, bool) {
	for _, c := range confirmations {
		if c.open(s) {
			return c.Confirmation, true
		}
	}
	return Confirmation{}, false
}
