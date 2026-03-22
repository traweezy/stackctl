package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestExecRequiresTargetCommand(t *testing.T) {
	_, _, err := executeRoot(t, "exec", "postgres")
	if err == nil || !strings.Contains(err.Error(), "usage: stackctl exec <service> -- <command...>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecUsesComposeExecWithCanonicalService(t *testing.T) {
	var called bool
	var capturedService string
	var capturedArgs []string
	var capturedTTY bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			called = true
			capturedService = service
			if len(env) != 0 {
				t.Fatalf("unexpected env: %q", env)
			}
			capturedArgs = append([]string(nil), commandArgs...)
			capturedTTY = tty
			return nil
		}
	})

	_, _, err := executeRoot(t, "exec", "pg", "--", "psql", "-U", "app")
	if err != nil {
		t.Fatalf("exec returned error: %v", err)
	}
	if !called {
		t.Fatal("expected exec to call compose exec")
	}
	if capturedService != "postgres" {
		t.Fatalf("unexpected service: %s", capturedService)
	}
	if !reflect.DeepEqual(capturedArgs, []string{"psql", "-U", "app"}) {
		t.Fatalf("unexpected command args: %q", capturedArgs)
	}
	if capturedTTY {
		t.Fatal("expected non-interactive exec by default in tests")
	}
}

func TestExecTTYRespectsNoTTYFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantTTY bool
	}{
		{name: "tty enabled", args: []string{"exec", "redis", "--", "redis-cli", "PING"}, wantTTY: true},
		{name: "tty disabled", args: []string{"exec", "redis", "--no-tty", "--", "redis-cli", "PING"}, wantTTY: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedTTY bool

			withTestDeps(t, func(d *commandDeps) {
				d.isTerminal = func() bool { return true }
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, _ []string, tty bool) error {
					capturedTTY = tty
					return nil
				}
			})

			_, _, err := executeRoot(t, tt.args...)
			if err != nil {
				t.Fatalf("exec returned error: %v", err)
			}
			if capturedTTY != tt.wantTTY {
				t.Fatalf("tty = %v, want %v", capturedTTY, tt.wantTTY)
			}
		})
	}
}

func TestExecRejectsDisabledPgAdmin(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePgAdmin = false
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, _, err := executeRoot(t, "exec", "pgadmin", "--", "printenv", "PGADMIN_DEFAULT_EMAIL")
	if err == nil || !strings.Contains(err.Error(), "pgadmin is not enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}
