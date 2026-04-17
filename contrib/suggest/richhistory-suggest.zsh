#!/usr/bin/env zsh

if [[ -n "${RICHHISTORY_SUGGEST_LOADED:-}" ]]; then
  return 0
fi
typeset -g RICHHISTORY_SUGGEST_LOADED=1

autoload -Uz add-zsh-hook

typeset -g RICHHISTORY_SUGGEST_HOME="${RICHHISTORY_SUGGEST_HOME:-${${(%):-%N}:A:h:h:h}}"
typeset -g RICHHISTORY_SUGGEST_MODEL="${RICHHISTORY_SUGGEST_MODEL:-gpt-5.4-mini}"
typeset -g RICHHISTORY_SUGGEST_LOADING_TEXT="${RICHHISTORY_SUGGEST_LOADING_TEXT:- ...}"
typeset -g RICHHISTORY_SUGGEST_LAST_STATUS=0
typeset -g RICHHISTORY_SUGGEST_VALUE=""
typeset -g RICHHISTORY_SUGGEST_SOURCE_BUFFER=""
typeset -g RICHHISTORY_SUGGEST_REGION_MEMO="richhistory_suggest"

function __richhistory_suggest_command_name() {
  print -r -- "richhistory"
}

function __richhistory_suggest_repo_root() {
  local root

  root="$(git -C "$PWD" rev-parse --show-toplevel 2>/dev/null)" || true
  if [[ -n "$root" ]]; then
    print -r -- "$root"
  fi
}

function __richhistory_suggest_trim() {
  local raw="$1"

  raw="${raw%%$'\n'*}"
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  print -r -- "$raw"
}

function __richhistory_suggest_clear_display() {
  RICHHISTORY_SUGGEST_VALUE=""
  RICHHISTORY_SUGGEST_SOURCE_BUFFER=""
}

function __richhistory_suggest_apply_display() {
  POSTDISPLAY=""
  region_highlight=(${region_highlight:#*memo=${RICHHISTORY_SUGGEST_REGION_MEMO}})

  if [[ -z "$RICHHISTORY_SUGGEST_VALUE" ]]; then
    return 0
  fi
  if [[ "${BUFFER:-}" != "$RICHHISTORY_SUGGEST_SOURCE_BUFFER" ]]; then
    __richhistory_suggest_clear_display
    return 0
  fi

  POSTDISPLAY="$RICHHISTORY_SUGGEST_VALUE"
  region_highlight+=(
    "${#BUFFER} $(( ${#BUFFER} + ${#POSTDISPLAY} )) fg=8 memo=${RICHHISTORY_SUGGEST_REGION_MEMO}"
  )
}

function __richhistory_suggest_build_prompt() {
  local cwd="$1"
  local exit_status="$2"
  local current_buffer="$3"
  local history_cmd repo_root

  history_cmd="$(__richhistory_suggest_command_name 2>/dev/null || print -r -- "richhistory")"
  repo_root="$(__richhistory_suggest_repo_root)"

  cat <<EOF
You are helping a zsh user from the command line.
Reply with exactly one shell command on one line.
Do not explain anything.
Do not use markdown, bullets, or code fences.
You may inspect richhistory if it helps. Useful commands include:
- ${history_cmd} show --cwd "$cwd" -n 8 --json
- ${history_cmd} show --status fail -n 5 --json
- ${history_cmd} search <query> --field stderr --n 20 --json

Current directory: ${cwd:A}
Repository root: ${repo_root:-"(none)"}
Most recent exit status: ${exit_status}
Current command line buffer: ${current_buffer:-"(empty)"}
If no useful suggestion is clear, return an empty response.
EOF
}

function __richhistory_suggest_run_codex() {
  local cwd="$1"
  local exit_status="$2"
  local current_buffer="$3"
  local prompt output_file log_file answer
  local -a args

  prompt="$(__richhistory_suggest_build_prompt "$cwd" "$exit_status" "$current_buffer")"
  output_file="$(mktemp)" || return 1
  log_file="$(mktemp)" || {
    rm -f "$output_file"
    return 1
  }

  args=(
    exec
    -C "$cwd"
    --skip-git-repo-check
    --sandbox read-only
    --ephemeral
    --color never
    -m "$RICHHISTORY_SUGGEST_MODEL"
    -c 'service_tier="fast"'
    -c 'model_reasoning_effort="medium"'
    -c 'model_verbosity="low"'
    -o "$output_file"
    "$prompt"
  )

  if ! codex "$args[@]" >"$log_file" 2>&1; then
    rm -f "$output_file" "$log_file"
    return 1
  fi

  answer="$(<"$output_file")"
  rm -f "$output_file" "$log_file"

  if [[ "$answer" == *$'\n'* ]] || [[ "$answer" == *'```'* ]] || [[ "$answer" == *$'\r'* ]]; then
    return 0
  fi

  print -r -- "$(__richhistory_suggest_trim "$answer")"
}

function __richhistory_suggest_generate() {
  local current_buffer="$1"
  local answer=""

  if ! command -v codex >/dev/null 2>&1; then
    __richhistory_suggest_clear_display
    return 0
  fi
  if ! command -v richhistory >/dev/null 2>&1; then
    __richhistory_suggest_clear_display
    return 0
  fi

  answer="$(__richhistory_suggest_run_codex "$PWD" "$RICHHISTORY_SUGGEST_LAST_STATUS" "$current_buffer" || true)"
  answer="$(__richhistory_suggest_trim "$answer")"
  if [[ -z "$answer" ]] || [[ "$answer" == *$'\n'* ]] || [[ "$answer" == *'```'* ]]; then
    __richhistory_suggest_clear_display
    return 0
  fi

  RICHHISTORY_SUGGEST_VALUE="$answer"
  RICHHISTORY_SUGGEST_SOURCE_BUFFER="$current_buffer"
}

function __richhistory_suggest_widget() {
  local current_buffer="${BUFFER:-}"

  RICHHISTORY_SUGGEST_VALUE="$RICHHISTORY_SUGGEST_LOADING_TEXT"
  RICHHISTORY_SUGGEST_SOURCE_BUFFER="$current_buffer"
  __richhistory_suggest_apply_display
  zle -R

  __richhistory_suggest_generate "$current_buffer"
  __richhistory_suggest_apply_display
  zle -R
}

function __richhistory_suggest_preexec() {
  __richhistory_suggest_clear_display
}

function __richhistory_suggest_precmd() {
  RICHHISTORY_SUGGEST_LAST_STATUS=$?
}

function __richhistory_suggest_line_finish() {
  __richhistory_suggest_clear_display
}

function __richhistory_suggest_line_pre_redraw() {
  __richhistory_suggest_apply_display
}

function __richhistory_suggest_install_widgets() {
  if (( ${+functions[zle-line-finish]} )); then
    functions -c zle-line-finish __richhistory_suggest_prev_zle_line_finish
  fi
  if (( ${+functions[zle-line-pre-redraw]} )); then
    functions -c zle-line-pre-redraw __richhistory_suggest_prev_zle_line_pre_redraw
  fi

  function zle-line-finish() {
    if (( ${+functions[__richhistory_suggest_prev_zle_line_finish]} )); then
      __richhistory_suggest_prev_zle_line_finish "$@"
    fi
    __richhistory_suggest_line_finish "$@"
  }

  function zle-line-pre-redraw() {
    if (( ${+functions[__richhistory_suggest_prev_zle_line_pre_redraw]} )); then
      __richhistory_suggest_prev_zle_line_pre_redraw "$@"
    fi
    __richhistory_suggest_line_pre_redraw "$@"
  }

  zle -N zle-line-finish
  zle -N zle-line-pre-redraw
  zle -N richhistory-suggest __richhistory_suggest_widget
  bindkey '^G' richhistory-suggest
}

if [[ -o interactive ]]; then
  add-zsh-hook preexec __richhistory_suggest_preexec
  add-zsh-hook precmd __richhistory_suggest_precmd
  __richhistory_suggest_install_widgets
fi
