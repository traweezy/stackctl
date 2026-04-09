package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkValidateSnapshotManifest(b *testing.B) {
	cfg := benchmarkCommandConfig()
	specs := persistentVolumeSpecs(cfg)
	manifest := benchmarkSnapshotManifest(specs)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateSnapshotManifest(cfg, specs, manifest); err != nil {
			b.Fatalf("validateSnapshotManifest returned error: %v", err)
		}
		benchmarkCountSink = len(manifest.Volumes)
	}
}

func BenchmarkValidateSnapshotArchivePayloads(b *testing.B) {
	cfg := benchmarkCommandConfig()
	specs := persistentVolumeSpecs(cfg)
	manifest := benchmarkSnapshotManifest(specs)
	payloadDir := b.TempDir()
	extracted := make(map[string]string, len(specs))
	for _, spec := range specs {
		payloadPath := filepath.Join(payloadDir, spec.ServiceKey+".tar")
		if err := os.WriteFile(payloadPath, []byte(spec.ServiceKey), 0o600); err != nil {
			b.Fatalf("write payload fixture %s: %v", spec.ServiceKey, err)
		}
		extracted[spec.ArchiveEntry] = payloadPath
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateSnapshotArchivePayloads(manifest, extracted); err != nil {
			b.Fatalf("validateSnapshotArchivePayloads returned error: %v", err)
		}
		benchmarkCountSink = len(extracted)
	}
}
