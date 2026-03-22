package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
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
	if len(snapshot.Health) == 0 {
		t.Fatalf("expected health lines in snapshot")
	}
	if len(snapshot.Connections) != 4 {
		t.Fatalf("expected 4 connection entries, got %d", len(snapshot.Connections))
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
