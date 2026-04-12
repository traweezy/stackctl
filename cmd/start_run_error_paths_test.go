package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestStartQuietSuppressesConnectionInfo(t *testing.T) {
	var composeUpCalled bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			composeUpCalled = true
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	})

	stdout, _, err := executeRoot(t, "--quiet", "start")
	if err != nil {
		t.Fatalf("start --quiet returned error: %v", err)
	}
	if !composeUpCalled {
		t.Fatal("expected start --quiet to run compose up")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected quiet start output to be empty, got:\n%s", stdout)
	}
}

func TestStartPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			t.Fatal("composeUp should not run when start status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"start"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected start status write failure, got %v", err)
	}
}

func TestRunPropagatesExternalCommandErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
		}
		d.runExternalCommand = func(context.Context, system.Runner, string, []string) error {
			return errors.New("host command failed")
		}
	})

	_, _, err := executeRoot(t, "run", "--no-start", "postgres", "--", "echo", "hi")
	if err == nil || !strings.Contains(err.Error(), "host command failed") {
		t.Fatalf("expected host command failure, got %v", err)
	}
}

func TestRunPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			t.Fatal("composeUp should not run when run status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"run", "postgres", "--", "echo", "hi"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected run status write failure, got %v", err)
	}
}
