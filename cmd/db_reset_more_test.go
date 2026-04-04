package cmd

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDBDumpRejectsPositionalAndFlagOutputTogether(t *testing.T) {
	_, _, err := executeRoot(t, "db", "dump", "dump.sql", "--output", "other.sql")
	if err == nil || !strings.Contains(err.Error(), "either a positional path or --output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBDumpReturnsCreateFileErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			t.Fatal("composeExec should not run when the dump output file cannot be created")
			return nil
		}
	})

	missingPath := filepath.Join(t.TempDir(), "missing", "dump.sql")
	_, _, err := executeRoot(t, "db", "dump", "--output", missingPath)
	if err == nil || !strings.Contains(err.Error(), "create dump file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBDumpPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"db", "dump", "--output", filepath.Join(t.TempDir(), "dump.sql")})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected db dump status write failure, got %v", err)
	}
}

func TestDBRestorePropagatesPromptErrorsAsForceHint(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, io.ErrUnexpectedEOF }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			t.Fatal("composeExec should not run when restore confirmation fails")
			return nil
		}
	})

	_, _, err := executeRootWithInput(t, strings.NewReader("select 1;\n"), "db", "restore", "-")
	if err == nil || !strings.Contains(err.Error(), "database restore confirmation required; rerun with --force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBRestoreReturnsOpenFileErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	missingPath := filepath.Join(t.TempDir(), "missing.sql")
	_, _, err := executeRoot(t, "db", "restore", missingPath, "--force")
	if err == nil || !strings.Contains(err.Error(), "open dump file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBRestorePropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			t.Fatal("composeExec should not run when restore status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetIn(strings.NewReader("select 1;\n"))
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"db", "restore", "-", "--force"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected db restore status write failure, got %v", err)
	}
}

func TestDBResetPropagatesPromptErrorsAsForceHint(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, io.ErrUnexpectedEOF }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			t.Fatal("composeExec should not run when reset confirmation fails")
			return nil
		}
	})

	_, _, err := executeRoot(t, "db", "reset")
	if err == nil || !strings.Contains(err.Error(), "database reset confirmation required; rerun with --force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBResetPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			t.Fatal("composeExec should not run when reset status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"db", "reset", "--force"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected db reset status write failure, got %v", err)
	}
}

func TestResetVolumesPromptErrorsReturnForceHint(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, io.ErrUnexpectedEOF }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			t.Fatal("composeDown should not run when reset confirmation fails")
			return nil
		}
	})

	_, _, err := executeRoot(t, "reset", "--volumes")
	if err == nil || !strings.Contains(err.Error(), "volume wipe confirmation required; rerun with --force") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetVolumesPropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			t.Fatal("composeDown should not run when reset status output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"reset", "--volumes", "--force"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected reset status write failure, got %v", err)
	}
}
