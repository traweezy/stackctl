package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/system"
)

type alwaysErrorWriter struct{}

func (alwaysErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type failAfterWritesWriter struct {
	writes int
	limit  int
}

func (w *failAfterWritesWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.limit {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

func TestRootCommandErrorPaths(t *testing.T) {
	t.Run("loadRuntimeConfig failures surface across runtime commands", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
		}{
			{name: "env", args: []string{"env"}},
			{name: "exec", args: []string{"exec", "postgres", "--", "printenv"}},
			{name: "logs", args: []string{"logs"}},
			{name: "open", args: []string{"open"}},
			{name: "services", args: []string{"services"}},
			{name: "reset", args: []string{"reset", "--force"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.configFilePath = func() (string, error) { return "", errors.New("config path failed") }
				})

				_, _, err := executeRoot(t, tc.args...)
				if err == nil || !strings.Contains(err.Error(), "config path failed") {
					t.Fatalf("expected %s to surface config path failures, got %v", tc.name, err)
				}
			})
		}
	})

	t.Run("config reset reports prompt guidance on confirmation failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
				return false, errors.New("prompt failed")
			}
		})

		_, _, err := executeRoot(t, "config", "reset")
		if err == nil || !strings.Contains(err.Error(), "rerun with --force or --yes") {
			t.Fatalf("expected config reset prompt guidance, got %v", err)
		}
	})

	t.Run("db restore returns compose exec failures while the context is active", func(t *testing.T) {
		expectedErr := errors.New("restore failed")
		dumpPath := filepath.Join(t.TempDir(), "dump.sql")
		if err := os.WriteFile(dumpPath, []byte("select 1;\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
				return expectedErr
			}
		})

		_, _, err := executeRoot(t, "db", "restore", dumpPath, "--force")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected db restore failure %v, got %v", expectedErr, err)
		}
	})

	t.Run("doctor surfaces markdown rendering failures", func(t *testing.T) {
		writer := &failAfterWritesWriter{limit: 2}
		cmd := new(cobra.Command)
		cmd.SetOut(writer)

		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
		})

		report := doctorpkg.Report{
			Checks:    []doctorpkg.Check{{Status: "FAIL", Message: "podman missing"}},
			FailCount: 1,
		}
		if err := printDoctorReport(cmd, report); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected doctor markdown write failure, got %v", err)
		}
	})

	t.Run("withinRoot returns false when filepath.Rel fails", func(t *testing.T) {
		if withinRoot("", "/tmp/stackctl") {
			t.Fatal("expected withinRoot to reject candidates when filepath.Rel cannot resolve a relative path")
		}
	})

	t.Run("health command surfaces output failures in single-shot and watch modes", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		})

		if err := executeRootWithIO(t, alwaysErrorWriter{}, io.Discard, "health"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected health output failure, got %v", err)
		}
		if err := executeRootWithIO(t, alwaysErrorWriter{}, io.Discard, "health", "--watch"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected watched health output failure, got %v", err)
		}
	})

	t.Run("reset surfaces runtime readiness and verbose compose output failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.commandExists = func(string) bool { return false }
		})
		if _, _, err := executeRoot(t, "reset", "--force"); err == nil {
			t.Fatal("expected reset to surface compose runtime readiness failures")
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		})
		if err := executeRootWithIO(t, alwaysErrorWriter{}, io.Discard, "--verbose", "reset", "--force"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected verbose compose-file output failure, got %v", err)
		}
	})

	t.Run("logs surfaces disabled-service errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeRedis = false
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "logs", "--service", "redis")
		if err == nil || !strings.Contains(err.Error(), "redis is not enabled") {
			t.Fatalf("expected disabled redis log error, got %v", err)
		}
	})

	t.Run("open all succeeds when every browser target is disabled", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = false
			cfg.Setup.IncludeMeilisearch = false
			cfg.Setup.IncludePgAdmin = false
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		if _, _, err := executeRoot(t, "open", "all"); err != nil {
			t.Fatalf("expected open all to noop when every target is disabled, got %v", err)
		}
	})
}
