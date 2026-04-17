# richhistory Specification

## Scope

- `richhistory` is a local shell journal for `zsh`.
- It captures command lifecycle metadata plus bounded output previews.
- Output capture is only enabled inside WezTerm when a `wezterm` CLI can be resolved.
- Storage is local NDJSON with rotation and pruning.
- Optional helpers may build on top of the journal, but they are not part of the core CLI contract.
- AI helpers under `contrib/` may tell external tools that `richhistory` is available, but they do not extend the core command set and may expose only manual shell helpers such as key bindings.

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
- Capture mode precedence is `skip > full in WezTerm with resolved wezterm CLI > metadata elsewhere`

## Event Shape

Each record contains:

- identity: `id`, `session_id`, `session_name`, `seq`
- execution: `command`, `shell`, `shell_pid`, `tty`, `host`
- directory context: `pwd_before`, `pwd_after`
- timing: `started_at`, `finished_at`, `duration_ms`
- result: `exit_code`, `capture_mode`
- bounded outputs: `stdout_text`, `stderr_text`
- WezTerm capture stores combined pane-text deltas in both `stdout_text` and `stderr_text` because the pane API does not expose separated streams
- byte accounting: `stdout_bytes_total`, `stderr_bytes_total`, `stdout_stored_bytes`, `stderr_stored_bytes`
- truncation flags: `stdout_truncated`, `stderr_truncated`
