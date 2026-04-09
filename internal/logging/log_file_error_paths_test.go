package logging

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFileErrorPaths(t *testing.T) {
	t.Run("openLogFile surfaces open-root failures", func(t *testing.T) {
		originalOpenLogRoot := openLogRoot
		t.Cleanup(func() { openLogRoot = originalOpenLogRoot })

		expectedErr := errors.New("open root failed")
		openLogRoot = func(string) (*os.Root, error) { return nil, expectedErr }

		path := filepath.Join(t.TempDir(), "stackctl.log")
		if _, err := openLogFile(path); !errors.Is(err, expectedErr) {
			t.Fatalf("expected openLogFile to surface %v, got %v", expectedErr, err)
		}
	})

	t.Run("resolveLogFilePath surfaces filepath.Abs failures when cwd disappears", func(t *testing.T) {
		originalWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd returned error: %v", err)
		}

		badWD := t.TempDir()
		if err := os.Chdir(badWD); err != nil {
			t.Fatalf("Chdir returned error: %v", err)
		}
		if err := os.RemoveAll(badWD); err != nil {
			t.Fatalf("RemoveAll returned error: %v", err)
		}
		t.Cleanup(func() { _ = os.Chdir(originalWD) })

		if _, err := resolveLogFilePath("stackctl.log"); err == nil || !strings.Contains(err.Error(), "getwd") {
			t.Fatalf("expected resolveLogFilePath to surface cwd resolution errors, got %v", err)
		}
	})

	t.Run("resolveLogFilePath rejects cleaned paths that still resolve to the root directory", func(t *testing.T) {
		target := string(filepath.Separator) + "tmp" + string(filepath.Separator) + ".."
		if _, err := resolveLogFilePath(target); err == nil || !strings.Contains(err.Error(), "must point to a file") {
			t.Fatalf("expected cleaned root-directory path error, got %v", err)
		}
	})
}
