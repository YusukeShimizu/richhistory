# richhistory

[![CI](https://github.com/YusukeShimizu/richhistory/actions/workflows/ci.yaml/badge.svg)](https://github.com/YusukeShimizu/richhistory/actions/workflows/ci.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](./go.mod)

`richhistory` keeps shell history with output, cwd, exit status, and session context.

10-second demo: [asciinema cast](./demo/quickstart.cast) | [plain text transcript](./demo/quickstart.txt)

```text
$ richhistory show 20260415T090000Z-...
id: 20260415T090000Z-...
session: demo-session
session_name: demo
command: echo hello
exit_code: 0
cwd_before: /path/to/repo
cwd_after: /path/to/repo
stdout:
hello

stderr:
```

It records:

- the command that ran
- `stdout` and `stderr` previews
- working directory before and after execution
- exit status, duration, and session identity

## Why richhistory

| Need | Built-in shell history | `richhistory` |
| --- | --- | --- |
| See what a command printed | command text only | bounded `stdout` and `stderr` previews |
| Understand where it ran | usually current shell context only | `pwd_before`, `pwd_after`, session name, exit status |
| Revisit a failure later | manual memory and scrollback | `show` and `search` across stored shell events |

## Install

### Go

```bash
go install github.com/YusukeShimizu/richhistory/cmd/richhistory@latest
command -v richhistory
```

## Enable In `zsh`

Current shell:

```bash
eval "$(richhistory term init zsh)"
```

Persistent:

```bash
printf '\neval "$(richhistory term init zsh)"\n' >> ~/.zshrc
exec zsh
```

Add a session label when useful:

```bash
eval "$(richhistory term init zsh --name deploy)"
```

## Verify

```bash
echo hello
ls /definitely-missing
richhistory show -n 5
richhistory search hello --field stdout
```

## Commands

```bash
richhistory term init zsh
richhistory show
richhistory show <event-id>
richhistory search <query>
```

`richhistory` stores local NDJSON files under XDG state/config directories, rotates event files, and keeps capture bounded. Fullscreen or raw-TTY tools are recorded as metadata-only entries.

## Optional Examples

[`contrib/`](./contrib/README.md) contains optional helpers built on top of the journal.

- [`contrib/autosuggest`](./contrib/autosuggest/README.md): command suggestions backed by `codex exec`
- [`contrib/atc`](./contrib/atc/README.md): an `@c` shell helper for short Codex questions

AI integrations are examples, not part of the core CLI.
