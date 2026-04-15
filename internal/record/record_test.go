package record_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YusukeShimizu/richhistory/internal/config"
	"github.com/YusukeShimizu/richhistory/internal/record"
	"github.com/YusukeShimizu/richhistory/internal/store"
)

func TestStartFinishWritesEventAndCleansLiveFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()
	cfg.MaxStdoutBytes = 8

	startedAt := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	result, err := record.Start(root, cfg, record.StartInput{
		SessionID:   "session-1",
		SessionName: "demo",
		Seq:         1,
		Shell:       "zsh",
		ShellPID:    42,
		TTY:         "ttys001",
		Command:     "echo hello world",
		PWD:         "/tmp",
		StartedAt:   startedAt,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if result.CaptureMode != "full" {
		t.Fatalf("unexpected capture mode: %q", result.CaptureMode)
	}

	stdoutErr := os.WriteFile(result.StdoutFile, []byte("hello world"), 0o600)
	if stdoutErr != nil {
		t.Fatalf("WriteFile stdout returned error: %v", stdoutErr)
	}
	stderrErr := os.WriteFile(result.StderrFile, []byte("warn"), 0o600)
	if stderrErr != nil {
		t.Fatalf("WriteFile stderr returned error: %v", stderrErr)
	}

	event, written, err := record.Finish(root, cfg, record.FinishInput{
		StateFile:  result.StateFile,
		PWDAfter:   "/tmp",
		ExitCode:   0,
		FinishedAt: startedAt.Add(250 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Finish returned error: %v", err)
	}
	if !written {
		t.Fatal("expected event to be written")
	}
	if event.StdoutText != "hello wo" {
		t.Fatalf("unexpected stdout text: %q", event.StdoutText)
	}
	if !event.StdoutTruncated {
		t.Fatal("expected stdout truncation")
	}

	for _, path := range []string{result.StateFile, result.StdoutFile, result.StderrFile} {
		_, statErr := os.Stat(path)
		if !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be removed, got err=%v", path, statErr)
		}
	}

	listed, err := store.List(root, store.Filter{Limit: 10, Status: store.StatusAny})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one event, got %d", len(listed))
	}
}

func TestStartUsesMetadataModeForFullscreenCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()
	result, err := record.Start(root, cfg, record.StartInput{
		SessionID: "session-1",
		Seq:       1,
		Shell:     "zsh",
		Command:   "man true",
		PWD:       "/tmp",
		StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if result.CaptureMode != "metadata" {
		t.Fatalf("unexpected capture mode: %q", result.CaptureMode)
	}
	if result.StdoutFile != "" || result.StderrFile != "" {
		t.Fatalf("metadata mode should not create output files: %#v", result)
	}
	_, statErr := os.Stat(filepath.Dir(result.StateFile))
	if statErr != nil {
		t.Fatalf("expected live dir to exist: %v", statErr)
	}
}

func TestStartSkipsSelfCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()

	for _, command := range []string{"richhistory show"} {
		result, err := record.Start(root, cfg, record.StartInput{
			SessionID: "session-1",
			Seq:       1,
			Shell:     "zsh",
			Command:   command,
			PWD:       "/tmp",
			StartedAt: time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("Start returned error for %q: %v", command, err)
		}
		if result.CaptureMode != "skip" {
			t.Fatalf("expected skip for %q, got %q", command, result.CaptureMode)
		}
	}
}
