# Security toolchain maintenance implementation plan

> **For agentic workers:** Use superpowers:executing-plans to implement these tasks in this session. Steps use checkbox syntax for tracking.

**Goal:** Restore dependency validation and build mirugit with the current security fixes for Go.

**Architecture:** Keep the existing Make targets and pinned GitHub Actions. Declare the minimum patched Go version and the preferred toolchain in go.mod, and have both workflows read that file. Keep dependency analysis failures visible to Make.

**Tech Stack:** Go 1.26.10 minimum; Go 1.27.3 preferred; golangci-lint v2.15.0; govulncheck v1.9.0; deadcode v0.52.0; existing Docker-based local checks.

**Spec:** The maintenance scope agreed in this conversation, with the additional fail-closed defect found in Makefile's deadcode pipeline. No related ADR exists in this repository.

## Global constraints

- Start from remote main at a6e9a68 in an isolated worktree; preserve the user's feature/check-for-updates checkout.
- Use Go commands to update go.mod and go.sum.
- Go 1.26.10 remains supported; Go 1.27.3 builds releases.
- Reuse the existing static checks, race tests, fuzz tests, source vulnerability scan, cross-builds, and license collection.
- Do not change remote branches, PR discussions, repository settings, or published releases without the required authorization.

## Review focus

- The release validation job and release build must select the same Go toolchain as CI.
- Source installs must not accept Go versions older than 1.26.10.
- Go 1.26.10 is a minimum version, not an allowlist excluding vulnerable Go 1.27 patches; the preferred toolchain and the vulnerability scans cover actual builds.
- A deadcode process that exits nonzero must fail supply-chain checks even when stdout is empty.
- Empty deadcode output and the existing event-method exception must still pass.
- Bubble Tea's dependency updates must preserve the repository's existing terminal-input, display, and shutdown behavior.

### Task 1: Update and consistently select the security-patched toolchain

**Files:** go.mod, go.sum, Makefile, .github/workflows/ci.yml, .github/workflows/release.yml, README.md, CONTRIBUTING.md, .golangci.yml, and the four existing code/test locations newly flagged by the upgraded linter.

**Interfaces:** Existing make check, make test-race, make fuzz, make supply-chain, and make ci-local remain the contributor entry points.

- [x] Reproduce the existing make check failure with GOTOOLCHAIN=go1.27.3; retain the compiler-export-format error.
- [x] Run GOTOOLCHAIN=go1.27.3 go get go@1.26.10 toolchain@go1.27.3.
- [x] Set LINTER_VERSION=v2.15.0, VULNCHECK_VERSION=v1.9.0, DEADCODE_VERSION=v0.52.0, and LINUX_IMAGE=golang:1.27.3 in Makefile.
- [x] Migrate exhaustive's renamed configuration keys and fix confirmed diagnostics without exclusions: use make for dynamic maps, explicitly list the ignored known mouse buttons, and assert the killed Git command returned an error.
- [x] Set golangci-lint's supported run.go configuration to language version 1.26. Its automatic detection selects the preferred compiler version instead of the module's minimum language version, which prevents make check from running on the supported Go 1.26.10 compiler.
- [x] Replace every go-version: stable in both workflows with go-version-file: go.mod. The setup-go action's documented toolchain preference selects go1.27.3.
- [x] Set CI permissions to contents: read and add a Monday scheduled run plus workflow_dispatch so the existing checks can discover new advisories without a code change.
- [x] Document Go 1.26.10 as the minimum and Go 1.27.3 as the CI/release toolchain in CONTRIBUTING.md. Preserve existing human comments.
- [x] Run make check with Go 1.27.3. Expected: static analysis and all tests pass.

### Task 2: Fail when deadcode analysis cannot complete

**Files:** Makefile, internal/docscheck/deadcode_test.go, .github/workflows/release.yml.

**Interfaces:** Add make deadcode for the existing final supply-chain check; make supply-chain invokes it after vulnerability checks and license collection. Add make check-binary BIN=<binary-directory> for govulncheck's binary scan and call it from supply-chain and each release build.

- [x] Add subprocess tests using a temporary Go-command fixture that returns controlled analyzer output and exit status. Cover a nonzero exit with empty stdout, a nonzero exit with stdout, an unreachable function, empty output, the existing event-method exception, and mixed event/ordinary function output.
- [x] Add a process-contract test of make supply-chain using a temporary project fixture with the real Makefile, a successful notices command, and a Go fixture that fails only for deadcode. Expected before the fix: supply-chain incorrectly succeeds. Assert that the real supply-chain entry point propagates this analysis failure.
- [x] Move the existing deadcode recipe to a .PHONY deadcode target. Capture analyzer output and propagate its exit status before filtering the existing event-method exception. Use a shell filter whose empty output is successful. Invoke the target with $(MAKE) deadcode as the final supply-chain recipe command.
- [x] Add check-binary and invoke the fixed govulncheck version with -mode=binary $(BIN)/mirugit. Call it before license generation in supply-chain and before packaging each release target.
- [x] Run go test ./internal/docscheck -run 'TestDeadcode|TestSupplyChain' -count=1. Expected: all process-contract cases pass.
- [x] Run make supply-chain. Expected: builds, clean module metadata, zero reachable reported vulnerabilities, complete notices, and no unexpected unreachable functions.

### Task 3: Validate the dependency updates and final release inputs

**Files:** go.mod, go.sum, existing tests and release build outputs.

**Interfaces:** Bubble Tea v2.1.0 includes x/ansi v0.11.9, so it covers the dependencies proposed by both PR #6 and PR #7.

- [x] Run GOTOOLCHAIN=go1.27.3 go get charm.land/bubbletea/v2@v2.1.0 and go mod tidy. Expected: Bubble Tea v2.1.0 and x/ansi v0.11.9 selected with valid checksums.
- [x] Run make ci-local. Expected: all four existing gates pass on macOS and Linux with Go 1.27.3, including 30-second fuzz runs per target.
- [x] Build the four existing release targets with CGO_ENABLED=0, trimpath, and a local verification version. Run make check-binary and the existing notices script for each binary, and inspect Go build metadata. Expected: Go 1.27.3 and the selected dependency versions in all binaries, complete third-party notices, and zero binary-scan findings.
- [x] Verify make check using GOTOOLCHAIN=go1.26.10 to prove the retained minimum supported version.
- [x] Obtain an independent review of the completed local changes and address correctness findings.
- [x] Report the local branch, validation evidence, and the concrete remaining remote actions. Do not merge, close, comment on, or publish either Dependabot PR as part of local implementation.
