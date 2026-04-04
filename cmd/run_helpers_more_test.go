package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunCommandUsageAndDefaultServiceSelection(t *testing.T) {
	t.Run("requires command separator", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "run", "postgres", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "usage: stackctl run [service...] -- <command...>") {
			t.Fatalf("unexpected missing-separator error: %v", err)
		}
	})

	t.Run("requires command after separator", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "run", "postgres", "--")
		if err == nil || !strings.Contains(err.Error(), "usage: stackctl run [service...] -- <command...>") {
			t.Fatalf("unexpected missing-command error: %v", err)
		}
	})

	t.Run("dry-run defaults to every enabled stack service", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeSeaweedFS = true
			cfg.Setup.IncludePgAdmin = false
			cfg.Connection.Host = "devbox"
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		stdout, _, err := executeRoot(t, "run", "--dry-run", "--", "echo", "hi")
		if err != nil {
			t.Fatalf("run --dry-run returned error: %v", err)
		}
		if !strings.Contains(stdout, "Services: Postgres, Redis, NATS, SeaweedFS") {
			t.Fatalf("expected enabled service list in dry-run output:\n%s", stdout)
		}
		if strings.Contains(stdout, "pgAdmin") {
			t.Fatalf("did not expect disabled pgAdmin in dry-run output:\n%s", stdout)
		}
	})
}

func TestParseRunInvocationAndServiceResolutionHelpers(t *testing.T) {
	t.Run("parse invocation with service args and command args", func(t *testing.T) {
		cmd := &cobra.Command{Use: "run"}
		if err := cmd.ParseFlags([]string{"postgres", "redis", "--", "go", "test", "./..."}); err != nil {
			t.Fatalf("ParseFlags returned error: %v", err)
		}

		serviceArgs, commandArgs, err := parseRunInvocation(cmd, cmd.Flags().Args())
		if err != nil {
			t.Fatalf("parseRunInvocation returned error: %v", err)
		}
		if !slices.Equal(serviceArgs, []string{"postgres", "redis"}) {
			t.Fatalf("unexpected service args: %+v", serviceArgs)
		}
		if !slices.Equal(commandArgs, []string{"go", "test", "./..."}) {
			t.Fatalf("unexpected command args: %+v", commandArgs)
		}
	})

	t.Run("resolve helpers normalize aliases and deduplicate services", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()

		keys := enabledStackServiceKeys(cfg)
		for _, service := range []string{"postgres", "redis", "nats", "seaweedfs"} {
			if !slices.Contains(keys, service) {
				t.Fatalf("expected enabled services to contain %q: %+v", service, keys)
			}
		}
		if slices.Contains(keys, "pgadmin") {
			t.Fatalf("did not expect disabled pgadmin in %+v", keys)
		}

		services, err := resolveRunTargetServices(cfg, []string{"pg", "postgres", "rd"})
		if err != nil {
			t.Fatalf("resolveRunTargetServices returned error: %v", err)
		}
		if !slices.Equal(services, []string{"postgres", "redis"}) {
			t.Fatalf("unexpected resolved services: %+v", services)
		}

		services, err = resolveRunTargetServices(cfg, nil)
		if err != nil {
			t.Fatalf("resolveRunTargetServices returned error for default selection: %v", err)
		}
		if !slices.Equal(services, keys) {
			t.Fatalf("expected default selection %+v, got %+v", keys, services)
		}
	})

	t.Run("disabled services fail resolution", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()

		_, err := resolveRunTargetServices(cfg, []string{"pgadmin"})
		if err == nil || !strings.Contains(err.Error(), "pgadmin is not enabled in this stack") {
			t.Fatalf("unexpected disabled-service error: %v", err)
		}
	})
}

func TestEnsureSelectedRunServicesReadyBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("ready services pass", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
		})

		if err := ensureSelectedRunServicesReady(context.Background(), cfg, []string{"postgres"}); err != nil {
			t.Fatalf("ensureSelectedRunServicesReady returned error: %v", err)
		}
	})

	t.Run("port check errors surface", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
			d.portInUse = func(int) (bool, error) { return false, errors.New("probe failed") }
		})

		err := ensureSelectedRunServicesReady(context.Background(), cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "port 5432 check failed: probe failed") {
			t.Fatalf("unexpected port-check error: %v", err)
		}
	})

	t.Run("port conflicts report a bind failure", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
			d.portInUse = func(int) (bool, error) { return true, nil }
		})

		err := ensureSelectedRunServicesReady(context.Background(), cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "postgres could not bind host port 5432") {
			t.Fatalf("unexpected port-conflict error: %v", err)
		}
	})

	t.Run("terminal container states fail fast", func(t *testing.T) {
		containerJSON := marshalContainersJSON(system.Container{
			ID:        "postgres123456",
			Image:     "postgres:latest",
			Names:     []string{cfg.Services.PostgresContainer},
			Status:    "Exited (1) 10 seconds ago",
			State:     "exited",
			CreatedAt: "now",
		})

		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: containerJSON}, nil
			}
		})

		err := ensureSelectedRunServicesReady(context.Background(), cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "postgres container failed to start (Exited (1) 10 seconds ago)") {
			t.Fatalf("unexpected terminal-container error: %v", err)
		}
	})

	t.Run("running services must still listen on the host port", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.portListening = func(int) bool { return false }
		})

		err := ensureSelectedRunServicesReady(context.Background(), cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "postgres is not ready: port 5432 is not listening yet") {
			t.Fatalf("unexpected not-listening error: %v", err)
		}
	})
}

func TestRunFormattingHelpers(t *testing.T) {
	command := formatShellCommand([]string{"air", "--build.cmd", "go test ./...", "it's"})
	if command != "'air' '--build.cmd' 'go test ./...' 'it'\"'\"'s'" {
		t.Fatalf("unexpected shell command formatting: %q", command)
	}

	assignments := mapEnvToAssignments(map[string]string{
		"REDIS_URL":    "redis://cache",
		"DATABASE_URL": "postgres://app",
		"APP_ENV":      "dev",
	})
	if !slices.Equal(assignments, []string{
		"APP_ENV=dev",
		"DATABASE_URL=postgres://app",
		"REDIS_URL=redis://cache",
	}) {
		t.Fatalf("unexpected env assignments: %+v", assignments)
	}
}
