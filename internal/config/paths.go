package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultStackName       = "dev-stack"
	DefaultComposeFileName = "compose.yaml"
	DefaultNATSConfigName  = "nats.conf"
	StackNameEnvVar        = "STACKCTL_STACK"
)

func ConfigDirPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(root, "stackctl"), nil
}

func ConfigFilePath() (string, error) {
	return ConfigFilePathForStack(SelectedStackName())
}

func ConfigStacksDirPath() (string, error) {
	dir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "stacks"), nil
}

func ConfigFilePathForStack(name string) (string, error) {
	selected := normalizeStackName(name)
	if err := ValidateStackName(selected); err != nil {
		return "", err
	}

	dir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	if selected == DefaultStackName {
		return filepath.Join(dir, "config.yaml"), nil
	}

	stacksDir, err := ConfigStacksDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(stacksDir, selected+".yaml"), nil
}

func SelectedStackName() string {
	return normalizeStackName(os.Getenv(StackNameEnvVar))
}

func KnownConfigPaths() ([]string, error) {
	configDir, err := ConfigDirPath()
	if err != nil {
		return nil, err
	}

	paths := make(map[string]struct{})
	defaultPath := filepath.Join(configDir, "config.yaml")
	if fileExists(defaultPath) {
		paths[defaultPath] = struct{}{}
	}

	stacksDir, err := ConfigStacksDirPath()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(stacksDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read stack config directory %s: %w", stacksDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(stacksDir, entry.Name())
		if fileExists(path) {
			paths[path] = struct{}{}
		}
	}

	values := make([]string, 0, len(paths))
	for path := range paths {
		values = append(values, path)
	}
	sort.Strings(values)

	return values, nil
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

	name := normalizeStackName(stackName)
	if err := ValidateStackName(name); err != nil {
		return "", err
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

func NATSConfigPath(cfg Config) string {
	return filepath.Join(cfg.Stack.Dir, DefaultNATSConfigName)
}

func normalizeStackName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return DefaultStackName
	}
	return trimmed
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
