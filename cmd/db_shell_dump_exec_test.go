package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDBShellPropagatesComposeExecErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			return errors.New("psql failed")
		}
	})

	_, _, err := executeRoot(t, "db", "shell")
	if err == nil || !strings.Contains(err.Error(), "psql failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBShellPropagatesVerboseComposeWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			t.Fatal("composeExec should not run when verbose compose output fails")
			return nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--verbose", "db", "shell"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected db shell verbose write failure, got %v", err)
	}
}

func TestDBDumpPropagatesComposeExecErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
			return errors.New("pg_dump failed")
		}
	})

	_, _, err := executeRoot(t, "db", "dump")
	if err == nil || !strings.Contains(err.Error(), "pg_dump failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBResetPropagatesDropAndCreateErrors(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Connection.PostgresDatabase = "stackdb"

	t.Run("drop database failure", func(t *testing.T) {
		callCount := 0

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, commandArgs []string, _ bool) error {
				callCount++
				if callCount == 2 && strings.Contains(strings.Join(commandArgs, " "), "DROP DATABASE IF EXISTS") {
					return errors.New("drop failed")
				}
				return nil
			}
		})

		_, _, err := executeRoot(t, "db", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "drop failed") {
			t.Fatalf("unexpected drop error: %v", err)
		}
		if callCount != 2 {
			t.Fatalf("expected reset to stop after the drop failure, got %d composeExec calls", callCount)
		}
	})

	t.Run("create database failure", func(t *testing.T) {
		callCount := 0

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, commandArgs []string, _ bool) error {
				callCount++
				if callCount == 3 && strings.Contains(strings.Join(commandArgs, " "), "CREATE DATABASE") {
					return errors.New("create failed")
				}
				return nil
			}
		})

		_, _, err := executeRoot(t, "db", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "create failed") {
			t.Fatalf("unexpected create error: %v", err)
		}
		if callCount != 3 {
			t.Fatalf("expected reset to reach the create step, got %d composeExec calls", callCount)
		}
	})
}
