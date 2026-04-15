package store_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YusukeShimizu/richhistory/internal/config"
	"github.com/YusukeShimizu/richhistory/internal/store"
)

func TestAppendListAndPrune(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Default()
	cfg.RotateBytes = 100
	cfg.MaxTotalBytes = 10 * 1024 * 1024
	cfg.MaxRetentionDays = 1

	first := baseEvent("one", time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC))
	first.StdoutText = stringsRepeat("a", 80)
	second := baseEvent("two", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC))
	second.Command = "echo second"

	if err := store.Append(root, cfg, first); err != nil {
		t.Fatalf("Append first returned error: %v", err)
	}
	if err := store.Append(root, cfg, second); err != nil {
		t.Fatalf("Append second returned error: %v", err)
	}

	events, err := store.List(root, store.Filter{Limit: 10, Status: store.StatusAny})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two events, got %d", len(events))
	}

	pruneErr := store.Prune(root, cfg, time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC))
	if pruneErr != nil {
		t.Fatalf("Prune returned error: %v", pruneErr)
	}

	pruned, err := store.List(root, store.Filter{Limit: 10, Status: store.StatusAny})
	if err != nil {
		t.Fatalf("List after prune returned error: %v", err)
	}
	if len(pruned) != 1 || pruned[0].ID != "two" {
		t.Fatalf("unexpected events after prune: %#v", pruned)
	}

	matches, err := store.List(root, store.Filter{
		Limit:      10,
		Status:     store.StatusAny,
		Query:      "second",
		QueryField: "cmd",
	})
	if err != nil {
		t.Fatalf("search list returned error: %v", err)
	}
	if len(matches) != 1 || matches[0].ID != "two" {
		t.Fatalf("unexpected matches: %#v", matches)
	}
}

func baseEvent(id string, finishedAt time.Time) store.Event {
	return store.Event{
		ID:         id,
		SessionID:  fmt.Sprintf("session-%s", id),
		Seq:        1,
		Shell:      "zsh",
		ShellPID:   10,
		TTY:        "tty1",
		Host:       "host",
		PWDBefore:  "/tmp",
		PWDAfter:   "/tmp",
		StartedAt:  finishedAt.Add(-time.Second),
		FinishedAt: finishedAt,
		Command:    "echo hello",
	}
}

func stringsRepeat(value string, count int) string {
	return strings.Repeat(value, count)
}
