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

func TestStartFinishWritesCapturedEventAndCleansLiveFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()
	cfg.MaxStdoutBytes = 8
	cfg.MaxStderrBytes = 8

	startedAt := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	result, err := record.Start(root, cfg, record.StartInput{
		SessionID:     "session-1",
		SessionName:   "demo",
		Seq:           1,
		Shell:         "zsh",
		ShellPID:      42,
		TTY:           "ttys001",
		Command:       "echo hello world",
		PWD:           "/tmp",
		CaptureOutput: true,
		StartedAt:     startedAt,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if result.CaptureMode != "full" {
		t.Fatalf("unexpected capture mode: %q", result.CaptureMode)
	}

	if writeErr := os.WriteFile(result.CaptureBeforeFile, []byte("PROMPT> echo hello world\n"), 0o600); writeErr != nil {
		t.Fatalf("WriteFile capture before returned error: %v", writeErr)
	}
	if writeErr := os.WriteFile(
		result.CaptureAfterFile,
		[]byte("PROMPT> echo hello world\nhello world"),
		0o600,
	); writeErr != nil {
		t.Fatalf("WriteFile capture after returned error: %v", writeErr)
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
	if event.StderrText != "hello wo" {
		t.Fatalf("unexpected stderr text: %q", event.StderrText)
	}
	if !event.StdoutTruncated {
		t.Fatal("expected stdout truncation")
	}
	if !event.StderrTruncated {
		t.Fatal("expected stderr truncation")
	}

	for _, path := range []string{result.StateFile, result.CaptureBeforeFile, result.CaptureAfterFile} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
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

func TestStartUsesMetadataModeWhenCaptureDisabled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()
	result, err := record.Start(root, cfg, record.StartInput{
		SessionID:     "session-1",
		Seq:           1,
		Shell:         "zsh",
		Command:       "man true",
		PWD:           "/tmp",
		CaptureOutput: false,
		StartedAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if result.CaptureMode != "metadata" {
		t.Fatalf("unexpected capture mode: %q", result.CaptureMode)
	}
	if result.CaptureBeforeFile != "" || result.CaptureAfterFile != "" {
		t.Fatalf("metadata mode should not create output files: %#v", result)
	}
	if _, statErr := os.Stat(filepath.Dir(result.StateFile)); statErr != nil {
		t.Fatalf("expected live dir to exist: %v", statErr)
	}
}

func TestStartUsesFullModeWhenCaptureEnabled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()
	result, err := record.Start(root, cfg, record.StartInput{
		SessionID:     "session-1",
		Seq:           1,
		Shell:         "zsh",
		Command:       "codex exec",
		PWD:           "/tmp",
		CaptureOutput: true,
		StartedAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if result.CaptureMode != "full" {
		t.Fatalf("expected full capture mode, got %q", result.CaptureMode)
	}
	if result.CaptureBeforeFile == "" || result.CaptureAfterFile == "" {
		t.Fatalf("full mode should create capture files: %#v", result)
	}
}

func TestStartFinishFallsBackToCommonSuffixWhenBaselineDiffers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()

	startedAt := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	result, err := record.Start(root, cfg, record.StartInput{
		SessionID:     "session-1",
		Seq:           1,
		Shell:         "zsh",
		Command:       "echo hello",
		PWD:           "/tmp",
		CaptureOutput: true,
		StartedAt:     startedAt,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if writeErr := os.WriteFile(result.CaptureBeforeFile, []byte("old screen"), 0o600); writeErr != nil {
		t.Fatalf("WriteFile capture before returned error: %v", writeErr)
	}
	if writeErr := os.WriteFile(result.CaptureAfterFile, []byte("new screen\nhello\n"), 0o600); writeErr != nil {
		t.Fatalf("WriteFile capture after returned error: %v", writeErr)
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
	if event.StdoutText != "new screen\nhello\n" {
		t.Fatalf("unexpected stdout text: %q", event.StdoutText)
	}
	if event.StderrText != "new screen\nhello\n" {
		t.Fatalf("unexpected stderr text: %q", event.StderrText)
	}
}

func TestStartSkipsSelfCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()

	for _, command := range []string{"richhistory show"} {
		result, err := record.Start(root, cfg, record.StartInput{
			SessionID:     "session-1",
			Seq:           1,
			Shell:         "zsh",
			Command:       command,
			PWD:           "/tmp",
			CaptureOutput: true,
			StartedAt:     time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("Start returned error for %q: %v", command, err)
		}
		if result.CaptureMode != "skip" {
			t.Fatalf("expected skip for %q, got %q", command, result.CaptureMode)
		}
	}
}
