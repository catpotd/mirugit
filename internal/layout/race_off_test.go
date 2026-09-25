//go:build !race

package layout

// raceDetector is false in an ordinary build. The allocation budget is a fact
// about this program, and the detector's shadow memory is not part of it: the
// same frame measured 287 KB here and 387 KB under -race.
const raceDetector = false
