package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func OpenCommandName() string {
	return openCommandNameFor(runtime.GOOS, CommandExists)
}

func openCommandNameFor(goos string, commandExists func(string) bool) string {
	switch goos {
	case "darwin":
		if commandExists("open") {
			return "open"
		}
	case "linux":
		if commandExists("xdg-open") {
			return "xdg-open"
		}
	}

	return ""
}

func OpenURL(ctx context.Context, runner Runner, target string) error {
	opener := OpenCommandName()
	if opener == "" {
		return fmt.Errorf("no supported browser opener found on %s", runtime.GOOS)
	}

	return runner.Run(ctx, "", opener, target)
}

type CockpitState struct {
	Installed bool
	Active    bool
	State     string
}

func CockpitStatus(ctx context.Context) CockpitState {
	if !CommandExists("systemctl") {
		return CockpitState{State: "systemctl unavailable"}
	}

	result, err := CaptureResult(ctx, "", "systemctl", "list-unit-files", "cockpit.socket", "--no-legend", "--plain")
	if err != nil {
		return CockpitState{State: "detection failed"}
	}

	output := strings.TrimSpace(result.Stdout)
	if output == "" || strings.Contains(output, "0 unit files listed") {
		return CockpitState{State: "not installed"}
	}

	activeResult, err := CaptureResult(ctx, "", "systemctl", "is-active", "cockpit.socket")
	if err != nil {
		return CockpitState{Installed: true, State: "state unknown"}
	}

	state := strings.TrimSpace(activeResult.Stdout)
	if state == "active" && activeResult.ExitCode == 0 {
		return CockpitState{Installed: true, Active: true, State: "active"}
	}
	if state == "" {
		state = strings.TrimSpace(activeResult.Stderr)
	}
	if state == "" {
		state = "inactive"
	}

	return CockpitState{Installed: true, State: state}
}

func PodmanComposeAvailable(ctx context.Context) bool {
	if !CommandExists("podman") {
		return false
	}

	env := []string(nil)
	if strings.TrimSpace(os.Getenv("PODMAN_COMPOSE_PROVIDER")) == "" && CommandExists("podman-compose") {
		env = []string{"PODMAN_COMPOSE_PROVIDER=podman-compose"}
	}

	result, err := CaptureResultWithEnv(ctx, "", env, "podman", "compose", "version")
	if err != nil {
		return false
	}

	return result.ExitCode == 0
}

func AnyContainerExists(ctx context.Context, containerNames []string) (bool, error) {
	if !CommandExists("podman") {
		return false, nil
	}

	for _, name := range containerNames {
		if strings.TrimSpace(name) == "" {
			continue
		}

		result, err := CaptureResult(ctx, "", "podman", "container", "exists", name)
		if err != nil {
			return false, err
		}
		if result.ExitCode == 0 {
			return true, nil
		}
	}

	return false, nil
}

func EnableCockpit(ctx context.Context, runner Runner) error {
	return runPrivileged(ctx, runner, "systemctl", "enable", "--now", "cockpit.socket")
}
