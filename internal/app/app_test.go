package app_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YusukeShimizu/richhistory/internal/app"
	"github.com/YusukeShimizu/richhistory/internal/store"
)

func TestShowAndSearchJSON(t *testing.T) {
	state := t.TempDir()
	configDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", configDir)

	root := filepath.Join(state, "richhistory", "events")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	event := store.Event{
		ID:         "event-1",
		SessionID:  "session-1",
		Seq:        1,
		Shell:      "zsh",
		ShellPID:   7,
		TTY:        "tty1",
		Host:       "host",
		PWDBefore:  "/tmp",
		PWDAfter:   "/tmp",
		StartedAt:  time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 4, 15, 0, 0, 1, 0, time.UTC),
		Command:    "echo hello",
		StdoutText: "hello\n",
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	path := filepath.Join(root, "2026-04-15.ndjson")
	writeErr := os.WriteFile(path, append(encoded, '\n'), 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile returned error: %v", writeErr)
	}

	var showOut bytes.Buffer
	var showErr bytes.Buffer
	if exitCode := app.RunAs("richhistory", []string{"show", "--json"}, &showOut, &showErr); exitCode != 0 {
		t.Fatalf("show exited %d: %s", exitCode, showErr.String())
	}
	if !bytes.Contains(showOut.Bytes(), []byte(`"event-1"`)) {
		t.Fatalf("show output missing event: %s", showOut.String())
	}

	var searchOut bytes.Buffer
	var searchErr bytes.Buffer
	if exitCode := app.RunAs("richhistory", []string{"search", "hello", "--json"}, &searchOut, &searchErr); exitCode != 0 {
		t.Fatalf("search exited %d: %s", exitCode, searchErr.String())
	}
	if !bytes.Contains(searchOut.Bytes(), []byte(`"stdout_text": "hello\n"`)) {
		t.Fatalf("search output missing stdout: %s", searchOut.String())
	}
}

func TestTermInitUsesInvokedCommandName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		invokedAs  string
		wantBinRef string
	}{
		{name: "canonical", invokedAs: "richhistory", wantBinRef: "command 'richhistory' record start"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := app.RunAs(tc.invokedAs, []string{"term", "init", "zsh"}, &stdout, &stderr)
			if exitCode != 0 {
				t.Fatalf("term init exited %d: %s", exitCode, stderr.String())
			}
			if !bytes.Contains(stdout.Bytes(), []byte(tc.wantBinRef)) {
				t.Fatalf("term init output missing %q:\n%s", tc.wantBinRef, stdout.String())
			}
		})
	}
}

func TestUsageUsesInvokedCommandName(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := app.RunAs("richhistory", []string{"search"}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatal("expected usage error")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("usage: richhistory search <query>")) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
