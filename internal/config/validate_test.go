package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateAcceptsDefaultConfig(t *testing.T) {
	cfg := Default()
	cfg.Stack.Dir = filepath.Join(t.TempDir(), "stacks", "dev-stack")

	if err := validateWithDir(cfg.Stack.Dir); err != nil {
		t.Fatal(err)
	}

	issues := Validate(cfg)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	cfg := Default()
	cfg.Stack.Dir = "relative/path"
	cfg.Stack.ComposeFile = ""
	cfg.Services.RedisContainer = ""
	cfg.Ports.Postgres = 0
	cfg.Behavior.StartupTimeoutSec = -1
	cfg.System.PackageManager = ""

	issues := Validate(cfg)
	if len(issues) < 6 {
		t.Fatalf("expected multiple issues, got %v", issues)
	}
}

func validateWithDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
