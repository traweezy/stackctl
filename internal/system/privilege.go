package system

import (
	"context"
	"fmt"
	"os"
)

var currentEUID = os.Geteuid

func privilegeCommand(command string, args ...string) (string, []string, error) {
	return privilegeCommandWithDeps(currentEUID, CommandExists, command, args...)
}

func privilegeCommandWithDeps(
	geteuid func() int,
	commandExists func(string) bool,
	command string,
	args ...string,
) (string, []string, error) {
	if geteuid() == 0 {
		return command, append([]string(nil), args...), nil
	}
	if commandExists("sudo") {
		return "sudo", append([]string{command}, args...), nil
	}

	return "", nil, fmt.Errorf(
		"automatic privileged operations require root or passwordless sudo, but sudo is not installed",
	)
}

func runPrivileged(ctx context.Context, runner Runner, command string, args ...string) error {
	name, commandArgs, err := privilegeCommand(command, args...)
	if err != nil {
		return err
	}

	return runner.Run(ctx, "", name, commandArgs...)
}
