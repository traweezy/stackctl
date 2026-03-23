package cmd

import (
	"context"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestLogsHelpDocumentsAliasesAndWatchMode(t *testing.T) {
	stdout, _, err := executeRoot(t, "logs", "--help")
	if err != nil {
		t.Fatalf("logs --help returned error: %v", err)
	}
	if !strings.Contains(stdout, "prints the last 100 lines and exits") {
		t.Fatalf("stdout missing default logs behavior: %s", stdout)
	}
	if !strings.Contains(stdout, "postgres|pg, redis|rd, nats|na, pgadmin") {
		t.Fatalf("stdout missing service aliases: %s", stdout)
	}
	if !strings.Contains(stdout, "--watch") {
		t.Fatalf("stdout missing watch flag: %s", stdout)
	}
}

func TestLogsAllServicesUsesComposeLogsDefaults(t *testing.T) {
	var called bool
	var capturedTail int
	var follow bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeLogs = func(_ context.Context, _ system.Runner, _ configpkg.Config, tail int, watch bool, _, service string) error {
			called = true
			capturedTail = tail
			follow = watch
			if service != "" {
				t.Fatalf("expected no service filter, got %q", service)
			}
			return nil
		}
	})

	_, _, err := executeRoot(t, "logs")
	if err != nil {
		t.Fatalf("logs returned error: %v", err)
	}
	if !called {
		t.Fatal("expected logs to use compose logs when no service is selected")
	}
	if capturedTail != 100 || follow {
		t.Fatalf("unexpected log options: tail=%d follow=%v", capturedTail, follow)
	}
}

func TestLogsAllServicesWatchUsesComposeLogs(t *testing.T) {
	var called bool
	var capturedTail int
	var follow bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeLogs = func(_ context.Context, _ system.Runner, _ configpkg.Config, tail int, watch bool, _, service string) error {
			called = true
			capturedTail = tail
			follow = watch
			if service != "" {
				t.Fatalf("expected no service filter, got %q", service)
			}
			return nil
		}
	})

	_, _, err := executeRoot(t, "logs", "-w", "-n", "50")
	if err != nil {
		t.Fatalf("logs returned error: %v", err)
	}
	if !called {
		t.Fatal("expected logs to use compose logs when following all services")
	}
	if capturedTail != 50 || !follow {
		t.Fatalf("unexpected log options: tail=%d follow=%v", capturedTail, follow)
	}
}
