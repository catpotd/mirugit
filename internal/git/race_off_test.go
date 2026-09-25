//go:build !race

package git

// raceDetector is false in an ordinary build. The memory budget is a fact about
// this program, and the detector's shadow memory is not part of it.
const raceDetector = false
