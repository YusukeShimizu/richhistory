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
	captureModeSkip     = "skip"
	captureModeMetadata = "metadata"
	captureModeFull     = "full"
)

type StartInput struct {
	SessionID     string
	SessionName   string
	Seq           int
	Shell         string
	ShellPID      int
	TTY           string
	Command       string
	PWD           string
	CaptureOutput bool
	StartedAt     time.Time
}

type StartResult struct {
	EventID           string
	StateFile         string
	CaptureBeforeFile string
	CaptureAfterFile  string
	CaptureMode       string
}

type FinishInput struct {
	StateFile  string
	PWDAfter   string
	ExitCode   int
	FinishedAt time.Time
}

type liveState struct {
	EventID           string    `json:"event_id"`
	SessionID         string    `json:"session_id"`
	SessionName       string    `json:"session_name"`
	Seq               int       `json:"seq"`
	Shell             string    `json:"shell"`
	ShellPID          int       `json:"shell_pid"`
	TTY               string    `json:"tty"`
	Command           string    `json:"command"`
	PWDBefore         string    `json:"pwd_before"`
	StartedAt         time.Time `json:"started_at"`
	CaptureBeforeFile string    `json:"capture_before_file,omitempty"`
	CaptureAfterFile  string    `json:"capture_after_file,omitempty"`
	CaptureMode       string    `json:"capture_mode"`
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
		CaptureMode: captureMode(input.CaptureOutput),
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
		EventID:           eventID,
		StateFile:         stateFile,
		CaptureBeforeFile: state.CaptureBeforeFile,
		CaptureAfterFile:  state.CaptureAfterFile,
		CaptureMode:       state.CaptureMode,
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

	stdoutBound, captureErr := readCapturedOutput(state, cfg)
	if captureErr != nil {
		return store.Event{}, false, captureErr
	}
	stderrBound := sanitize.BoundedText{}

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

	state.CaptureBeforeFile = filepath.Join(livePath, state.EventID+".before")
	state.CaptureAfterFile = filepath.Join(livePath, state.EventID+".after")

	beforeErr := os.WriteFile(state.CaptureBeforeFile, nil, filePerm)
	if beforeErr != nil {
		return fmt.Errorf("create capture before file: %w", beforeErr)
	}

	afterErr := os.WriteFile(state.CaptureAfterFile, nil, filePerm)
	if afterErr != nil {
		return fmt.Errorf("create capture after file: %w", afterErr)
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

func readOutputFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read output file %s: %w", path, readErr)
	}

	return data, nil
}

func readCapturedOutput(state liveState, cfg config.Config) (sanitize.BoundedText, error) {
	if state.CaptureMode != captureModeFull {
		return sanitize.BoundedText{}, nil
	}

	beforeData, beforeErr := readOutputFile(state.CaptureBeforeFile)
	if beforeErr != nil {
		return sanitize.BoundedText{}, beforeErr
	}
	afterData, afterErr := readOutputFile(state.CaptureAfterFile)
	if afterErr != nil {
		return sanitize.BoundedText{}, afterErr
	}

	captured := paneOutputDelta(beforeData, afterData)
	stdout := sanitize.BoundText([]byte(captured), cfg.MaxStdoutBytes)
	return stdout, nil
}

func cleanup(state liveState, stateFile string) error {
	for _, path := range []string{state.CaptureBeforeFile, state.CaptureAfterFile, stateFile} {
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

func captureMode(captureOutput bool) string {
	if captureOutput {
		return captureModeFull
	}

	return captureModeMetadata
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

func paneOutputDelta(beforeData []byte, afterData []byte) string {
	before := sanitize.Clean(beforeData)
	after := sanitize.Clean(afterData)
	if before == "" {
		return after
	}
	if strings.HasPrefix(after, before) {
		return after[len(before):]
	}

	prefixLen := commonPrefixLen(before, after)
	return after[prefixLen:]
}

func commonPrefixLen(left string, right string) int {
	maxLen := min(len(left), len(right))

	index := 0
	for index < maxLen && left[index] == right[index] {
		index++
	}

	return index
}
