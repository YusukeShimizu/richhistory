# Codex Autosuggest

This helper asks `codex exec` for a single likely next command and lets Codex inspect `richhistory` when recent shell context is not enough.

It is useful when you want suggestions that can draw on earlier commands, failures, and output across shell sessions. With the current WezTerm pane capture path, `richhistory` output is primarily available through `stdout_text`, so the helper biases Codex toward `show --status fail` and `search --field stdout`.

## Requirements

- `zsh`
- `codex`
- `richhistory`

Initialize the journal first:

```bash
eval "$(richhistory term init zsh --name my-project)"
```

## Install

Add this to `~/.zshrc`:

```bash
export RICHHISTORY_AUTOSUGGEST_HOME="$HOME/src/richhistory"
export RICHHISTORY_CODEX_AUTOSUGGEST_HOME="$RICHHISTORY_AUTOSUGGEST_HOME"
export RICHHISTORY_CODEX_AUTOSUGGEST_MODEL="gpt-5.4"
[ -f "$RICHHISTORY_AUTOSUGGEST_HOME/contrib/autosuggest/richhistory-codex-autosuggest.zsh" ] && \
  source "$RICHHISTORY_AUTOSUGGEST_HOME/contrib/autosuggest/richhistory-codex-autosuggest.zsh"
```

Reload your shell:

```bash
exec zsh
```

## Behavior

- fetches one command suggestion on shell start, after each command, and after `cd`
- keeps the prompt responsive and fills only an empty buffer
- runs `codex exec` in `read-only` sandbox mode with `--ephemeral`
- uses `gpt-5.4` with `service_tier="fast"`, low reasoning, and low verbosity
- tells Codex that `richhistory` is available, so Codex can inspect prior shell work when useful
- never executes the suggested command automatically

## Test

```bash
zsh contrib/autosuggest/test_autosuggest.zsh
```
