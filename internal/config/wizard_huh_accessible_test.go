package config

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRunHuhWizardWithPlatformAccessibleAcceptsManagedDefaults(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "dnf",
		ServiceManager: system.ServiceManagerSystemd,
	}
	base := DefaultForStackOnPlatform("dev-stack", platform)
	var out bytes.Buffer

	cfg, err := runHuhWizardWithPlatform(
		strings.NewReader(wizardAnswersForState(newWizardState(base), platform, "y")),
		&out,
		base,
		platform,
	)
	if err != nil {
		t.Fatalf("runHuhWizardWithPlatform returned error: %v\noutput:\n%s", err, out.String())
	}
	if cfg.Stack.Name != base.Stack.Name || !cfg.Stack.Managed {
		t.Fatalf("expected managed defaults to survive the Huh wizard, got %+v", cfg.Stack)
	}
	if !cfg.Setup.IncludeCockpit || !cfg.Setup.InstallCockpit {
		t.Fatalf("expected Linux defaults to keep Cockpit enabled, got %+v", cfg.Setup)
	}
	if cfg.System.PackageManager != "dnf" {
		t.Fatalf("expected platform package manager to apply, got %q", cfg.System.PackageManager)
	}
	if got := out.String(); !strings.Contains(got, "Save this configuration?") {
		t.Fatalf("expected accessible Huh output to include the review prompt, got %q", got)
	}
}

func TestRunHuhWizardWithPlatformAccessibleHandlesExternalMissingPath(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	platform := system.Platform{
		GOOS:           "darwin",
		PackageManager: "brew",
		ServiceManager: system.ServiceManagerNone,
	}
	base := DefaultForStackOnPlatform("staging", platform)
	base.Stack.Managed = false
	base.Setup.ScaffoldDefaultStack = false
	base.Stack.Dir = filepath.Join(t.TempDir(), "missing-stack")
	base.Stack.ComposeFile = "compose.yaml"
	base.Stack.Name = "staging"
	base.Setup.IncludeCockpit = false
	base.Setup.InstallCockpit = false

	state := newWizardState(base)
	state.AllowMissingStackDir = true

	var out bytes.Buffer
	cfg, err := runHuhWizardWithPlatform(
		strings.NewReader(wizardAnswersForAllServiceFields(state, platform, "y")),
		&out,
		base,
		platform,
	)
	if err != nil {
		t.Fatalf("expected external wizard run to succeed, got %v\noutput:\n%s", err, out.String())
	}
	if got := out.String(); !strings.Contains(got, "Use the missing external directory anyway?") {
		t.Fatalf("expected external-path confirmation prompt, got %q", got)
	}
	if cfg.Stack.Managed || cfg.Stack.Dir != base.Stack.Dir || cfg.Stack.ComposeFile != base.Stack.ComposeFile {
		t.Fatalf("expected external stack settings to persist, got %+v", cfg.Stack)
	}
	if cfg.System.PackageManager != "brew" {
		t.Fatalf("expected explicit external package manager to persist, got %q", cfg.System.PackageManager)
	}
}

func wizardAnswersForState(state wizardState, platform system.Platform, review string) string {
	lines := []string{
		state.StackName,
		wizardStackModeAnswer(state.StackMode),
	}
	if state.StackMode == wizardStackModeExternal {
		lines = append(lines, state.ExternalStackDir, state.ExternalComposeFile)
		if state.needsMissingExternalDirConfirmation() {
			lines = append(lines, yesNoAnswer(state.AllowMissingStackDir))
		}
	}

	lines = append(lines, "0")

	if state.includesService("postgres") {
		lines = append(lines,
			state.PostgresContainer,
			state.PostgresImage,
			state.PostgresDataVolume,
			state.PostgresMaintenanceDB,
			state.PostgresMaxConnections,
			state.PostgresSharedBuffers,
			state.PostgresLogDurationMS,
			state.PostgresPort,
			state.PostgresDatabase,
			state.PostgresUsername,
			state.PostgresPassword,
		)
	}
	if state.includesService("redis") {
		lines = append(lines,
			state.RedisContainer,
			state.RedisImage,
			state.RedisDataVolume,
			yesNoAnswer(state.RedisAppendOnly),
			state.RedisSavePolicy,
			state.RedisMaxMemoryPolicy,
			state.RedisPort,
			state.RedisPassword,
			state.RedisACLUsername,
			state.RedisACLPassword,
		)
	}
	if state.includesService("nats") {
		lines = append(lines,
			state.NATSContainer,
			state.NATSImage,
			state.NATSPort,
			state.NATSToken,
		)
	}
	if state.includesService("seaweedfs") {
		lines = append(lines,
			state.SeaweedFSContainer,
			state.SeaweedFSImage,
			state.SeaweedFSDataVolume,
			state.SeaweedFSVolumeSizeMB,
			state.SeaweedFSPort,
			state.SeaweedFSAccessKey,
			state.SeaweedFSSecretKey,
		)
	}
	if state.includesService("meilisearch") {
		lines = append(lines,
			state.MeilisearchContainer,
			state.MeilisearchImage,
			state.MeilisearchDataVolume,
			state.MeilisearchPort,
			state.MeilisearchMasterKey,
		)
	}
	if state.includesService("pgadmin") {
		lines = append(lines,
			state.PgAdminContainer,
			state.PgAdminImage,
			state.PgAdminDataVolume,
			yesNoAnswer(state.PgAdminServerMode),
			yesNoAnswer(state.PgAdminBootstrapPostgresServer),
			state.PgAdminBootstrapServerName,
			state.PgAdminBootstrapServerGroup,
			state.PgAdminPort,
			state.PgAdminEmail,
			state.PgAdminPassword,
		)
	}

	lines = append(lines, yesNoAnswer(state.IncludeCockpit))
	if state.IncludeCockpit {
		lines = append(lines, state.CockpitPort)
		if platform.SupportsCockpit() {
			lines = append(lines, yesNoAnswer(state.InstallCockpit))
		}
	}

	lines = append(lines,
		yesNoAnswer(state.WaitForServicesStart),
		state.StartupTimeoutSec,
		state.PackageManager,
		review,
	)

	return strings.Join(lines, "\n") + "\n"
}

func wizardAnswersForAllServiceFields(state wizardState, platform system.Platform, review string) string {
	lines := []string{
		state.StackName,
		wizardStackModeAnswer(state.StackMode),
		state.ExternalStackDir,
		state.ExternalComposeFile,
		yesNoAnswer(state.AllowMissingStackDir),
		"0",
		state.PostgresContainer,
		state.PostgresImage,
		state.PostgresDataVolume,
		state.PostgresMaintenanceDB,
		state.PostgresMaxConnections,
		state.PostgresSharedBuffers,
		state.PostgresLogDurationMS,
		state.PostgresPort,
		state.PostgresDatabase,
		state.PostgresUsername,
		state.PostgresPassword,
		state.RedisContainer,
		state.RedisImage,
		state.RedisDataVolume,
		yesNoAnswer(state.RedisAppendOnly),
		state.RedisSavePolicy,
		state.RedisMaxMemoryPolicy,
		state.RedisPort,
		state.RedisPassword,
		state.RedisACLUsername,
		state.RedisACLPassword,
		state.NATSContainer,
		state.NATSImage,
		state.NATSPort,
		state.NATSToken,
		state.SeaweedFSContainer,
		state.SeaweedFSImage,
		state.SeaweedFSDataVolume,
		state.SeaweedFSVolumeSizeMB,
		state.SeaweedFSPort,
		state.SeaweedFSAccessKey,
		state.SeaweedFSSecretKey,
		state.MeilisearchContainer,
		state.MeilisearchImage,
		state.MeilisearchDataVolume,
		state.MeilisearchPort,
		state.MeilisearchMasterKey,
		state.PgAdminContainer,
		state.PgAdminImage,
		state.PgAdminDataVolume,
		yesNoAnswer(state.PgAdminServerMode),
		yesNoAnswer(state.PgAdminBootstrapPostgresServer),
		state.PgAdminBootstrapServerName,
		state.PgAdminBootstrapServerGroup,
		state.PgAdminPort,
		state.PgAdminEmail,
		state.PgAdminPassword,
		yesNoAnswer(state.IncludeCockpit),
		state.CockpitPort,
	}
	if platform.SupportsCockpit() {
		lines = append(lines, yesNoAnswer(state.InstallCockpit))
	}
	lines = append(lines,
		yesNoAnswer(state.WaitForServicesStart),
		state.StartupTimeoutSec,
		state.PackageManager,
		review,
	)
	return strings.Join(lines, "\n") + "\n"
}

func wizardStackModeAnswer(mode string) string {
	if mode == wizardStackModeExternal {
		return "2"
	}
	return "1"
}

func yesNoAnswer(value bool) string {
	if value {
		return "y"
	}
	return "n"
}
