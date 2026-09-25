// Package docscheck holds the tests that read this repository's documents.
//
// They check what the documents claim against what the repository does: that a
// command written in one parses, that the commands CI runs are the ones a
// contributor is told to run, that an issue template names a label that exists,
// that no document promises a wall-clock time, and that the coverage ledger
// counts what it says it counts.
//
// They live here rather than beside the code because they are about the
// documents, not about any one package. There is nothing to import: the
// package is the tests.
package docscheck
