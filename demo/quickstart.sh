#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
display_pwd="/workspace/richhistory"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

bin_dir="$tmp_dir/bin"
state_dir="$tmp_dir/state"
config_dir="$tmp_dir/config"
mkdir -p "$bin_dir" "$state_dir" "$config_dir"

export PATH="$bin_dir:$PATH"
export XDG_STATE_HOME="$state_dir"
export XDG_CONFIG_HOME="$config_dir"

(cd "$repo_root" && go build -o "$bin_dir/richhistory" ./cmd/richhistory >/dev/null 2>&1)

create_event() {
  local seq="$1"
  local command_text="$2"
  local stdout_text="$3"
  local stderr_text="$4"
  local exit_code="$5"
  local started_at="$6"
  local finished_at="$7"
  local event_id=""
  local assignments=""

  assignments="$(richhistory record start \
    --format shell \
    --session-id demo-session \
    --session-name demo \
    --seq "$seq" \
    --shell zsh \
    --shell-pid 4242 \
    --tty /dev/pts/1 \
    --pwd "$display_pwd" \
    --command "$command_text" \
    --started-at "$started_at")"
  eval "$assignments"

  if [[ -n "$stdout_text" ]]; then
    printf '%s' "$stdout_text" > "$RICHHISTORY_STDOUT_FILE"
  fi
  if [[ -n "$stderr_text" ]]; then
    printf '%s' "$stderr_text" > "$RICHHISTORY_STDERR_FILE"
  fi

  event_id="$(richhistory record finish \
    --state-file "$RICHHISTORY_EVENT_STATE" \
    --pwd-after "$display_pwd" \
    --exit-code "$exit_code" \
    --finished-at "$finished_at")"
  printf '%s\n' "$event_id"
}

run() {
  local command_text="$1"

  printf '$ %s\n' "$command_text"
  eval "$command_text"
  printf '\n'
}

hello_id="$(create_event \
  1 \
  "echo hello" \
  $'hello\n' \
  "" \
  0 \
  "2026-04-15T09:00:00Z" \
  "2026-04-15T09:00:01Z")"

failed_id="$(create_event \
  2 \
  "ls /definitely-missing" \
  "" \
  $'ls: /definitely-missing: No such file or directory\n' \
  1 \
  "2026-04-15T09:00:12Z" \
  "2026-04-15T09:00:13Z")"

run "richhistory show -n 2"
run "richhistory show $hello_id"
run "richhistory search hello --field stdout"

# Keep the failed event referenced so shellcheck doesn't complain if enabled later.
test -n "$failed_id"
