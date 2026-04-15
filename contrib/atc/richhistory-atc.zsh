#!/usr/bin/env zsh

if [[ -n "${RICHHISTORY_ATC_LOADED:-}" ]]; then
  return 0
fi
typeset -g RICHHISTORY_ATC_LOADED=1

autoload -Uz add-zsh-hook

typeset -g RICHHISTORY_ATC_HOME="${RICHHISTORY_ATC_HOME:-${${(%):-%N}:A:h:h:h}}"
typeset -g RICHHISTORY_ATC_MODEL="${RICHHISTORY_ATC_MODEL:-gpt-5.4}"
typeset -g RICHHISTORY_ATC_LAST_COMMAND=""
typeset -g RICHHISTORY_ATC_LAST_STATUS=0
typeset -ga RICHHISTORY_ATC_RECENT_COMMANDS=()

function __richhistory_command_name() {
  print -r -- "richhistory"
}

function __richhistory_atc_repo_root() {
  local root
  root="$(git -C "$PWD" rev-parse --show-toplevel 2>/dev/null)" || true
  if [[ -n "$root" ]]; then
    print -r -- "$root"
  fi
}

function __richhistory_atc_trim() {
  local raw="$1"
  raw="${raw%%$'\n'*}"
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  print -r -- "$raw"
}

function __richhistory_atc_recent_commands_block() {
  local cmd

  if (( ${#RICHHISTORY_ATC_RECENT_COMMANDS[@]} == 0 )); then
    print -r -- "(none)"
    return 0
  fi

  for cmd in "${RICHHISTORY_ATC_RECENT_COMMANDS[@]}"; do
    print -r -- "- $cmd"
  done
}

function __richhistory_atc_build_prompt() {
  local question="$1"
  local history_cmd last_command recent_commands repo_root

  history_cmd="$(__richhistory_command_name 2>/dev/null || print -r -- "richhistory")"
  repo_root="$(__richhistory_atc_repo_root)"
  recent_commands="$(__richhistory_atc_recent_commands_block)"
  last_command="${RICHHISTORY_ATC_LAST_COMMAND:-"(none)"}"

  cat <<EOF
You are helping a zsh user from the command line.
Answer the user's question concisely in plain text.
Do not execute anything.
If a shell command would help, you may include a short command example.
You may inspect recent shell history with ${history_cmd} if it helps. Useful commands include:
- ${history_cmd} show --cwd "$PWD" -n 20 --json
- ${history_cmd} show --status fail -n 10 --json
- ${history_cmd} search <query> --field stderr --n 20 --json

Current directory: ${PWD:A}
Repository root: ${repo_root:-"(none)"}
Most recent executed command: ${last_command}
Most recent exit status: ${RICHHISTORY_ATC_LAST_STATUS}
Recent commands:
${recent_commands}

User question:
${question}
EOF
}

function __richhistory_atc_run_codex() {
  local question="$1"
  local prompt output_file log_file answer
  local -a args

  prompt="$(__richhistory_atc_build_prompt "$question")"
  output_file="$(mktemp)" || return 1
  log_file="$(mktemp)" || {
    rm -f "$output_file"
    return 1
  }

  args=(
    exec
    -C "$PWD"
    --skip-git-repo-check
    --sandbox read-only
    --ephemeral
    --color never
    -m "$RICHHISTORY_ATC_MODEL"
    -c 'service_tier="fast"'
    -c 'model_reasoning_summary="none"'
    -c 'model_reasoning_effort="low"'
    -c 'model_verbosity="low"'
    -o "$output_file"
    "$prompt"
  )

  if ! codex "$args[@]" >"$log_file" 2>&1; then
    cat "$log_file" >&2
    rm -f "$output_file" "$log_file"
    return 1
  fi

  answer="$(<"$output_file")"
  rm -f "$output_file" "$log_file"
  print -r -- "$answer"
}

function __richhistory_atc_note_command() {
  local command="$(__richhistory_atc_trim "$1")"
  local count

  [[ -n "$command" ]] || return 0

  RICHHISTORY_ATC_LAST_COMMAND="$command"
  RICHHISTORY_ATC_RECENT_COMMANDS+=("$command")
  count=${#RICHHISTORY_ATC_RECENT_COMMANDS[@]}
  if (( count > 5 )); then
    RICHHISTORY_ATC_RECENT_COMMANDS=("${(@)RICHHISTORY_ATC_RECENT_COMMANDS[count-4,count]}")
  fi
}

function __richhistory_atc_update_status() {
  RICHHISTORY_ATC_LAST_STATUS=$?
}

function @c() {
  local question

  if (( $# == 0 )); then
    print -u2 -- 'usage: @c "question"'
    return 2
  fi
  if ! command -v codex >/dev/null 2>&1; then
    print -u2 -- "codex is required"
    return 127
  fi
  if ! command -v richhistory >/dev/null 2>&1; then
    print -u2 -- "richhistory is required"
    return 127
  fi

  question="$*"
  __richhistory_atc_run_codex "$question"
}

if [[ -o interactive ]]; then
  add-zsh-hook preexec __richhistory_atc_note_command
  add-zsh-hook precmd __richhistory_atc_update_status
fi
