package cmd

import (
	"archive/tar"
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSnapshotSaveRequiresPersistentManagedVolumes(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePostgres = false
		cfg.Setup.IncludeRedis = false
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, _, err := executeRoot(t, "snapshot", "save", filepath.Join(t.TempDir(), "empty.tar"))
	if err == nil || !strings.Contains(err.Error(), "no persistent managed service volumes are configured in this stack") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSnapshotRestoreRequiresPersistentManagedVolumes(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePostgres = false
		cfg.Setup.IncludeRedis = false
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, _, err := executeRoot(t, "snapshot", "restore", filepath.Join(t.TempDir(), "empty.tar"), "--force")
	if err == nil || !strings.Contains(err.Error(), "no persistent managed service volumes are configured in this stack") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSnapshotSavePropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"snapshot", "save", filepath.Join(t.TempDir(), "snapshot.tar")})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected snapshot save status write failure, got %v", err)
	}
}

func TestSnapshotSaveFailsWhenManagedVolumeIsMissing(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 1}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	_, _, err := executeRoot(t, "snapshot", "save", filepath.Join(t.TempDir(), "missing.tar"))
	if err == nil || !strings.Contains(err.Error(), "managed volume postgres_data for Postgres does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteSnapshotArchiveReturnsEntryErrorsForMissingPayloads(t *testing.T) {
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
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteTarFileRejectsDirectories(t *testing.T) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	t.Cleanup(func() { _ = writer.Close() })

	err := writeTarFile(writer, "volumes/postgres.tar", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
