package cmd

import (
	"context"
	"reflect"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDBShellUsesComposeExecWithConfiguredDatabase(t *testing.T) {
	var capturedService string
	var capturedEnv []string
	var capturedArgs []string
	var capturedTTY bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			capturedService = service
			capturedEnv = append([]string(nil), env...)
			capturedArgs = append([]string(nil), commandArgs...)
			capturedTTY = tty
			return nil
		}
	})

	_, _, err := executeRoot(t, "db", "shell", "--", "-tAc", "select current_database()")
	if err != nil {
		t.Fatalf("db shell returned error: %v", err)
	}

	if capturedService != "postgres" {
		t.Fatalf("unexpected service: %s", capturedService)
	}
	if !reflect.DeepEqual(capturedEnv, []string{"PGPASSWORD=stackpass"}) {
		t.Fatalf("unexpected env: %q", capturedEnv)
	}
	wantArgs := []string{
		"psql",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "stackuser",
		"-d", "stackdb",
		"-tAc", "select current_database()",
	}
	if !reflect.DeepEqual(capturedArgs, wantArgs) {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", capturedArgs, wantArgs)
	}
	if capturedTTY {
		t.Fatal("expected non-interactive db shell by default in tests")
	}
}

func TestDBShellNoTTYFlagDisablesTTY(t *testing.T) {
	var capturedTTY bool

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, _ []string, tty bool) error {
			capturedTTY = tty
			return nil
		}
	})

	_, _, err := executeRoot(t, "db", "shell", "--no-tty")
	if err != nil {
		t.Fatalf("db shell returned error: %v", err)
	}
	if capturedTTY {
		t.Fatal("expected --no-tty to disable tty allocation")
	}
}
