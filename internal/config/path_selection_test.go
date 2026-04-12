package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPathHelpersCoverSelectionRemovalAndKnownConfigDiscovery(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	currentPath, err := CurrentStackPath()
	if err != nil {
		t.Fatalf("CurrentStackPath returned error: %v", err)
	}

	if err := SetCurrentStackName("staging"); err != nil {
		t.Fatalf("SetCurrentStackName returned error: %v", err)
	}
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if strings.TrimSpace(string(data)) != "staging" {
		t.Fatalf("expected current stack file to contain staging, got %q", string(data))
	}

	if err := SetCurrentStackName(DefaultStackName); err != nil {
		t.Fatalf("SetCurrentStackName(default) returned error: %v", err)
	}
	if _, err := os.Stat(currentPath); !os.IsNotExist(err) {
		t.Fatalf("expected default stack selection to remove current stack file, got %v", err)
	}

	configDir, err := ConfigDirPath()
	if err != nil {
		t.Fatalf("ConfigDirPath returned error: %v", err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(currentPath, []byte("INVALID!\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := CurrentStackName(); err == nil || !strings.Contains(err.Error(), "parse current stack selection") {
		t.Fatalf("expected invalid current stack parse error, got %v", err)
	}

	stacksDir, err := ConfigStacksDirPath()
	if err != nil {
		t.Fatalf("ConfigStacksDirPath returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(stacksDir, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll nested returned error: %v", err)
	}
	defaultConfig := filepath.Join(configDir, "config.yaml")
	stagingConfig := filepath.Join(stacksDir, "staging.yaml")
	opsConfig := filepath.Join(stacksDir, "ops.yaml")
	for _, path := range []string{defaultConfig, stagingConfig, opsConfig} {
		if err := os.WriteFile(path, []byte("stack:\n  name: test\n"), 0o600); err != nil {
			t.Fatalf("WriteFile %s returned error: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(stacksDir, "notes.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("WriteFile notes.txt returned error: %v", err)
	}

	paths, err := KnownConfigPaths()
	if err != nil {
		t.Fatalf("KnownConfigPaths returned error: %v", err)
	}
	want := []string{defaultConfig, opsConfig, stagingConfig}
	if !slices.Equal(paths, want) {
		t.Fatalf("KnownConfigPaths() = %+v, want %+v", paths, want)
	}
}

func TestResolveSelectedStackNamePrefersEnvAndRejectsInvalidEnvValues(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv(StackNameEnvVar, " staging ")

	selected, err := ResolveSelectedStackName()
	if err != nil {
		t.Fatalf("ResolveSelectedStackName returned error: %v", err)
	}
	if selected != "staging" {
		t.Fatalf("expected env selection to normalize to staging, got %q", selected)
	}

	t.Setenv(StackNameEnvVar, "INVALID!")
	if _, err := ResolveSelectedStackName(); err == nil || !strings.Contains(err.Error(), "validate "+StackNameEnvVar) {
		t.Fatalf("expected invalid env selection to fail, got %v", err)
	}
}

func TestCurrentAndSelectedStackNameCoverAdditionalBranches(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	currentPath, err := CurrentStackPath()
	if err != nil {
		t.Fatalf("CurrentStackPath returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	if err := os.WriteFile(currentPath, []byte("ops\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if got := SelectedStackName(); got != "ops" {
		t.Fatalf("expected selected stack name ops, got %q", got)
	}

	if err := os.WriteFile(currentPath, []byte(" \n"), 0o600); err != nil {
		t.Fatalf("WriteFile blank current stack returned error: %v", err)
	}
	if got, err := CurrentStackName(); err != nil || got != DefaultStackName {
		t.Fatalf("expected blank current stack file to fall back to default, got (%q, %v)", got, err)
	}

	if err := os.Remove(currentPath); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if err := os.Mkdir(currentPath, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	if _, err := CurrentStackName(); err == nil || !strings.Contains(err.Error(), "read current stack selection") {
		t.Fatalf("expected current stack read error, got %v", err)
	}
}
