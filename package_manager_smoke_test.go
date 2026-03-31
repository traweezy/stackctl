package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageManagerSmokeRetriesZypperOnRefreshFailure(t *testing.T) {
	workspace := t.TempDir()
	fakeBin := filepath.Join(workspace, "fake-bin")
	stateDir := filepath.Join(workspace, "state")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	writePackageManagerTestScript(t, filepath.Join(fakeBin, "zypper"), `#!/usr/bin/env bash
set -euo pipefail
state_dir="${FAKE_STATE_DIR:?missing state dir}"
refresh_count_file="$state_dir/refresh-count"
install_log="$state_dir/install.log"
clean_log="$state_dir/clean.log"
args="$*"
if [[ "$args" == *" clean --all"* ]]; then
  printf 'clean\n' >>"$clean_log"
  exit 0
fi
if [[ "$args" == *" refresh --force"* ]]; then
  count=0
  if [[ -f "$refresh_count_file" ]]; then
    count="$(cat "$refresh_count_file")"
  fi
  count=$((count + 1))
  printf '%s' "$count" >"$refresh_count_file"
  if [[ "$count" -eq 1 ]]; then
    echo "Repository 'openSUSE-Tumbleweed-Oss' is invalid." >&2
    exit 104
  fi
  exit 0
fi
if [[ "$args" == *" install "* ]]; then
  printf '%s\n' "$args" >>"$install_log"
  exit 0
fi
echo "unexpected zypper args: $args" >&2
exit 1
`)

	writePackageManagerTestScript(t, filepath.Join(fakeBin, "rpm"), `#!/usr/bin/env bash
set -euo pipefail
pkg="${@: -1}"
if [[ ! -f "${FAKE_STATE_DIR}/install.log" ]]; then
  exit 1
fi
grep -q -- " $pkg" "${FAKE_STATE_DIR}/install.log"
`)

	for _, bin := range []string{"podman", "podman-compose", "skopeo", "buildah"} {
		writePackageManagerTestScript(t, filepath.Join(fakeBin, bin), "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n")
	}

	cmd := exec.Command("sh", "scripts/package-manager-smoke.sh")
	cmd.Dir = "/home/tylers/Dev/go/github.com/traweezy/stackctl"
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"FAKE_STATE_DIR="+stateDir,
		"STACKCTL_PACKAGE_MANAGER=zypper",
		"STACKCTL_EXPECT_COCKPIT=1",
		"STACKCTL_ZYPPER_ATTEMPTS=2",
		"STACKCTL_ZYPPER_RETRY_DELAY_SECONDS=0",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("package-manager smoke failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	refreshCount, err := os.ReadFile(filepath.Join(stateDir, "refresh-count"))
	if err != nil {
		t.Fatalf("read refresh count: %v", err)
	}
	if strings.TrimSpace(string(refreshCount)) != "2" {
		t.Fatalf("expected two refresh attempts, got %q", strings.TrimSpace(string(refreshCount)))
	}
	if !strings.Contains(stderr.String(), "zypper install attempt 1 failed; cleaning metadata and retrying") {
		t.Fatalf("expected retry notice in stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "package-manager smoke passed for zypper") {
		t.Fatalf("expected success message in stdout:\n%s", stdout.String())
	}
}

func TestPackageManagerSmokeFailsAfterExhaustingZypperRetries(t *testing.T) {
	workspace := t.TempDir()
	fakeBin := filepath.Join(workspace, "fake-bin")
	stateDir := filepath.Join(workspace, "state")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	writePackageManagerTestScript(t, filepath.Join(fakeBin, "zypper"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *" clean --all"* ]]; then
  exit 0
fi
if [[ "$*" == *" refresh --force"* ]]; then
  echo "Repository 'openSUSE-Tumbleweed-Oss' is invalid." >&2
  exit 104
fi
echo "unexpected zypper args: $*" >&2
exit 1
`)

	cmd := exec.Command("sh", "scripts/package-manager-smoke.sh")
	cmd.Dir = "/home/tylers/Dev/go/github.com/traweezy/stackctl"
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"FAKE_STATE_DIR="+stateDir,
		"STACKCTL_PACKAGE_MANAGER=zypper",
		"STACKCTL_EXPECT_COCKPIT=0",
		"STACKCTL_ZYPPER_ATTEMPTS=2",
		"STACKCTL_ZYPPER_RETRY_DELAY_SECONDS=0",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected zypper retry exhaustion to fail\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "zypper install failed after 2 attempts") {
		t.Fatalf("expected exhausted retry message in stderr:\n%s", stderr.String())
	}
}

func writePackageManagerTestScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write test script %s: %v", path, err)
	}
}
