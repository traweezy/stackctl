package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRestoreSnapshotArchiveRemovesExistingVolumesBeforeRecreate(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeFakePodman(t, dir, logPath)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	payloadPath := filepath.Join(dir, "postgres.tar")
	if err := os.WriteFile(payloadPath, []byte("postgres-volume"), 0o644); err != nil {
		t.Fatalf("write snapshot payload: %v", err)
	}

	specs := []persistentVolumeSpec{{
		ServiceKey:   "postgres",
		DisplayName:  "Postgres",
		VolumeName:   "postgres_data",
		ArchiveEntry: "volumes/postgres.tar",
	}}
	manifest := snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
		Volumes: []snapshotVolumeRecord{{
			Service:    "postgres",
			SourceName: "postgres_data",
			Archive:    "volumes/postgres.tar",
		}},
	}

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 0}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	cmd := &cobra.Command{Use: "snapshot"}
	if err := restoreSnapshotArchive(cmd, specs, manifest, map[string]string{"volumes/postgres.tar": payloadPath}); err != nil {
		t.Fatalf("restoreSnapshotArchive returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read podman log: %v", err)
	}
	logText := string(data)
	for _, fragment := range []string{
		"volume rm postgres_data",
		"volume create postgres_data",
		"volume import postgres_data",
	} {
		if !strings.Contains(logText, fragment) {
			t.Fatalf("expected podman log to contain %q:\n%s", fragment, logText)
		}
	}
}

func TestRestoreSnapshotArchiveSurfacesCreateFailures(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "failing-podman.log")
	scriptPath := filepath.Join(dir, "podman")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + logPath + "\"\n" +
		"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"create\" ]; then\n" +
		"  echo \"create failed\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write failing podman: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	payloadPath := filepath.Join(dir, "postgres.tar")
	if err := os.WriteFile(payloadPath, []byte("postgres-volume"), 0o644); err != nil {
		t.Fatalf("write snapshot payload: %v", err)
	}

	specs := []persistentVolumeSpec{{
		ServiceKey:   "postgres",
		DisplayName:  "Postgres",
		VolumeName:   "postgres_data",
		ArchiveEntry: "volumes/postgres.tar",
	}}
	manifest := snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
		Volumes: []snapshotVolumeRecord{{
			Service:    "postgres",
			SourceName: "postgres_data",
			Archive:    "volumes/postgres.tar",
		}},
	}

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 1}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	cmd := &cobra.Command{Use: "snapshot"}
	err := restoreSnapshotArchive(cmd, specs, manifest, map[string]string{"volumes/postgres.tar": payloadPath})
	if err == nil || !strings.Contains(err.Error(), "podman volume create postgres_data") {
		t.Fatalf("unexpected create error: %v", err)
	}
}
