package system

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
)

func TestRedisOvercommitStatus(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("expected linux host, got %s", runtime.GOOS)
	}

	status, err := RedisOvercommitStatus(context.Background())
	if err != nil {
		t.Fatalf("RedisOvercommitStatus returned error: %v", err)
	}
	if !status.Supported {
		t.Fatalf("expected linux host to report an overcommit status: %+v", status)
	}
}

func TestRedisOvercommitStatusWithDeps(t *testing.T) {
	t.Run("non-linux host", func(t *testing.T) {
		status, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"darwin",
			func(string) ([]byte, error) {
				t.Fatal("readFile should not run on non-linux")
				return nil, nil
			},
			func(string) bool {
				t.Fatal("commandExists should not run on non-linux")
				return false
			},
			func(context.Context, string, string, ...string) (CommandResult, error) {
				t.Fatal("capture should not run on non-linux")
				return CommandResult{}, nil
			},
		)
		if err != nil || status != (OvercommitStatus{}) {
			t.Fatalf("expected empty status on non-linux, got status=%+v err=%v", status, err)
		}
	})

	t.Run("proc file success", func(t *testing.T) {
		status, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return []byte("1\n"), nil },
			func(string) bool {
				t.Fatal("commandExists should not run when proc file exists")
				return false
			},
			func(context.Context, string, string, ...string) (CommandResult, error) {
				t.Fatal("capture should not run when proc file exists")
				return CommandResult{}, nil
			},
		)
		if err != nil || !status.Supported || status.Value != 1 {
			t.Fatalf("expected proc-backed status, got status=%+v err=%v", status, err)
		}
	})

	t.Run("proc parse error", func(t *testing.T) {
		_, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return []byte("nope"), nil },
			func(string) bool { return false },
			func(context.Context, string, string, ...string) (CommandResult, error) { return CommandResult{}, nil },
		)
		if err == nil {
			t.Fatal("expected proc parse error")
		}
	})

	t.Run("proc missing and sysctl unavailable", func(t *testing.T) {
		status, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return nil, os.ErrNotExist },
			func(string) bool { return false },
			func(context.Context, string, string, ...string) (CommandResult, error) {
				t.Fatal("capture should not run when sysctl is unavailable")
				return CommandResult{}, nil
			},
		)
		if err != nil || status != (OvercommitStatus{}) {
			t.Fatalf("expected empty status when sysctl is missing, got status=%+v err=%v", status, err)
		}
	})

	t.Run("sysctl capture error", func(t *testing.T) {
		expectedErr := errors.New("boom")
		_, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return nil, os.ErrNotExist },
			func(string) bool { return true },
			func(context.Context, string, string, ...string) (CommandResult, error) {
				return CommandResult{}, expectedErr
			},
		)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected capture error, got %v", err)
		}
	})

	t.Run("sysctl non-zero exit", func(t *testing.T) {
		status, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return nil, os.ErrNotExist },
			func(string) bool { return true },
			func(context.Context, string, string, ...string) (CommandResult, error) {
				return CommandResult{ExitCode: 1, Stderr: "unknown"}, nil
			},
		)
		if err != nil || status != (OvercommitStatus{}) {
			t.Fatalf("expected empty status on sysctl failure exit, got status=%+v err=%v", status, err)
		}
	})

	t.Run("sysctl parse error", func(t *testing.T) {
		_, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return nil, os.ErrNotExist },
			func(string) bool { return true },
			func(context.Context, string, string, ...string) (CommandResult, error) {
				return CommandResult{Stdout: "bad-value"}, nil
			},
		)
		if err == nil {
			t.Fatal("expected sysctl parse error")
		}
	})

	t.Run("sysctl success", func(t *testing.T) {
		status, err := redisOvercommitStatusWithDeps(
			context.Background(),
			"linux",
			func(string) ([]byte, error) { return nil, os.ErrNotExist },
			func(string) bool { return true },
			func(context.Context, string, string, ...string) (CommandResult, error) {
				return CommandResult{Stdout: "0\n"}, nil
			},
		)
		if err != nil || !status.Supported || status.Value != 0 {
			t.Fatalf("expected sysctl-backed status, got status=%+v err=%v", status, err)
		}
	})
}
