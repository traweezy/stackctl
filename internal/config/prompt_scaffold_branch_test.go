package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func overflowPromptFile() *os.File {
	return os.NewFile(^uintptr(0), "overflow")
}

func TestPromptAndScaffoldCoverageBatchSix(t *testing.T) {
	t.Run("prompt helpers handle overflowing descriptors", func(t *testing.T) {
		if !shouldUsePlainWizard(overflowPromptFile(), os.Stdout) {
			t.Fatal("expected invalid input descriptor to force the plain wizard")
		}
		if !shouldUsePlainWizard(os.Stdin, overflowPromptFile()) {
			t.Fatal("expected invalid output descriptor to force the plain wizard")
		}
		if _, ok := terminalFD(overflowPromptFile()); ok {
			t.Fatal("expected terminalFD to reject an overflowing descriptor")
		}
		if err := clearWizardScreen(overflowPromptFile()); err != nil {
			t.Fatalf("clearWizardScreen returned error: %v", err)
		}
	})

	t.Run("askStackDir propagates askString failures", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("value\n"),
			out:    &failOnSubstringWriter{err: errors.New("write boom")},
		}
		if _, err := session.askStackDir("/tmp/stackctl"); !errors.Is(err, session.out.(*failOnSubstringWriter).err) {
			t.Fatalf("expected askStackDir to surface the askString write error, got %v", err)
		}
	})

	t.Run("scaffold helpers surface additional filesystem errors", func(t *testing.T) {
		t.Run("ScaffoldManagedStack rejects invalid managed stack names", func(t *testing.T) {
			cfg := Default()
			cfg.Stack.Name = "INVALID!"
			_, err := ScaffoldManagedStack(cfg, false)
			if err == nil || !strings.Contains(err.Error(), "stack name") {
				t.Fatalf("expected invalid stack-name error, got %v", err)
			}
		})

		t.Run("ScaffoldManagedStack surfaces create-directory failures", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("permission-based mkdir errors are unreliable when running as root")
			}

			dataRoot := t.TempDir()
			t.Setenv("XDG_DATA_HOME", dataRoot)

			cfg := Default()
			parentDir := filepath.Dir(cfg.Stack.Dir)
			if err := os.MkdirAll(parentDir, 0o700); err != nil {
				t.Fatalf("MkdirAll returned error: %v", err)
			}
			if err := os.Chmod(parentDir, 0o500); err != nil {
				t.Fatalf("Chmod returned error: %v", err)
			}
			t.Cleanup(func() {
				_ = os.Chmod(parentDir, 0o700)
			})

			_, err := ScaffoldManagedStack(cfg, false)
			if err == nil || !strings.Contains(err.Error(), "create managed stack directory") {
				t.Fatalf("expected create-directory scaffold error, got %v", err)
			}
		})

		t.Run("ScaffoldManagedStack surfaces inspect-directory failures", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("permission-based stat errors are unreliable when running as root")
			}

			dataRoot := t.TempDir()
			t.Setenv("XDG_DATA_HOME", dataRoot)

			cfg := Default()
			stacksDir := filepath.Dir(cfg.Stack.Dir)
			if err := os.MkdirAll(stacksDir, 0o700); err != nil {
				t.Fatalf("MkdirAll returned error: %v", err)
			}
			if err := os.Chmod(stacksDir, 0o000); err != nil {
				t.Fatalf("Chmod returned error: %v", err)
			}
			t.Cleanup(func() {
				_ = os.Chmod(stacksDir, 0o700)
			})

			_, err := ScaffoldManagedStack(cfg, false)
			if err == nil || !strings.Contains(err.Error(), "inspect managed stack directory") {
				t.Fatalf("expected inspect-directory scaffold error, got %v", err)
			}
		})

		t.Run("scaffoldFileMissing returns underlying stat errors", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("permission-based stat errors are unreliable when running as root")
			}

			baseDir := t.TempDir()
			protectedDir := filepath.Join(baseDir, "protected")
			if err := os.MkdirAll(protectedDir, 0o700); err != nil {
				t.Fatalf("MkdirAll returned error: %v", err)
			}
			if err := os.Chmod(protectedDir, 0o000); err != nil {
				t.Fatalf("Chmod returned error: %v", err)
			}
			t.Cleanup(func() {
				_ = os.Chmod(protectedDir, 0o700)
			})

			_, err := scaffoldFileMissing(filepath.Join(protectedDir, "compose.yaml"))
			if err == nil {
				t.Fatal("expected scaffoldFileMissing to surface a stat error")
			}
		})

		t.Run("writeScaffoldFile surfaces write failures", func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("permission-based write errors are unreliable when running as root")
			}

			readOnlyDir := filepath.Join(t.TempDir(), "readonly")
			if err := os.MkdirAll(readOnlyDir, 0o500); err != nil {
				t.Fatalf("MkdirAll returned error: %v", err)
			}

			_, err := writeScaffoldFile(filepath.Join(readOnlyDir, "compose.yaml"), []byte("services: {}\n"), false)
			if err == nil {
				t.Fatal("expected writeScaffoldFile to fail in a read-only directory")
			}
		})
	})
}
