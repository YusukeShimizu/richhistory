package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/YusukeShimizu/richhistory/internal/config"
)

func TestLoadAndSaveRoundTrip(t *testing.T) {
	stateRoot := t.TempDir()
	configRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	cfg := config.Default()
	cfg.IgnoreCommandPatterns = []string{`^secret `}
	cfg.MetadataCommandNames = []string{"codex", "claude"}
	cfg.ForceFullPatterns = []string{`^codex exec --json`}
	cfg.AutoAddMetadata = true

	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !loaded.IgnoreCommand("secret show") {
		t.Fatal("expected ignore_command_patterns to round-trip")
	}
	if !loaded.HasMetadataCommandName("codex") || !loaded.HasMetadataCommandName("claude") {
		t.Fatalf("expected metadata command names to round-trip: %#v", loaded.MetadataCommandNames)
	}
	if !loaded.ForceFullCommand("codex exec --json") {
		t.Fatal("expected force_full_command_patterns to round-trip")
	}
	if !loaded.AutoAddMetadata {
		t.Fatal("expected auto_add_metadata_commands to round-trip")
	}
}

func TestAppendMetadataCommandNameDedupesAndPreservesConfig(t *testing.T) {
	stateRoot := t.TempDir()
	configRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	cfg := config.Default()
	cfg.MetadataCommandNames = []string{"codex"}
	cfg.ForceFullPatterns = []string{`^foo`}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if err := config.AppendMetadataCommandName("claude"); err != nil {
		t.Fatalf("AppendMetadataCommandName returned error: %v", err)
	}
	if err := config.AppendMetadataCommandName("codex"); err != nil {
		t.Fatalf("AppendMetadataCommandName duplicate returned error: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got, want := loaded.MetadataCommandNames, []string{"codex", "claude"}; len(got) != len(want) ||
		got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected metadata command names: %#v", got)
	}
	if !loaded.ForceFullCommand("foo") {
		t.Fatal("expected force_full_command_patterns to be preserved")
	}

	path := filepath.Join(configRoot, "richhistory", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("expected saved config to end with newline: %q", string(data))
	}
}
