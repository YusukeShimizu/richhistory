package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	configEnv      = "XDG_CONFIG_HOME"
	stateEnv       = "XDG_STATE_HOME"
	canonicalApp   = "richhistory"
	configFilename = "config.json"
)

func ConfigPath() (string, error) {
	base, err := configRoot()
	if err != nil {
		return "", err
	}

	return chooseConfigPath(base), nil
}

func StateRoot() (string, error) {
	base, err := stateRoot()
	if err != nil {
		return "", err
	}

	return chooseStateRoot(base), nil
}

func configRoot() (string, error) {
	if value := os.Getenv(configEnv); value != "" {
		return value, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".config"), nil
}

func stateRoot() (string, error) {
	if value := os.Getenv(stateEnv); value != "" {
		return value, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".local", "state"), nil
}

func chooseConfigPath(base string) string {
	return filepath.Join(base, canonicalApp, configFilename)
}

func chooseStateRoot(base string) string {
	return filepath.Join(base, canonicalApp)
}
