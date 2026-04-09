package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSnapshotRestoreCommandErrorPaths(t *testing.T) {
	cfg := snapshotFixtureConfig()
	archivePath := writeSnapshotFixtureArchive(t, cfg)

	t.Run("restore command surfaces stop-stack failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("compose down failed")
			}
		})

		_, _, err := executeRoot(t, "snapshot", "restore", archivePath, "--stop", "--force")
		if err == nil || !strings.Contains(err.Error(), "compose down failed") {
			t.Fatalf("expected stop-stack failure, got %v", err)
		}
	})

	t.Run("restore command surfaces status output failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		err := executeRootWithIO(t, alwaysErrorWriter{}, io.Discard, "snapshot", "restore", archivePath, "--force")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected snapshot restore status write failure, got %v", err)
		}
	})

	t.Run("restore command surfaces archive restore failures", func(t *testing.T) {
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "podman")
		script := "#!/bin/sh\n" +
			"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"import\" ]; then\n" +
			"  echo \"import failed\" >&2\n" +
			"  exit 1\n" +
			"fi\n" +
			"exit 0\n"
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			t.Fatalf("write failing podman: %v", err)
		}
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
				if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
					return system.CommandResult{ExitCode: 1}, nil
				}
				return system.CommandResult{Stdout: "[]"}, nil
			}
		})

		_, _, err := executeRoot(t, "snapshot", "restore", archivePath, "--force")
		if err == nil || !strings.Contains(err.Error(), "podman volume import") {
			t.Fatalf("expected snapshot restore failure, got %v", err)
		}
	})
}
