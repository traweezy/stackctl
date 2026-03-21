package cmd

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestConfigInitNonInteractiveSavesConfig(t *testing.T) {
	var savedPath string
	var savedConfig configpkg.Config
	scaffolded := false

	withTestDeps(t, func(d *commandDeps) {
		d.saveConfig = func(path string, cfg configpkg.Config) error {
			savedPath = path
			savedConfig = cfg
			return nil
		}
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "init", "--non-interactive")
	if err != nil {
		t.Fatalf("config init returned error: %v", err)
	}

	if savedPath != "/tmp/stackctl/config.yaml" {
		t.Fatalf("saved path = %q", savedPath)
	}
	if savedConfig.Stack.Name != "dev-stack" {
		t.Fatalf("saved config stack name = %q", savedConfig.Stack.Name)
	}
	if !savedConfig.Stack.Managed || !savedConfig.Setup.ScaffoldDefaultStack {
		t.Fatalf("saved config should use managed stack defaults: %+v", savedConfig)
	}
	if !scaffolded {
		t.Fatal("expected config init to scaffold the managed stack")
	}
	if !strings.Contains(stdout, "Saved config to /tmp/stackctl/config.yaml") {
		t.Fatalf("stdout missing save message: %s", stdout)
	}
}

func TestConfigInitExistingWithoutTerminalNeedsForce(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return false }
	})

	_, _, err := executeRoot(t, "config", "init", "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "rerun with --force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigInitExistingDeclineCancels(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
	})

	stdout, _, err := executeRoot(t, "config", "init", "--non-interactive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "ℹ️ config init cancelled") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigViewPrintsYAML(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			return configpkg.Default(), nil
		}
		d.marshalConfig = func(configpkg.Config) ([]byte, error) {
			return []byte("stack:\n  name: dev-stack\n"), nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "view")
	if err != nil {
		t.Fatalf("config view returned error: %v", err)
	}
	if stdout != "stack:\n  name: dev-stack\n" {
		t.Fatalf("unexpected config view output: %q", stdout)
	}
}

func TestConfigValidatePrintsIssues(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			return configpkg.Default(), nil
		}
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.dir", Message: "must exist"}}
		}
	})

	stdout, _, err := executeRoot(t, "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail")
	}
	if !strings.Contains(stdout, "stack.dir: must exist") {
		t.Fatalf("stdout missing validation issue: %s", stdout)
	}
}

func TestConfigValidatePrintsSuccess(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			return configpkg.Default(), nil
		}
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue { return nil }
	})

	stdout, _, err := executeRoot(t, "config", "validate")
	if err != nil {
		t.Fatalf("config validate returned error: %v", err)
	}
	if !strings.Contains(stdout, "✅ config is valid") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigResetDeleteRemovesFile(t *testing.T) {
	var removedPath string

	withTestDeps(t, func(d *commandDeps) {
		d.removeFile = func(path string) error {
			removedPath = path
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "reset", "--delete", "--force")
	if err != nil {
		t.Fatalf("config reset --delete returned error: %v", err)
	}
	if removedPath != "/tmp/stackctl/config.yaml" {
		t.Fatalf("removed path = %q", removedPath)
	}
	if !strings.Contains(stdout, "Deleted config at /tmp/stackctl/config.yaml") {
		t.Fatalf("stdout missing delete message: %s", stdout)
	}
}

func TestConfigResetDeletePropagatesRemoveError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.removeFile = func(string) error { return errors.New("wrong error") }
	})

	_, _, err := executeRoot(t, "config", "reset", "--delete", "--force")
	if err == nil || !strings.Contains(err.Error(), "delete config /tmp/stackctl/config.yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigResetDeleteMissingFileStillSucceeds(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.removeFile = func(string) error { return os.ErrNotExist }
	})

	stdout, _, err := executeRoot(t, "config", "reset", "--delete", "--force")
	if err != nil {
		t.Fatalf("unexpected not-found handling: %v", err)
	}
	if !strings.Contains(stdout, "Deleted config at /tmp/stackctl/config.yaml") {
		t.Fatalf("stdout missing delete message: %s", stdout)
	}
}

func TestConfigResetDeclineCancels(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
	})

	stdout, _, err := executeRoot(t, "config", "reset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "ℹ️ config reset cancelled") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigScaffoldRequiresManagedStack(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Stack.Managed = false
			cfg.Setup.ScaffoldDefaultStack = false
			return cfg, nil
		}
	})

	_, _, err := executeRoot(t, "config", "scaffold")
	if err == nil || !strings.Contains(err.Error(), "external stack") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigScaffoldRunsHelper(t *testing.T) {
	scaffolded := false

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "scaffold")
	if err != nil {
		t.Fatalf("config scaffold returned error: %v", err)
	}
	if !scaffolded {
		t.Fatal("expected config scaffold to call the scaffold helper")
	}
	if !strings.Contains(stdout, "wrote managed compose file") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestMissingConfigHint(t *testing.T) {
	err := missingConfigHint(configpkg.ErrNotFound)
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected missing config hint: %v", err)
	}

	other := errors.New("boom")
	if missingConfigHint(other) != other {
		t.Fatal("expected non-config error to pass through")
	}
}

func TestResolveConfigFromFlagsRunsWizardInteractively(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.runWizard = func(_ io.Reader, _ io.Writer, cfg configpkg.Config) (configpkg.Config, error) {
			cfg.Stack.Name = "custom"
			return cfg, nil
		}
	})

	cfg, err := resolveConfigFromFlags(NewRootCmd(NewApp()), configpkg.Default(), false)
	if err != nil {
		t.Fatalf("resolveConfigFromFlags returned error: %v", err)
	}
	if cfg.Stack.Name != "custom" {
		t.Fatalf("unexpected config from wizard: %+v", cfg)
	}
}

func TestLoadExistingConfigPassesThroughUnexpectedError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("boom") }
	})

	_, exists, err := loadExistingConfig("/tmp/stackctl/config.yaml")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("config should not exist on unexpected load error")
	}
}
