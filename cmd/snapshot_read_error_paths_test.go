package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSnapshotArchiveRejectsTruncatedPayloads(t *testing.T) {
	dir := t.TempDir()
	payloadPath := filepath.Join(dir, "redis.tar")
	if err := os.WriteFile(payloadPath, []byte("redis-volume"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	archivePath := filepath.Join(dir, "truncated.tar")
	manifest := snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
		Volumes: []snapshotVolumeRecord{{
			Service:    "redis",
			SourceName: "redis_data",
			Archive:    "volumes/redis.tar",
		}},
	}
	if err := writePartialSnapshotArchive(archivePath, manifest, map[string]string{
		"volumes/redis.tar": payloadPath,
	}); err != nil {
		t.Fatalf("writePartialSnapshotArchive returned error: %v", err)
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if err := os.Truncate(archivePath, info.Size()-2); err != nil {
		t.Fatalf("Truncate returned error: %v", err)
	}

	_, _, cleanup, err := readSnapshotArchive(archivePath)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil || (!strings.Contains(err.Error(), "extract snapshot entry volumes/redis.tar") && !strings.Contains(err.Error(), "read snapshot archive")) {
		t.Fatalf("unexpected truncated payload error: %v", err)
	}
}

func TestReadSnapshotArchiveRejectsTruncatedManifest(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "truncated-manifest.tar")
	manifest := snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
	}
	if err := writePartialSnapshotArchive(archivePath, manifest, nil); err != nil {
		t.Fatalf("writePartialSnapshotArchive returned error: %v", err)
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	if len(manifestData) == 0 {
		t.Fatal("expected non-empty manifest data")
	}

	if err := os.Truncate(archivePath, int64(512+len(manifestData)-1)); err != nil {
		t.Fatalf("Truncate returned error: %v", err)
	}

	_, _, cleanup, err := readSnapshotArchive(archivePath)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil || !strings.Contains(err.Error(), "read snapshot manifest") {
		t.Fatalf("unexpected truncated manifest error: %v", err)
	}
}
