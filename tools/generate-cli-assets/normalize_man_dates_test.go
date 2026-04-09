package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeManDatesSurfacesReadErrors(t *testing.T) {
	rootDir := t.TempDir()
	manDir := filepath.Join(rootDir, "docs", "man")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	broken := filepath.Join(manDir, "stackctl-broken.1")
	if err := os.Symlink(filepath.Join(rootDir, "missing.1"), broken); err != nil {
		t.Fatalf("Symlink returned error: %v", err)
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatalf("OpenRoot returned error: %v", err)
	}
	defer func() { _ = root.Close() }()

	if err := normalizeManDates(root, filepath.Join("docs", "man")); err == nil {
		t.Fatal("expected normalizeManDates to surface broken-symlink read errors")
	}
}
