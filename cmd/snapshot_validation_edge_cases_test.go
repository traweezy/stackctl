package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestValidateSnapshotManifestCoversVersionServiceAndArchiveChecks(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	specs := []persistentVolumeSpec{
		{ServiceKey: "postgres", ArchiveEntry: "volumes/postgres.tar"},
		{ServiceKey: "redis", ArchiveEntry: "volumes/redis.tar"},
	}

	t.Run("rejects unsupported versions", func(t *testing.T) {
		err := validateSnapshotManifest(cfg, specs, snapshotManifest{Version: 2})
		if err == nil || !strings.Contains(err.Error(), "unsupported snapshot archive version 2") {
			t.Fatalf("unexpected version error: %v", err)
		}
	})

	t.Run("rejects service mismatches", func(t *testing.T) {
		err := validateSnapshotManifest(cfg, specs, snapshotManifest{
			Version:   1,
			StackName: cfg.Stack.Name,
			Volumes: []snapshotVolumeRecord{
				{Service: "postgres", Archive: "volumes/postgres.tar"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "snapshot services do not match the current stack") {
			t.Fatalf("unexpected service mismatch error: %v", err)
		}
	})

	t.Run("rejects missing archive paths", func(t *testing.T) {
		err := validateSnapshotManifest(cfg, specs, snapshotManifest{
			Version:   1,
			StackName: cfg.Stack.Name,
			Volumes: []snapshotVolumeRecord{
				{Service: "postgres", Archive: ""},
				{Service: "redis", Archive: "volumes/redis.tar"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "snapshot entry for postgres is missing its archive path") {
			t.Fatalf("unexpected missing-archive error: %v", err)
		}
	})

	t.Run("rejects duplicate archive entries", func(t *testing.T) {
		err := validateSnapshotManifest(cfg, specs, snapshotManifest{
			Version:   1,
			StackName: cfg.Stack.Name,
			Volumes: []snapshotVolumeRecord{
				{Service: "postgres", Archive: "volumes/shared.tar"},
				{Service: "redis", Archive: "volumes/shared.tar"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "snapshot archive manifest contains duplicate volume entries") {
			t.Fatalf("unexpected duplicate-entry error: %v", err)
		}
	})

	t.Run("allows cross-stack restores when services match", func(t *testing.T) {
		err := validateSnapshotManifest(cfg, specs, snapshotManifest{
			Version:   1,
			StackName: "qa-stack",
			Volumes: []snapshotVolumeRecord{
				{Service: "postgres", Archive: "volumes/postgres.tar"},
				{Service: "redis", Archive: "volumes/redis.tar"},
			},
		})
		if err != nil {
			t.Fatalf("expected cross-stack manifest to validate, got %v", err)
		}
	})
}

func TestValidateSnapshotArchivePayloadsCoversMissingStatAndDirectoryCases(t *testing.T) {
	manifest := snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
		Volumes: []snapshotVolumeRecord{
			{Service: "postgres", Archive: "volumes/postgres.tar"},
		},
	}

	t.Run("rejects missing payload mappings", func(t *testing.T) {
		err := validateSnapshotArchivePayloads(manifest, map[string]string{})
		if err == nil || !strings.Contains(err.Error(), "snapshot archive is missing payload volumes/postgres.tar for postgres") {
			t.Fatalf("unexpected missing-payload error: %v", err)
		}
	})

	t.Run("rejects stat failures", func(t *testing.T) {
		err := validateSnapshotArchivePayloads(manifest, map[string]string{
			"volumes/postgres.tar": filepath.Join(t.TempDir(), "missing.tar"),
		})
		if err == nil || !strings.Contains(err.Error(), "inspect snapshot payload volumes/postgres.tar for postgres") {
			t.Fatalf("unexpected stat error: %v", err)
		}
	})

	t.Run("rejects directory payloads", func(t *testing.T) {
		err := validateSnapshotArchivePayloads(manifest, map[string]string{
			"volumes/postgres.tar": t.TempDir(),
		})
		if err == nil || !strings.Contains(err.Error(), "snapshot payload volumes/postgres.tar for postgres is a directory") {
			t.Fatalf("unexpected directory-payload error: %v", err)
		}
	})

	t.Run("accepts extracted files", func(t *testing.T) {
		payloadPath := filepath.Join(t.TempDir(), "postgres.tar")
		if err := os.WriteFile(payloadPath, []byte("snapshot-payload"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		if err := validateSnapshotArchivePayloads(manifest, map[string]string{
			"volumes/postgres.tar": payloadPath,
		}); err != nil {
			t.Fatalf("validateSnapshotArchivePayloads returned error: %v", err)
		}
	})
}
