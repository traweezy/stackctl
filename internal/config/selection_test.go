package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrentStackNameDefaultsWhenSelectionMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	current, err := CurrentStackName()
	if err != nil {
		t.Fatalf("CurrentStackName returned error: %v", err)
	}
	if current != DefaultStackName {
		t.Fatalf("unexpected current stack: %s", current)
	}
}

func TestSetCurrentStackNamePersistsSelection(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	if err := SetCurrentStackName("staging"); err != nil {
		t.Fatalf("SetCurrentStackName returned error: %v", err)
	}

	current, err := CurrentStackName()
	if err != nil {
		t.Fatalf("CurrentStackName returned error: %v", err)
	}
	if current != "staging" {
		t.Fatalf("unexpected current stack: %s", current)
	}

	configPath, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath returned error: %v", err)
	}
	if want := filepath.Join(configRoot, "stackctl", "stacks", "staging.yaml"); configPath != want {
		t.Fatalf("unexpected config path: %s", configPath)
	}
}

func TestResolveSelectedStackNameEnvOverridesSavedSelection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SetCurrentStackName("staging"); err != nil {
		t.Fatalf("SetCurrentStackName returned error: %v", err)
	}
	t.Setenv(StackNameEnvVar, "demo")

	selected, err := ResolveSelectedStackName()
	if err != nil {
		t.Fatalf("ResolveSelectedStackName returned error: %v", err)
	}
	if selected != "demo" {
		t.Fatalf("unexpected selected stack: %s", selected)
	}
}

func TestSetCurrentStackNameDefaultClearsSelectionFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := SetCurrentStackName("staging"); err != nil {
		t.Fatalf("SetCurrentStackName returned error: %v", err)
	}
	if err := SetCurrentStackName(DefaultStackName); err != nil {
		t.Fatalf("SetCurrentStackName returned error: %v", err)
	}

	path, err := CurrentStackPath()
	if err != nil {
		t.Fatalf("CurrentStackPath returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected current stack file to be removed, got %v", err)
	}
}

func TestCurrentStackNameRejectsInvalidSelectionFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := CurrentStackPath()
	if err != nil {
		t.Fatalf("CurrentStackPath returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("INVALID\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err = CurrentStackName()
	if err == nil || !strings.Contains(err.Error(), "current stack selection") {
		t.Fatalf("unexpected error: %v", err)
	}
}
