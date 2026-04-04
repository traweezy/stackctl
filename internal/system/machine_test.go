package system

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestPodmanMachineStatusWithDeps(t *testing.T) {
	t.Run("unsupported OS", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "linux", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			t.Fatal("linux should not capture podman machine status")
			return CommandResult{}, nil
		})
		if state.Supported {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("missing podman", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return false }, func(context.Context, string, string, ...string) (CommandResult, error) {
			t.Fatal("missing podman should not capture")
			return CommandResult{}, nil
		})
		if state.State != "podman unavailable" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("not initialized", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{Stdout: "[]"}, nil
		})
		if state.Initialized || state.State != "not initialized" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("running default machine", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{Stdout: `[{"Name":"dev","Default":true,"Running":true}]`}, nil
		})
		if !state.Initialized || !state.Running || state.Name != "dev" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("detection failure", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{}, errors.New("boom")
		})
		if state.State != "detection failed" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{ExitCode: 125}, nil
		})
		if state.State != "detection failed" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("invalid json output", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{Stdout: "{"}, nil
		})
		if state.State != "detection failed" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})

	t.Run("stopped default machine", func(t *testing.T) {
		state := podmanMachineStatusWithDeps(context.Background(), "darwin", func(string) bool { return true }, func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{Stdout: `[{"Name":"other","Default":false,"Running":true},{"Name":"dev","Default":true,"Running":false}]`}, nil
		})
		if !state.Initialized || state.Running || state.Name != "dev" || state.State != "stopped" {
			t.Fatalf("unexpected state: %+v", state)
		}
	})
}

func TestPodmanMachineStatusReflectsUnsupportedLinuxHost(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("expected linux host, got %s", runtime.GOOS)
	}

	state := PodmanMachineStatus(context.Background())
	if state.Supported {
		t.Fatalf("expected unsupported podman machine state on linux, got %+v", state)
	}
}

func TestPreparePodmanMachineReturnsNilWhenUnsupported(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("expected linux host, got %s", runtime.GOOS)
	}

	if err := PreparePodmanMachine(context.Background(), Runner{}); err != nil {
		t.Fatalf("PreparePodmanMachine returned error: %v", err)
	}
}

func TestPreparePodmanMachineWithDeps(t *testing.T) {
	t.Run("missing podman", func(t *testing.T) {
		err := preparePodmanMachineWithDeps(
			context.Background(),
			func(string) bool { return false },
			func(context.Context) PodmanMachineState {
				return PodmanMachineState{Supported: true}
			},
			func(context.Context, string, string, ...string) error {
				t.Fatal("run should not be called when podman is missing")
				return nil
			},
		)
		if err == nil || err.Error() != "podman is not installed" {
			t.Fatalf("expected missing podman error, got %v", err)
		}
	})

	t.Run("already running", func(t *testing.T) {
		err := preparePodmanMachineWithDeps(
			context.Background(),
			func(string) bool { return true },
			func(context.Context) PodmanMachineState {
				return PodmanMachineState{Supported: true, Initialized: true, Running: true}
			},
			func(context.Context, string, string, ...string) error {
				t.Fatal("run should not be called when machine is already running")
				return nil
			},
		)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("initializes and starts", func(t *testing.T) {
		var calls []string
		states := []PodmanMachineState{
			{Supported: true, Initialized: false},
			{Supported: true, Initialized: true, Running: false},
		}

		err := preparePodmanMachineWithDeps(
			context.Background(),
			func(string) bool { return true },
			func(context.Context) PodmanMachineState {
				state := states[0]
				states = states[1:]
				return state
			},
			func(_ context.Context, _ string, name string, args ...string) error {
				calls = append(calls, strings.Join(append([]string{name}, args...), " "))
				return nil
			},
		)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(calls) != 2 || calls[0] != "podman machine init" || calls[1] != "podman machine start" {
			t.Fatalf("unexpected command sequence: %+v", calls)
		}
	})

	t.Run("init failure", func(t *testing.T) {
		expectedErr := errors.New("init failed")
		err := preparePodmanMachineWithDeps(
			context.Background(),
			func(string) bool { return true },
			func(context.Context) PodmanMachineState {
				return PodmanMachineState{Supported: true, Initialized: false}
			},
			func(_ context.Context, _ string, _ string, args ...string) error {
				if len(args) >= 2 && args[0] == "machine" && args[1] == "init" {
					return expectedErr
				}
				t.Fatal("unexpected command")
				return nil
			},
		)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected init failure, got %v", err)
		}
	})

	t.Run("still not initialized after init", func(t *testing.T) {
		states := []PodmanMachineState{
			{Supported: true, Initialized: false},
			{Supported: true, Initialized: false},
		}
		err := preparePodmanMachineWithDeps(
			context.Background(),
			func(string) bool { return true },
			func(context.Context) PodmanMachineState {
				state := states[0]
				states = states[1:]
				return state
			},
			func(context.Context, string, string, ...string) error { return nil },
		)
		if err == nil || err.Error() != "podman machine is still not initialized" {
			t.Fatalf("expected recheck failure, got %v", err)
		}
	})

	t.Run("start failure", func(t *testing.T) {
		expectedErr := errors.New("start failed")
		states := []PodmanMachineState{
			{Supported: true, Initialized: true, Running: false},
			{Supported: true, Initialized: true, Running: false},
		}
		err := preparePodmanMachineWithDeps(
			context.Background(),
			func(string) bool { return true },
			func(context.Context) PodmanMachineState {
				state := states[0]
				states = states[1:]
				return state
			},
			func(_ context.Context, _ string, _ string, args ...string) error {
				if len(args) >= 2 && args[0] == "machine" && args[1] == "start" {
					return expectedErr
				}
				return nil
			},
		)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected start failure, got %v", err)
		}
	})
}
