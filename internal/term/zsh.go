package term

import (
	"fmt"
	"strings"
)

const weztermCaptureStartLine = -20000

func ZshInit(commandName string, sessionName string) string {
	quotedCommand := ShellQuote(commandName)
	lines := []string{
		"autoload -Uz add-zsh-hook",
		`function __richhistory_resolve_wezterm_cli() {`,
		`  local candidate gui_path`,
		`  if [[ -n "${RICHHISTORY_WEZTERM_CLI:-}" ]]; then`,
		`    if [[ "${RICHHISTORY_WEZTERM_CLI}" == */* ]]; then`,
		`      [[ -x "${RICHHISTORY_WEZTERM_CLI}" ]] && print -r -- "${RICHHISTORY_WEZTERM_CLI}"`,
		`      return 0`,
		`    fi`,
		`    candidate="$(command -v -- "${RICHHISTORY_WEZTERM_CLI}" 2>/dev/null)" || true`,
		`    [[ -n "$candidate" ]] && print -r -- "$candidate"`,
		`    return 0`,
		`  fi`,
		`  candidate="$(command -v -- wezterm 2>/dev/null)" || true`,
		`  if [[ -n "$candidate" ]]; then`,
		`    print -r -- "$candidate"`,
		`    return 0`,
		`  fi`,
		`  gui_path="$(command -v -- wezterm-gui 2>/dev/null)" || true`,
		`  if [[ -n "$gui_path" ]]; then`,
		`    candidate="${gui_path:h}/wezterm"`,
		`    if [[ -x "$candidate" ]]; then`,
		`      print -r -- "$candidate"`,
		`      return 0`,
		`    fi`,
		`  fi`,
		`  for candidate in "/Applications/WezTerm.app/Contents/MacOS/wezterm" "$HOME/Applications/WezTerm.app/Contents/MacOS/wezterm"; do`,
		`    if [[ -x "$candidate" ]]; then`,
		`      print -r -- "$candidate"`,
		`      return 0`,
		`    fi`,
		`  done`,
		`  return 0`,
		`}`,
		"typeset -g RICHHISTORY_COMMAND=${RICHHISTORY_COMMAND:-" + quotedCommand + "}",
		`if [[ "$RICHHISTORY_COMMAND" != */* ]]; then`,
		`  RICHHISTORY_COMMAND="$(command -v -- "$RICHHISTORY_COMMAND" 2>/dev/null || print -r -- "$RICHHISTORY_COMMAND")"`,
		`fi`,
		`typeset -g RICHHISTORY_WEZTERM_CLI="${RICHHISTORY_WEZTERM_CLI:-}"`,
		`typeset -g RICHHISTORY_SESSION_ID="${RICHHISTORY_SESSION_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM$RANDOM}"`,
		`typeset -g RICHHISTORY_SESSION_SEQ="${RICHHISTORY_SESSION_SEQ:-0}"`,
		`typeset -g RICHHISTORY_CAPTURE_MODE="${RICHHISTORY_CAPTURE_MODE:-skip}"`,
		`typeset -g RICHHISTORY_EVENT_ID="${RICHHISTORY_EVENT_ID:-}"`,
		`typeset -g RICHHISTORY_EVENT_STATE="${RICHHISTORY_EVENT_STATE:-}"`,
		`typeset -g RICHHISTORY_CAPTURE_BEFORE_FILE="${RICHHISTORY_CAPTURE_BEFORE_FILE:-}"`,
		`typeset -g RICHHISTORY_CAPTURE_AFTER_FILE="${RICHHISTORY_CAPTURE_AFTER_FILE:-}"`,
	}
	if sessionName != "" {
		lines = append(lines, "typeset -g RICHHISTORY_SESSION_NAME="+ShellQuote(sessionName))
	}
	lines = append(lines, zshHookLines()...)

	return strings.Join(lines, "\n") + "\n"
}

func zshHookLines() []string {
	return []string{
		`function __richhistory_preexec() {`,
		`  RICHHISTORY_SESSION_SEQ=$(( RICHHISTORY_SESSION_SEQ + 1 ))`,
		`  local started_at`,
		`  local tty_name`,
		`  local capture_output`,
		`  local wezterm_cli`,
		`  local assignments`,
		`  started_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"`,
		`  tty_name="$(tty 2>/dev/null || printf 'unknown')"`,
		`  wezterm_cli="$(__richhistory_resolve_wezterm_cli)"`,
		`  if [[ -n "${WEZTERM_PANE:-}" ]] && [[ -n "$wezterm_cli" ]]; then`,
		`    RICHHISTORY_WEZTERM_CLI="$wezterm_cli"`,
		`    capture_output=true`,
		`  else`,
		`    capture_output=false`,
		`  fi`,
		`  assignments="$(command "$RICHHISTORY_COMMAND" record start --format shell --session-id "$RICHHISTORY_SESSION_ID" --session-name "${RICHHISTORY_SESSION_NAME:-}" --seq "$RICHHISTORY_SESSION_SEQ" --shell zsh --shell-pid "$$" --tty "$tty_name" --pwd "$PWD" --command "$1" --capture-output="$capture_output" --started-at "$started_at")" || return`,
		`  eval "$assignments"`,
		`  if [[ "$RICHHISTORY_CAPTURE_MODE" == "full" ]]; then`,
		fmt.Sprintf(
			`    "$RICHHISTORY_WEZTERM_CLI" cli get-text --pane-id "$WEZTERM_PANE" --start-line %d >| "$RICHHISTORY_CAPTURE_BEFORE_FILE" 2>/dev/null || true`,
			weztermCaptureStartLine,
		),
		`  fi`,
		`}`,
		`function __richhistory_precmd() {`,
		`  local exit_code="$?"`,
		`  if [[ "$RICHHISTORY_CAPTURE_MODE" == "full" ]]; then`,
		fmt.Sprintf(
			`    "$RICHHISTORY_WEZTERM_CLI" cli get-text --pane-id "$WEZTERM_PANE" --start-line %d >| "$RICHHISTORY_CAPTURE_AFTER_FILE" 2>/dev/null || true`,
			weztermCaptureStartLine,
		),
		`  fi`,
		`  command "$RICHHISTORY_COMMAND" record finish --state-file "$RICHHISTORY_EVENT_STATE" --pwd-after "$PWD" --exit-code "$exit_code" --finished-at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" >/dev/null 2>&1 || true`,
		`  RICHHISTORY_CAPTURE_MODE=skip`,
		`  RICHHISTORY_EVENT_ID=`,
		`  RICHHISTORY_EVENT_STATE=`,
		`  RICHHISTORY_CAPTURE_BEFORE_FILE=`,
		`  RICHHISTORY_CAPTURE_AFTER_FILE=`,
		`}`,
		`function __richhistory_zshexit() {`,
		`  rm -f "$RICHHISTORY_EVENT_STATE" "$RICHHISTORY_CAPTURE_BEFORE_FILE" "$RICHHISTORY_CAPTURE_AFTER_FILE" >/dev/null 2>&1 || true`,
		`  RICHHISTORY_CAPTURE_MODE=skip`,
		`  RICHHISTORY_EVENT_ID=`,
		`  RICHHISTORY_EVENT_STATE=`,
		`  RICHHISTORY_CAPTURE_BEFORE_FILE=`,
		`  RICHHISTORY_CAPTURE_AFTER_FILE=`,
		`}`,
		`add-zsh-hook preexec __richhistory_preexec`,
		`add-zsh-hook precmd __richhistory_precmd`,
		`add-zsh-hook zshexit __richhistory_zshexit`,
	}
}

func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func ShellAssignments(values map[string]string) string {
	keys := []string{
		"RICHHISTORY_CAPTURE_MODE",
		"RICHHISTORY_EVENT_ID",
		"RICHHISTORY_EVENT_STATE",
		"RICHHISTORY_CAPTURE_BEFORE_FILE",
		"RICHHISTORY_CAPTURE_AFTER_FILE",
	}
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, ShellQuote(values[key])))
	}

	return strings.Join(lines, "\n")
}
