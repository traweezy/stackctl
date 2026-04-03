package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	charmlog "charm.land/log/v2"
)

func TestLogOutputDisabledWithoutTarget(t *testing.T) {
	ResetForTests()
	t.Cleanup(ResetForTests)

	writer, closer, enabled := logOutput()
	if enabled {
		t.Fatal("expected logging to be disabled")
	}
	if closer != nil {
		t.Fatal("expected no closer when logging is disabled")
	}
	if writer == nil {
		t.Fatal("expected a writer even when logging is disabled")
	}
}

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

func TestLogOutputUsesStderrForDashTarget(t *testing.T) {
	t.Setenv(EnvLogFile, "-")

	writer, closer, enabled := logOutput()
	if !enabled {
		t.Fatal("expected dash target to enable logging")
	}
	if closer != nil {
		t.Fatal("expected stderr target to skip closer creation")
	}
	if writer != os.Stderr {
		t.Fatalf("expected stderr writer, got %T", writer)
	}
}

func TestTUIDebugLogPathTrimsWhitespace(t *testing.T) {
	t.Setenv(EnvTUIDebugLogFile, "  /tmp/stackctl-debug.log  ")

	if got := TUIDebugLogPath(); got != "/tmp/stackctl-debug.log" {
		t.Fatalf("unexpected debug log path %q", got)
	}
}

func TestParseLevelFallsBackToInfo(t *testing.T) {
	if got := parseLevel("not-a-level"); got != charmlog.InfoLevel {
		t.Fatalf("expected invalid level to fall back to info, got %v", got)
	}
}

func TestValidateLevelAcceptsKnownLevels(t *testing.T) {
	for _, value := range []string{"", "debug", "info", "warn", "error"} {
		if err := ValidateLevel(value); err != nil {
			t.Fatalf("expected %q to validate: %v", value, err)
		}
	}
}

func TestValidateLevelRejectsUnknownLevel(t *testing.T) {
	if err := ValidateLevel("loud"); err == nil {
		t.Fatal("expected invalid level error")
	}
}

func TestValidateFormatAcceptsKnownFormats(t *testing.T) {
	for _, value := range []string{"", "text", "json", "logfmt"} {
		if err := ValidateFormat(value); err != nil {
			t.Fatalf("expected %q to validate: %v", value, err)
		}
	}
}

func TestValidateFormatRejectsUnknownFormat(t *testing.T) {
	if err := ValidateFormat("yaml"); err == nil {
		t.Fatal("expected invalid format error")
	}
}

func TestResolveLogFilePathRejectsEmptyTarget(t *testing.T) {
	_, err := resolveLogFilePath("  ")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty path error, got %v", err)
	}
}

func TestResolveLogFilePathRejectsReservedDashTarget(t *testing.T) {
	_, err := resolveLogFilePath("-")
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved path error, got %v", err)
	}
}

func TestResolveLogFilePathRejectsDirectoryTarget(t *testing.T) {
	target := t.TempDir() + string(filepath.Separator)

	_, err := resolveLogFilePath(target)
	if err == nil || !strings.Contains(err.Error(), "must point to a file") {
		t.Fatalf("expected directory target error, got %v", err)
	}
}
