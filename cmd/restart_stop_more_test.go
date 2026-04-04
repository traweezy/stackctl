package cmd

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRestartQuietFullStackUsesVerificationModeAndSuppressesOutput(t *testing.T) {
	var downCalled bool
	var upCalled bool
	var waitForPortCalled bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Behavior.WaitForServicesStart = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			downCalled = true
			return nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			upCalled = true
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.portListening = func(port int) bool {
			return port == cfg.Ports.Postgres || port == cfg.Ports.Redis || port == cfg.Ports.NATS || port == cfg.Ports.PgAdmin || port == cfg.Ports.Cockpit
		}
		d.waitForPort = func(context.Context, int, time.Duration) error {
			waitForPortCalled = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "--quiet", "restart")
	if err != nil {
		t.Fatalf("restart --quiet returned error: %v", err)
	}
	if !downCalled || !upCalled {
		t.Fatalf("expected full-stack quiet restart to run down/up, got down=%v up=%v", downCalled, upCalled)
	}
	if waitForPortCalled {
		t.Fatal("verification mode should not call waitForPort for quiet full-stack restart")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("quiet full-stack restart should suppress stdout, got:\n%s", stdout)
	}
}

func TestRestartServiceFailsWhenSelectedPortIsBusy(t *testing.T) {
	var composeUpServicesCalled bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: `[]`}, nil
		}
		d.portInUse = func(port int) (bool, error) { return port == cfg.Ports.Postgres, nil }
		d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
			composeUpServicesCalled = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "restart", "postgres")
	if err == nil || !strings.Contains(err.Error(), "cannot start postgres") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "port 5432 is in use by another process or container, not postgres") {
		t.Fatalf("unexpected port conflict error: %v", err)
	}
	if composeUpServicesCalled {
		t.Fatal("composeUpServices should not run when restart preflight finds a port conflict")
	}
}

func TestRestartServicePropagatesPortCheckErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: `[]`}, nil
		}
		d.portInUse = func(int) (bool, error) { return false, errors.New("probe failed") }
	})

	_, _, err := executeRoot(t, "restart", "postgres")
	if err == nil || !strings.Contains(err.Error(), "port 5432 check failed: probe failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestartPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			t.Fatal("composeDown should not run when restart status output fails")
			return nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			t.Fatal("composeUp should not run when restart status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"restart"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected restart status write failure, got %v", err)
	}
}

func TestStopServicePropagatesComposeStopErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeStopServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, services []string) error {
			if !reflect.DeepEqual(services, []string{"redis"}) {
				t.Fatalf("unexpected services: %v", services)
			}
			return errors.New("stop failed")
		}
	})

	_, _, err := executeRoot(t, "stop", "redis")
	if err == nil || !strings.Contains(err.Error(), "stop failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			t.Fatal("composeDown should not run when stop status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"stop"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected stop status write failure, got %v", err)
	}
}
