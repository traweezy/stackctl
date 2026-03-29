package cmd

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSnapshotSaveWritesManagedVolumeArchive(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeFakePodman(t, dir, logPath)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 0}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	archivePath := filepath.Join(dir, "snapshot.tar")
	_, _, err := executeRoot(t, "snapshot", "save", archivePath)
	if err != nil {
		t.Fatalf("snapshot save returned error: %v", err)
	}

	manifest, entries := readSnapshotTar(t, archivePath)
	if manifest.Version != 1 || manifest.StackName != "dev-stack" {
		t.Fatalf("unexpected snapshot manifest: %+v", manifest)
	}
	if len(manifest.Volumes) != 3 {
		t.Fatalf("expected 3 snapshot volumes, got %+v", manifest.Volumes)
	}
	for _, entry := range []string{"volumes/postgres.tar", "volumes/redis.tar", "volumes/pgadmin.tar"} {
		if got := entries[entry]; got == "" {
			t.Fatalf("expected snapshot entry %s in archive, got %+v", entry, entries)
		}
	}
}

func TestSnapshotRestoreRecreatesAndImportsManagedVolumes(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeFakePodman(t, dir, logPath)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	files := make(map[string]string)
	for _, spec := range persistentVolumeSpecs(cfg) {
		path := filepath.Join(dir, spec.ServiceKey+".tar")
		if err := os.WriteFile(path, []byte(spec.VolumeName), 0o644); err != nil {
			t.Fatalf("write snapshot payload %s: %v", path, err)
		}
		files[spec.ArchiveEntry] = path
	}
	manifest := snapshotManifest{Version: 1, StackName: "dev-stack"}
	for _, spec := range persistentVolumeSpecs(cfg) {
		manifest.Volumes = append(manifest.Volumes, snapshotVolumeRecord{
			Service:    spec.ServiceKey,
			SourceName: spec.VolumeName,
			Archive:    spec.ArchiveEntry,
		})
	}
	archivePath := filepath.Join(dir, "restore.tar")
	if err := writeSnapshotArchive(archivePath, manifest, files); err != nil {
		t.Fatalf("write snapshot archive: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 0}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	_, _, err := executeRoot(t, "snapshot", "restore", archivePath, "--force")
	if err != nil {
		t.Fatalf("snapshot restore returned error: %v", err)
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
		"volume rm redis_data",
		"volume import redis_data",
		"volume rm pgadmin_data",
		"volume import pgadmin_data",
	} {
		if !strings.Contains(logText, fragment) {
			t.Fatalf("expected podman log to contain %q:\n%s", fragment, logText)
		}
	}
}

func TestSnapshotRestoreRejectsMissingPayloadBeforeChangingVolumes(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman.log")
	writeFakePodman(t, dir, logPath)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	files := make(map[string]string)
	manifest := snapshotManifest{Version: 1, StackName: "dev-stack"}
	for _, spec := range persistentVolumeSpecs(cfg) {
		manifest.Volumes = append(manifest.Volumes, snapshotVolumeRecord{
			Service:    spec.ServiceKey,
			SourceName: spec.VolumeName,
			Archive:    spec.ArchiveEntry,
		})
		if spec.ServiceKey == "redis" {
			continue
		}
		path := filepath.Join(dir, spec.ServiceKey+".tar")
		if err := os.WriteFile(path, []byte(spec.VolumeName), 0o644); err != nil {
			t.Fatalf("write snapshot payload %s: %v", path, err)
		}
		files[spec.ArchiveEntry] = path
	}
	archivePath := filepath.Join(dir, "broken-restore.tar")
	if err := writePartialSnapshotArchive(archivePath, manifest, files); err != nil {
		t.Fatalf("write partial snapshot archive: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
			if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
				return system.CommandResult{ExitCode: 0}, nil
			}
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	_, _, err := executeRoot(t, "snapshot", "restore", archivePath, "--force")
	if err == nil || !strings.Contains(err.Error(), "snapshot archive is missing payload volumes/redis.tar for redis") {
		t.Fatalf("unexpected snapshot restore error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read podman log: %v", err)
	}
	if strings.TrimSpace(string(data)) != "" {
		t.Fatalf("snapshot restore should not change volumes when payload validation fails:\n%s", string(data))
	}
}

func writeFakePodman(t *testing.T, dir, logPath string) {
	t.Helper()

	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"" + logPath + "\"\n" +
		"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"export\" ]; then\n" +
		"  printf \"%s\\n\" \"$3\" > \"$5\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"import\" ]; then\n" +
		"  cat \"$4\" >/dev/null\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"

	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
}

func readSnapshotTar(t *testing.T, path string) (snapshotManifest, map[string]string) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open snapshot archive %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	reader := tar.NewReader(file)
	entries := make(map[string]string)
	manifest := snapshotManifest{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read snapshot archive: %v", err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read snapshot entry %s: %v", header.Name, err)
		}
		if header.Name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
			continue
		}
		entries[header.Name] = string(data)
	}
	return manifest, entries
}

func writePartialSnapshotArchive(path string, manifest snapshotManifest, files map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	writer := tar.NewWriter(file)
	defer func() { _ = writer.Close() }()

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeTarEntry(writer, "manifest.json", manifestData); err != nil {
		return err
	}
	for archivePath, sourcePath := range files {
		if err := writeTarFile(writer, archivePath, sourcePath); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return file.Close()
}
