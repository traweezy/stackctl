package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSaveSnapshotArchiveRequiresManagedVolumes(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	specs := []persistentVolumeSpec{{
		ServiceKey:   "postgres",
		DisplayName:  "Postgres",
		VolumeName:   "postgres_data",
		ArchiveEntry: "volumes/postgres.tar",
	}}

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 1}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	cmd := &cobra.Command{Use: "snapshot"}
	err := saveSnapshotArchive(cmd, cfg, specs, filepath.Join(t.TempDir(), "snapshot.tar"))
	if err == nil || !strings.Contains(err.Error(), "managed volume postgres_data for Postgres does not exist") {
		t.Fatalf("unexpected saveSnapshotArchive error: %v", err)
	}
}

func TestSaveSnapshotArchiveSurfacesExportFailures(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "podman")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"export\" ]; then\n" +
		"  echo \"export failed\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write failing podman: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()
	specs := []persistentVolumeSpec{{
		ServiceKey:   "postgres",
		DisplayName:  "Postgres",
		VolumeName:   "postgres_data",
		ArchiveEntry: "volumes/postgres.tar",
	}}

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 0}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	cmd := &cobra.Command{Use: "snapshot"}
	err := saveSnapshotArchive(cmd, cfg, specs, filepath.Join(t.TempDir(), "snapshot.tar"))
	if err == nil || !strings.Contains(err.Error(), "podman volume export postgres_data") {
		t.Fatalf("unexpected export error: %v", err)
	}
}

func TestWriteSnapshotArchiveSurfacesMissingPayloadErrors(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "snapshot.tar")
	manifest := snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
		Volumes: []snapshotVolumeRecord{{
			Service:    "postgres",
			SourceName: "postgres_data",
			Archive:    "volumes/postgres.tar",
		}},
	}

	err := writeSnapshotArchive(archivePath, manifest, map[string]string{
		"volumes/postgres.tar": filepath.Join(t.TempDir(), "missing.tar"),
	})
	if err == nil || !strings.Contains(err.Error(), "write snapshot entry volumes/postgres.tar") {
		t.Fatalf("unexpected writeSnapshotArchive error: %v", err)
	}
}
