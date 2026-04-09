package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestConfigPathAndDefaultEdgeCases(t *testing.T) {
	t.Run("Load and Save surface resolvePath failures", func(t *testing.T) {
		t.Setenv(StackNameEnvVar, "staging")
		t.Setenv("XDG_CONFIG_HOME", "relative-config-root")
		if _, err := loadWithPlatform("", system.Platform{}); err == nil {
			t.Fatal("expected loadWithPlatform to fail when the selected config path cannot be resolved")
		}
		if err := Save("", Default()); err == nil {
			t.Fatal("expected Save to fail when the selected config path cannot be resolved")
		}
	})

	t.Run("legacy setup defaults restore scaffold_default_stack", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("stack:\n  name: dev-stack\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		cfg, err := loadWithPlatform(path, system.Platform{})
		if err != nil {
			t.Fatalf("loadWithPlatform returned error: %v", err)
		}
		if !cfg.Setup.ScaffoldDefaultStack {
			t.Fatal("expected missing scaffold_default_stack to default to true")
		}
	})

	t.Run("ApplyPlatformDefaults ignores nil configs", func(t *testing.T) {
		ApplyPlatformDefaults(nil, system.Platform{PackageManager: "brew"})
	})

	t.Run("packageManagerWizardSuggestions skips blank entries", func(t *testing.T) {
		values := packageManagerWizardSuggestions("   ")
		if len(values) == 0 {
			t.Fatal("expected package manager suggestions")
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("did not expect blank suggestion in %+v", values)
			}
		}
	})

	t.Run("ConfigFilePathForStack and KnownConfigPaths surface config-dir resolution failures", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "relative-config-root")

		if _, err := ConfigFilePathForStack("staging"); err == nil {
			t.Fatal("expected ConfigFilePathForStack to fail without a config root")
		}
		if _, err := KnownConfigPaths(); err == nil {
			t.Fatal("expected KnownConfigPaths to fail without a config root")
		}
	})

	t.Run("SetCurrentStackName surfaces mkdir and write failures", func(t *testing.T) {
		parentFile := filepath.Join(t.TempDir(), "config-file")
		if err := os.WriteFile(parentFile, []byte("not-a-directory"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		t.Setenv("XDG_CONFIG_HOME", parentFile)
		if err := SetCurrentStackName("staging"); err == nil || !strings.Contains(err.Error(), "create current stack directory") {
			t.Fatalf("expected mkdir failure, got %v", err)
		}

		configRoot := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configRoot)
		blockingPath := filepath.Join(configRoot, "stackctl", CurrentStackFileName)
		if err := os.MkdirAll(filepath.Join(configRoot, "stackctl"), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := os.Mkdir(blockingPath, 0o755); err != nil {
			t.Fatalf("Mkdir returned error: %v", err)
		}
		if err := SetCurrentStackName("staging"); err == nil || !strings.Contains(err.Error(), "write current stack selection") {
			t.Fatalf("expected write failure, got %v", err)
		}
	})
}
