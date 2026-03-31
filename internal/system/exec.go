package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Runner struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (r Runner) Run(ctx context.Context, dir, name string, args ...string) error {
	if err := validateExecutable(name); err != nil {
		return err
	}

	// #nosec G204 -- executable names are restricted to an internal allowlist.
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdin = r.Stdin
	command.Stdout = r.Stdout
	command.Stderr = r.Stderr
	if len(r.Env) > 0 {
		command.Env = mergeEnv(os.Environ(), r.Env)
	}

	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", formatCommand(name, args), err)
	}

	return nil
}

func RunExternalCommand(ctx context.Context, runner Runner, dir string, commandArgs []string) error {
	if len(commandArgs) == 0 {
		return errors.New("no command specified")
	}

	// #nosec G204 -- the command is supplied explicitly by the local CLI user.
	command := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...)
	command.Dir = dir
	command.Stdin = runner.Stdin
	command.Stdout = runner.Stdout
	command.Stderr = runner.Stderr
	if len(runner.Env) > 0 {
		command.Env = mergeEnv(os.Environ(), runner.Env)
	}

	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", formatCommand(commandArgs[0], commandArgs[1:]), err)
	}

	return nil
}

func (r Runner) Capture(ctx context.Context, dir, name string, args ...string) (string, error) {
	result, err := CaptureResult(ctx, dir, name, args...)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		if detail != "" {
			return "", fmt.Errorf("run %s: %s", formatCommand(name, args), detail)
		}
		return "", fmt.Errorf("run %s exited with code %d", formatCommand(name, args), result.ExitCode)
	}

	return result.Stdout, nil
}

func CaptureResult(ctx context.Context, dir, name string, args ...string) (CommandResult, error) {
	return CaptureResultWithEnv(ctx, dir, nil, name, args...)
}

func CaptureResultWithEnv(ctx context.Context, dir string, env []string, name string, args ...string) (CommandResult, error) {
	if err := validateExecutable(name); err != nil {
		return CommandResult{}, err
	}

	// #nosec G204 -- executable names are restricted to an internal allowlist.
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	if len(env) > 0 {
		command.Env = mergeEnv(os.Environ(), env)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		result := CommandResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return result, fmt.Errorf("run %s: %w", formatCommand(name, args), err)
	}

	return CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, nil
}

func formatCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}

func mergeEnv(base, overrides []string) []string {
	if len(overrides) == 0 {
		return append([]string(nil), base...)
	}

	merged := make([]string, 0, len(base)+len(overrides))
	replaced := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		replaced[key] = struct{}{}
	}

	for _, entry := range base {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		if _, ok := replaced[key]; ok {
			continue
		}
		merged = append(merged, entry)
	}

	merged = append(merged, overrides...)
	return merged
}

func validateExecutable(name string) error {
	if _, ok := allowedExecutables[name]; ok {
		return nil
	}

	return fmt.Errorf("unsupported executable %q", name)
}

var allowedExecutables = map[string]struct{}{
	"apk":            {},
	"apt-get":        {},
	"brew":           {},
	"dnf":            {},
	"open":           {},
	"pacman":         {},
	"pbcopy":         {},
	"podman":         {},
	"podman-compose": {},
	"sudo":           {},
	"systemctl":      {},
	"sysctl":         {},
	"wl-copy":        {},
	"xclip":          {},
	"xsel":           {},
	"xdg-open":       {},
	"yum":            {},
	"zypper":         {},
}
