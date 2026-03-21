package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultStackName       = "dev-stack"
	DefaultComposeFileName = "compose.yaml"
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

func DataDirPath() (string, error) {
	if root := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); root != "" {
		return filepath.Join(root, "stackctl"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Join(home, ".local", "share", "stackctl"), nil
}

func ManagedStacksDirPath() (string, error) {
	root, err := DataDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "stacks"), nil
}

func ManagedStackDir(stackName string) (string, error) {
	root, err := ManagedStacksDirPath()
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(stackName)
	if name == "" {
		name = DefaultStackName
	}

	return filepath.Join(root, name), nil
}

func DefaultManagedStackDir() string {
	dir, err := ManagedStackDir(DefaultStackName)
	if err != nil {
		return ""
	}

	return dir
}

func ComposePath(cfg Config) string {
	return filepath.Join(cfg.Stack.Dir, cfg.Stack.ComposeFile)
}
