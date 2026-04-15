#!/usr/bin/env zsh

if [[ -n "${RICHHISTORY_CODEX_AUTOSUGGEST_LOADED:-}" ]]; then
  return 0
fi
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_LOADED=1

autoload -Uz add-zsh-hook
autoload -Uz add-zle-hook-widget 2>/dev/null || true

typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_HOME="${RICHHISTORY_CODEX_AUTOSUGGEST_HOME:-${${(%):-%N}:A:h:h:h}}"
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_MODEL="${RICHHISTORY_CODEX_AUTOSUGGEST_MODEL:-gpt-5.4}"
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_FALLBACK_MODEL="${RICHHISTORY_CODEX_AUTOSUGGEST_FALLBACK_MODEL:-}"
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_PRIMARY_FAILED=0
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_PENDING=""
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY=""
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_LAST_KEY=""
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_LAST_COMMAND=""
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_LAST_STATUS=0
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_NEEDS_REFRESH=1
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD=""
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_GENERATION=0
typeset -g RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION=0
typeset -ga RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS=()

function __richhistory_command_name() {
  print -r -- "richhistory"
}

function __richhistory_codex_autosuggest_repo_root() {
  local root
  root="$(git -C "$PWD" rev-parse --show-toplevel 2>/dev/null)" || true
  if [[ -n "$root" ]]; then
    print -r -- "$root"
  fi
}

function __richhistory_codex_autosuggest_context_key() {
  local repo_root
  repo_root="$(__richhistory_codex_autosuggest_repo_root)"
  print -r -- "${repo_root:-"(none)"}::${PWD:A}"
}

function __richhistory_codex_autosuggest_trim() {
  local raw="$1"
  raw="${raw%%$'\n'*}"
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  print -r -- "$raw"
}

function __richhistory_codex_autosuggest_recent_commands_block() {
  local cmd

  if (( ${#RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS[@]} == 0 )); then
    print -r -- "(none)"
    return 0
  fi

  for cmd in "${RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS[@]}"; do
    print -r -- "- $cmd"
  done
}

function __richhistory_codex_autosuggest_build_prompt() {
  local repo_root="$1"
  local history_cmd last_command recent_commands

  history_cmd="$(__richhistory_command_name 2>/dev/null || print -r -- "richhistory")"
  last_command="${RICHHISTORY_CODEX_AUTOSUGGEST_LAST_COMMAND:-"(none)"}"
  recent_commands="$(__richhistory_codex_autosuggest_recent_commands_block)"
  cat <<EOF
You are generating a zsh command autosuggestion.
Return exactly one shell command.
Output one line only.
Do not include markdown, bullets, quotes, or explanations.
Do not prefix the command with numbering such as 1. or 1).
Do not execute anything.
Prefer a safe command that helps the user continue the current task.
If the most recent command was a question to an AI assistant such as @g, @goose, or codex, prefer a concrete follow-up command that verifies or inspects the answer.
You may inspect recent shell history with ${history_cmd} if it helps disambiguate. Useful commands include:
- ${history_cmd} show --cwd "$PWD" -n 20 --json
- ${history_cmd} show --status fail -n 10 --json
- ${history_cmd} search <query> --field stderr --n 20 --json

Current directory: ${PWD:A}
Repository root: ${repo_root:-"(none)"}
Most recent executed command: ${last_command}
Most recent exit status: ${RICHHISTORY_CODEX_AUTOSUGGEST_LAST_STATUS}
Recent commands:
${recent_commands}
EOF
}

function __richhistory_codex_autosuggest_sanitize() {
  local raw="$1"
  local line_count

  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  [[ -n "$raw" ]] || return 0

  line_count="$(print -r -- "$raw" | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
  [[ "$line_count" == "1" ]] || return 0

  raw="${raw#Command: }"
  raw="${raw#Suggested command: }"
  raw="${raw#Suggested Command: }"
  raw="${raw#Run: }"
  if [[ "$raw" =~ '^[[:space:]]*[0-9]+[.)、,:-][[:space:]]+' ]]; then
    raw="${raw#$MATCH}"
  fi
  if [[ "$raw" =~ '^[[:space:]]*[-*][[:space:]]+' ]]; then
    raw="${raw#$MATCH}"
  fi
  raw="${raw#\`}"
  raw="${raw%\`}"
  if [[ "$raw" == \"*\" ]]; then
    raw="${raw#\"}"
    raw="${raw%\"}"
  fi
  if [[ "$raw" == \'*\' ]]; then
    raw="${raw#\'}"
    raw="${raw%\'}"
  fi
  raw="$(__richhistory_codex_autosuggest_trim "$raw")"

  [[ -n "$raw" ]] || return 0
  [[ "$raw" != *$'\n'* ]] || return 0
  [[ "$raw" != *'```'* ]] || return 0
  [[ "$raw" != <-> ]] || return 0

  print -r -- "$raw"
}

function __richhistory_codex_autosuggest_run_codex() {
  local model="$1"
  local prompt="$2"
  local output_file log_file suggestion
  local -a args

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
    -c 'service_tier="fast"'
    -c 'model_reasoning_summary="none"'
  )
  if [[ -n "$model" ]]; then
    args+=(-m "$model")
  fi
  if [[ "$model" != "$RICHHISTORY_CODEX_AUTOSUGGEST_FALLBACK_MODEL" ]]; then
    args+=(
      -c 'model_reasoning_effort="low"'
      -c 'model_verbosity="low"'
    )
  fi
  args+=(
    -o "$output_file"
    "$prompt"
  )

  if codex "$args[@]" >"$log_file" 2>&1; then
    suggestion="$(<"$output_file")"
    rm -f "$output_file" "$log_file"
    print -r -- "$suggestion"
    return 0
  fi

  rm -f "$output_file" "$log_file"
  return 1
}

function __richhistory_codex_autosuggest_fetch() {
  local repo_root prompt suggestion sanitized primary_failed
  primary_failed=0

  if ! command -v codex >/dev/null 2>&1 || ! command -v richhistory >/dev/null 2>&1; then
    print -r -- "$primary_failed"
    print -r --
    return 0
  fi

  repo_root="$(__richhistory_codex_autosuggest_repo_root)"
  prompt="$(__richhistory_codex_autosuggest_build_prompt "$repo_root")"

  if [[ "$RICHHISTORY_CODEX_AUTOSUGGEST_PRIMARY_FAILED" != "1" ]]; then
    if suggestion="$(__richhistory_codex_autosuggest_run_codex "$RICHHISTORY_CODEX_AUTOSUGGEST_MODEL" "$prompt")"; then
      sanitized="$(__richhistory_codex_autosuggest_sanitize "$suggestion")"
      if [[ -n "$sanitized" ]]; then
        print -r -- "$primary_failed"
        print -r -- "$sanitized"
        return 0
      fi
    else
      primary_failed=1
    fi
  fi

  if [[ -n "$RICHHISTORY_CODEX_AUTOSUGGEST_FALLBACK_MODEL" ]]; then
    if suggestion="$(__richhistory_codex_autosuggest_run_codex "$RICHHISTORY_CODEX_AUTOSUGGEST_FALLBACK_MODEL" "$prompt")"; then
      sanitized="$(__richhistory_codex_autosuggest_sanitize "$suggestion")"
    fi
  fi

  print -r -- "$primary_failed"
  print -r -- "$sanitized"
}

function __richhistory_codex_autosuggest_fetch_payload() {
  local generation="$1"
  local key="$2"
  local result primary_failed suggestion
  local -a lines

  result="$(__richhistory_codex_autosuggest_fetch)"
  lines=("${(@f)result}")
  primary_failed="${lines[1]:-0}"
  if (( ${#lines[@]} > 1 )); then
    suggestion="${(j:$'\n':)lines[2,-1]}"
  else
    suggestion=""
  fi
  print -r -- "$generation"
  print -r -- "$key"
  print -r -- "$primary_failed"
  print -r -- "$suggestion"
}

function __richhistory_codex_autosuggest_clear_active() {
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD=""
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_GENERATION=0
}

function __richhistory_codex_autosuggest_close_fd() {
  local fd="$1"

  [[ -n "$fd" ]] || return 0
  if [[ -o interactive ]]; then
    zle -F "$fd" 2>/dev/null || true
  fi
  exec {fd}<&- 2>/dev/null || true

  if [[ "$RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD" == "$fd" ]]; then
    __richhistory_codex_autosuggest_clear_active
  fi
}

function __richhistory_codex_autosuggest_on_readable() {
  local fd="$1"
  local generation=""
  local key=""
  local primary_failed=""
  local suggestion=""

  if ! IFS= read -ru "$fd" generation; then
    __richhistory_codex_autosuggest_close_fd "$fd"
    return 0
  fi
  IFS= read -ru "$fd" key || key=""
  IFS= read -ru "$fd" primary_failed || primary_failed="0"
  IFS= read -ru "$fd" suggestion || suggestion=""
  __richhistory_codex_autosuggest_close_fd "$fd"

  if [[ "$generation" != "$RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION" ]]; then
    return 0
  fi
  if [[ "$primary_failed" == "1" ]]; then
    RICHHISTORY_CODEX_AUTOSUGGEST_PRIMARY_FAILED=1
  fi
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING="$suggestion"
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY="$key"
  __richhistory_codex_autosuggest_apply_pending
}

function __richhistory_codex_autosuggest_start_async() {
  local key="$1"
  local generation fd

  generation=$((RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION + 1))
  RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION="$generation"

  if [[ -n "$RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD" ]]; then
    __richhistory_codex_autosuggest_close_fd "$RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD"
  fi

  exec {fd}< <(__richhistory_codex_autosuggest_fetch_payload "$generation" "$key")
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD="$fd"
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_GENERATION="$generation"
  if [[ -o interactive ]]; then
    zle -F "$fd" __richhistory_codex_autosuggest_on_readable
  else
    __richhistory_codex_autosuggest_on_readable "$fd"
  fi
}

function __richhistory_codex_autosuggest_queue() {
  local key last_status

  last_status="$?"
  RICHHISTORY_CODEX_AUTOSUGGEST_LAST_STATUS="$last_status"
  key="$(__richhistory_codex_autosuggest_context_key)"
  if [[ "$RICHHISTORY_CODEX_AUTOSUGGEST_NEEDS_REFRESH" != "1" && "$RICHHISTORY_CODEX_AUTOSUGGEST_LAST_KEY" == "$key" ]]; then
    return 0
  fi

  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING=""
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY=""
  RICHHISTORY_CODEX_AUTOSUGGEST_LAST_KEY="$key"
  RICHHISTORY_CODEX_AUTOSUGGEST_NEEDS_REFRESH=0
  __richhistory_codex_autosuggest_start_async "$key"
}

function __richhistory_codex_autosuggest_apply_pending() {
  if [[ -n "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" && "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY" != "$(__richhistory_codex_autosuggest_context_key)" ]]; then
    RICHHISTORY_CODEX_AUTOSUGGEST_PENDING=""
    RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY=""
  fi

  if [[ -n "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" && -z "$BUFFER" ]]; then
    BUFFER="$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING"
    CURSOR=${#BUFFER}
    RICHHISTORY_CODEX_AUTOSUGGEST_PENDING=""
    RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY=""
    if [[ -o interactive ]]; then
      zle redisplay
    fi
  fi
}

function __richhistory_codex_autosuggest_mark_dirty() {
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING=""
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY=""
  RICHHISTORY_CODEX_AUTOSUGGEST_NEEDS_REFRESH=1
}

function __richhistory_codex_autosuggest_note_command() {
  local command="$(__richhistory_codex_autosuggest_trim "$1")"
  local count

  [[ -n "$command" ]] || return 0

  RICHHISTORY_CODEX_AUTOSUGGEST_LAST_COMMAND="$command"
  RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS+=("$command")
  RICHHISTORY_CODEX_AUTOSUGGEST_NEEDS_REFRESH=1
  count=${#RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS[@]}
  if (( count > 5 )); then
    RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS=("${(@)RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS[count-4,count]}")
  fi
}

if [[ -o interactive ]]; then
  add-zsh-hook preexec __richhistory_codex_autosuggest_note_command
  add-zsh-hook precmd __richhistory_codex_autosuggest_queue
  add-zsh-hook chpwd __richhistory_codex_autosuggest_mark_dirty
  zle -N zle-line-init __richhistory_codex_autosuggest_apply_pending
  zle -N zle-keymap-select __richhistory_codex_autosuggest_apply_pending
fi
