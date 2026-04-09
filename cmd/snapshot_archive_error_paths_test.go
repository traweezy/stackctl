package cmd

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestSnapshotArchiveErrorPaths(t *testing.T) {
	t.Run("writeSnapshotArchive surfaces manifest marshal failures", func(t *testing.T) {
		originalMarshal := marshalSnapshotManifestJSON
		t.Cleanup(func() { marshalSnapshotManifestJSON = originalMarshal })

		expectedErr := errors.New("marshal failed")
		marshalSnapshotManifestJSON = func(any, string, string) ([]byte, error) { return nil, expectedErr }

		err := writeSnapshotArchive(filepath.Join(t.TempDir(), "snapshot.tar"), snapshotManifest{Version: 1, StackName: "dev-stack"}, nil)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected manifest marshal error %v, got %v", expectedErr, err)
		}
	})

	t.Run("writeSnapshotArchive surfaces manifest entry failures", func(t *testing.T) {
		originalWriteManifest := writeSnapshotManifestEntry
		t.Cleanup(func() { writeSnapshotManifestEntry = originalWriteManifest })

		expectedErr := errors.New("manifest entry failed")
		writeSnapshotManifestEntry = func(*tar.Writer, []byte) error { return expectedErr }

		err := writeSnapshotArchive(filepath.Join(t.TempDir(), "snapshot.tar"), snapshotManifest{Version: 1, StackName: "dev-stack"}, nil)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected manifest entry error %v, got %v", expectedErr, err)
		}
	})

	t.Run("writeSnapshotArchive surfaces tar writer finalization failures", func(t *testing.T) {
		originalCloseWriter := closeSnapshotArchiveWriter
		t.Cleanup(func() { closeSnapshotArchiveWriter = originalCloseWriter })

		expectedErr := errors.New("close failed")
		closeSnapshotArchiveWriter = func(*tar.Writer) error { return expectedErr }

		err := writeSnapshotArchive(filepath.Join(t.TempDir(), "snapshot.tar"), snapshotManifest{Version: 1, StackName: "dev-stack"}, nil)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected tar writer close error %v, got %v", expectedErr, err)
		}
	})

	t.Run("readSnapshotArchive surfaces extraction-root failures", func(t *testing.T) {
		archivePath := writeSnapshotFixtureArchive(t, snapshotFixtureConfig())

		originalOpenExtractionRoot := openSnapshotExtractionRoot
		t.Cleanup(func() { openSnapshotExtractionRoot = originalOpenExtractionRoot })

		expectedErr := errors.New("open extraction root failed")
		openSnapshotExtractionRoot = func(string) (*os.Root, error) { return nil, expectedErr }

		_, _, cleanup, err := readSnapshotArchive(archivePath)
		if cleanup != nil {
			defer cleanup()
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected extraction-root error %v, got %v", expectedErr, err)
		}
	})

	t.Run("readSnapshotArchive surfaces extracted-file creation failures", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "conflicting-entry.tar")
		writeSnapshotTarFixture(t, archivePath, []snapshotFixtureRawEntry{
			{
				name:      "manifest.json",
				sizeField: snapshotTarSizeField(int64(len(snapshotManifestFixtureBytes(t)))),
				payload:   snapshotManifestFixtureBytes(t),
				pad:       true,
			},
			{
				name:      "volumes/redis.tar",
				sizeField: snapshotTarSizeField(7),
				payload:   []byte("payload"),
				pad:       true,
			},
			{
				name:      "volumes",
				sizeField: snapshotTarSizeField(7),
				payload:   []byte("payload"),
				pad:       true,
			},
		}, true)

		_, _, cleanup, err := readSnapshotArchive(archivePath)
		if cleanup != nil {
			defer cleanup()
		}
		if err == nil || !strings.Contains(err.Error(), "create extracted snapshot file volumes") {
			t.Fatalf("expected extracted-file creation error, got %v", err)
		}
	})

	t.Run("readSnapshotArchive surfaces payload extraction failures", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "truncated-entry.tar")
		writeSnapshotTarFixture(t, archivePath, []snapshotFixtureRawEntry{
			{
				name:      "manifest.json",
				sizeField: snapshotTarSizeField(int64(len(snapshotManifestFixtureBytes(t)))),
				payload:   snapshotManifestFixtureBytes(t),
				pad:       true,
			},
			{
				name:      "volumes/redis.tar",
				sizeField: snapshotTarSizeField(10),
				payload:   []byte("short"),
			},
		}, false)

		_, _, cleanup, err := readSnapshotArchive(archivePath)
		if cleanup != nil {
			defer cleanup()
		}
		if err == nil || !strings.Contains(err.Error(), "extract snapshot entry volumes/redis.tar") {
			t.Fatalf("expected extracted-entry copy error, got %v", err)
		}
	})

	t.Run("readSnapshotArchive surfaces extracted-file close failures", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "close-entry.tar")
		writeSnapshotTarFixture(t, archivePath, []snapshotFixtureRawEntry{
			{
				name:      "manifest.json",
				sizeField: snapshotTarSizeField(int64(len(snapshotManifestFixtureBytes(t)))),
				payload:   snapshotManifestFixtureBytes(t),
				pad:       true,
			},
			{
				name:      "volumes/redis.tar",
				sizeField: snapshotTarSizeField(7),
				payload:   []byte("payload"),
				pad:       true,
			},
		}, true)

		originalCloseExtractedFile := closeExtractedSnapshotFile
		t.Cleanup(func() { closeExtractedSnapshotFile = originalCloseExtractedFile })

		expectedErr := errors.New("close failed")
		closeExtractedSnapshotFile = func(*os.File) error { return expectedErr }

		_, _, cleanup, err := readSnapshotArchive(archivePath)
		if cleanup != nil {
			defer cleanup()
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected extracted-file close error %v, got %v", expectedErr, err)
		}
	})

	t.Run("writeTarFile surfaces source open failures", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("file permission semantics do not reliably block opens on Windows")
		}

		payloadPath := filepath.Join(t.TempDir(), "payload.tar")
		if err := os.WriteFile(payloadPath, []byte("payload"), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}
		if err := os.Chmod(payloadPath, 0); err != nil {
			t.Fatalf("chmod payload: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(payloadPath, 0o600) })

		err := writeTarFile(tar.NewWriter(io.Discard), "volumes/postgres.tar", payloadPath)
		if err == nil {
			t.Fatal("expected source-open failure")
		}
	})
}

type snapshotFixtureRawEntry struct {
	name      string
	sizeField []byte
	payload   []byte
	pad       bool
}

func snapshotFixtureConfig() configpkg.Config {
	cfg := configpkg.Default()
	cfg.Setup.IncludeRedis = false
	cfg.Setup.IncludeNATS = false
	cfg.Setup.IncludeSeaweedFS = false
	cfg.Setup.IncludeMeilisearch = false
	cfg.Setup.IncludePgAdmin = false
	cfg.ApplyDerivedFields()
	return cfg
}

func writeSnapshotFixtureArchive(t *testing.T, cfg configpkg.Config) string {
	t.Helper()

	specs := persistentVolumeSpecs(cfg)
	if len(specs) != 1 {
		t.Fatalf("expected a single persistent volume, got %+v", specs)
	}

	path := filepath.Join(t.TempDir(), "snapshot.tar")
	payloadPath := filepath.Join(t.TempDir(), specs[0].ServiceKey+".tar")
	if err := os.WriteFile(payloadPath, []byte(specs[0].VolumeName), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	manifest := snapshotManifest{
		Version:   1,
		StackName: cfg.Stack.Name,
		Volumes: []snapshotVolumeRecord{{
			Service:    specs[0].ServiceKey,
			SourceName: specs[0].VolumeName,
			Archive:    specs[0].ArchiveEntry,
		}},
	}
	if err := writeSnapshotArchive(path, manifest, map[string]string{specs[0].ArchiveEntry: payloadPath}); err != nil {
		t.Fatalf("write snapshot archive: %v", err)
	}

	return path
}

func snapshotManifestFixtureBytes(t *testing.T) []byte {
	t.Helper()

	data, err := json.Marshal(snapshotManifest{
		Version:   1,
		StackName: "dev-stack",
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return data
}

func snapshotTarSizeField(size int64) []byte {
	return []byte(fmt.Sprintf("%011o\x00", size))
}

func writeSnapshotTarFixture(t *testing.T, path string, entries []snapshotFixtureRawEntry, writeEndBlocks bool) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer func() { _ = file.Close() }()

	for _, entry := range entries {
		header := snapshotTarHeaderBlock(entry.name, entry.sizeField)
		if _, err := file.Write(header[:]); err != nil {
			t.Fatalf("write raw header for %s: %v", entry.name, err)
		}
		if len(entry.payload) > 0 {
			if _, err := file.Write(entry.payload); err != nil {
				t.Fatalf("write raw payload for %s: %v", entry.name, err)
			}
		}
		if entry.pad {
			padding := (512 - (len(entry.payload) % 512)) % 512
			if padding > 0 {
				if _, err := file.Write(make([]byte, padding)); err != nil {
					t.Fatalf("write raw padding for %s: %v", entry.name, err)
				}
			}
		}
	}

	if writeEndBlocks {
		if _, err := file.Write(make([]byte, 1024)); err != nil {
			t.Fatalf("write end blocks: %v", err)
		}
	}
}

func snapshotTarHeaderBlock(name string, sizeField []byte) [512]byte {
	var block [512]byte
	copy(block[0:100], []byte(name))
	copy(block[100:108], []byte("0000600\x00"))
	copy(block[108:116], []byte("0000000\x00"))
	copy(block[116:124], []byte("0000000\x00"))
	copy(block[124:136], sizeField)
	copy(block[136:148], []byte("00000000000\x00"))
	for idx := 148; idx < 156; idx++ {
		block[idx] = ' '
	}
	block[156] = '0'
	copy(block[257:263], []byte("ustar "))
	copy(block[263:265], []byte(" \x00"))

	var checksum int64
	for _, value := range block {
		checksum += int64(value)
	}
	copy(block[148:156], []byte(fmt.Sprintf("%06o\x00 ", checksum)))

	return block
}
