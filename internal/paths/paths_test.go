package paths_test

import (
	"path/filepath"
	"testing"

	"github.com/YusukeShimizu/richhistory/internal/paths"
)

func TestConfigPathPrefersCanonicalLocation(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	path, err := paths.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned error: %v", err)
	}

	want := filepath.Join(configRoot, "richhistory", "config.json")
	if path != want {
		t.Fatalf("expected %s, got %s", want, path)
	}
}

func TestStateRootPrefersCanonicalLocation(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateRoot)

	path, err := paths.StateRoot()
	if err != nil {
		t.Fatalf("StateRoot returned error: %v", err)
	}

	want := filepath.Join(stateRoot, "richhistory")
	if path != want {
		t.Fatalf("expected %s, got %s", want, path)
	}
}
