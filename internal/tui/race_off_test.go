//go:build !race

package tui

// raceDetector says whether the detector is on. Its own allocations land in the
// same counters this package budgets, so a budget measured under it is not this
// program's.
const raceDetector = false
