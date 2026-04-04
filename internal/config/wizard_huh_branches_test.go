package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestWizardStateHelpersCoverReviewParsingAndVisibilityBranches(t *testing.T) {
	managed := newWizardState(Default())
	managed.Services = []string{"postgres", "redis", "nats", "seaweedfs", "meilisearch", "pgadmin", "custom"}
	managed.IncludeCockpit = true
	managed.CockpitPort = "9090"
	managed.SeaweedFSPort = "8333"
	managed.MeilisearchPort = "7700"
	summary := managed.reviewSummary()
	for _, fragment := range []string{
		"Stack: dev-stack",
		"Mode: Managed",
		"Services: Postgres, Redis, NATS, SeaweedFS, Meilisearch, pgAdmin, custom",
		"  - Postgres: 5432",
		"  - Redis: 6379",
		"  - NATS: 4222",
		"  - SeaweedFS: 8333",
		"  - Meilisearch: 7700",
		"  - pgAdmin: 8081",
		"  - Cockpit: 9090",
		"Managed stack dir:",
	} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("expected managed review summary to contain %q:\n%s", fragment, summary)
		}
	}

	externalDir := filepath.Join(t.TempDir(), "external-stack")
	external := managed
	external.StackMode = wizardStackModeExternal
	external.ExternalStackDir = externalDir
	external.ExternalComposeFile = "compose.custom.yaml"
	external.IncludeCockpit = false
	externalSummary := external.reviewSummary()
	for _, fragment := range []string{
		"Mode: External",
		"External stack dir: " + externalDir,
		"Compose file: compose.custom.yaml",
	} {
		if !strings.Contains(externalSummary, fragment) {
			t.Fatalf("expected external review summary to contain %q:\n%s", fragment, externalSummary)
		}
	}

	if got := external.stackModeLabel(); got != "External" {
		t.Fatalf("unexpected external stack mode label %q", got)
	}

	if values := external.serviceDisplayNames(); values[len(values)-1] != "custom" {
		t.Fatalf("expected unknown wizard services to fall back to raw names, got %+v", values)
	}

	if got := wizardStepLabel("missing"); got != "Setup" {
		t.Fatalf("expected unknown wizard step label fallback, got %q", got)
	}

	blank := wizardState{}
	if blank.needsMissingExternalDirConfirmation() {
		t.Fatal("expected blank external path confirmation to stay false")
	}

	filePath := filepath.Join(t.TempDir(), "compose.yaml")
	if err := os.WriteFile(filePath, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	fileState := wizardState{StackMode: wizardStackModeExternal, ExternalStackDir: filePath}
	if !fileState.needsMissingExternalDirConfirmation() {
		t.Fatal("expected file path to require confirmation")
	}

	missingState := wizardState{StackMode: wizardStackModeExternal, ExternalStackDir: filepath.Join(t.TempDir(), "missing")}
	if !missingState.needsMissingExternalDirConfirmation() {
		t.Fatal("expected missing directory to require confirmation")
	}

	existingDir := t.TempDir()
	dirState := wizardState{StackMode: wizardStackModeExternal, ExternalStackDir: existingDir}
	if dirState.needsMissingExternalDirConfirmation() {
		t.Fatal("expected existing directory not to require confirmation")
	}

	visible := visibleWizardSteps(&missingState)
	if len(visible) == 0 {
		t.Fatal("expected visible wizard steps")
	}
	position, total := wizardStepPosition(&missingState, wizardStepExternalPath)
	if position == 0 || total == 0 {
		t.Fatalf("expected external path confirmation step to be visible, got position=%d total=%d", position, total)
	}
	if got := wizardNextStepLabel(&missingState, wizardStepExternalPath); got == "" {
		t.Fatal("expected a next wizard step after external path confirmation")
	}
	if got := wizardNextStepLabel(&missingState, wizardStepReview); got != "" {
		t.Fatalf("expected review step to have no next label, got %q", got)
	}

	parseCases := []struct {
		name  string
		fn    func(string) (int, error)
		value string
		want  int
		ok    bool
	}{
		{name: "parse port", fn: parsePort, value: "5432", want: 5432, ok: true},
		{name: "parse positive int", fn: parsePositiveInt, value: "42", want: 42, ok: true},
		{name: "parse postgres log duration", fn: parsePostgresLogDurationMS, value: "-1", want: -1, ok: true},
		{name: "invalid port", fn: parsePort, value: "70000", ok: false},
		{name: "invalid positive int", fn: parsePositiveInt, value: "0", ok: false},
		{name: "invalid duration", fn: parsePostgresLogDurationMS, value: "0", ok: false},
	}
	for _, tc := range parseCases {
		got, err := tc.fn(tc.value)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("%s: got (%d, %v), want (%d, nil)", tc.name, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: expected parsing %q to fail", tc.name, tc.value)
		}
	}
}

func TestWizardStateToConfigForPlatformCoversManagedExternalAndInvalidModes(t *testing.T) {
	base := Default()

	managed := newWizardState(base)
	managed.StackMode = wizardStackModeManaged
	managed.StackName = "ops"
	managed.Services = []string{"postgres", "pgadmin"}
	managed.IncludeCockpit = true
	managed.InstallCockpit = true

	cfg, err := managed.toConfigForPlatform(base, system.Platform{
		GOOS:           "darwin",
		PackageManager: "brew",
		ServiceManager: system.ServiceManagerNone,
	})
	if err != nil {
		t.Fatalf("managed toConfigForPlatform returned error: %v", err)
	}
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		t.Fatalf("expected managed wizard config, got %+v %+v", cfg.Stack, cfg.Setup)
	}
	if cfg.Setup.InstallCockpit {
		t.Fatalf("expected unsupported cockpit install to be normalized off, got %+v", cfg.Setup)
	}
	if cfg.Stack.ComposeFile != DefaultComposeFileName {
		t.Fatalf("expected default managed compose file, got %q", cfg.Stack.ComposeFile)
	}

	externalDir := filepath.Join(t.TempDir(), "ext")
	external := newWizardState(base)
	external.StackMode = wizardStackModeExternal
	external.ExternalStackDir = externalDir
	external.ExternalComposeFile = "compose.ext.yaml"
	external.Services = []string{"redis"}
	external.IncludeCockpit = false
	external.InstallCockpit = true
	external.PgAdminBootstrapPostgresServer = true

	cfg, err = external.toConfigForPlatform(base, system.Platform{
		GOOS:           "linux",
		PackageManager: "dnf",
		ServiceManager: system.ServiceManagerSystemd,
	})
	if err != nil {
		t.Fatalf("external toConfigForPlatform returned error: %v", err)
	}
	if cfg.Stack.Managed || cfg.Setup.ScaffoldDefaultStack {
		t.Fatalf("expected external wizard config, got %+v %+v", cfg.Stack, cfg.Setup)
	}
	if cfg.Stack.Dir != externalDir || cfg.Stack.ComposeFile != "compose.ext.yaml" {
		t.Fatalf("unexpected external stack target %+v", cfg.Stack)
	}
	if cfg.Services.PgAdmin.BootstrapPostgresServer {
		t.Fatalf("expected pgAdmin bootstrap to disable when Postgres is not selected, got %+v", cfg.Services.PgAdmin)
	}

	invalid := newWizardState(base)
	invalid.StackMode = "broken"
	if _, err := invalid.toConfigForPlatform(base, system.CurrentPlatform()); err == nil || !strings.Contains(err.Error(), "invalid stack mode") {
		t.Fatalf("expected invalid stack mode error, got %v", err)
	}
}
