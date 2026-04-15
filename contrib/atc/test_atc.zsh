#!/usr/bin/env zsh

set -euo pipefail

LIB_PATH="${0:A:h}/richhistory-atc.zsh"
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

function codex() {
  local model=""
  local outfile=""
  local prompt=""
  local response

  response="${CODEX_RESPONSE:-config file: config.toml}"

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
  return 0
}

source "$LIB_PATH"

CODEX_LOG="$TMP_DIR/codex.log"
: > "$CODEX_LOG"
export CODEX_LOG
ORIGINAL_PATH="$PATH"

__richhistory_atc_note_command '@g その設定ファイルはどこ？'
RICHHISTORY_ATC_LAST_STATUS=1

PROMPT_TEXT="$(__richhistory_atc_build_prompt "設定ファイルはどこ？")"
assert_contains "$PROMPT_TEXT" 'Most recent executed command: @g その設定ファイルはどこ？' "prompt should include the most recent command"
assert_contains "$PROMPT_TEXT" "richhistory show --cwd \"$PWD\" -n 20 --json" "prompt should include richhistory cwd hint"
assert_contains "$PROMPT_TEXT" 'User question:' "prompt should include a question section"
assert_contains "$PROMPT_TEXT" '設定ファイルはどこ？' "prompt should include the user question"

ANSWER="$(@c "設定ファイルはどこ？")"
assert_eq "config file: config.toml" "$ANSWER" "@c should print the final Codex answer"
assert_file_contains "$CODEX_LOG" 'model=gpt-5.4' "@c should use the configured model"
assert_file_contains "$CODEX_LOG" 'config=service_tier="fast"' "@c should force fast service tier"
assert_file_contains "$CODEX_LOG" 'config=model_reasoning_effort="low"' "@c should use low reasoning"
assert_file_contains "$CODEX_LOG" 'config=model_verbosity="low"' "@c should use low verbosity"

USAGE_OUTPUT="$({ @c; } 2>&1 || true)"
assert_contains "$USAGE_OUTPUT" 'usage: @c "question"' "@c should print usage when no question is given"

unfunction richhistory
PATH="$TMP_DIR"
MISSING_RICHHISTORY_OUTPUT="$({ @c "why did it fail?"; } 2>&1 || true)"
assert_contains "$MISSING_RICHHISTORY_OUTPUT" 'richhistory is required' "@c should fail clearly when history CLI is unavailable"
function richhistory() {
  return 0
}
PATH="$ORIGINAL_PATH"

unfunction codex
PATH="$TMP_DIR"
MISSING_CODEX_OUTPUT="$({ @c "why did it fail?"; } 2>&1 || true)"
assert_contains "$MISSING_CODEX_OUTPUT" 'codex is required' "@c should fail clearly when codex is unavailable"
PATH="$ORIGINAL_PATH"

print -- "ok"
