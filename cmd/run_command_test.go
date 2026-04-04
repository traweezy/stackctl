package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

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
		d.commandExists = func(name string) bool { return name == "podman" }
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

func TestStartRunServicesChoosesComposeMode(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("empty selection starts the full stack", func(t *testing.T) {
		var upCalled bool

		withTestDeps(t, func(d *commandDeps) {
			d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
				upCalled = true
				return nil
			}
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				t.Fatal("composeUpServices should not run for the full stack")
				return nil
			}
		})

		if err := startRunServices(NewRootCmd(NewApp()), cfg, nil); err != nil {
			t.Fatalf("startRunServices returned error: %v", err)
		}
		if !upCalled {
			t.Fatal("expected composeUp to run for the full stack")
		}
	})

	t.Run("full enabled selection starts the full stack", func(t *testing.T) {
		var upCalled bool

		withTestDeps(t, func(d *commandDeps) {
			d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
				upCalled = true
				return nil
			}
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				t.Fatal("composeUpServices should not run when all enabled services are requested")
				return nil
			}
		})

		if err := startRunServices(NewRootCmd(NewApp()), cfg, enabledStackServiceKeys(cfg)); err != nil {
			t.Fatalf("startRunServices returned error: %v", err)
		}
		if !upCalled {
			t.Fatal("expected composeUp to run when all enabled services are selected")
		}
	})

	t.Run("subset starts only selected services", func(t *testing.T) {
		var captured []string

		withTestDeps(t, func(d *commandDeps) {
			d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
				t.Fatal("composeUp should not run for a subset")
				return nil
			}
			d.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, forceRecreate bool, services []string) error {
				if forceRecreate {
					t.Fatal("run helper should not force recreate services")
				}
				captured = append([]string(nil), services...)
				return nil
			}
		})

		if err := startRunServices(NewRootCmd(NewApp()), cfg, []string{"postgres"}); err != nil {
			t.Fatalf("startRunServices returned error: %v", err)
		}
		if !slices.Equal(captured, []string{"postgres"}) {
			t.Fatalf("unexpected composeUpServices targets: %+v", captured)
		}
	})
}

func TestWaitForRunServicesWaitsForSelectedPorts(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("waits for ready selected services", func(t *testing.T) {
		waited := make([]int, 0, 2)

		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres", "redis")}, nil
			}
			d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
				waited = append(waited, port)
				return nil
			}
		})

		if err := waitForRunServices(context.Background(), cfg, []string{"postgres", "redis"}); err != nil {
			t.Fatalf("waitForRunServices returned error: %v", err)
		}
		if !slices.Equal(waited, []int{5432, 6379}) {
			t.Fatalf("unexpected waited ports: %+v", waited)
		}
	})

	t.Run("surfaces wait failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.waitForPort = func(context.Context, int, time.Duration) error {
				return errors.New("timed out")
			}
		})

		err := waitForRunServices(context.Background(), cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "postgres port 5432 did not become ready") {
			t.Fatalf("unexpected wait error: %v", err)
		}
	})
}

func TestPrintRunDryRunFormatsNoStartMode(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Stack.Name = "qa"

	root := NewRootCmd(NewApp())
	var stdout strings.Builder
	root.SetOut(&stdout)

	err := printRunDryRun(root, cfg, []string{"postgres"}, []string{"go", "test", "./..."}, []envGroup{
		{
			Title: "Postgres",
			Entries: []envEntry{
				{Name: "DATABASE_URL", Value: "postgres://qa"},
			},
		},
	}, true)
	if err != nil {
		t.Fatalf("printRunDryRun returned error: %v", err)
	}

	output := stdout.String()
	for _, fragment := range []string{
		"Stack: qa",
		"Services: Postgres",
		"Service mode: require already running",
		"Command: 'go' 'test' './...'",
		"# Postgres",
		"export DATABASE_URL='postgres://qa'",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected dry-run output to contain %q:\n%s", fragment, output)
		}
	}
}
