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

func TestDBResetAcceptsInteractiveConfirmationAndRunsCommands(t *testing.T) {
	var promptMessage string
	var promptDefault bool
	var commands [][]string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		cfg.Services.Postgres.MaintenanceDatabase = "template1"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(_ io.Reader, _ io.Writer, message string, defaultYes bool) (bool, error) {
			promptMessage = message
			promptDefault = defaultYes
			return true, nil
		}
		d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, commandArgs []string, tty bool) error {
			if tty {
				t.Fatal("expected db reset to run without a tty")
			}
			commands = append(commands, append([]string(nil), commandArgs...))
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "db", "reset")
	if err != nil {
		t.Fatalf("db reset returned error: %v", err)
	}
	if promptMessage != "This will drop and recreate stackdb. Continue?" {
		t.Fatalf("unexpected prompt message: %q", promptMessage)
	}
	if promptDefault {
		t.Fatal("expected db reset confirmation default to be false")
	}
	if len(commands) != 3 {
		t.Fatalf("expected 3 db reset commands, got %d", len(commands))
	}
	if !containsSequence(commands[1], []string{"-c", `DROP DATABASE IF EXISTS "stackdb"`}) {
		t.Fatalf("expected drop database command, got %q", commands[1])
	}
	if !containsSequence(commands[2], []string{"-c", `CREATE DATABASE "stackdb"`}) {
		t.Fatalf("expected create database command, got %q", commands[2])
	}
	for _, fragment := range []string{
		"resetting database stackdb...",
		"database stackdb reset",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("expected db reset output to contain %q:\n%s", fragment, stdout)
		}
	}
}

func TestDBResetReturnsCreateStepErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Services.Postgres.MaintenanceDatabase = "template1"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, commandArgs []string, _ bool) error {
			if containsSequence(commandArgs, []string{"-c", `CREATE DATABASE "stackdb"`}) {
				return errors.New("create failed")
			}
			return nil
		}
	})

	_, _, err := executeRoot(t, "db", "reset", "--force")
	if err == nil || !strings.Contains(err.Error(), "create failed") {
		t.Fatalf("expected create-step failure, got %v", err)
	}
}
