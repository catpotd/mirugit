# Contributing

## Requirements

- Go 1.26 or later.
- Git 2.41 or later on `PATH`; the tests create real repositories.
- Docker to run the full local check on Linux.
- Network access to download Go modules and the linter on first use.

## Set up

```sh
git clone https://github.com/catpotd/mirugit
cd mirugit
make install
```

`make install` downloads Go modules into the local module cache. It does not
build or install the `mirugit` command.

## Checks

While editing, run the shorter check:

```sh
make quick
```

`make quick` runs formatting, static checks, and short tests. It skips tests
that shell out to Git.

Before opening a pull request, run the full check:

```sh
make ci-local
```

`make ci-local` runs every CI gate on this machine and in a Linux container.
Docker must be running for the Linux checks.

GitHub runs the `ci` workflow for pull requests and pushes to `main`. Run
`make ci-local` before opening a pull request. The commands below let you rerun
one gate at a time:

Individual CI commands:

```sh
make check
make test-race
make fuzz FUZZTIME=30s
make supply-chain
```

### Git identity

Tests set their own committer through `GIT_AUTHOR_*` and `GIT_COMMITTER_*`, so
an unset global identity is fine. An identity set to the empty string is not:
`git commit` then fails with `fatal: empty ident name` and every test that
builds a repository goes red. If you see that error, check for
`GIT_AUTHOR_NAME=""` in your environment.

## Git hooks

To enable the repository's hooks for this checkout, run:

```sh
make tools
```

The pre-commit hook runs formatting and static checks. The pre-push hook runs
`make check`. `--no-verify` skips either hook.

## Pull requests

Target `main`. Describe the change, why it is needed, and how you tested it.
For a bug fix, include steps that reproduce the original behavior and a test
that covers the correction when practical.

Use `feature/` for features and `fix/` for fixes, followed by a short
description. Write commit subjects in English and use a concise prefix such as
`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, or `chore:`. Use a concise pull
request title; GitHub includes pull request titles in generated release notes.

## Project structure

| Path | Role |
|---|---|
| `cmd/mirugit` | Command-line entry point |
| `internal/git` | Reads and updates repository data through Git |
| `internal/state` | Holds application state and applies events |
| `internal/layout` | Draws terminal rows and interaction regions |
| `internal/tui` | Handles terminal input and coordinates the views |
| `internal/osproc` | Opens a browser and copies text to the clipboard |
| `internal/docscheck` | Checks documentation against repository configuration |

## Coverage

Run this command to list functions with no test coverage:

```sh
make cover-zero
```

Use this list to review untested behavior. Add tests for behavior that needs a
regression guard, remove code that cannot be reached, or record an intentional
exception in this section. Do not add tests solely to raise coverage. Current
exceptions are:

- **the `event()` methods in `internal/state/event.go`** — empty methods
  that exist only to make a type an `Event`. `TestApplyHandlesEveryEventType`
  checks that `Apply` handles each event type.
- **`main`** — it reads flags, prints errors, and calls `os.Exit`. The tested
  `run` function holds the behavior worth asserting.
- **`staleMsg.failed`** — a stale answer is applied before its failure is
  reported, so the generic error check skips this method.

## Releasing (maintainers)

Pushing a `v*` tag starts a draft release. Before tagging, run `make ci-local`
and confirm the working tree is clean. Review the generated release notes and
artifacts, then publish the draft release.
