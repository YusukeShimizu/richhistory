package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

type event struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	SessionName string `json:"session_name"`
	Command     string `json:"command"`
	CaptureMode string `json:"capture_mode"`
	PWDBefore   string `json:"pwd_before"`
	PWDAfter    string `json:"pwd_after"`
	ExitCode    int    `json:"exit_code"`
	StdoutText  string `json:"stdout_text"`
	StderrText  string `json:"stderr_text"`
}

func TestZshIntegrationCapturesCommands(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(mustGetwd(t))
	binPath := buildBinary(t, repoRoot, "richhistory", "./cmd/richhistory")
	stateRoot := t.TempDir()
	configRoot := t.TempDir()
	ptmx, cmd, buffer, readDone := startShell(
		t,
		binPath,
		"richhistory",
		stateRoot,
		configRoot,
		map[string]string{"WEZTERM_PANE": "1"},
	)
	defer ptmx.Close()

	waitForPrompt(t, buffer)
	runCommand(t, ptmx, buffer, "pwd")
	runCommand(t, ptmx, buffer, "echo hello")
	runCommand(t, ptmx, buffer, "ls /definitely-missing")
	runCommand(t, ptmx, buffer, "cd /tmp")
	exitShell(t, ptmx, cmd, readDone)

	events := readEvents(t, filepath.Join(stateRoot, "richhistory", "events"))
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	assertSingleSession(t, events)
	assertCommand(t, events, "echo hello", func(item event) {
		if item.CaptureMode != "full" {
			t.Fatalf("expected full capture mode: %#v", item)
		}
		if !strings.Contains(item.StdoutText, "hello") {
			t.Fatalf("echo output missing: %#v", item)
		}
	})
	assertCommand(t, events, "ls /definitely-missing", func(item event) {
		if item.ExitCode == 0 || !strings.Contains(item.StderrText, "definitely-missing") {
			t.Fatalf("ls failure not captured: %#v", item)
		}
	})
	assertCommand(t, events, "cd /tmp", func(item event) {
		if item.PWDAfter != "/tmp" {
			t.Fatalf("cd pwd_after mismatch: %#v", item)
		}
	})
}

func TestZshIntegrationRecordsMetadataOutsideWezTerm(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(mustGetwd(t))
	binPath := buildBinary(t, repoRoot, "richhistory", "./cmd/richhistory")
	stateRoot := t.TempDir()
	configRoot := t.TempDir()
	ptmx, cmd, buffer, readDone := startShell(
		t,
		binPath,
		"richhistory",
		stateRoot,
		configRoot,
		map[string]string{"WEZTERM_PANE": ""},
	)
	defer ptmx.Close()

	waitForPrompt(t, buffer)
	runCommand(t, ptmx, buffer, "echo hello")
	runCommand(t, ptmx, buffer, "ls /definitely-missing")
	runCommand(t, ptmx, buffer, "true")
	exitShell(t, ptmx, cmd, readDone)

	events := readEvents(t, filepath.Join(stateRoot, "richhistory", "events"))
	assertCommand(t, events, "echo hello", func(item event) {
		if item.CaptureMode != "metadata" {
			t.Fatalf("expected metadata capture mode: %#v", item)
		}
		if item.StdoutText != "" || item.StderrText != "" {
			t.Fatalf("metadata command should not capture output: %#v", item)
		}
	})
	assertCommand(t, events, "ls /definitely-missing", func(item event) {
		if item.CaptureMode != "metadata" {
			t.Fatalf("expected metadata capture mode: %#v", item)
		}
		if item.ExitCode == 0 {
			t.Fatalf("expected failing command to keep exit code: %#v", item)
		}
		if item.StdoutText != "" || item.StderrText != "" {
			t.Fatalf("metadata command should not capture output: %#v", item)
		}
	})
}

func buildBinary(t *testing.T, repoRoot string, binaryName string, packagePath string) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, binaryName)
	build := exec.Command("go", "build", "-o", binPath, packagePath)
	build.Dir = repoRoot

	output, buildErr := build.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("go build failed: %v\n%s", buildErr, output)
	}

	return binPath
}

func startShell(
	t *testing.T,
	binPath string,
	commandName string,
	stateRoot string,
	configRoot string,
	extraEnv map[string]string,
) (*os.File, *exec.Cmd, *syncBuffer, <-chan error) {
	t.Helper()

	binDir := filepath.Dir(binPath)
	home := t.TempDir()
	zdotdir := t.TempDir()
	rcfile := filepath.Join(zdotdir, ".zshrc")
	rc := strings.Join([]string{
		fmt.Sprintf("export PATH=%s:$PATH", binDir),
		"PS1='PROMPT> '",
		fmt.Sprintf("eval \"$(%s term init zsh --name integration)\"", commandName),
		"",
	}, "\n")

	writeErr := os.WriteFile(rcfile, []byte(rc), 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile rcfile returned error: %v", writeErr)
	}

	cmd := exec.Command("zsh", "-d", "-i")
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"ZDOTDIR="+zdotdir,
		"XDG_STATE_HOME="+stateRoot,
		"XDG_CONFIG_HOME="+configRoot,
	)
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	ptmx, ptyErr := pty.Start(cmd)
	if ptyErr != nil {
		t.Fatalf("pty.Start returned error: %v", ptyErr)
	}

	reader := bufio.NewReader(ptmx)
	buffer := &syncBuffer{}
	readDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(buffer, reader)
		readDone <- copyErr
	}()

	return ptmx, cmd, buffer, readDone
}

func waitForPrompt(t *testing.T, buffer *syncBuffer) {
	t.Helper()
	waitForOutput(t, buffer, 0, "PROMPT> ")
}

func exitShell(t *testing.T, ptmx *os.File, cmd *exec.Cmd, readDone <-chan error) {
	t.Helper()

	_, writeErr := io.WriteString(ptmx, "exit\n")
	if writeErr != nil {
		t.Fatalf("write exit returned error: %v", writeErr)
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		t.Fatalf("zsh exited with error: %v", waitErr)
	}

	readErr := <-readDone
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, syscall.EIO) {
		t.Fatalf("reader goroutine returned error: %v", readErr)
	}
}

func readEvents(t *testing.T, eventsPath string) []event {
	t.Helper()

	files, readDirErr := os.ReadDir(eventsPath)
	if readDirErr != nil {
		t.Fatalf("ReadDir returned error: %v", readDirErr)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one event file")
	}

	events := make([]event, 0)
	for _, entry := range files {
		fileEvents := readEventFile(t, filepath.Join(eventsPath, entry.Name()))
		events = append(events, fileEvents...)
	}

	return events
}

func readEventFile(t *testing.T, path string) []event {
	t.Helper()

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile event file returned error: %v", readErr)
	}

	events := make([]event, 0)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		var item event
		unmarshalErr := json.Unmarshal(scanner.Bytes(), &item)
		if unmarshalErr != nil {
			t.Fatalf("Unmarshal returned error: %v", unmarshalErr)
		}

		events = append(events, item)
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		t.Fatalf("scanner.Err returned error: %v", scanErr)
	}

	return events
}

func assertSingleSession(t *testing.T, events []event) {
	t.Helper()

	sessionID := events[0].SessionID
	for _, item := range events {
		if item.SessionID != sessionID {
			t.Fatalf("session mismatch: %#v", events)
		}
	}
}

func runCommand(t *testing.T, ptmx *os.File, buffer *syncBuffer, command string) {
	t.Helper()

	start := buffer.Len()
	_, writeErr := io.WriteString(ptmx, command+"\n")
	if writeErr != nil {
		t.Fatalf("write command %q returned error: %v", command, writeErr)
	}
	waitForOutput(t, buffer, start, "PROMPT> ")
}

func waitForOutput(t *testing.T, buffer *syncBuffer, start int, needle string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q in output: %s", needle, buffer.String())
		}

		segment := buffer.Since(start)
		if strings.Contains(segment, needle) {
			return
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func assertCommand(t *testing.T, events []event, command string, check func(event)) {
	t.Helper()

	for _, item := range events {
		if item.Command == command {
			check(item)
			return
		}
	}

	t.Fatalf("command %q not found in events: %#v", command, events)
}

func mustGetwd(t *testing.T) string {
	t.Helper()

	dir, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("Getwd returned error: %v", getwdErr)
	}

	return dir
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (buffer *syncBuffer) Write(p []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	return buffer.buf.Write(p)
}

func (buffer *syncBuffer) Len() int {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	return buffer.buf.Len()
}

func (buffer *syncBuffer) Since(start int) string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	data := buffer.buf.Bytes()
	if start >= len(data) {
		return ""
	}

	return string(data[start:])
}

func (buffer *syncBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	return buffer.buf.String()
}
