package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunDryRunPrintsSelectedServicesCommandAndEnv(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "run", "--dry-run", "--", "go", "test", "./...")
	if err != nil {
		t.Fatalf("run --dry-run returned error: %v", err)
	}

	for _, fragment := range []string{
		"Stack: dev-stack",
		"Services: Postgres, Redis, NATS, pgAdmin",
		"Command: 'go' 'test' './...'",
		"export DATABASE_URL='postgres://app:app@devbox:5432/app'",
		"export PGMAINTENANCE_DB='postgres'",
		"export REDIS_URL='redis://devbox:6379'",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("expected dry-run output to contain %q:\n%s", fragment, stdout)
		}
	}
}

func TestRunStartsSelectedServicesAndExecutesHostCommand(t *testing.T) {
	var (
		startedServices []string
		commandArgs     []string
		envVars         []string
		started         bool
	)

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ bool, services []string) error {
			started = true
			startedServices = append([]string(nil), services...)
			return nil
		}
		d.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
			if started {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
		d.runExternalCommand = func(_ context.Context, runner system.Runner, _ string, command []string) error {
			commandArgs = append([]string(nil), command...)
			envVars = append([]string(nil), runner.Env...)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "--verbose", "run", "postgres", "--", "app", "serve")
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !slices.Equal(startedServices, []string{"postgres"}) {
		t.Fatalf("unexpected started services: %+v", startedServices)
	}
	if !slices.Equal(commandArgs, []string{"app", "serve"}) {
		t.Fatalf("unexpected command args: %+v", commandArgs)
	}
	joinedEnv := strings.Join(envVars, "\n")
	for _, fragment := range []string{
		"STACKCTL_STACK=dev-stack",
		"DATABASE_URL=postgres://app:app@devbox:5432/app",
		"PGMAINTENANCE_DB=postgres",
	} {
		if !strings.Contains(joinedEnv, fragment) {
			t.Fatalf("expected runner env to contain %q:\n%s", fragment, joinedEnv)
		}
	}
	if !strings.Contains(stdout, "Using compose file /tmp/stackctl/compose.yaml") {
		t.Fatalf("expected verbose run output to include compose detail:\n%s", stdout)
	}
}

func TestRunNoStartRequiresReadyServices(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	_, _, err := executeRoot(t, "run", "--no-start", "postgres", "--", "echo", "hi")
	if err == nil || !strings.Contains(err.Error(), "postgres is not ready") {
		t.Fatalf("unexpected no-start error: %v", err)
	}
}

func TestRunDryRunDoesNotScaffoldManagedStack(t *testing.T) {
	var scaffolded bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{
				{Field: "stack.dir", Message: fmt.Sprintf("directory does not exist: %s", cfg.Stack.Dir)},
				{Field: "stack.compose_file", Message: fmt.Sprintf("file does not exist: %s", configpkg.ComposePath(cfg))},
			}
		}
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, _ bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{
				StackDir:    cfg.Stack.Dir,
				ComposePath: configpkg.ComposePath(cfg),
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "run", "--dry-run", "--", "echo", "hi")
	if err != nil {
		t.Fatalf("run --dry-run returned error: %v", err)
	}
	if scaffolded {
		t.Fatal("run --dry-run should not scaffold managed stack files")
	}
	if !strings.Contains(stdout, "Command: 'echo' 'hi'") {
		t.Fatalf("unexpected dry-run output: %s", stdout)
	}
}

func TestRunNoStartDoesNotRequireComposeRuntimeWhenServiceReady(t *testing.T) {
	var ran bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.commandExists = func(string) bool { return false }
		d.podmanComposeAvail = func(context.Context) bool { return false }
		d.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		d.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
		}
		d.runExternalCommand = func(_ context.Context, runner system.Runner, _ string, command []string) error {
			ran = true
			if !slices.Equal(command, []string{"echo", "hi"}) {
				t.Fatalf("unexpected command args: %+v", command)
			}
			joinedEnv := strings.Join(runner.Env, "\n")
			if !strings.Contains(joinedEnv, "DATABASE_URL=postgres://app:app@devbox:5432/app") {
				t.Fatalf("expected runner env to include postgres dsn:\n%s", joinedEnv)
			}
			return nil
		}
	})

	_, _, err := executeRoot(t, "run", "--no-start", "postgres", "--", "echo", "hi")
	if err != nil {
		t.Fatalf("run --no-start returned error: %v", err)
	}
	if !ran {
		t.Fatal("expected run --no-start to execute the host command")
	}
}
