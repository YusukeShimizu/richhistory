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

## Interactive Command Safety

`richhistory` captures output by swapping `stdout` and `stderr` in `preexec`. That can break interactive CLIs which expect real terminal file descriptors. To keep those tools usable, `richhistory` defaults many known interactive commands to `metadata` capture mode instead of full output capture.

Capture mode precedence is:

- `ignore_command_patterns` or `ignore_cwd_patterns`: `skip`
- `force_full_command_patterns`: `full`
- built-in interactive command list or `metadata_command_names`: `metadata`
- everything else: `full`

Built-in `metadata` defaults:

- AI CLIs: `aider`, `claude`, `claudecode`, `codex`, `gemini`, `goose`, `opencode`
- Editors and pagers: `emacs`, `helix`, `hx`, `kak`, `less`, `man`, `more`, `most`, `nano`, `nvim`, `vi`, `vim`
- Remote and multiplexers: `mosh`, `screen`, `sftp`, `ssh`, `tmux`, `zellij`
- TUI tools: `atop`, `btop`, `gitui`, `htop`, `k9s`, `lazygit`, `mc`, `nnn`, `ranger`, `tig`, `top`, `watch`, `yazi`
- Debuggers: `dlv`, `gdb`, `lldb`

Config example:

```json
{
  "ignore_command_patterns": ["^secret "],
  "ignore_cwd_patterns": [],
  "metadata_command_names": ["my-interactive-cli"],
  "force_full_command_patterns": ["^codex exec --json$"],
  "auto_add_metadata_commands": false
}
```

Use `metadata_command_names` when a command should stay usable but still be recorded as an event. Use `force_full_command_patterns` only when you accept the risk that an interactive command may lose its terminal behavior.

If you opt into `"auto_add_metadata_commands": true`, `richhistory` will add a command basename to `metadata_command_names` when a short-lived `full` capture fails with a terminal-related error such as `not a tty` or `stdin is not a terminal`.

## Known Limitations

- The default protection is name-based. Wrapper scripts or renamed binaries can still slip through.
- A broad default list avoids breakage, but it also means some commands that could have been fully captured will be recorded as metadata-only.
- Auto-add is conservative and off by default. Unknown commands can still fail once before being learned.
- `force_full_command_patterns` can re-introduce the original TTY breakage for matching commands.

## Planned Improvements

- Detect interactive behavior from shell-visible terminal signals instead of relying mostly on command names.
- Support finer-grained policies so one binary can stay `metadata` by default while known non-interactive subcommands use `full`.
- Add a simpler shell-side escape hatch for one-off overrides without editing config files.

## Optional Examples

[`contrib/`](./contrib/README.md) contains optional helpers built on top of the journal.

- [`contrib/autosuggest`](./contrib/autosuggest/README.md): command suggestions backed by `codex exec`
- [`contrib/atc`](./contrib/atc/README.md): an `@c` shell helper for short Codex questions

AI integrations are examples, not part of the core CLI.
