package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Runner struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
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

	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", formatCommand(name, args), err)
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
	if err := validateExecutable(name); err != nil {
		return CommandResult{}, err
	}

	// #nosec G204 -- executable names are restricted to an internal allowlist.
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir

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

func validateExecutable(name string) error {
	if _, ok := allowedExecutables[name]; ok {
		return nil
	}

	return fmt.Errorf("unsupported executable %q", name)
}

var allowedExecutables = map[string]struct{}{
	"open":      {},
	"podman":    {},
	"sudo":      {},
	"systemctl": {},
	"sysctl":    {},
	"xdg-open":  {},
}
