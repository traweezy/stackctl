package system

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type OvercommitStatus struct {
	Supported bool
	Value     int
}

func RedisOvercommitStatus(ctx context.Context) (OvercommitStatus, error) {
	return redisOvercommitStatusWithDeps(ctx, runtime.GOOS, os.ReadFile, CommandExists, CaptureResult)
}

func redisOvercommitStatusWithDeps(
	ctx context.Context,
	goos string,
	readFile func(string) ([]byte, error),
	commandExists func(string) bool,
	capture func(context.Context, string, string, ...string) (CommandResult, error),
) (OvercommitStatus, error) {
	if goos != "linux" {
		return OvercommitStatus{}, nil
	}

	if data, err := readFile("/proc/sys/vm/overcommit_memory"); err == nil {
		value, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr != nil {
			return OvercommitStatus{}, parseErr
		}
		return OvercommitStatus{Supported: true, Value: value}, nil
	}

	if !commandExists("sysctl") {
		return OvercommitStatus{}, nil
	}

	result, err := capture(ctx, "", "sysctl", "-n", "vm.overcommit_memory")
	if err != nil {
		return OvercommitStatus{}, err
	}
	if result.ExitCode != 0 {
		return OvercommitStatus{}, nil
	}

	value, parseErr := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if parseErr != nil {
		return OvercommitStatus{}, parseErr
	}

	return OvercommitStatus{Supported: true, Value: value}, nil
}
