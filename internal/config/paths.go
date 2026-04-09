package config

import (
	"errors"
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
	DefaultRedisACLName    = "redis.acl"
	DefaultPgAdminServers  = "pgadmin-servers.json"
	DefaultPGPassName      = "pgpass"
	StackNameEnvVar        = "STACKCTL_STACK"
	CurrentStackFileName   = "current-stack"
)

func ConfigDirPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(root, "stackctl"), nil
}

func ConfigFilePath() (string, error) {
	selected, err := ResolveSelectedStackName()
	if err != nil {
		return "", err
	}

	return ConfigFilePathForStack(selected)
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

	stacksDir := filepath.Join(dir, "stacks")
	return filepath.Join(stacksDir, selected+".yaml"), nil
}

func SelectedStackName() string {
	selected, err := ResolveSelectedStackName()
	if err != nil {
		return DefaultStackName
	}

	return selected
}

func CurrentStackPath() (string, error) {
	dir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, CurrentStackFileName), nil
}

func ResolveSelectedStackName() (string, error) {
	if value := strings.TrimSpace(os.Getenv(StackNameEnvVar)); value != "" {
		if err := ValidateStackName(value); err != nil {
			return "", fmt.Errorf("validate %s: %w", StackNameEnvVar, err)
		}

		return normalizeStackName(value), nil
	}

	return CurrentStackName()
}

func CurrentStackName() (string, error) {
	configDir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}

	root, err := os.OpenRoot(configDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultStackName, nil
		}
		return "", fmt.Errorf("open current stack root %q: %w", configDir, err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(CurrentStackFileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultStackName, nil
		}
		return "", fmt.Errorf("read current stack selection %q: %w", filepath.Join(configDir, CurrentStackFileName), err)
	}

	selected := strings.TrimSpace(string(data))
	if selected == "" {
		return DefaultStackName, nil
	}
	if err := ValidateStackName(selected); err != nil {
		return "", fmt.Errorf("parse current stack selection %q: %w", filepath.Join(configDir, CurrentStackFileName), err)
	}

	return normalizeStackName(selected), nil
}

func SetCurrentStackName(name string) error {
	selected := normalizeStackName(name)
	if err := ValidateStackName(selected); err != nil {
		return err
	}

	path, err := CurrentStackPath()
	if err != nil {
		return err
	}

	if selected == DefaultStackName {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("clear current stack selection %q: %w", path, err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create current stack directory for %q: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(selected+"\n"), 0o600); err != nil {
		return fmt.Errorf("write current stack selection %q: %w", path, err)
	}

	return nil
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

	stacksDir := filepath.Join(configDir, "stacks")
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

func RedisACLPath(cfg Config) string {
	return filepath.Join(cfg.Stack.Dir, DefaultRedisACLName)
}

func PgAdminServersPath(cfg Config) string {
	return filepath.Join(cfg.Stack.Dir, DefaultPgAdminServers)
}

func PGPassPath(cfg Config) string {
	return filepath.Join(cfg.Stack.Dir, DefaultPGPassName)
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
