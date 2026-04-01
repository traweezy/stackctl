package cmd

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func TestLoadTUISnapshotBuildsReadOnlyDashboardState(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.configFilePath = func() (string, error) { return "/tmp/test-config.yaml", nil }
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" {
				return system.CommandResult{Stdout: runningContainerJSON(cfg), ExitCode: 0}, nil
			}
			return system.CommandResult{Stdout: "", ExitCode: 0}, nil
		}
		value.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
		value.portListening = func(port int) bool {
			return port == cfg.Ports.Postgres || port == cfg.Ports.Redis
		}
	})

	snapshot, err := loadTUISnapshot()
	if err != nil {
		t.Fatalf("loadTUISnapshot returned error: %v", err)
	}
	if snapshot.ConfigPath != "/tmp/test-config.yaml" {
		t.Fatalf("unexpected config path: %s", snapshot.ConfigPath)
	}
	if snapshot.StackName != "dev-stack" || !snapshot.Managed {
		t.Fatalf("unexpected snapshot metadata: %+v", snapshot)
	}
	if len(snapshot.Stacks) != 1 || snapshot.Stacks[0].Name != "dev-stack" || !snapshot.Stacks[0].Current {
		t.Fatalf("expected snapshot stacks to include the current profile, got %+v", snapshot.Stacks)
	}
	if len(snapshot.Services) != 5 {
		t.Fatalf("expected 5 services in snapshot, got %d", len(snapshot.Services))
	}
	if !snapshot.Services[0].PortListening || !snapshot.Services[1].PortListening {
		t.Fatalf("expected postgres and redis ports to be marked reachable: %+v", snapshot.Services[:2])
	}
	if snapshot.Services[2].PortListening || snapshot.Services[3].PortListening || snapshot.Services[4].PortListening {
		t.Fatalf("expected nats, pgadmin, and cockpit ports to be marked unreachable: %+v", snapshot.Services[2:])
	}
	if len(snapshot.Health) == 0 {
		t.Fatalf("expected health lines in snapshot")
	}
	if len(snapshot.Connections) != 5 {
		t.Fatalf("expected 5 connection entries, got %d", len(snapshot.Connections))
	}
	for label, value := range map[string]string{
		"connect": snapshot.ConnectText,
		"env":     snapshot.EnvExportText,
		"ports":   snapshot.PortsText,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("expected %s text in snapshot, got %q", label, value)
		}
	}
}

func TestTUICmdHelpDocumentsInspectionPanelsAndLiveLogs(t *testing.T) {
	stdout, _, err := executeRoot(t, "tui", "--help")
	if err != nil {
		t.Fatalf("tui --help returned error: %v", err)
	}
	collapsed := strings.Join(strings.Fields(stdout), " ")
	for _, fragment := range []string{
		"overview, stacks, config, services, health, and action history",
		"services pane includes host ports, URLs, endpoints, DSNs, copy actions, shell handoff, and live-log handoff",
		"left rail keeps section navigation, session context, and stack actions together",
		"Stacks pane lets you inspect saved profiles, switch the active stack, start or stop selected stack profiles, and remove profiles",
		"switch the active item inside split inspection panes",
		"press w from the service and health panels to open live logs",
		"--alt-screen string",
		"--help-view string",
	} {
		if !strings.Contains(collapsed, fragment) {
			t.Fatalf("expected tui help to contain %q:\n%s", fragment, stdout)
		}
	}
}

func TestResolveTUIMouseModeAccessibleAutoDisablesMouse(t *testing.T) {
	original := rootOutput
	rootOutput.Accessible = true
	t.Cleanup(func() { rootOutput = original })

	enabled, err := resolveTUIMouseMode("auto")
	if err != nil {
		t.Fatalf("resolveTUIMouseMode returned error: %v", err)
	}
	if enabled {
		t.Fatal("expected accessible auto mode to disable mouse support")
	}
}

func TestResolveTUIAltScreenModeAutoHonorsAccessibleAndEnv(t *testing.T) {
	original := rootOutput
	rootOutput.Accessible = true
	t.Cleanup(func() { rootOutput = original })
	t.Setenv("STACKCTL_TUI_NO_ALT_SCREEN", "")

	enabled, err := resolveTUIAltScreenMode("auto")
	if err != nil {
		t.Fatalf("resolveTUIAltScreenMode returned error: %v", err)
	}
	if enabled {
		t.Fatal("expected accessible auto mode to disable alt-screen")
	}

	rootOutput.Accessible = false
	t.Setenv("STACKCTL_TUI_NO_ALT_SCREEN", "1")
	enabled, err = resolveTUIAltScreenMode("auto")
	if err != nil {
		t.Fatalf("resolveTUIAltScreenMode returned error: %v", err)
	}
	if enabled {
		t.Fatal("expected env automation override to disable alt-screen")
	}
}

func TestResolveTUIHelpViewModeAutoHonorsAccessible(t *testing.T) {
	original := rootOutput
	rootOutput.Accessible = true
	t.Cleanup(func() { rootOutput = original })

	expanded, err := resolveTUIHelpViewMode("auto")
	if err != nil {
		t.Fatalf("resolveTUIHelpViewMode returned error: %v", err)
	}
	if !expanded {
		t.Fatal("expected accessible auto mode to expand help")
	}
}

func TestLoadTUISnapshotCarriesValidationIssuesIntoTheEditor(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{
				{Field: "stack.dir", Message: "must not be empty"},
				{Field: "stack.compose_file", Message: "must not be empty"},
			}
		}
	})

	snapshot, err := loadTUISnapshot()
	if err != nil {
		t.Fatalf("expected validation issues to stay in the snapshot, got %v", err)
	}
	if len(snapshot.ConfigIssues) != 2 {
		t.Fatalf("expected validation issues in snapshot, got %+v", snapshot.ConfigIssues)
	}
	if !strings.Contains(snapshot.ServiceError, "Config has 2 validation issue(s)") {
		t.Fatalf("expected service error summary, got %q", snapshot.ServiceError)
	}
}

func TestLoadTUISnapshotUsesDefaultsWhenConfigIsMissing(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.defaultConfig = configpkg.Default
		value.validateConfig = configpkg.Validate
		value.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		value.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
	})

	snapshot, err := loadTUISnapshot()
	if err != nil {
		t.Fatalf("expected missing config to stay recoverable in the TUI, got %v", err)
	}
	if snapshot.ConfigSource != stacktui.ConfigSourceMissing {
		t.Fatalf("expected missing config source, got %q", snapshot.ConfigSource)
	}
	if snapshot.ConfigData.Stack.Name != configpkg.Default().Stack.Name {
		t.Fatalf("expected default config draft, got %+v", snapshot.ConfigData.Stack)
	}
	if len(snapshot.ConfigIssues) != 0 {
		t.Fatalf("expected pending scaffold to stay out of validation issues, got %+v", snapshot.ConfigIssues)
	}
	if !snapshot.ConfigNeedsScaffold {
		t.Fatalf("expected missing config draft to mark scaffold as pending")
	}
	if !strings.Contains(snapshot.ConfigProblem, "No stackctl config was found") {
		t.Fatalf("expected missing-config guidance, got %q", snapshot.ConfigProblem)
	}
}

func TestLoadTUISnapshotKeepsUnreadableConfigEditable(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("parse config broken") }
	})

	snapshot, err := loadTUISnapshot()
	if err != nil {
		t.Fatalf("expected unreadable config to remain recoverable in the TUI, got %v", err)
	}
	if snapshot.ConfigSource != stacktui.ConfigSourceUnavailable {
		t.Fatalf("expected unavailable config source, got %q", snapshot.ConfigSource)
	}
	if !strings.Contains(snapshot.ConfigProblem, "Current config could not be loaded") {
		t.Fatalf("expected unreadable-config guidance, got %q", snapshot.ConfigProblem)
	}
}

func TestBuildTUISnapshotTreatsPendingManagedScaffoldAsGuidance(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.validateConfig = configpkg.Validate
		value.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		value.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
			t.Fatal("runtime inspection should not run while scaffold is pending")
			return system.CommandResult{}, nil
		}
	})

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	snapshot := buildTUISnapshot("/tmp/test-config.yaml", cfg, stacktui.ConfigSourceLoaded, "")
	if len(snapshot.ConfigIssues) != 0 {
		t.Fatalf("expected scaffold-pending snapshot to stay validation-clean, got %+v", snapshot.ConfigIssues)
	}
	if !snapshot.ConfigNeedsScaffold {
		t.Fatalf("expected scaffold-pending snapshot to mark pending scaffold")
	}
	for _, message := range []string{snapshot.ServiceError, snapshot.HealthError, snapshot.DoctorError} {
		if !strings.Contains(message, "Use g in Config") {
			t.Fatalf("expected scaffold guidance message, got %q", message)
		}
	}
}

func TestBuildTUISnapshotPreservesServiceLoadErrors(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
			return system.CommandResult{}, errors.New("podman unavailable")
		}
		value.portListening = func(int) bool { return false }
		value.cockpitStatus = func(context.Context) system.CockpitState { return system.CockpitState{State: "not installed"} }
	})

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	snapshot := buildTUISnapshot("/tmp/test-config.yaml", cfg, stacktui.ConfigSourceLoaded, "")
	if snapshot.ServiceError == "" {
		t.Fatalf("expected service error in snapshot")
	}
	if len(snapshot.Health) == 0 {
		t.Fatalf("expected health lines even when services fail")
	}
	if snapshot.Health[0].Status != output.StatusFail {
		t.Fatalf("expected health failure when container inspection breaks, got %+v", snapshot.Health[0])
	}
	if !strings.Contains(snapshot.Health[0].Message, "container status check failed") {
		t.Fatalf("expected health error detail, got %+v", snapshot.Health[0])
	}
}

func TestBuildTUISnapshotIncludesDoctorSummary(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman present"},
				doctorpkg.Check{Status: output.StatusWarn, Message: "podman compose alias missing"},
				doctorpkg.Check{Status: output.StatusFail, Message: "cockpit inactive"},
			), nil
		}
	})

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	snapshot := buildTUISnapshot("/tmp/test-config.yaml", cfg, stacktui.ConfigSourceLoaded, "")
	if snapshot.DoctorSummary.OK != 1 || snapshot.DoctorSummary.Warn != 1 || snapshot.DoctorSummary.Fail != 1 {
		t.Fatalf("unexpected doctor summary: %+v", snapshot.DoctorSummary)
	}
	if len(snapshot.DoctorChecks) != 3 {
		t.Fatalf("expected doctor checks to be preserved, got %+v", snapshot.DoctorChecks)
	}
}

func TestBuildTUILogWatchCommandCanonicalizesAliasesAndUsesWatchMode(t *testing.T) {
	var capturedTail int
	var capturedService string
	var follow bool

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeLogs = func(_ context.Context, runner system.Runner, _ configpkg.Config, tail int, watch bool, _ string, service string) error {
			capturedTail = tail
			capturedService = service
			follow = watch
			_, _ = io.WriteString(runner.Stdout, "postgres up\n")
			return nil
		}
	})

	command, err := buildTUILogWatchCommand(stacktui.LogWatchRequest{Service: "pg"})
	if err != nil {
		t.Fatalf("buildTUILogWatchCommand returned error: %v", err)
	}
	command.SetStdout(io.Discard)
	command.SetStderr(io.Discard)
	if err := command.Run(); err != nil {
		t.Fatalf("watch command run returned error: %v", err)
	}
	if capturedTail != tuiLogWatchTail {
		t.Fatalf("expected watch tail of %d, got %d", tuiLogWatchTail, capturedTail)
	}
	if capturedService != "postgres" {
		t.Fatalf("expected postgres service alias to be normalized, got %q", capturedService)
	}
	if !follow {
		t.Fatalf("expected live log watch mode")
	}
}

func TestBuildTUILogWatchCommandRejectsUnknownServiceAlias(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, err := buildTUILogWatchCommand(stacktui.LogWatchRequest{Service: "unknown"})
	if err == nil {
		t.Fatal("expected invalid service alias to fail")
	}
	if !strings.Contains(err.Error(), "invalid service") {
		t.Fatalf("expected invalid service error, got %v", err)
	}
}

func TestTUICmdHelpDocumentsPaletteAndShellShortcuts(t *testing.T) {
	stdout, _, err := executeRoot(t, "tui", "--help")
	if err != nil {
		t.Fatalf("tui --help returned error: %v", err)
	}
	collapsed := strings.Join(strings.Fields(stdout), " ")
	for _, fragment := range []string{
		"use c to copy service values",
		"g to jump between services",
		"ctrl+k for the command palette",
		"stack-wide connect/env/ports copy helpers",
		"e for a service shell",
		"d for the Postgres db shell",
	} {
		if !strings.Contains(collapsed, fragment) {
			t.Fatalf("expected tui help to contain %q:\n%s", fragment, stdout)
		}
	}
}

func TestCopyTUIValueToClipboardUsesClipboardDependency(t *testing.T) {
	copied := ""
	withTestDeps(t, func(value *commandDeps) {
		value.copyToClipboard = func(_ context.Context, _ system.Runner, target string) error {
			copied = target
			return nil
		}
	})

	if err := copyTUIValueToClipboard("postgres://app:secret@localhost:5432/app"); err != nil {
		t.Fatalf("copyTUIValueToClipboard returned error: %v", err)
	}
	if copied != "postgres://app:secret@localhost:5432/app" {
		t.Fatalf("unexpected copied value: %q", copied)
	}
}

func TestBuildTUIServiceShellCommandCanonicalizesAliasAndUsesTTYShell(t *testing.T) {
	var capturedService string
	var capturedTTY bool
	var capturedArgs []string

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeExec = func(_ context.Context, runner system.Runner, _ configpkg.Config, service string, _ []string, commandArgs []string, tty bool) error {
			capturedService = service
			capturedTTY = tty
			capturedArgs = append([]string(nil), commandArgs...)
			_, _ = io.WriteString(runner.Stdout, "shell\n")
			return nil
		}
	})

	command, err := buildTUIServiceShellCommand(stacktui.ServiceShellRequest{Service: "pg"})
	if err != nil {
		t.Fatalf("buildTUIServiceShellCommand returned error: %v", err)
	}
	command.SetStdout(io.Discard)
	command.SetStderr(io.Discard)
	if err := command.Run(); err != nil {
		t.Fatalf("service shell run returned error: %v", err)
	}
	if capturedService != "postgres" {
		t.Fatalf("expected postgres alias to normalize, got %q", capturedService)
	}
	if !capturedTTY {
		t.Fatalf("expected service shell to allocate a TTY")
	}
	if len(capturedArgs) != 3 || capturedArgs[0] != "sh" || capturedArgs[1] != "-lc" {
		t.Fatalf("unexpected shell command args: %+v", capturedArgs)
	}
}

func TestBuildTUIDBShellCommandUsesConfiguredDatabase(t *testing.T) {
	var capturedService string
	var capturedTTY bool
	var capturedEnv []string
	var capturedArgs []string

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeExec = func(_ context.Context, runner system.Runner, _ configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			capturedService = service
			capturedTTY = tty
			capturedEnv = append([]string(nil), env...)
			capturedArgs = append([]string(nil), commandArgs...)
			_, _ = io.WriteString(runner.Stdout, "psql\n")
			return nil
		}
	})

	command, err := buildTUIDBShellCommand(stacktui.DBShellRequest{Service: "postgres"})
	if err != nil {
		t.Fatalf("buildTUIDBShellCommand returned error: %v", err)
	}
	command.SetStdout(io.Discard)
	command.SetStderr(io.Discard)
	if err := command.Run(); err != nil {
		t.Fatalf("db shell run returned error: %v", err)
	}
	if capturedService != "postgres" {
		t.Fatalf("expected postgres db shell service, got %q", capturedService)
	}
	if !capturedTTY {
		t.Fatalf("expected db shell to allocate a TTY")
	}
	if len(capturedEnv) != 1 || capturedEnv[0] != "PGPASSWORD=stackpass" {
		t.Fatalf("unexpected db shell env: %+v", capturedEnv)
	}
	expectedArgs := []string{"psql", "-h", "127.0.0.1", "-p", "5432", "-U", "stackuser", "-d", "stackdb"}
	if strings.Join(capturedArgs, " ") != strings.Join(expectedArgs, " ") {
		t.Fatalf("unexpected db shell args: %+v", capturedArgs)
	}
}

func TestBuildTUIServiceShellCommandSuppressesInteractiveExitStatus(t *testing.T) {
	exitErr := interactiveExitError(t)

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			return exitErr
		}
	})

	command, err := buildTUIServiceShellCommand(stacktui.ServiceShellRequest{Service: "postgres"})
	if err != nil {
		t.Fatalf("buildTUIServiceShellCommand returned error: %v", err)
	}
	command.SetStdout(io.Discard)
	command.SetStderr(io.Discard)
	if err := command.Run(); err != nil {
		t.Fatalf("expected interactive shell exit status to be suppressed, got %v", err)
	}
}

func TestBuildTUIDBShellCommandSuppressesInteractiveExitStatus(t *testing.T) {
	exitErr := interactiveExitError(t)

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			return exitErr
		}
	})

	command, err := buildTUIDBShellCommand(stacktui.DBShellRequest{Service: "postgres"})
	if err != nil {
		t.Fatalf("buildTUIDBShellCommand returned error: %v", err)
	}
	command.SetStdout(io.Discard)
	command.SetStderr(io.Discard)
	if err := command.Run(); err != nil {
		t.Fatalf("expected interactive db shell exit status to be suppressed, got %v", err)
	}
}

func interactiveExitError(t *testing.T) error {
	t.Helper()

	err := exec.Command("sh", "-c", "exit 1").Run()
	if err == nil {
		t.Fatal("expected shell exit error")
	}

	return err
}

func TestTUIExecCommandsSetStdin(t *testing.T) {
	reader := strings.NewReader("input")

	logCommand := &tuiLogWatchCommand{}
	logCommand.SetStdin(reader)
	if logCommand.stdin != reader {
		t.Fatal("expected log watch command stdin to be stored")
	}

	serviceCommand := &tuiServiceShellCommand{}
	serviceCommand.SetStdin(reader)
	if serviceCommand.stdin != reader {
		t.Fatal("expected service shell command stdin to be stored")
	}

	dbCommand := &tuiDBShellCommand{}
	dbCommand.SetStdin(reader)
	if dbCommand.stdin != reader {
		t.Fatal("expected db shell command stdin to be stored")
	}
}

func TestValidationIssuesErrorFormatsIssues(t *testing.T) {
	if err := validationIssuesError(nil); err != nil {
		t.Fatalf("expected no error for empty issues, got %v", err)
	}

	err := validationIssuesError([]configpkg.ValidationIssue{
		{Field: "stack.dir", Message: "must not be empty"},
		{Field: "ports.postgres", Message: "must be a valid port"},
	})
	if err == nil {
		t.Fatal("expected validationIssuesError to return an error")
	}
	for _, fragment := range []string{"config validation failed", "stack.dir: must not be empty", "ports.postgres: must be a valid port"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("validation error missing %q: %v", fragment, err)
		}
	}
}
