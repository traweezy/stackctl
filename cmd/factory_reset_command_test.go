package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestFactoryResetForceRemovesLocalStateAndManagedStacks(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config", "stackctl")
	dataDir := filepath.Join(root, "data", "stackctl")
	configPath := filepath.Join(configDir, "config.yaml")
	stackDir := filepath.Join(dataDir, "stacks", "dev-stack")
	composePath := filepath.Join(stackDir, configpkg.DefaultComposeFileName)

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir stack dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("stack:\n  name: dev-stack\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	var downCalls []string
	var removed []string

	withTestDeps(t, func(d *commandDeps) {
		d.configDirPath = func() (string, error) { return configDir, nil }
		d.configFilePath = func() (string, error) { return configPath, nil }
		d.knownConfigPaths = func() ([]string, error) { return []string{configPath}, nil }
		d.dataDirPath = func() (string, error) { return dataDir, nil }
		d.composeDownPath = func(_ context.Context, _ system.Runner, dir, composePath string, removeVolumes bool) error {
			if !removeVolumes {
				t.Fatal("factory reset should remove volumes")
			}
			downCalls = append(downCalls, dir+"|"+composePath)
			return nil
		}
		d.removeAll = func(path string) error {
			removed = append(removed, path)
			return os.RemoveAll(path)
		}
	})

	stdout, _, err := executeRoot(t, "factory-reset", "--force")
	if err != nil {
		t.Fatalf("factory-reset returned error: %v", err)
	}

	if len(downCalls) != 1 || downCalls[0] != stackDir+"|"+composePath {
		t.Fatalf("unexpected managed stack teardown calls: %+v", downCalls)
	}
	if len(removed) != 2 || removed[0] != configDir || removed[1] != dataDir {
		t.Fatalf("unexpected removed paths: %+v", removed)
	}
	if _, err := os.Stat(configDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config dir removal, got err=%v", err)
	}
	if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected data dir removal, got err=%v", err)
	}
	for _, fragment := range []string{
		"factory-resetting stackctl local state",
		"removing managed stack",
		"deleted config dir",
		"deleted data dir",
		"stackctl local state removed",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stdout missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestFactoryResetUsesCurrentManagedConfigComposePath(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config", "stackctl")
	configPath := filepath.Join(configDir, "stacks", "custom.yaml")
	dataDir := filepath.Join(root, "data", "stackctl")
	stackDir := filepath.Join(dataDir, "stacks", "custom")
	composePath := filepath.Join(stackDir, configpkg.DefaultComposeFileName)

	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir stack dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config stack dir: %v", err)
	}
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write custom compose: %v", err)
	}

	cfg := configpkg.DefaultForStack("custom")

	var downCalls []string

	withTestDeps(t, func(d *commandDeps) {
		d.configDirPath = func() (string, error) { return configDir, nil }
		d.configFilePath = func() (string, error) { return configPath, nil }
		d.knownConfigPaths = func() ([]string, error) { return []string{configPath}, nil }
		d.dataDirPath = func() (string, error) { return dataDir, nil }
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composePath = configpkg.ComposePath
		d.composeDownPath = func(_ context.Context, _ system.Runner, dir, composePath string, removeVolumes bool) error {
			downCalls = append(downCalls, dir+"|"+composePath)
			return nil
		}
	})

	if _, _, err := executeRoot(t, "factory-reset", "--force"); err != nil {
		t.Fatalf("factory-reset returned error: %v", err)
	}
	if len(downCalls) != 1 || downCalls[0] != stackDir+"|"+composePath {
		t.Fatalf("unexpected teardown calls: %+v", downCalls)
	}
}

func TestFactoryResetWithoutTerminalNeedsForce(t *testing.T) {
	_, _, err := executeRoot(t, "factory-reset")
	if err == nil || !strings.Contains(err.Error(), "rerun with --force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactoryResetDeclineCancels(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		d.removeAll = func(string) error {
			t.Fatal("removeAll should not be called when confirmation is declined")
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "factory-reset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "ℹ️ factory reset cancelled") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestFactoryResetStopsBeforeDeletingWhenComposeTeardownFails(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data", "stackctl")
	stackDir := filepath.Join(dataDir, "stacks", "dev-stack")
	composePath := filepath.Join(stackDir, configpkg.DefaultComposeFileName)

	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir stack dir: %v", err)
	}
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	removed := false

	withTestDeps(t, func(d *commandDeps) {
		d.configDirPath = func() (string, error) { return filepath.Join(root, "config", "stackctl"), nil }
		d.configFilePath = func() (string, error) { return filepath.Join(root, "config", "stackctl", "config.yaml"), nil }
		d.knownConfigPaths = func() ([]string, error) { return []string{filepath.Join(root, "config", "stackctl", "config.yaml")}, nil }
		d.dataDirPath = func() (string, error) { return dataDir, nil }
		d.composeDownPath = func(context.Context, system.Runner, string, string, bool) error { return errors.New("boom") }
		d.removeAll = func(string) error {
			removed = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "factory-reset", "--force")
	if err == nil || !strings.Contains(err.Error(), composePath) || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed {
		t.Fatal("expected removal to be skipped when compose teardown fails")
	}
}
