package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogOutputDisablesLoggingWhenConfiguredFileCannotBeOpened(t *testing.T) {
	ResetForTests()
	t.Cleanup(ResetForTests)

	blockingParent := filepath.Join(t.TempDir(), "logs")
	if err := os.WriteFile(blockingParent, []byte("blocking file"), 0o600); err != nil {
		t.Fatalf("write blocking parent: %v", err)
	}
	t.Setenv(EnvLogFile, filepath.Join(blockingParent, "stackctl.log"))

	writer, closer, enabled := logOutput()
	if enabled {
		t.Fatal("expected logging to stay disabled when the configured file cannot be opened")
	}
	if closer != nil {
		t.Fatalf("expected no closer when log file open fails, got %T", closer)
	}
	if writer != io.Discard {
		t.Fatalf("expected logOutput to fall back to io.Discard, got %T", writer)
	}
}

func TestOpenLogFileRejectsInvalidTargets(t *testing.T) {
	if _, err := openLogFile("   "); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty-target openLogFile error, got %v", err)
	}
}

func TestResolveLogFilePathRejectsRootDirectoryTargets(t *testing.T) {
	_, err := resolveLogFilePath(string(filepath.Separator))
	if err == nil || !strings.Contains(err.Error(), "must point to a file") {
		t.Fatalf("expected root-directory path error, got %v", err)
	}
}
