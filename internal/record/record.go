package record

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YusukeShimizu/richhistory/internal/config"
	"github.com/YusukeShimizu/richhistory/internal/sanitize"
	"github.com/YusukeShimizu/richhistory/internal/store"
)

const (
	dirPerm             = 0o750
	filePerm            = 0o600
	liveDir             = "live"
	autoAddMaxDuration  = 3 * time.Second
	captureModeSkip     = "skip"
	captureModeMetadata = "metadata"
	captureModeFull     = "full"
)

type StartInput struct {
	SessionID   string
	SessionName string
	Seq         int
	Shell       string
	ShellPID    int
	TTY         string
	Command     string
	PWD         string
	StartedAt   time.Time
}

type StartResult struct {
	EventID     string
	StateFile   string
	StdoutFile  string
	StderrFile  string
	CaptureMode string
}

type FinishInput struct {
	StateFile  string
	PWDAfter   string
	ExitCode   int
	FinishedAt time.Time
}

type liveState struct {
	EventID     string    `json:"event_id"`
	SessionID   string    `json:"session_id"`
	SessionName string    `json:"session_name"`
	Seq         int       `json:"seq"`
	Shell       string    `json:"shell"`
	ShellPID    int       `json:"shell_pid"`
	TTY         string    `json:"tty"`
	Command     string    `json:"command"`
	PWDBefore   string    `json:"pwd_before"`
	StartedAt   time.Time `json:"started_at"`
	StdoutFile  string    `json:"stdout_file,omitempty"`
	StderrFile  string    `json:"stderr_file,omitempty"`
	CaptureMode string    `json:"capture_mode"`
}

func Start(root string, cfg config.Config, input StartInput) (StartResult, error) {
	if err := cfg.Prepared(); err != nil {
		return StartResult{}, err
	}

	if shouldSkip(cfg, input) {
		return StartResult{CaptureMode: captureModeSkip}, nil
	}

	eventID, eventIDErr := newEventID(input.StartedAt)
	if eventIDErr != nil {
		return StartResult{}, eventIDErr
	}

	state := liveState{
		EventID:     eventID,
		SessionID:   input.SessionID,
		SessionName: input.SessionName,
		Seq:         input.Seq,
		Shell:       input.Shell,
		ShellPID:    input.ShellPID,
		TTY:         input.TTY,
		Command:     boundCommand(input.Command, cfg.MaxCommandBytes),
		PWDBefore:   input.PWD,
		StartedAt:   input.StartedAt.UTC(),
		CaptureMode: captureMode(cfg, input.Command),
	}

	livePath := filepath.Join(root, liveDir, input.SessionID)
	mkdirErr := os.MkdirAll(livePath, dirPerm)
	if mkdirErr != nil {
		return StartResult{}, fmt.Errorf("create live dir: %w", mkdirErr)
	}

	outputErr := prepareOutputFiles(livePath, &state)
	if outputErr != nil {
		return StartResult{}, outputErr
	}

	stateFile := filepath.Join(livePath, eventID+".json")
	writeErr := writeStateFile(stateFile, state)
	if writeErr != nil {
		return StartResult{}, writeErr
	}

	return StartResult{
		EventID:     eventID,
		StateFile:   stateFile,
		StdoutFile:  state.StdoutFile,
		StderrFile:  state.StderrFile,
		CaptureMode: state.CaptureMode,
	}, nil
}

func Finish(root string, cfg config.Config, input FinishInput) (store.Event, bool, error) {
	if err := cfg.Prepared(); err != nil {
		return store.Event{}, false, err
	}

	if input.StateFile == "" {
		return store.Event{}, false, nil
	}

	state, loadErr := loadState(input.StateFile)
	if loadErr != nil {
		if errors.Is(loadErr, os.ErrNotExist) {
			return store.Event{}, false, nil
		}

		return store.Event{}, false, loadErr
	}

	host, hostErr := os.Hostname()
	if hostErr != nil {
		return store.Event{}, false, fmt.Errorf("resolve hostname: %w", hostErr)
	}

	stdoutBound, readStdoutErr := readBoundedOutput(state.StdoutFile, cfg.MaxStdoutBytes)
	if readStdoutErr != nil {
		return store.Event{}, false, readStdoutErr
	}

	stderrBound, readStderrErr := readBoundedOutput(state.StderrFile, cfg.MaxStderrBytes)
	if readStderrErr != nil {
		return store.Event{}, false, readStderrErr
	}

	finishedAt := input.FinishedAt.UTC()
	duration := finishedAt.Sub(state.StartedAt)
	event := store.Event{
		ID:                state.EventID,
		SessionID:         state.SessionID,
		SessionName:       state.SessionName,
		Seq:               state.Seq,
		Shell:             state.Shell,
		ShellPID:          state.ShellPID,
		TTY:               state.TTY,
		Host:              host,
		PWDBefore:         state.PWDBefore,
		PWDAfter:          input.PWDAfter,
		StartedAt:         state.StartedAt,
		FinishedAt:        finishedAt,
		DurationMS:        duration.Milliseconds(),
		ExitCode:          input.ExitCode,
		Command:           state.Command,
		CaptureMode:       state.CaptureMode,
		StdoutText:        stdoutBound.Text,
		StderrText:        stderrBound.Text,
		StdoutBytesTotal:  stdoutBound.TotalBytes,
		StderrBytesTotal:  stderrBound.TotalBytes,
		StdoutStoredBytes: stdoutBound.StoredBytes,
		StderrStoredBytes: stderrBound.StoredBytes,
		StdoutTruncated:   stdoutBound.Truncated,
		StderrTruncated:   stderrBound.Truncated,
	}

	appendErr := store.Append(root, cfg, event)
	if appendErr != nil {
		return store.Event{}, false, appendErr
	}

	autoAddErr := maybeAutoAddMetadataCommand(cfg, state, input.ExitCode, duration, stderrBound.Text)
	if autoAddErr != nil {
		return store.Event{}, false, autoAddErr
	}

	pruneErr := store.Prune(root, cfg, finishedAt)
	if pruneErr != nil {
		return store.Event{}, false, pruneErr
	}

	cleanupErr := cleanup(state, input.StateFile)
	if cleanupErr != nil {
		return store.Event{}, false, cleanupErr
	}

	return event, true, nil
}

func shouldSkip(cfg config.Config, input StartInput) bool {
	if input.Command == "" || input.SessionID == "" {
		return true
	}

	trimmed := strings.TrimSpace(input.Command)
	if trimmed == "richhistory" || strings.HasPrefix(trimmed, "richhistory ") {
		return true
	}

	return cfg.IgnoreCommand(input.Command) || cfg.IgnoreCWD(input.PWD)
}

func prepareOutputFiles(livePath string, state *liveState) error {
	if state.CaptureMode != captureModeFull {
		return nil
	}

	state.StdoutFile = filepath.Join(livePath, state.EventID+".stdout")
	state.StderrFile = filepath.Join(livePath, state.EventID+".stderr")

	stdoutErr := os.WriteFile(state.StdoutFile, nil, filePerm)
	if stdoutErr != nil {
		return fmt.Errorf("create stdout file: %w", stdoutErr)
	}

	stderrErr := os.WriteFile(state.StderrFile, nil, filePerm)
	if stderrErr != nil {
		return fmt.Errorf("create stderr file: %w", stderrErr)
	}

	return nil
}

func writeStateFile(path string, state liveState) error {
	encoded, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		return fmt.Errorf("marshal live state: %w", marshalErr)
	}

	writeErr := os.WriteFile(path, encoded, filePerm)
	if writeErr != nil {
		return fmt.Errorf("write live state: %w", writeErr)
	}

	return nil
}

func loadState(path string) (liveState, error) {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return liveState{}, fmt.Errorf("read state file %s: %w", path, readErr)
		}

		return liveState{}, fmt.Errorf("read state file %s: %w", path, readErr)
	}

	var state liveState
	unmarshalErr := json.Unmarshal(data, &state)
	if unmarshalErr != nil {
		return liveState{}, fmt.Errorf("parse state file %s: %w", path, unmarshalErr)
	}

	return state, nil
}

func readBoundedOutput(path string, maxBytes int) (sanitize.BoundedText, error) {
	if path == "" {
		return sanitize.BoundedText{}, nil
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return sanitize.BoundedText{}, nil
		}

		return sanitize.BoundedText{}, fmt.Errorf("read output file %s: %w", path, readErr)
	}

	return sanitize.BoundText(data, maxBytes), nil
}

func cleanup(state liveState, stateFile string) error {
	for _, path := range []string{state.StdoutFile, state.StderrFile, stateFile} {
		if path == "" {
			continue
		}

		removeErr := os.Remove(path)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, removeErr)
		}
	}

	return nil
}

func captureMode(cfg config.Config, command string) string {
	if cfg.ForceFullCommand(command) {
		return captureModeFull
	}

	name := commandName(command)
	if isMetadataCommand(cfg, name) {
		return captureModeMetadata
	}

	return captureModeFull
}

func commandName(command string) string {
	fields := strings.Fields(command)
	index := 0
	for index < len(fields) {
		for index < len(fields) && isEnvAssignment(fields[index]) {
			index++
		}
		if index >= len(fields) {
			return ""
		}

		token := fields[index]
		switch token {
		case "env":
			index = skipEnvCommand(fields, index+1)
			continue
		case "command", "builtin", "noglob", "nocorrect":
			index++
			continue
		case "time":
			index = skipTimeCommand(fields, index+1)
			continue
		case "sudo":
			index = skipSudoCommand(fields, index+1)
			continue
		case "doas":
			index = skipDoasCommand(fields, index+1)
			continue
		default:
			return baseName(token)
		}
	}

	return ""
}

func newEventID(startedAt time.Time) (string, error) {
	var suffix [4]byte
	_, readErr := rand.Read(suffix[:])
	if readErr != nil {
		return "", fmt.Errorf("generate random event id suffix: %w", readErr)
	}

	return fmt.Sprintf(
		"%s-%s",
		startedAt.UTC().Format("20060102T150405.000000000Z"),
		hex.EncodeToString(suffix[:]),
	), nil
}

func boundCommand(command string, maxBytes int) string {
	bounded := sanitize.BoundText([]byte(command), maxBytes)
	return bounded.Text
}

func maybeAutoAddMetadataCommand(
	cfg config.Config,
	state liveState,
	exitCode int,
	duration time.Duration,
	stderrText string,
) error {
	if !cfg.AutoAddMetadata || state.CaptureMode != captureModeFull || exitCode == 0 {
		return nil
	}
	if duration < 0 || duration > autoAddMaxDuration {
		return nil
	}
	if cfg.ForceFullCommand(state.Command) {
		return nil
	}

	name := commandName(state.Command)
	if name == "" || isMetadataCommand(cfg, name) || !matchesTTYFailure(stderrText) {
		return nil
	}

	appendErr := config.AppendMetadataCommandName(name)
	if appendErr != nil {
		return fmt.Errorf("persist metadata command name %q: %w", name, appendErr)
	}

	return nil
}

func isMetadataCommand(cfg config.Config, name string) bool {
	if name == "" {
		return false
	}
	if isDefaultMetadataCommand(name) {
		return true
	}

	return cfg.HasMetadataCommandName(name)
}

func matchesTTYFailure(stderrText string) bool {
	lowerText := strings.ToLower(stderrText)
	return strings.Contains(lowerText, "not a tty") ||
		strings.Contains(lowerText, "not a terminal") ||
		strings.Contains(lowerText, "stdout is not a terminal") ||
		strings.Contains(lowerText, "stdin is not a terminal") ||
		strings.Contains(lowerText, "failed to get terminal") ||
		strings.Contains(lowerText, "inappropriate ioctl for device")
}

func isEnvAssignment(token string) bool {
	if token == "" || strings.HasPrefix(token, "=") {
		return false
	}

	index := strings.IndexByte(token, '=')
	if index <= 0 {
		return false
	}

	return !strings.ContainsAny(token[:index], "/:")
}

func skipEnvCommand(fields []string, index int) int {
	for index < len(fields) {
		token := fields[index]
		if strings.HasPrefix(token, "-") || isEnvAssignment(token) {
			index++
			continue
		}

		break
	}

	return index
}

func skipTimeCommand(fields []string, index int) int {
	for index < len(fields) && strings.HasPrefix(fields[index], "-") {
		index++
	}

	return index
}

func skipSudoCommand(fields []string, index int) int {
	return skipOptionCommand(fields, index, map[string]struct{}{
		"-a":           {},
		"-C":           {},
		"-c":           {},
		"-D":           {},
		"-g":           {},
		"-h":           {},
		"-p":           {},
		"-R":           {},
		"-T":           {},
		"-U":           {},
		"-u":           {},
		"--appname":    {},
		"--chdir":      {},
		"--close-from": {},
		"--group":      {},
		"--host":       {},
		"--other-user": {},
		"--prompt":     {},
		"--user":       {},
	})
}

func skipDoasCommand(fields []string, index int) int {
	return skipOptionCommand(fields, index, map[string]struct{}{
		"-C":     {},
		"-L":     {},
		"-u":     {},
		"--user": {},
	})
}

func skipOptionCommand(fields []string, index int, optionsWithArgs map[string]struct{}) int {
	for index < len(fields) {
		token := fields[index]
		if !strings.HasPrefix(token, "-") {
			break
		}

		index++
		if _, ok := optionsWithArgs[token]; ok && index < len(fields) {
			index++
		}
	}

	return index
}

func baseName(token string) string {
	if slash := strings.LastIndex(token, "/"); slash >= 0 {
		token = token[slash+1:]
	}

	return token
}

func isDefaultMetadataCommand(name string) bool {
	switch name {
	case "aider", "atop", "btop", "claude", "claudecode", "codex", "dlv", "emacs",
		"gdb", "gemini", "gitui", "goose", "helix", "htop", "hx", "k9s", "kak",
		"lazygit", "less", "lldb", "man", "mc", "more", "mosh", "most", "nano",
		"nnn", "nvim", "opencode", "ranger", "screen", "sftp", "ssh", "tig",
		"tmux", "top", "vi", "vim", "watch", "yazi", "zellij":
		return true
	default:
		return false
	}
}
