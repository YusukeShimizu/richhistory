# richhistory

[![CI](https://github.com/YusukeShimizu/richhistory/actions/workflows/ci.yaml/badge.svg)](https://github.com/YusukeShimizu/richhistory/actions/workflows/ci.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](./go.mod)

`richhistory` keeps shell history with output, cwd, exit status, and session context.

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
- bounded output previews when capture is available
- working directory before and after execution
- exit status, duration, and session identity

## Why richhistory

| Need | Built-in shell history | `richhistory` |
| --- | --- | --- |
| See what a command printed | command text only | bounded output previews |
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

`richhistory` stores local NDJSON files under XDG state/config directories, rotates event files, and keeps capture bounded.

## WezTerm Output Capture

`richhistory` only captures output when the shell is running inside WezTerm and it can resolve a `wezterm` CLI. It detects that from `WEZTERM_PANE`, snapshots pane text with `wezterm cli get-text`, and stores the pre/post delta. If `wezterm` is not on `PATH`, `richhistory` also checks `RICHHISTORY_WEZTERM_CLI`, the sibling of `wezterm-gui`, and common macOS app bundle paths.

Capture mode precedence is:

- `ignore_command_patterns` or `ignore_cwd_patterns`: `skip`
- `WEZTERM_PANE` is set and a `wezterm` CLI is available: `full`
- otherwise: `metadata`

Outside WezTerm, events still record command text, cwd, exit status, duration, and session metadata, but output fields stay empty.

Because WezTerm exposes pane text rather than stream-separated pipes, pane capture is treated as combined output. `richhistory` stores that combined pane delta in both `stdout_text` and `stderr_text` so either field remains searchable, while `show` notes that the streams were not actually separated.

Config example:

```json
{
  "ignore_command_patterns": ["^secret "],
  "ignore_cwd_patterns": [],
  "max_stdout_bytes": 65536,
  "max_stderr_bytes": 32768
}
```

## Known Limitations

- Output capture is intentionally unavailable outside WezTerm.
- WezTerm capture reads pane text, so it cannot truly distinguish `stdout` from `stderr`.
- Pane snapshots depend on the available scrollback range and the `wezterm` CLI succeeding.

## Planned Improvements

- Improve pane-diff quality for cases where the visible screen rewrites heavily during a command.
- Expand shell support beyond `zsh`.

## Optional Examples

[`contrib/`](./contrib/README.md) contains optional helpers built on top of the journal.

- [`contrib/atc`](./contrib/atc/README.md): an `@c` shell helper for short Codex questions
- [`contrib/suggest`](./contrib/suggest/README.md): a `Ctrl-g` ghost-text helper that asks Codex for a one-line command suggestion

AI integrations are examples, not part of the core CLI.
