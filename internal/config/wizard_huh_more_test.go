package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestBuildWizardFormCoversSupportedAndUnsupportedCockpitPlatforms(t *testing.T) {
	state := newWizardState(Default())
	state.StackMode = wizardStackModeExternal
	state.ExternalStackDir = filepath.Join(t.TempDir(), "missing")
	state.IncludeCockpit = true

	supported := buildWizardForm(&state, system.Platform{
		GOOS:           "linux",
		PackageManager: "dnf",
		ServiceManager: system.ServiceManagerSystemd,
	})
	if supported == nil {
		t.Fatal("expected supported-platform wizard form")
	}
	if supported.Init() == nil {
		t.Fatal("expected wizard form init command")
	}

	unsupported := buildWizardForm(&state, system.Platform{
		GOOS:           "darwin",
		PackageManager: "brew",
		ServiceManager: system.ServiceManagerNone,
	})
	if unsupported == nil {
		t.Fatal("expected unsupported-platform wizard form")
	}
	if unsupported.Init() == nil {
		t.Fatal("expected wizard form init command")
	}
}

func TestRunWizardReviewAccessibleDefaultsToSave(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	state := newWizardState(Default())
	var out bytes.Buffer

	confirmed, err := runWizardReview(strings.NewReader("\n"), &out, state)
	if err != nil {
		t.Fatalf("runWizardReview returned error: %v", err)
	}
	if !confirmed {
		t.Fatal("expected accessible review to accept the default save choice")
	}
	if got := out.String(); !strings.Contains(got, "Save this configuration?") {
		t.Fatalf("expected review output to include confirmation prompt, got %q", got)
	}
}

func TestRunWizardReviewAccessibleAllowsCancel(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	state := newWizardState(Default())

	confirmed, err := runWizardReview(strings.NewReader("n\n"), io.Discard, state)
	if err != nil {
		t.Fatalf("runWizardReview returned error: %v", err)
	}
	if confirmed {
		t.Fatal("expected review cancellation to return false")
	}
}

func TestWizardThemeAndFilePickerStartDirHelpers(t *testing.T) {
	theme := wizardTheme(io.Discard)
	if theme.Theme(true) == nil {
		t.Fatal("expected dark wizard theme styles")
	}
	if theme.Theme(false) == nil {
		t.Fatal("expected light wizard theme styles")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	if got := wizardFilePickerStartDir("   "); got != cwd {
		t.Fatalf("expected blank picker path to use cwd, got %q want %q", got, cwd)
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(filePath, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if got := wizardFilePickerStartDir(filePath); got != dir {
		t.Fatalf("expected file picker to start in file directory, got %q want %q", got, dir)
	}

	missing := filepath.Join(dir, "future-stack")
	if got := wizardFilePickerStartDir(missing); got != missing {
		t.Fatalf("expected missing picker path to resolve absolutely, got %q want %q", got, missing)
	}
}
