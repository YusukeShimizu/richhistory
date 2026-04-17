#!/usr/bin/env zsh

set -euo pipefail

LIB_PATH="${0:A:h}/richhistory-suggest.zsh"
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

function assert_file_empty() {
  local file="$1"
  local message="$2"

  if [[ -s "$file" ]]; then
    print -u2 -- "FAIL: $message"
    cat "$file" >&2
    exit 1
  fi
}

function codex() {
  local model=""
  local outfile=""
  local prompt=""
  local response

  response="${CODEX_RESPONSE:-git status --short}"
  print -r -- "${RICHHISTORY_SUGGEST_VALUE:-}|${RICHHISTORY_SUGGEST_SOURCE_BUFFER:-}|${POSTDISPLAY:-}" >| "$LOADING_SNAPSHOT_FILE"

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
  [[ -n "$outfile" ]] || return 1
  print -r -- "$response" > "$outfile"
  return 0
}

function richhistory() {
  print -r -- "$*" >> "$RICHHISTORY_LOG"
  return 0
}

typeset -ga ZLE_CALLS=()
typeset -gi ZLE_REDRAW_COUNT=0

function zle() {
  case "$1" in
    -N)
      ZLE_CALLS+=("widget:$2")
      ;;
    -R)
      ZLE_REDRAW_COUNT=$(( ZLE_REDRAW_COUNT + 1 ))
      ;;
  esac
  return 0
}

function bindkey() {
  print -r -- "bind:$1:$2" >> "$BINDKEY_LOG"
}

source "$LIB_PATH"

CODEX_LOG="$TMP_DIR/codex.log"
RICHHISTORY_LOG="$TMP_DIR/richhistory.log"
BINDKEY_LOG="$TMP_DIR/bindkey.log"
LOADING_SNAPSHOT_FILE="$TMP_DIR/loading.snapshot"
: > "$CODEX_LOG"
: > "$RICHHISTORY_LOG"
: > "$BINDKEY_LOG"
: > "$LOADING_SNAPSHOT_FILE"
export CODEX_LOG
export RICHHISTORY_LOG
ORIGINAL_PATH="$PATH"

__richhistory_suggest_install_widgets

PROMPT_TEXT="$(__richhistory_suggest_build_prompt "$PWD" 23 "git sta")"
assert_contains "$PROMPT_TEXT" "Current directory: ${PWD:A}" "prompt should include cwd"
assert_contains "$PROMPT_TEXT" "Most recent exit status: 23" "prompt should include exit status"
assert_contains "$PROMPT_TEXT" 'Current command line buffer: git sta' "prompt should include the current buffer"
assert_contains "$PROMPT_TEXT" "richhistory show --cwd \"$PWD\" -n 8 --json" "prompt should include richhistory cwd hint"

BUFFER="git sta"
CURSOR=${#BUFFER}
region_highlight=()
RICHHISTORY_SUGGEST_LAST_STATUS=7
CODEX_RESPONSE="git status --short"
__richhistory_suggest_widget
assert_eq " ...|git sta| ..." "$(cat "$LOADING_SNAPSHOT_FILE")" "widget should show loading ghost text before running Codex"
assert_eq "git status --short" "$RICHHISTORY_SUGGEST_VALUE" "widget should store the suggestion"
assert_eq "git sta" "$RICHHISTORY_SUGGEST_SOURCE_BUFFER" "widget should remember the source buffer"
assert_eq 2 "$ZLE_REDRAW_COUNT" "widget should redraw once for loading and once for the final suggestion"
assert_contains "$(cat "$CODEX_LOG")" 'model=gpt-5.4-mini' "helper should use gpt-5.4-mini by default"
assert_contains "$(cat "$CODEX_LOG")" 'config=service_tier="fast"' "helper should use fast service tier"
assert_file_empty "$RICHHISTORY_LOG" "helper should not call richhistory directly"

__richhistory_suggest_line_pre_redraw
assert_eq "git status --short" "$POSTDISPLAY" "ghost suggestion should be displayed after the current buffer"
assert_contains "${(j: :)region_highlight}" "memo=richhistory_suggest" "ghost suggestion should be dimmed via region_highlight"

BUFFER="git stat"
__richhistory_suggest_line_pre_redraw
assert_eq "" "$POSTDISPLAY" "ghost suggestion should disappear when typing changes the buffer"
assert_eq "" "$RICHHISTORY_SUGGEST_VALUE" "ghost suggestion state should clear once the buffer changes"

BUFFER=""
CODEX_RESPONSE=$'git status\nexplain'
__richhistory_suggest_widget
assert_eq "" "$RICHHISTORY_SUGGEST_VALUE" "multi-line responses should be ignored"

unfunction codex
PATH="$TMP_DIR"
BUFFER="echo hi"
__richhistory_suggest_widget
assert_eq "" "$RICHHISTORY_SUGGEST_VALUE" "helper should no-op when codex is unavailable"
PATH="$ORIGINAL_PATH"

assert_contains "$(cat "$BINDKEY_LOG")" 'bind:^G:richhistory-suggest' "helper should bind Ctrl-g to the widget"

print -- "ok"
