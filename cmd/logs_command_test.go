package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
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
	if !strings.Contains(stdout, "postgres|pg, redis|rd, nats, seaweedfs|seaweed, meilisearch|meili, pgadmin") {
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

func TestDefaultComposeLogsUsesContainerLogsForSingleService(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeLogsFakePodman(t, dir, logPath, "#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuoteForLogsTest(logPath)+"\nif [ \"$1\" = \"logs\" ]; then\n  exit 0\nfi\necho \"unexpected podman args: $*\" >&2\nexit 1\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.Stack.Dir = dir
	runner := system.Runner{Stdout: io.Discard, Stderr: io.Discard}
	err := defaultCommandDeps().composeLogs(context.Background(), runner, cfg, 25, true, "2m", "postgres")
	if err != nil {
		t.Fatalf("composeLogs returned error: %v", err)
	}

	assertLogsRecordedArgs(t, logPath, []string{
		"logs",
		"-f",
		"--tail",
		"25",
		"--since",
		"2m",
		cfg.Services.PostgresContainer,
	})
}

func TestDefaultComposeLogsSingleServiceSurfacesMissingContainer(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeLogsFakePodman(t, dir, logPath, "#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuoteForLogsTest(logPath)+"\nif [ \"$1\" = \"logs\" ]; then\n  echo \"Error: no container with name or ID \\\"local-postgres\\\" found: no such container\" >&2\n  exit 1\nfi\nif [ \"$1\" = \"compose\" ]; then\n  echo \"compose logs should not be used for single-service logs\" >&2\n  exit 99\nfi\necho \"unexpected podman args: $*\" >&2\nexit 1\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.Stack.Dir = dir
	runner := system.Runner{Stdout: io.Discard, Stderr: io.Discard}
	err := defaultCommandDeps().composeLogs(context.Background(), runner, cfg, 1, false, "", "postgres")
	if err == nil {
		t.Fatal("expected composeLogs to fail when the selected container is missing")
	}
	if !strings.Contains(err.Error(), "podman logs --tail 1 "+cfg.Services.PostgresContainer) {
		t.Fatalf("unexpected error: %v", err)
	}

	assertLogsRecordedArgs(t, logPath, []string{
		"logs",
		"--tail",
		"1",
		cfg.Services.PostgresContainer,
	})
}

func TestDefaultComposeLogsAllServicesStillUsesComposeLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeLogsFakePodman(t, dir, logPath, "#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuoteForLogsTest(logPath)+"\nif [ \"$1\" = \"compose\" ]; then\n  exit 0\nfi\necho \"unexpected podman args: $*\" >&2\nexit 1\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.Stack.Dir = dir
	runner := system.Runner{Stdout: io.Discard, Stderr: io.Discard}
	err := defaultCommandDeps().composeLogs(context.Background(), runner, cfg, 10, false, "", "")
	if err != nil {
		t.Fatalf("composeLogs returned error: %v", err)
	}

	assertLogsRecordedArgs(t, logPath, []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"logs",
		"--tail",
		"10",
	})
}

func writeLogsFakePodman(t *testing.T, dir, logPath, script string) {
	t.Helper()

	path := filepath.Join(dir, "podman")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
}

func assertLogsRecordedArgs(t *testing.T, logPath string, want []string) {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	got := strings.Fields(strings.TrimSpace(string(data)))
	if !equalStringSlices(got, want) {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", got, want)
	}
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func shellQuoteForLogsTest(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}
