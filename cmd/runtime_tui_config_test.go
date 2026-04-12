package cmd

import (
	"errors"
	"io"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestLoadRuntimeConfigAllowFirstRunRequiresTerminal(t *testing.T) {
	withTestDeps(t, nil)

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), true)
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeConfigFirstRunDeclinedReturnsSetupHint(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(_ io.Reader, _ io.Writer, prompt string, defaultYes bool) (bool, error) {
			if !strings.Contains(prompt, "Run interactive setup now?") {
				t.Fatalf("unexpected prompt: %q", prompt)
			}
			if !defaultYes {
				t.Fatal("expected first-run prompt to default yes")
			}
			return false, nil
		}
	})

	root := NewRootCmd(NewApp())
	var stdout strings.Builder
	root.SetOut(&stdout)

	_, err := loadRuntimeConfig(root, true)
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No stackctl config was found.") {
		t.Fatalf("stdout missing first-run preamble: %s", stdout.String())
	}
}

func TestLoadRuntimeConfigFirstRunPropagatesPromptErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, errors.New("prompt boom") }
	})

	root := NewRootCmd(NewApp())
	var stdout strings.Builder
	root.SetOut(&stdout)

	_, err := loadRuntimeConfig(root, true)
	if err == nil || !strings.Contains(err.Error(), "prompt boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No stackctl config was found.") {
		t.Fatalf("stdout missing first-run preamble: %s", stdout.String())
	}
}

func TestLoadRuntimeConfigFirstRunPropagatesWizardErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
		d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
			return configpkg.Config{}, errors.New("wizard boom")
		}
	})

	root := NewRootCmd(NewApp())
	var stdout strings.Builder
	root.SetOut(&stdout)

	_, err := loadRuntimeConfig(root, true)
	if err == nil || !strings.Contains(err.Error(), "wizard boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No stackctl config was found.") {
		t.Fatalf("stdout missing first-run preamble: %s", stdout.String())
	}
}

func TestLoadRuntimeConfigFirstRunPropagatesPreambleWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})

	_, err := loadRuntimeConfig(root, true)
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected preamble write failure, got %v", err)
	}
}

func TestLoadTUIConfigWrapsMissingConfigHint(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := loadTUIConfig()
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadTUIConfigReturnsValidationIssues(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{
				{Field: "stack.dir", Message: "invalid"},
				{Field: "ports.postgres", Message: "must be positive"},
			}
		}
	})

	_, _, err := loadTUIConfig()
	if err == nil || !strings.Contains(err.Error(), "config validation failed: stack.dir: invalid; ports.postgres: must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadTUIConfigReturnsConfigPathAndConfig(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.configFilePath = func() (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	configPath, loaded, err := loadTUIConfig()
	if err != nil {
		t.Fatalf("loadTUIConfig returned error: %v", err)
	}
	if configPath != "/tmp/stackctl/stacks/staging.yaml" {
		t.Fatalf("unexpected config path: %q", configPath)
	}
	if loaded.Stack.Name != "staging" || loaded.Stack.Dir != cfg.Stack.Dir {
		t.Fatalf("unexpected config returned: %+v", loaded.Stack)
	}
}
