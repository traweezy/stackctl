package system

import (
	"context"
	"errors"
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
}
