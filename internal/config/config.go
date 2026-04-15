package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/YusukeShimizu/richhistory/internal/paths"
)

const (
	DefaultMaxStdoutBytes   = 64 * 1024
	DefaultMaxStderrBytes   = 32 * 1024
	DefaultMaxCommandBytes  = 4 * 1024
	DefaultMaxTotalBytes    = 128 * 1024 * 1024
	DefaultMaxRetentionDays = 30
	DefaultRotateBytes      = 8 * 1024 * 1024
)

type Config struct {
	IgnoreCommandPatterns []string `json:"ignore_command_patterns"`
	IgnoreCWDPatterns     []string `json:"ignore_cwd_patterns"`
	MaxStdoutBytes        int      `json:"max_stdout_bytes"`
	MaxStderrBytes        int      `json:"max_stderr_bytes"`
	MaxCommandBytes       int      `json:"max_command_bytes"`
	MaxTotalBytes         int64    `json:"max_total_bytes"`
	MaxRetentionDays      int      `json:"max_retention_days"`
	RotateBytes           int64    `json:"rotate_bytes"`

	commandPatterns []*regexp.Regexp
	cwdPatterns     []*regexp.Regexp
}

func Default() Config {
	cfg := Config{
		MaxStdoutBytes:   DefaultMaxStdoutBytes,
		MaxStderrBytes:   DefaultMaxStderrBytes,
		MaxCommandBytes:  DefaultMaxCommandBytes,
		MaxTotalBytes:    DefaultMaxTotalBytes,
		MaxRetentionDays: DefaultMaxRetentionDays,
		RotateBytes:      DefaultRotateBytes,
	}
	_ = cfg.compile()

	return cfg
}

func Load() (Config, error) {
	cfg := Default()
	path, err := paths.ConfigPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}

		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	unmarshalErr := json.Unmarshal(data, &cfg)
	if unmarshalErr != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, unmarshalErr)
	}

	cfg.applyDefaults()
	compileErr := cfg.compile()
	if compileErr != nil {
		return Config{}, compileErr
	}

	return cfg, nil
}

func EnsureConfigDir() (string, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(path)
	mkdirErr := os.MkdirAll(dir, 0o750)
	if mkdirErr != nil {
		return "", fmt.Errorf("create config dir %s: %w", dir, mkdirErr)
	}

	return dir, nil
}

func (cfg *Config) IgnoreCommand(command string) bool {
	for _, pattern := range cfg.commandPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}

	return false
}

func (cfg *Config) IgnoreCWD(cwd string) bool {
	for _, pattern := range cfg.cwdPatterns {
		if pattern.MatchString(cwd) {
			return true
		}
	}

	return false
}

func (cfg *Config) applyDefaults() {
	if cfg.MaxStdoutBytes <= 0 {
		cfg.MaxStdoutBytes = DefaultMaxStdoutBytes
	}
	if cfg.MaxStderrBytes <= 0 {
		cfg.MaxStderrBytes = DefaultMaxStderrBytes
	}
	if cfg.MaxCommandBytes <= 0 {
		cfg.MaxCommandBytes = DefaultMaxCommandBytes
	}
	if cfg.MaxTotalBytes <= 0 {
		cfg.MaxTotalBytes = DefaultMaxTotalBytes
	}
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = DefaultMaxRetentionDays
	}
	if cfg.RotateBytes <= 0 {
		cfg.RotateBytes = DefaultRotateBytes
	}
}

func (cfg *Config) compile() error {
	var err error
	cfg.commandPatterns, err = compilePatterns(cfg.IgnoreCommandPatterns, "ignore_command_patterns")
	if err != nil {
		return err
	}

	cfg.cwdPatterns, err = compilePatterns(cfg.IgnoreCWDPatterns, "ignore_cwd_patterns")
	if err != nil {
		return err
	}

	return nil
}

func compilePatterns(patterns []string, field string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiledPattern, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile %s pattern %q: %w", field, pattern, err)
		}

		compiled = append(compiled, compiledPattern)
	}

	return compiled, nil
}
