package cmd

import (
	"context"
	"io"
	"os"
	"reflect"
	"strings"
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

func TestDBDumpWritesToFileThroughComposeExec(t *testing.T) {
	var capturedArgs []string
	var capturedEnv []string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeExec = func(_ context.Context, runner system.Runner, _ configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			if service != "postgres" {
				t.Fatalf("unexpected service: %s", service)
			}
			if tty {
				t.Fatal("expected db dump to run without a tty")
			}
			capturedEnv = append([]string(nil), env...)
			capturedArgs = append([]string(nil), commandArgs...)
			_, err := io.WriteString(runner.Stdout, "-- dump --\n")
			return err
		}
	})

	path := t.TempDir() + "/dump.sql"
	stdout, _, err := executeRoot(t, "db", "dump", path)
	if err != nil {
		t.Fatalf("db dump returned error: %v", err)
	}
	if !strings.Contains(stdout, "wrote database dump to "+path) {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
	if !reflect.DeepEqual(capturedEnv, []string{"PGPASSWORD=stackpass"}) {
		t.Fatalf("unexpected env: %q", capturedEnv)
	}
	wantArgs := []string{
		"pg_dump",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "stackuser",
		"-d", "stackdb",
		"--format=plain",
		"--no-owner",
		"--no-privileges",
	}
	if !reflect.DeepEqual(capturedArgs, wantArgs) {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", capturedArgs, wantArgs)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump file: %v", err)
	}
	if string(data) != "-- dump --\n" {
		t.Fatalf("unexpected dump content: %q", string(data))
	}
}

func TestDBRestoreStreamsInputFileToComposeExec(t *testing.T) {
	var capturedInput string
	var capturedArgs []string

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(_ context.Context, runner system.Runner, _ configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			if service != "postgres" {
				t.Fatalf("unexpected service: %s", service)
			}
			if tty {
				t.Fatal("expected db restore to run without a tty")
			}
			data, err := io.ReadAll(runner.Stdin)
			if err != nil {
				return err
			}
			capturedInput = string(data)
			capturedArgs = append([]string(nil), commandArgs...)
			return nil
		}
	})

	path := t.TempDir() + "/dump.sql"
	if err := os.WriteFile(path, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write dump file: %v", err)
	}

	stdout, _, err := executeRoot(t, "db", "restore", path, "--force")
	if err != nil {
		t.Fatalf("db restore returned error: %v", err)
	}
	if capturedInput != "select 1;\n" {
		t.Fatalf("unexpected restore input: %q", capturedInput)
	}
	wantArgs := []string{
		"psql",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "app",
		"-d", "app",
		"-v", "ON_ERROR_STOP=1",
		"-f", "-",
	}
	if !reflect.DeepEqual(capturedArgs, wantArgs) {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", capturedArgs, wantArgs)
	}
	if !strings.Contains(stdout, "database restore completed") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestDBResetRunsTerminateDropAndCreateCommands(t *testing.T) {
	var commands [][]string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		cfg.Services.Postgres.MaintenanceDatabase = "template1"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			if service != "postgres" {
				t.Fatalf("unexpected service: %s", service)
			}
			if tty {
				t.Fatal("expected db reset to run without a tty")
			}
			if !reflect.DeepEqual(env, []string{"PGPASSWORD=stackpass"}) {
				t.Fatalf("unexpected env: %q", env)
			}
			commands = append(commands, append([]string(nil), commandArgs...))
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "db", "reset", "--force")
	if err != nil {
		t.Fatalf("db reset returned error: %v", err)
	}
	if !strings.Contains(stdout, "database stackdb reset") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
	if len(commands) != 3 {
		t.Fatalf("expected 3 compose exec calls, got %d", len(commands))
	}
	for idx, sql := range []string{
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'stackdb' AND pid <> pg_backend_pid()",
		"DROP DATABASE IF EXISTS \"stackdb\"",
		"CREATE DATABASE \"stackdb\"",
	} {
		if commands[idx][0] != "psql" || !containsSequence(commands[idx], []string{"-d", "template1"}) || !containsSequence(commands[idx], []string{"-c", sql}) {
			t.Fatalf("unexpected reset command %d: %q", idx, commands[idx])
		}
	}
}

func TestDBResetRejectsMaintenanceDatabase(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "postgres"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, _, err := executeRoot(t, "db", "reset", "--force")
	if err == nil || !strings.Contains(err.Error(), "maintenance database") {
		t.Fatalf("expected maintenance database error, got %v", err)
	}
}

func containsSequence(haystack, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	for idx := 0; idx <= len(haystack)-len(needle); idx++ {
		if reflect.DeepEqual(haystack[idx:idx+len(needle)], needle) {
			return true
		}
	}

	return false
}
