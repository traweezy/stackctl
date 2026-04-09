package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func TestGenerateCLIAssetsSurfacesInjectedErrors(t *testing.T) {
	restoreHooks := func(t *testing.T) {
		t.Helper()

		originalOpen := openCLIAssetsRoot
		originalClose := closeCLIAssetsRoot
		originalRootCmd := newCLIAssetsRootCommand
		originalRecreate := recreateCLIAssetsDir
		originalMarkdown := generateCLIAssetsMarkdown
		originalMan := generateCLIAssetsMan
		originalNormalize := normalizeCLIAssetsManDates
		originalCompletion := writeCLIAssetsCompletion
		t.Cleanup(func() {
			openCLIAssetsRoot = originalOpen
			closeCLIAssetsRoot = originalClose
			newCLIAssetsRootCommand = originalRootCmd
			recreateCLIAssetsDir = originalRecreate
			generateCLIAssetsMarkdown = originalMarkdown
			generateCLIAssetsMan = originalMan
			normalizeCLIAssetsManDates = originalNormalize
			writeCLIAssetsCompletion = originalCompletion
		})
	}

	withWorkdir := func(t *testing.T) {
		t.Helper()

		originalWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd returned error: %v", err)
		}
		tempDir := t.TempDir()
		if err := os.Chdir(tempDir); err != nil {
			t.Fatalf("Chdir returned error: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(originalWD)
		})
	}

	t.Run("open root error", func(t *testing.T) {
		restoreHooks(t)
		expectedErr := errors.New("open root failed")
		openCLIAssetsRoot = func(string) (*os.Root, error) { return nil, expectedErr }

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected open-root error %v, got %v", expectedErr, err)
		}
	})

	t.Run("close root error", func(t *testing.T) {
		restoreHooks(t)
		withWorkdir(t)
		expectedErr := errors.New("close root failed")
		closeCLIAssetsRoot = func(root *os.Root) error {
			_ = root.Close()
			return expectedErr
		}

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected close-root error %v, got %v", expectedErr, err)
		}
	})

	t.Run("markdown generation error", func(t *testing.T) {
		restoreHooks(t)
		withWorkdir(t)
		expectedErr := errors.New("markdown failed")
		generateCLIAssetsMarkdown = func(*cobra.Command, string, func(string) string, func(string) string) error {
			return expectedErr
		}

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected markdown error %v, got %v", expectedErr, err)
		}
	})

	t.Run("man dir recreate error", func(t *testing.T) {
		restoreHooks(t)
		withWorkdir(t)
		expectedErr := errors.New("man dir failed")
		generateCLIAssetsMarkdown = func(*cobra.Command, string, func(string) string, func(string) string) error { return nil }
		recreateCLIAssetsDir = func(root *os.Root, dir string) error {
			if dir == manDir {
				return expectedErr
			}
			return recreateDir(root, dir)
		}

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected man dir recreate error %v, got %v", expectedErr, err)
		}
	})

	t.Run("man generation error", func(t *testing.T) {
		restoreHooks(t)
		withWorkdir(t)
		expectedErr := errors.New("man generation failed")
		generateCLIAssetsMan = func(*cobra.Command, *doc.GenManHeader, string) error {
			return expectedErr
		}

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected man generation error %v, got %v", expectedErr, err)
		}
	})

	t.Run("man normalize error", func(t *testing.T) {
		restoreHooks(t)
		withWorkdir(t)
		expectedErr := errors.New("normalize failed")
		normalizeCLIAssetsManDates = func(*os.Root, string) error { return expectedErr }

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected normalize error %v, got %v", expectedErr, err)
		}
	})

	t.Run("completions dir recreate error", func(t *testing.T) {
		restoreHooks(t)
		withWorkdir(t)
		expectedErr := errors.New("completions dir failed")
		recreateCLIAssetsDir = func(root *os.Root, dir string) error {
			if dir == completionsDir {
				return expectedErr
			}
			return recreateDir(root, dir)
		}

		if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
			t.Fatalf("expected completions dir recreate error %v, got %v", expectedErr, err)
		}
	})

	for _, tc := range []struct {
		name   string
		target string
	}{
		{name: "bash completion error", target: filepath.Join(completionsDir, "stackctl.bash")},
		{name: "zsh completion error", target: filepath.Join(completionsDir, "_stackctl")},
		{name: "fish completion error", target: filepath.Join(completionsDir, "stackctl.fish")},
		{name: "powershell completion error", target: filepath.Join(completionsDir, "stackctl.ps1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreHooks(t)
			withWorkdir(t)
			expectedErr := errors.New("completion failed")
			writeCLIAssetsCompletion = func(root *os.Root, path string, generate func(io.Writer) error) error {
				if path == tc.target {
					return expectedErr
				}
				return writeCompletionFile(root, path, generate)
			}

			if err := generateCLIAssets(); !errors.Is(err, expectedErr) {
				t.Fatalf("expected completion error %v, got %v", expectedErr, err)
			}
		})
	}
}
