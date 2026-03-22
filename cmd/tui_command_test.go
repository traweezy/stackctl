package cmd

import (
	"context"
	"errors"
	"io"
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
				return system.CommandResult{Stdout: "[]", ExitCode: 0}, nil
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
	if len(snapshot.Services) != 4 {
		t.Fatalf("expected 4 services in snapshot, got %d", len(snapshot.Services))
	}
	if !snapshot.Services[0].PortListening || !snapshot.Services[1].PortListening {
		t.Fatalf("expected postgres and redis ports to be marked reachable: %+v", snapshot.Services[:2])
	}
	if snapshot.Services[2].PortListening || snapshot.Services[3].PortListening {
		t.Fatalf("expected pgadmin and cockpit ports to be marked unreachable: %+v", snapshot.Services[2:])
	}
	if len(snapshot.Health) == 0 {
		t.Fatalf("expected health lines in snapshot")
	}
	if len(snapshot.Connections) != 4 {
		t.Fatalf("expected 4 connection entries, got %d", len(snapshot.Connections))
	}
}

func TestTUICmdHelpDocumentsInspectionPanelsAndLogFollow(t *testing.T) {
	stdout, _, err := executeRoot(t, "tui", "--help")
	if err != nil {
		t.Fatalf("tui --help returned error: %v", err)
	}
	collapsed := strings.Join(strings.Fields(stdout), " ")
	for _, fragment := range []string{
		"overview, services, ports,",
		"health, connections, logs, and action history",
		"switch the active service/filter inside split inspection panes",
		"press f in Logs to toggle follow mode",
	} {
		if !strings.Contains(collapsed, fragment) {
			t.Fatalf("expected tui help to contain %q:\n%s", fragment, stdout)
		}
	}
}

func TestLoadTUISnapshotReturnsValidationError(t *testing.T) {
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

	_, err := loadTUISnapshot()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	for _, fragment := range []string{
		"config validation failed",
		"stack.dir: must not be empty",
		"stack.compose_file: must not be empty",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected validation error to contain %q, got %v", fragment, err)
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

	snapshot := buildTUISnapshot("/tmp/test-config.yaml", cfg)
	if snapshot.ServiceError == "" {
		t.Fatalf("expected service error in snapshot")
	}
	if len(snapshot.Health) == 0 {
		t.Fatalf("expected health lines even when services fail")
	}
	if snapshot.Health[0].Status != output.StatusWarn {
		t.Fatalf("expected health line status to be preserved, got %+v", snapshot.Health[0])
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

	snapshot := buildTUISnapshot("/tmp/test-config.yaml", cfg)
	if snapshot.DoctorSummary.OK != 1 || snapshot.DoctorSummary.Warn != 1 || snapshot.DoctorSummary.Fail != 1 {
		t.Fatalf("unexpected doctor summary: %+v", snapshot.DoctorSummary)
	}
	if len(snapshot.DoctorChecks) != 3 {
		t.Fatalf("expected doctor checks to be preserved, got %+v", snapshot.DoctorChecks)
	}
}

func TestLoadTUILogsUsesDefaultTailForAllServices(t *testing.T) {
	var capturedTail int
	var capturedService string

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeLogs = func(_ context.Context, runner system.Runner, _ configpkg.Config, tail int, watch bool, _ string, service string) error {
			capturedTail = tail
			capturedService = service
			if watch {
				t.Fatalf("expected TUI logs to use snapshot mode, not watch mode")
			}
			if _, err := io.WriteString(runner.Stdout, "postgres up\nredis up\n"); err != nil {
				return err
			}
			return nil
		}
	})

	logs, err := loadTUILogs(stacktui.LogRequest{})
	if err != nil {
		t.Fatalf("loadTUILogs returned error: %v", err)
	}
	if capturedTail != 200 {
		t.Fatalf("expected default tail of 200, got %d", capturedTail)
	}
	if capturedService != "" {
		t.Fatalf("expected no service filter, got %q", capturedService)
	}
	if !strings.Contains(logs.Output, "postgres up") || !strings.Contains(logs.Output, "redis up") {
		t.Fatalf("expected combined logs output, got %q", logs.Output)
	}
}

func TestLoadTUILogsCanonicalizesAliasesAndReturnsPartialOutputOnError(t *testing.T) {
	var capturedTail int
	var capturedService string

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeLogs = func(_ context.Context, runner system.Runner, _ configpkg.Config, tail int, watch bool, _ string, service string) error {
			capturedTail = tail
			capturedService = service
			if _, err := io.WriteString(runner.Stdout, "partial stdout\n"); err != nil {
				return err
			}
			if _, err := io.WriteString(runner.Stderr, "partial stderr\n"); err != nil {
				return err
			}
			return errors.New("compose logs failed")
		}
	})

	logs, err := loadTUILogs(stacktui.LogRequest{Service: "pg", Tail: 50})
	if err == nil {
		t.Fatal("expected compose logs error")
	}
	if capturedTail != 50 {
		t.Fatalf("expected custom tail, got %d", capturedTail)
	}
	if capturedService != "postgres" {
		t.Fatalf("expected postgres service alias to be normalized, got %q", capturedService)
	}
	if logs.Service != "postgres" {
		t.Fatalf("expected returned snapshot service to be postgres, got %q", logs.Service)
	}
	for _, fragment := range []string{"partial stdout", "partial stderr"} {
		if !strings.Contains(logs.Output, fragment) {
			t.Fatalf("expected partial log output to contain %q, got %q", fragment, logs.Output)
		}
	}
	if !strings.Contains(err.Error(), "compose logs failed") {
		t.Fatalf("expected wrapped logs error, got %v", err)
	}
}

func TestLoadTUILogsRejectsUnknownServiceAlias(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, err := loadTUILogs(stacktui.LogRequest{Service: "unknown"})
	if err == nil {
		t.Fatal("expected invalid service alias to fail")
	}
	if !strings.Contains(err.Error(), "invalid service") {
		t.Fatalf("expected invalid service error, got %v", err)
	}
}
