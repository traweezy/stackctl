package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecCoverageBatchThree(t *testing.T) {
	t.Run("runner run rejects unsupported executables", func(t *testing.T) {
		if err := (Runner{}).Run(context.Background(), "", "bash", "-lc", "true"); err == nil {
			t.Fatal("expected Runner.Run to reject unsupported executables")
		}
	})

	t.Run("runner run merges environment before execution", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "runner.log")
		writeExecTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf '%s\\n' \"${STACKCTL_BATCH_THREE:-missing}\" > \""+logPath+"\"\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		err := (Runner{Env: []string{"STACKCTL_BATCH_THREE=present"}}).Run(context.Background(), "", "podman", "compose", "version")
		if err != nil {
			t.Fatalf("Runner.Run returned error: %v", err)
		}

		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read runner log: %v", err)
		}
		if strings.TrimSpace(string(data)) != "present" {
			t.Fatalf("expected merged env value, got %q", string(data))
		}
	})

	t.Run("run external command merges environment before execution", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "external.log")
		writeExecTestScript(t, filepath.Join(dir, "external-tool"), "#!/bin/sh\nprintf '%s\\n' \"${STACKCTL_BATCH_THREE:-missing}\" > \""+logPath+"\"\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		err := RunExternalCommand(context.Background(), Runner{Env: []string{"STACKCTL_BATCH_THREE=external"}}, "", []string{"external-tool"})
		if err != nil {
			t.Fatalf("RunExternalCommand returned error: %v", err)
		}

		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read external log: %v", err)
		}
		if strings.TrimSpace(string(data)) != "external" {
			t.Fatalf("expected merged external env value, got %q", string(data))
		}
	})

	t.Run("runner capture propagates capture errors", func(t *testing.T) {
		if _, err := (Runner{}).Capture(context.Background(), "", "bash", "-lc", "true"); err == nil {
			t.Fatal("expected Runner.Capture to propagate capture errors")
		}
	})

	t.Run("capture result with env rejects unsupported executables", func(t *testing.T) {
		if _, err := CaptureResultWithEnv(context.Background(), "", nil, "bash", "-lc", "true"); err == nil {
			t.Fatal("expected CaptureResultWithEnv to reject unsupported executables")
		}
	})

	t.Run("merge env copies the base when overrides are empty", func(t *testing.T) {
		base := []string{"FOO=bar", "BAR=baz"}
		merged := mergeEnv(base, nil)
		if strings.Join(merged, ",") != strings.Join(base, ",") {
			t.Fatalf("expected base env copy, got %+v", merged)
		}
		if len(merged) != len(base) {
			t.Fatalf("expected copied env length %d, got %d", len(base), len(merged))
		}
	})
}

func TestPackageAndServiceCoverageBatchThree(t *testing.T) {
	t.Run("cockpit status defaults blank inactive states", func(t *testing.T) {
		dir := t.TempDir()
		writeExecTestScript(t, filepath.Join(dir, "systemctl"), `#!/bin/sh
set -eu
case "$1" in
  list-unit-files)
    printf 'cockpit.socket enabled\n'
    ;;
  is-active)
    exit 3
    ;;
  *)
    exit 1
    ;;
esac
`)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		state := CockpitStatus(context.Background())
		if !state.Installed || state.Active || state.State != "inactive" {
			t.Fatalf("unexpected cockpit state: %+v", state)
		}
	})

	t.Run("zypper retry tracks install-step failures", func(t *testing.T) {
		originalEUID := currentEUID
		currentEUID = func() int { return 0 }
		t.Cleanup(func() { currentEUID = originalEUID })

		dir := t.TempDir()
		writeExecTestScript(t, filepath.Join(dir, "zypper"), `#!/bin/sh
set -eu
case "$*" in
  *' clean --all')
    exit 0
    ;;
  *' refresh --force')
    exit 0
    ;;
  *' install '*)
    echo 'install failed' >&2
    exit 17
    ;;
  *)
    exit 1
    ;;
esac
`)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		err := runZypperInstallWithRetry(context.Background(), Runner{}, []string{"podman"})
		if err == nil || !strings.Contains(err.Error(), "zypper install failed after 3 attempts") {
			t.Fatalf("expected zypper install-step failure, got %v", err)
		}
	})
}
