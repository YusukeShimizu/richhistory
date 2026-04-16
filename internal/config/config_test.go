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
	cfg.IgnoreCWDPatterns = []string{`^/private`}

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
	if !loaded.IgnoreCWD("/private/tmp") {
		t.Fatal("expected ignore_cwd_patterns to round-trip")
	}
}

func TestSaveKeepsTrailingNewline(t *testing.T) {
	stateRoot := t.TempDir()
	configRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	if err := config.Save(config.Default()); err != nil {
		t.Fatalf("Save returned error: %v", err)
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
