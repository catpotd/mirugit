package git

import (
	"context"
	"strings"
)

// TopLevel resolves the repository root for dir. The path is passed with -C
// so git still runs when dir does not exist and writes the cause to stderr.
func TopLevel(ctx context.Context, dir string) (string, error) {
	out, err := execGitIn(ctx, "", nil, nil, []string{"-C", dir, "rev-parse", "--show-toplevel"})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsBare says whether dir is a repository git keeps without a working tree.
// The answer is the word "true" or "false", which git prints the same way
// whatever the reader's language is; the refusals it writes to stderr are
// prose, and a caller that reads those breaks when git rewords one.
func IsBare(ctx context.Context, dir string) (bool, error) {
	out, err := execGitIn(ctx, "", nil, nil, []string{"-C", dir, "rev-parse", "--is-bare-repository"})
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}
