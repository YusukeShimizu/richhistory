# `@c` Shell Question Helper

`@c` is a small `zsh` helper that asks `codex exec` a question from the shell.
It tells Codex that `richhistory` is available, so Codex can inspect recent shell history when your question depends on earlier commands, failures, or output.

This makes it useful for questions like:

- "Why did that command fail?"
- "Where is the config file in this repo?"
- "What should I inspect next after this error?"
- "What command should I run to verify what `@g` just told me?"

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
export RICHHISTORY_ATC_HOME="$HOME/src/richhistory"
export RICHHISTORY_ATC_MODEL="gpt-5.4"
[ -f "$RICHHISTORY_ATC_HOME/contrib/atc/richhistory-atc.zsh" ] && \
  source "$RICHHISTORY_ATC_HOME/contrib/atc/richhistory-atc.zsh"
```

Reload your shell:

```bash
exec zsh
```

## Usage

```bash
@c "where is the config file in this repo?"
@c "why did my last command fail?"
@c "what should I inspect next after the error above?"
```

## Behavior

- sends your question to `codex exec` from the current working directory
- includes current directory, repository root, recent commands, and last exit status in the prompt
- tells Codex that it can query `richhistory` for richer shell history if needed
- runs in `read-only` sandbox mode with `--ephemeral`
- uses `gpt-5.4` with `service_tier="fast"`, low reasoning, and low verbosity
- prints the final Codex answer to stdout without piping raw journal JSON from the shell

## Test

```bash
zsh contrib/atc/test_atc.zsh
```
