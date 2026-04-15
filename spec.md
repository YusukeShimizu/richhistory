# richhistory Specification

## Scope

- `richhistory` is a local shell journal for `zsh`.
- It captures command lifecycle metadata plus bounded output previews.
- Capture is best-effort and prioritizes preserving interactive terminal behavior over full output capture.
- Fullscreen, raw-TTY, and other known interactive commands remain metadata-only by default.
- Storage is local NDJSON with rotation and pruning.
- Optional helpers may build on top of the journal, but they are not part of the core CLI contract.

## Public Commands

- `richhistory term init zsh [--name NAME]`
- `richhistory show [--n N] [--session ID|NAME] [--cwd PREFIX] [--status ok|fail|any] [--json]`
- `richhistory show <event-id> [--json]`
- `richhistory search <query> [--field cmd|cwd|stdout|stderr|all] [--n N] [--json]`

## Storage Contract

- Canonical config path: `$XDG_CONFIG_HOME/richhistory/config.json` or `~/.config/richhistory/config.json`
- Canonical state root: `$XDG_STATE_HOME/richhistory` or `~/.local/state/richhistory`
- Event files: `events/YYYY-MM-DD.ndjson`
- Rotation threshold: 8 MiB per file
- Default retention: 30 days
- Default total `events/` cap: 128 MiB

## Config Contract

- `ignore_command_patterns`: regex list; matching commands are skipped entirely
- `ignore_cwd_patterns`: regex list; matching working directories are skipped entirely
- `metadata_command_names`: exact basename list added to the built-in metadata-only defaults
- `force_full_command_patterns`: regex list; matching commands force `full` capture even if their basename is metadata-only by default
- `auto_add_metadata_commands`: disabled by default; when enabled, short-lived `full` capture failures with terminal-related stderr may append the command basename to `metadata_command_names`
- Capture mode precedence is `skip > force_full > metadata > full`

Built-in metadata-only command names:

- AI CLIs: `aider`, `claude`, `claudecode`, `codex`, `gemini`, `goose`, `opencode`
- Editors and pagers: `emacs`, `helix`, `hx`, `kak`, `less`, `man`, `more`, `most`, `nano`, `nvim`, `vi`, `vim`
- Remote and multiplexers: `mosh`, `screen`, `sftp`, `ssh`, `tmux`, `zellij`
- TUI tools: `atop`, `btop`, `gitui`, `htop`, `k9s`, `lazygit`, `mc`, `nnn`, `ranger`, `tig`, `top`, `watch`, `yazi`
- Debuggers: `dlv`, `gdb`, `lldb`

## Event Shape

Each record contains:

- identity: `id`, `session_id`, `session_name`, `seq`
- execution: `command`, `shell`, `shell_pid`, `tty`, `host`
- directory context: `pwd_before`, `pwd_after`
- timing: `started_at`, `finished_at`, `duration_ms`
- result: `exit_code`, `capture_mode`
- bounded outputs: `stdout_text`, `stderr_text`
- byte accounting: `stdout_bytes_total`, `stderr_bytes_total`, `stdout_stored_bytes`, `stderr_stored_bytes`
- truncation flags: `stdout_truncated`, `stderr_truncated`
