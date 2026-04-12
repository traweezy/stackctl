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

func TestFactoryResetPromptErrorsReturnForceHint(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, io.ErrUnexpectedEOF }
	})

	_, _, err := executeRoot(t, "factory-reset")
	if err == nil || !strings.Contains(err.Error(), "factory reset confirmation required; rerun with --force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactoryResetPropagatesInitialStatusWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.removeAll = func(string) error {
			t.Fatal("removeAll should not run when the initial status line fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"factory-reset", "--force"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected factory-reset write failure, got %v", err)
	}
}

func TestFactoryResetPropagatesConfigRemovalErrors(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config", "stackctl")
	dataDir := filepath.Join(root, "data", "stackctl")

	withTestDeps(t, func(d *commandDeps) {
		d.configDirPath = func() (string, error) { return configDir, nil }
		d.dataDirPath = func() (string, error) { return dataDir, nil }
		d.removeAll = func(path string) error {
			if path == configDir {
				return errors.New("config removal failed")
			}
			t.Fatal("data dir removal should not run after config removal failure")
			return nil
		}
	})

	_, _, err := executeRoot(t, "factory-reset", "--force")
	if err == nil || !strings.Contains(err.Error(), "remove config dir "+configDir) || !strings.Contains(err.Error(), "config removal failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactoryResetPropagatesDataRemovalErrors(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config", "stackctl")
	dataDir := filepath.Join(root, "data", "stackctl")

	withTestDeps(t, func(d *commandDeps) {
		d.configDirPath = func() (string, error) { return configDir, nil }
		d.dataDirPath = func() (string, error) { return dataDir, nil }
		d.removeAll = func(path string) error {
			if path == dataDir {
				return errors.New("data removal failed")
			}
			return nil
		}
	})

	_, _, err := executeRoot(t, "factory-reset", "--force")
	if err == nil || !strings.Contains(err.Error(), "remove data dir "+dataDir) || !strings.Contains(err.Error(), "data removal failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFactoryResetPropagatesPerTargetStatusWriteErrors(t *testing.T) {
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

	withTestDeps(t, func(d *commandDeps) {
		d.configDirPath = func() (string, error) { return filepath.Join(root, "config", "stackctl"), nil }
		d.dataDirPath = func() (string, error) { return dataDir, nil }
		d.composeDownPath = func(context.Context, system.Runner, string, string, bool) error {
			t.Fatal("composeDownPath should not run when per-target status output fails")
			return nil
		}
	})

	rootCmd := NewRootCmd(NewApp())
	rootCmd.SetOut(&failingWriteBuffer{failAfter: 2})
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"factory-reset", "--force"})

	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected per-target status write failure, got %v", err)
	}
}

func TestComposeFileExistsRejectsDirectories(t *testing.T) {
	if composeFileExists(filepath.Join(t.TempDir(), "missing.yaml")) {
		t.Fatal("expected composeFileExists to return false for missing files")
	}

	dir := t.TempDir()
	if composeFileExists(dir) {
		t.Fatalf("expected composeFileExists to reject directories: %s", dir)
	}
}
