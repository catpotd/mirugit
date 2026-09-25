# Security

## Reporting a vulnerability

Use GitHub's private reporting: on
[the Security tab](https://github.com/catpotd/mirugit/security/advisories/new),
choose **Report a vulnerability**. That page is private between you and the
maintainers until an advisory is published. Do not open a public issue for
something exploitable.

Include the version (`mirugit -version`), the operating system, and the steps
that reproduce it. A repository state that triggers the problem helps more than
a description of it.

Reports are handled as maintainer availability allows; no response time is
guaranteed. Keep follow-up details in the private report.

## What is in scope

mirugit runs `git`, reads the repository it is pointed at, and writes to
`$XDG_STATE_HOME/mirugit` (or `~/.local/state/mirugit`). Things worth reporting:

- A repository that makes mirugit run a command it was not asked to run —
  crafted paths, refs, or branch names reaching a shell
- A write outside the repository under the cursor or the state directory
- A path in the repository escaping into the state file's directory
- Reading a file the reader did not ask to see
- Terminal control sequences from the repository reaching the terminal. Every
  string git reports goes through `layout.Printable` before it is drawn, so a
  path or a diff line cannot clear the screen or rewrite the window title. A
  route around that is worth reporting.

## What is not

- Anything git itself does when you press the key that says it will. Discard
  removes work; that is the verb, not a vulnerability.
- Denial of service from a repository large enough to be slow.

## Supported versions

The newest tagged release. Fixes go on `main` and are released from there.
