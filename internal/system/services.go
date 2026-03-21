package system

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func OpenCommandName() string {
	switch runtime.GOOS {
	case "darwin":
		if CommandExists("open") {
			return "open"
		}
	case "linux":
		if CommandExists("xdg-open") {
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

	result, err := CaptureResult(ctx, "", "podman", "compose", "version")
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

func DetectPackageManager() string {
	if CommandExists("apt-get") {
		return "apt"
	}

	return ""
}

func InstallPackages(ctx context.Context, runner Runner, packageManager string, packages []string) ([]string, error) {
	if len(packages) == 0 {
		return nil, nil
	}

	switch packageManager {
	case "apt":
		if err := runner.Run(ctx, "", "sudo", "apt-get", "update"); err != nil {
			return nil, err
		}

		args := append([]string{"apt-get", "install", "-y"}, packages...)
		if err := runner.Run(ctx, "", "sudo", args...); err != nil {
			return nil, err
		}

		return packages, nil
	default:
		return nil, fmt.Errorf("unsupported package manager %q; install manually: %s", packageManager, strings.Join(packages, ", "))
	}
}

func EnableCockpit(ctx context.Context, runner Runner) error {
	return runner.Run(ctx, "", "sudo", "systemctl", "enable", "--now", "cockpit.socket")
}
