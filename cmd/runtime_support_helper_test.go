package cmd

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDefaultTerminalInteractiveReturnsFalseForRegularFiles(t *testing.T) {
	stdinFile, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatalf("CreateTemp stdin returned error: %v", err)
	}
	defer func() { _ = stdinFile.Close() }()

	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("CreateTemp stdout returned error: %v", err)
	}
	defer func() { _ = stdoutFile.Close() }()

	originalStdin := os.Stdin
	originalStdout := os.Stdout
	os.Stdin = stdinFile
	os.Stdout = stdoutFile
	defer func() {
		os.Stdin = originalStdin
		os.Stdout = originalStdout
	}()

	if defaultTerminalInteractive() {
		t.Fatal("expected regular files to be treated as non-interactive")
	}
}

func TestSyncManagedScaffoldIfNeededForConfig(t *testing.T) {
	t.Run("skips when stack is unmanaged", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Stack.Managed = false

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
				t.Fatal("managedStackNeedsScaffold should not run for unmanaged stacks")
				return false, nil
			}
		})

		if err := syncManagedScaffoldIfNeededForConfig(cfg); err != nil {
			t.Fatalf("syncManagedScaffoldIfNeededForConfig returned error: %v", err)
		}
	})

	t.Run("skips when default scaffold support is disabled", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.ScaffoldDefaultStack = false

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
				t.Fatal("managedStackNeedsScaffold should not run when scaffold sync is disabled")
				return false, nil
			}
		})

		if err := syncManagedScaffoldIfNeededForConfig(cfg); err != nil {
			t.Fatalf("syncManagedScaffoldIfNeededForConfig returned error: %v", err)
		}
	})

	t.Run("propagates scaffold detection errors", func(t *testing.T) {
		cfg := configpkg.Default()
		want := errors.New("scaffold check failed")

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
				return false, want
			}
		})

		err := syncManagedScaffoldIfNeededForConfig(cfg)
		if !errors.Is(err, want) {
			t.Fatalf("expected scaffold detection error %v, got %v", want, err)
		}
	})

	t.Run("returns nil when scaffold is already current", func(t *testing.T) {
		cfg := configpkg.Default()
		scaffoldCalled := false

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, nil }
			d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				scaffoldCalled = true
				return configpkg.ScaffoldResult{}, nil
			}
		})

		if err := syncManagedScaffoldIfNeededForConfig(cfg); err != nil {
			t.Fatalf("syncManagedScaffoldIfNeededForConfig returned error: %v", err)
		}
		if scaffoldCalled {
			t.Fatal("scaffoldManagedStack should not run when no refresh is needed")
		}
	})

	t.Run("propagates scaffold refresh failures", func(t *testing.T) {
		cfg := configpkg.Default()
		want := errors.New("scaffold failed")

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{}, want
			}
		})

		err := syncManagedScaffoldIfNeededForConfig(cfg)
		if !errors.Is(err, want) {
			t.Fatalf("expected scaffold error %v, got %v", want, err)
		}
	})

	t.Run("forces scaffold refresh when needed", func(t *testing.T) {
		cfg := configpkg.Default()
		called := false

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.scaffoldManagedStack = func(got configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
				called = true
				if !force {
					t.Fatal("expected scaffold sync to force-refresh managed files")
				}
				if got.Stack.Name != cfg.Stack.Name {
					t.Fatalf("unexpected scaffold config: %+v", got)
				}
				return configpkg.ScaffoldResult{}, nil
			}
		})

		if err := syncManagedScaffoldIfNeededForConfig(cfg); err != nil {
			t.Fatalf("syncManagedScaffoldIfNeededForConfig returned error: %v", err)
		}
		if !called {
			t.Fatal("expected scaffoldManagedStack to run")
		}
	})
}

func TestEnsurePodmanRuntimeReadyAdditionalBranches(t *testing.T) {
	t.Run("ignores version lookup errors on linux", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.commandExists = func(string) bool { return true }
			d.podmanVersion = func(context.Context) (string, error) { return "", errors.New("version unavailable") }
			d.platform = func() system.Platform {
				return system.Platform{
					GOOS:           "linux",
					PackageManager: "apt",
					ServiceManager: system.ServiceManagerSystemd,
				}
			}
		})

		if err := ensurePodmanRuntimeReady(); err != nil {
			t.Fatalf("ensurePodmanRuntimeReady returned error: %v", err)
		}
	})

	t.Run("requires podman machine to be running on darwin", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.commandExists = func(string) bool { return true }
			d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
			d.podmanMachineStatus = func(context.Context) system.PodmanMachineState {
				return system.PodmanMachineState{Supported: true, Initialized: true, Running: false}
			}
		})

		err := ensurePodmanRuntimeReady()
		if err == nil || !strings.Contains(err.Error(), "podman machine is not running") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestInspectHostPortBranches(t *testing.T) {
	t.Run("zero port", func(t *testing.T) {
		if state := inspectHostPort(0); state != (stackServicePortState{}) {
			t.Fatalf("unexpected zero-port state: %+v", state)
		}
	})

	t.Run("listening port", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.portListening = func(int) bool { return true }
			d.portInUse = func(int) (bool, error) {
				t.Fatal("portInUse should not run when the port is already listening")
				return false, nil
			}
		})

		state := inspectHostPort(5432)
		if !state.Listening || state.Conflict || state.CheckErr != nil {
			t.Fatalf("unexpected listening state: %+v", state)
		}
	})

	t.Run("conflicting port", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.portListening = func(int) bool { return false }
			d.portInUse = func(int) (bool, error) { return true, nil }
		})

		state := inspectHostPort(5432)
		if state.Listening || !state.Conflict || state.CheckErr != nil {
			t.Fatalf("unexpected conflict state: %+v", state)
		}
	})

	t.Run("port check failure", func(t *testing.T) {
		want := errors.New("ss failed")

		withTestDeps(t, func(d *commandDeps) {
			d.portListening = func(int) bool { return false }
			d.portInUse = func(int) (bool, error) { return false, want }
		})

		state := inspectHostPort(5432)
		if !errors.Is(state.CheckErr, want) {
			t.Fatalf("expected check error %v, got %+v", want, state)
		}
	})
}

func TestSelectedConnectionEntriesFiltersUnknownTargets(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	all := selectedConnectionEntries(cfg, nil)
	if !reflect.DeepEqual(all, connectionEntries(cfg)) {
		t.Fatalf("expected nil selection to return all connection entries")
	}

	postgresDefinition, ok := serviceDefinitionByKey("postgres")
	if !ok {
		t.Fatal("expected postgres definition")
	}
	redisDefinition, ok := serviceDefinitionByKey("redis")
	if !ok {
		t.Fatal("expected redis definition")
	}

	want := append([]connectionEntry{}, postgresDefinition.ConnectionEntries(cfg)...)
	want = append(want, redisDefinition.ConnectionEntries(cfg)...)

	got := selectedConnectionEntries(cfg, []string{"postgres", "bogus", "redis"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected connection entries:\nwant: %+v\ngot:  %+v", want, got)
	}
}
