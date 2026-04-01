package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateExecutableAcceptsSupportedCommands(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"podman", "systemctl", "sysctl", "sudo", "xdg-open", "open"} {
		if err := validateExecutable(name); err != nil {
			t.Fatalf("validateExecutable(%q) returned error: %v", name, err)
		}
	}
}

func TestValidateExecutableRejectsUnexpectedCommands(t *testing.T) {
	t.Parallel()

	if err := validateExecutable("bash"); err == nil {
		t.Fatal("expected validateExecutable to reject unsupported commands")
	}
}

func TestCaptureResultWithEnvCapturesExitCodeAndMergedEnv(t *testing.T) {
	dir := t.TempDir()
	writeExecTestScript(t, filepath.Join(dir, "podman"), `#!/bin/sh
set -eu
printf '%s\n' "${STACKCTL_TEST_VALUE:-missing}"
printf 'compose failed\n' >&2
exit 7
`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	result, err := CaptureResultWithEnv(
		context.Background(),
		"",
		[]string{"STACKCTL_TEST_VALUE=present"},
		"podman",
		"compose",
		"version",
	)
	if err != nil {
		t.Fatalf("CaptureResultWithEnv returned error: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("unexpected exit code: %+v", result)
	}
	if strings.TrimSpace(result.Stdout) != "present" {
		t.Fatalf("unexpected stdout: %q", result.Stdout)
	}
	if strings.TrimSpace(result.Stderr) != "compose failed" {
		t.Fatalf("unexpected stderr: %q", result.Stderr)
	}
}

func TestRunnerCaptureReturnsHelpfulNonZeroOutput(t *testing.T) {
	dir := t.TempDir()
	writeExecTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf 'podman compose failed\\n' >&2\nexit 9\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := (Runner{}).Capture(context.Background(), "", "podman", "compose", "version")
	if err == nil {
		t.Fatal("expected Runner.Capture to return an error")
	}
	if !strings.Contains(err.Error(), "podman compose failed") {
		t.Fatalf("unexpected capture error: %v", err)
	}
}

func TestRunExternalCommandRejectsEmptyCommand(t *testing.T) {
	if err := RunExternalCommand(context.Background(), Runner{}, "", nil); err == nil {
		t.Fatal("expected RunExternalCommand to reject an empty command slice")
	}
}

func TestRunExternalCommandRunsProvidedExecutable(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "external.log")
	writeExecTestScript(t, filepath.Join(dir, "external-tool"), "#!/bin/sh\nprintf '%s\\n' \"$*\" > \""+logPath+"\"\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	if err := RunExternalCommand(context.Background(), Runner{}, "", []string{"external-tool", "hello", "world"}); err != nil {
		t.Fatalf("RunExternalCommand returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read external command log: %v", err)
	}
	if strings.TrimSpace(string(data)) != "hello world" {
		t.Fatalf("unexpected external command output: %q", string(data))
	}
}

func TestMergeEnvOverridesExistingValues(t *testing.T) {
	merged := mergeEnv([]string{"FOO=old", "BAR=keep"}, []string{"FOO=new", "BAZ=add"})
	got := strings.Join(merged, ",")

	for _, fragment := range []string{"BAR=keep", "FOO=new", "BAZ=add"} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("merged env is missing %q: %q", fragment, got)
		}
	}
	if strings.Contains(got, "FOO=old") {
		t.Fatalf("merged env should replace overridden values: %q", got)
	}
}

func writeExecTestScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write exec test script %s: %v", path, err)
	}
}
