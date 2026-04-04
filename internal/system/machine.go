package system

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

type PodmanMachineState struct {
	Supported   bool
	Initialized bool
	Running     bool
	Name        string
	State       string
}

type podmanMachineListEntry struct {
	Name    string `json:"Name"`
	Default bool   `json:"Default"`
	Running bool   `json:"Running"`
}

func PodmanMachineStatus(ctx context.Context) PodmanMachineState {
	return podmanMachineStatusWithDeps(ctx, runtime.GOOS, CommandExists, CaptureResult)
}

func podmanMachineStatusWithDeps(
	ctx context.Context,
	goos string,
	commandExists func(string) bool,
	capture func(context.Context, string, string, ...string) (CommandResult, error),
) PodmanMachineState {
	if goos != "darwin" {
		return PodmanMachineState{State: "not supported"}
	}

	state := PodmanMachineState{Supported: true}
	if !commandExists("podman") {
		state.State = "podman unavailable"
		return state
	}

	result, err := capture(ctx, "", "podman", "machine", "list", "--format", "json")
	if err != nil {
		state.State = "detection failed"
		return state
	}
	if result.ExitCode != 0 {
		state.State = "detection failed"
		return state
	}

	var entries []podmanMachineListEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &entries); err != nil {
		state.State = "detection failed"
		return state
	}
	if len(entries) == 0 {
		state.State = "not initialized"
		return state
	}

	selected := entries[0]
	for _, entry := range entries {
		if entry.Default {
			selected = entry
			break
		}
	}

	state.Initialized = true
	state.Running = selected.Running
	state.Name = selected.Name
	if state.Running {
		state.State = "running"
	} else {
		state.State = "stopped"
	}

	return state
}

func PreparePodmanMachine(ctx context.Context, runner Runner) error {
	return preparePodmanMachineWithDeps(
		ctx,
		CommandExists,
		PodmanMachineStatus,
		func(ctx context.Context, dir, name string, args ...string) error {
			return runner.Run(ctx, dir, name, args...)
		},
	)
}

func preparePodmanMachineWithDeps(
	ctx context.Context,
	commandExists func(string) bool,
	status func(context.Context) PodmanMachineState,
	run func(context.Context, string, string, ...string) error,
) error {
	state := status(ctx)
	if !state.Supported {
		return nil
	}
	if !commandExists("podman") {
		return fmt.Errorf("podman is not installed")
	}
	if !state.Initialized {
		if err := run(ctx, "", "podman", "machine", "init"); err != nil {
			return err
		}
	}

	updated := status(ctx)
	if !updated.Initialized {
		return fmt.Errorf("podman machine is still not initialized")
	}
	if updated.Running {
		return nil
	}

	return run(ctx, "", "podman", "machine", "start")
}
