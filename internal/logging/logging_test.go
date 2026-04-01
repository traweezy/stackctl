package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogOutputCreatesConfiguredFile(t *testing.T) {
	ResetForTests()
	t.Cleanup(ResetForTests)

	target := filepath.Join(t.TempDir(), "logs", "stackctl.log")
	t.Setenv(EnvLogFile, target)

	writer, closer, enabled := logOutput()
	if !enabled {
		t.Fatal("expected logging to be enabled")
	}
	if closer == nil {
		t.Fatal("expected configured log file to provide a closer")
	}

	if _, err := io.WriteString(writer, "hello\n"); err != nil {
		t.Fatalf("write log output: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close log output: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read created log file: %v", err)
	}
	if got := string(data); got != "hello\n" {
		t.Fatalf("unexpected log file contents %q", got)
	}
}

func TestResolveLogFilePathRejectsDirectoryTarget(t *testing.T) {
	target := t.TempDir() + string(filepath.Separator)

	_, err := resolveLogFilePath(target)
	if err == nil || !strings.Contains(err.Error(), "must point to a file") {
		t.Fatalf("expected directory target error, got %v", err)
	}
}
