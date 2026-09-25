package git

import "time"

// termGrace is how long git gets to put its lock files back. A git that ignores
// the term still ends, half a second later than it would have.
//
// Measured with a clean filter holding .git/index.lock: the whole stop took 6 ms
// idle, and under sixteen spinning processes 1 run in 30 used the full grace.
// The lock was gone in all 38 under that load: git releases it on the term well
// before the group goes, and what the grace waits out is the filter git
// started, which holds nothing.
//
// Under a different load it can still be left. Two full test suites running at
// once left it once in fifteen — process and IO pressure rather than CPU, where
// the term handler itself can be starved past the grace. So a grace that
// expires is not usually a lock left behind, and is not proof of one either.
const termGrace = 500 * time.Millisecond
