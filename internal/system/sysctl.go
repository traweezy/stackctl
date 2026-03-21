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
	if runtime.GOOS != "linux" {
		return OvercommitStatus{}, nil
	}

	if data, err := os.ReadFile("/proc/sys/vm/overcommit_memory"); err == nil {
		value, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr != nil {
			return OvercommitStatus{}, parseErr
		}
		return OvercommitStatus{Supported: true, Value: value}, nil
	}

	if !CommandExists("sysctl") {
		return OvercommitStatus{}, nil
	}

	result, err := CaptureResult(ctx, "", "sysctl", "-n", "vm.overcommit_memory")
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
