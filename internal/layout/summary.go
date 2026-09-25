package layout

import (
	"fmt"
	"strings"

	"github.com/catpotd/mirugit/internal/git"
	"github.com/catpotd/mirugit/internal/state"
)

// verbBarGaps is the separator space the bar needs around and between its
// words, held apart from the words so that dropping one frees its gap too.
const verbBarGaps = 8

func keyedVerbs(verbs []state.VerbName) []string {
	out := make([]string, len(verbs))
	for i, v := range verbs {
		out[i] = keyedVerb(v)
	}
	return out
}

// fitVerbs drops from the middle. The first verb is what the reader came for
// and the last is the way out of the selection, which a reader on the mouse has
// no other route to; dropping from the end would take the exit first.
func fitVerbs(w Renderer, verbs []state.VerbName, room int) []state.VerbName {
	for len(verbs) > 2 {
		if w.Of(strings.Join(keyedVerbs(verbs), "   ")) <= room {
			return verbs
		}
		verbs = append(verbs[:len(verbs)-2:len(verbs)-2], verbs[len(verbs)-1])
	}
	for len(verbs) > 0 {
		if w.Of(strings.Join(keyedVerbs(verbs), "   ")) <= room {
			return verbs
		}
		verbs = verbs[:len(verbs)-1]
	}
	return nil
}

// betweenVerbs separates one verb from the next inside the bar, and the
// regions step by it, so the two cannot drift apart.
const betweenVerbs = "   "

func summaryRight(syncBtn, verbStr, commitBtn string) string {
	right := syncBtn
	if verbStr != "" {
		if right != "" {
			right = right + "    " + verbStr
		} else {
			right = verbStr
		}
	}
	if commitBtn != "" {
		if right != "" {
			right = right + "    " + commitBtn
		} else {
			right = commitBtn
		}
	}
	return right
}

func summaryButtons(sum summary) (syncBtn, commitBtn string) {
	if !sum.NoUpstream {
		syncBtn = syncButtonLabel(sum.Behind, sum.Ahead)
	}
	if sum.Staged > 0 {
		commitBtn = fmt.Sprintf("[commit %d %s]", sum.Staged, state.FileWord(sum.Staged))
	}
	return syncBtn, commitBtn
}

// summaryRegions walks the right half from its left edge, in the order
// summaryRight draws it: the sync button, then the selection's verbs, then the
// commit button, with four blank columns between each pair that is drawn.
//
// It used to work each piece out on its own by counting back from the right
// edge, and all three answers assumed the sync button was last when it is
// first. The button's own region landed on the verbs at the other end of the
// line, and the verbs' landed on the button: measured, clicking the drawn
// "s stage" did nothing and clicking the sync button staged the selection.
func (w Renderer) summaryRegions(width int, right, syncBtn, commitBtn, verbStr string,
	barVerbs []state.VerbName) []Region {
	var regions []Region
	at := width - w.Of(right)
	const betweenPieces = "    "

	if syncBtn != "" {
		regions = append(regions, Region{
			Target:   Target{Kind: TargetButton, Verb: state.VerbNameSync},
			ColStart: at,
			ColEnd:   at + w.Of(syncBtn),
		})
		at += w.Of(syncBtn)
	}
	if verbStr != "" {
		if len(regions) > 0 {
			at += w.Of(betweenPieces)
		}
		for _, v := range barVerbs {
			tok := keyedVerb(v)
			regions = append(regions, Region{
				Target:   Target{Kind: TargetVerb, Verb: v},
				ColStart: at,
				ColEnd:   at + w.Of(tok),
			})
			at += w.Of(tok) + w.Of(betweenVerbs)
		}
		at -= w.Of(betweenVerbs)
	}
	if commitBtn != "" {
		if len(regions) > 0 {
			at += w.Of(betweenPieces)
		}
		regions = append(regions, Region{
			Target:   Target{Kind: TargetCommit},
			ColStart: at,
			ColEnd:   at + w.Of(commitBtn),
		})
	}
	return regions
}

// SummaryLine draws the repository-wide counts and the commit action on one row.
func (w Renderer) SummaryLine(sum summary, width int) (string, []Region) {
	leftStr := summaryLeft(sum)
	syncBtn, commitBtn := summaryButtons(sum)

	// The selection's verbs sit left of the commit button, which acts on the
	// index rather than on the selection and so stays put.
	var barVerbs []state.VerbName
	if sum.Selected > 0 && sum.DiscardConfirm == nil {
		verbs := append(append([]state.VerbName(nil), sum.SelectionBarVerbs...), state.VerbNameClear)
		barVerbs = fitVerbs(w, verbs,
			width-w.Of(leftStr)-w.Of(syncBtn)-w.Of(commitBtn)-verbBarGaps)
	}
	verbStr := strings.Join(keyedVerbs(barVerbs), betweenVerbs)

	right := summaryRight(syncBtn, verbStr, commitBtn)
	// The right half drops one piece at a time until it fits, keeping the sync
	// button longest because it is the one the reader cannot reach another way.
shrink:
	for right != "" && w.Of(leftStr)+1+w.Of(right) > width {
		switch {
		case commitBtn != "":
			commitBtn = ""
		case verbStr != "":
			barVerbs, verbStr = nil, ""
		case syncBtn != "":
			syncBtn = ""
		default:
			break shrink
		}
		right = summaryRight(syncBtn, verbStr, commitBtn)
	}
	if w.Of(leftStr) > width {
		leftStr = w.Truncate(leftStr, width)
	}

	line := w.Pad(leftStr, right, width)
	if w.Enabled {
		coloredRight := w.colorSummaryRight(barVerbs, syncBtn, commitBtn, right)
		line = w.paintTail(line, width, coloredRight, w.Of(right))
		left := w.colorStatusWords(w.colorCounts(w.colorDots(leftStr)))
		line = left + line[len(leftStr):]
	}
	return line, w.summaryRegions(width, right, syncBtn, commitBtn, verbStr, barVerbs)
}

// SyncOffered reports whether the summary draws the sync button. The keyboard
// asks so that S runs exactly where the button is: the two used to answer this
// separately, with a comment on one saying it matched the other and nothing
// checking that it did.
func SyncOffered(head git.Head) bool {
	return head.HasUpstream && syncButtonLabel(head.Behind, head.Ahead) != ""
}

func syncButtonLabel(behind, ahead int) string {
	if behind > 0 && ahead > 0 {
		return fmt.Sprintf("[sync %d↓ %d↑]", behind, ahead)
	}
	if ahead > 0 {
		return fmt.Sprintf("[push %d↑]", ahead)
	}
	if behind > 0 {
		return fmt.Sprintf("[pull %d↓]", behind)
	}
	return ""
}
