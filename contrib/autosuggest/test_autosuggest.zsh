#!/usr/bin/env zsh

set -euo pipefail

LIB_PATH="${0:A:h}/richhistory-codex-autosuggest.zsh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

function assert_eq() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  if [[ "$expected" != "$actual" ]]; then
    print -u2 -- "FAIL: $message"
    print -u2 -- "expected: $expected"
    print -u2 -- "actual:   $actual"
    exit 1
  fi
}

function assert_contains() {
  local value="$1"
  local pattern="$2"
  local message="$3"
  if [[ "$value" != *"$pattern"* ]]; then
    print -u2 -- "FAIL: $message"
    print -u2 -- "missing pattern: $pattern"
    exit 1
  fi
}

function assert_file_contains() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if ! grep -Fq "$pattern" "$file"; then
    print -u2 -- "FAIL: $message"
    exit 1
  fi
}

function reset_state() {
  BUFFER=""
  CURSOR=0
  WIDGET=""
  RICHHISTORY_CODEX_AUTOSUGGEST_PRIMARY_FAILED=0
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING=""
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY=""
  RICHHISTORY_CODEX_AUTOSUGGEST_LAST_KEY=""
  RICHHISTORY_CODEX_AUTOSUGGEST_LAST_COMMAND=""
  RICHHISTORY_CODEX_AUTOSUGGEST_LAST_STATUS=0
  RICHHISTORY_CODEX_AUTOSUGGEST_NEEDS_REFRESH=1
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD=""
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_GENERATION=0
  RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION=0
  RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS=()
  RICHHISTORY_CODEX_AUTOSUGGEST_MODEL="gpt-5.4"
  RICHHISTORY_CODEX_AUTOSUGGEST_FALLBACK_MODEL="gpt-5.1-codex-mini"
  CODEX_FAIL_PRIMARY=0
  CODEX_PRIMARY_RESPONSE=""
  CODEX_FALLBACK_RESPONSE=""
  CODEX_LOG="$TMP_DIR/codex.log"
  : > "$CODEX_LOG"
  export CODEX_LOG
}

function codex() {
  local model=""
  local outfile=""
  local prompt=""
  local response=""

  while (( $# > 0 )); do
    case "$1" in
      -m)
        model="$2"
        shift 2
        ;;
      -o|--output-last-message)
        outfile="$2"
        shift 2
        ;;
      -c)
        print -r -- "config=$2" >> "$CODEX_LOG"
        shift 2
        ;;
      exec|--skip-git-repo-check|--ephemeral|--color|read-only|--sandbox|-C)
        if [[ "$1" == "-C" || "$1" == "--color" || "$1" == "--sandbox" ]]; then
          shift 2
        else
          shift
        fi
        ;;
      *)
        prompt="$1"
        shift
        ;;
    esac
  done

  print -r -- "model=$model" >> "$CODEX_LOG"
  print -r -- "prompt=$prompt" >> "$CODEX_LOG"

  if [[ "$model" == "$RICHHISTORY_CODEX_AUTOSUGGEST_MODEL" ]]; then
    if [[ "$CODEX_FAIL_PRIMARY" == "1" ]]; then
      return 1
    fi
    response="$CODEX_PRIMARY_RESPONSE"
  else
    response="$CODEX_FALLBACK_RESPONSE"
  fi

  [[ -n "$outfile" ]] || return 1
  print -r -- "$response" > "$outfile"
  return 0
}

function richhistory() {
  return 0
}

function zle() {
  return 0
}

source "$LIB_PATH"

function __richhistory_codex_autosuggest_start_async() {
  local key="$1"
  local generation result primary_failed suggestion
  local -a lines

  generation=$((RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION + 1))
  RICHHISTORY_CODEX_AUTOSUGGEST_REQUEST_GENERATION="$generation"
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_FD=""
  RICHHISTORY_CODEX_AUTOSUGGEST_ACTIVE_GENERATION="$generation"
  result="$(__richhistory_codex_autosuggest_fetch)"
  lines=("${(@f)result}")
  primary_failed="${lines[1]:-0}"
  if (( ${#lines[@]} > 1 )); then
    suggestion="${(j:$'\n':)lines[2,-1]}"
  else
    suggestion=""
  fi
  if [[ "$primary_failed" == "1" ]]; then
    RICHHISTORY_CODEX_AUTOSUGGEST_PRIMARY_FAILED=1
  fi
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING="$suggestion"
  RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY="$key"
  __richhistory_codex_autosuggest_apply_pending
}

function drain_active_request() {
  return 0
}

reset_state
__richhistory_codex_autosuggest_note_command '@g codexの設定ファイルはどこ？'
assert_eq '@g codexの設定ファイルはどこ？' "$RICHHISTORY_CODEX_AUTOSUGGEST_LAST_COMMAND" "last command should be recorded"
assert_eq "1" "${#RICHHISTORY_CODEX_AUTOSUGGEST_RECENT_COMMANDS[@]}" "recent commands should keep the latest command"
PROMPT_TEXT="$(__richhistory_codex_autosuggest_build_prompt "$(__richhistory_codex_autosuggest_repo_root)")"
assert_contains "$PROMPT_TEXT" 'Most recent executed command: @g codexの設定ファイルはどこ？' "prompt should include the last command"
assert_contains "$PROMPT_TEXT" 'You may inspect recent shell history with richhistory' "prompt should mention richhistory"
assert_contains "$PROMPT_TEXT" "richhistory show --cwd \"$PWD\" -n 20 --json" "prompt should include cwd richhistory hint"

reset_state
CODEX_PRIMARY_RESPONSE="git status"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "git status" "$BUFFER" "first prompt should auto-apply suggestion when buffer stays empty"
assert_eq "" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" "auto-applied suggestion should not remain pending"
assert_file_contains "$CODEX_LOG" 'model=gpt-5.4' "primary model should be used first"
assert_file_contains "$CODEX_LOG" 'config=model_reasoning_effort="low"' "primary model should use low reasoning"
assert_file_contains "$CODEX_LOG" 'config=model_verbosity="low"' "primary model should use low verbosity"
assert_file_contains "$CODEX_LOG" 'config=service_tier="fast"' "suggestion should force fast service tier"

BUFFER="echo keep"
CURSOR=${#BUFFER}
RICHHISTORY_CODEX_AUTOSUGGEST_PENDING="git diff"
RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY="$(__richhistory_codex_autosuggest_context_key)"
__richhistory_codex_autosuggest_apply_pending
assert_eq "echo keep" "$BUFFER" "non-empty buffer should not be overwritten"
assert_eq "git diff" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" "pending should remain when buffer is non-empty"

CODEX_PRIMARY_RESPONSE="git diff"
__richhistory_codex_autosuggest_queue
assert_eq "1" "$({ grep -c 'model=gpt-5.4' "$CODEX_LOG" || true; } | tr -d ' ')" "same prompt without refresh should not refetch"

__richhistory_codex_autosuggest_note_command "ls"
CODEX_PRIMARY_RESPONSE="git diff --stat"
BUFFER=""
CURSOR=0
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "git diff --stat" "$BUFFER" "post-command prompt should receive a fresh suggestion"

reset_state
BUFFER="echo keep"
CURSOR=${#BUFFER}
CODEX_PRIMARY_RESPONSE="git diff"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "echo keep" "$BUFFER" "async completion should not overwrite typed input"
assert_eq "git diff" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" "async completion should stay pending when input already exists"
assert_eq "$(__richhistory_codex_autosuggest_context_key)" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING_KEY" "pending suggestion should be tagged to the current context"

OLD_PWD="$PWD"
cd "$TMP_DIR"
__richhistory_codex_autosuggest_mark_dirty
CODEX_PRIMARY_RESPONSE="ls"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "ls" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" "cd should refresh pending suggestion"
cd "$OLD_PWD"

reset_state
CODEX_PRIMARY_RESPONSE=$'git status\npwd'
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" "multiline output should be rejected"

reset_state
CODEX_PRIMARY_RESPONSE="1. git status"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "git status" "$BUFFER" "numbered single-line output should be normalized to a plain command"

reset_state
CODEX_PRIMARY_RESPONSE="1"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "" "$RICHHISTORY_CODEX_AUTOSUGGEST_PENDING" "bare numbering should be rejected"
assert_eq "" "$BUFFER" "bare numbering should not be applied as a command"

reset_state
CODEX_PRIMARY_RESPONSE="- git diff --stat"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "git diff --stat" "$BUFFER" "bulleted single-line output should be normalized to a plain command"

reset_state
CODEX_FAIL_PRIMARY=1
CODEX_FALLBACK_RESPONSE="git status"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "git status" "$BUFFER" "fallback model should be used when primary fails"
assert_eq "1" "$RICHHISTORY_CODEX_AUTOSUGGEST_PRIMARY_FAILED" "primary failure should disable future primary retries"
assert_file_contains "$CODEX_LOG" 'model=gpt-5.1-codex-mini' "fallback model should run after primary failure"

BUFFER=""
CURSOR=0
CODEX_FALLBACK_RESPONSE="git diff --stat"
__richhistory_codex_autosuggest_note_command "pwd"
__richhistory_codex_autosuggest_queue
drain_active_request
assert_eq "git diff --stat" "$BUFFER" "subsequent fetches should keep using fallback after primary failure"
assert_eq "2" "$({ grep -c 'model=gpt-5.1-codex-mini' "$CODEX_LOG" || true; } | tr -d ' ')" "fallback should be called twice"
assert_eq "1" "$({ grep -c 'model=gpt-5.4' "$CODEX_LOG" || true; } | tr -d ' ')" "primary should only be attempted once after failure"

print -- "ok"
