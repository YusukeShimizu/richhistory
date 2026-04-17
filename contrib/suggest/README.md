# `suggest` Ghost Command Helper

`suggest` is an optional `zsh` helper that asks `codex exec` for a single next command suggestion and shows it as ghost text when you press `Ctrl-g`.

It does not execute anything.
It renders a thin gray suggestion to the right of the current command line.
If `codex` wants more context, it may inspect `richhistory`.

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
export RICHHISTORY_SUGGEST_HOME="$HOME/src/richhistory"
export RICHHISTORY_SUGGEST_MODEL="gpt-5.4-mini"
[ -f "$RICHHISTORY_SUGGEST_HOME/contrib/suggest/richhistory-suggest.zsh" ] && \
  source "$RICHHISTORY_SUGGEST_HOME/contrib/suggest/richhistory-suggest.zsh"
```

Reload your shell:

```bash
exec zsh
```

## Behavior

- pressing `Ctrl-g` runs `codex exec` once and waits for a one-line command suggestion
- tells Codex that `richhistory` is available and gives example commands it may use
- includes the current command line buffer in the prompt it sends to Codex
- shows a thin gray loading marker first, then replaces it with the final suggestion
- shows the final one-line command suggestion as ghost text to the right of the current buffer
- clears the suggestion as soon as you change the buffer or run a command
- runs in `read-only` sandbox mode with `--ephemeral`
- uses `gpt-5.4-mini` with `service_tier="fast"`, low reasoning, and low verbosity

## Usage

Type part of a command if you want, then press `Ctrl-g`.

Examples:

```bash
git sta
<Ctrl-g>

<empty prompt>
<Ctrl-g>
```

## Test

```bash
zsh contrib/suggest/test_suggest.zsh
```
