package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func ConfigDirPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(root, "stackctl"), nil
}

func ConfigFilePath() (string, error) {
	dir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.yaml"), nil
}

func ComposePath(cfg Config) string {
	return filepath.Join(cfg.Stack.Dir, cfg.Stack.ComposeFile)
}

func DefaultStackDir() string {
	wd, err := os.Getwd()
	if err == nil {
		if found, ok := findStackDir(wd); ok {
			return found
		}
	}

	exePath, err := os.Executable()
	if err == nil {
		if found, ok := findStackDir(filepath.Dir(exePath)); ok {
			return found
		}
	}

	if wd == "" {
		return ""
	}

	absPath, err := filepath.Abs(filepath.Join(wd, "stacks", "dev-stack"))
	if err != nil {
		return filepath.Join(wd, "stacks", "dev-stack")
	}

	return absPath
}

func findStackDir(start string) (string, bool) {
	current := start
	for {
		candidate := filepath.Join(current, "stacks", "dev-stack")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			absPath, err := filepath.Abs(candidate)
			if err != nil {
				return candidate, true
			}
			return absPath, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}
