package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWatchCIScriptTracksHeadRunToSuccess(t *testing.T) {
	workspace := t.TempDir()
	fakeBin := filepath.Join(workspace, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}

	stateDir := filepath.Join(workspace, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	writeExecutable(t, filepath.Join(fakeBin, "gh"), `#!/usr/bin/env bash
set -euo pipefail

state_dir="${FAKE_GH_STATE_DIR:?}"
list_count_file="$state_dir/list-count"
view_count_file="$state_dir/view-count"

read_count() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo 0
    return
  fi
  cat "$path"
}

write_count() {
  printf '%s' "$2" >"$1"
}

if [[ "${1:-}" == "run" && "${2:-}" == "list" ]]; then
  count="$(read_count "$list_count_file")"
  write_count "$list_count_file" "$((count + 1))"
  printf '12345\n'
  exit 0
fi

if [[ "${1:-}" == "run" && "${2:-}" == "view" ]]; then
  count="$(read_count "$view_count_file")"
  write_count "$view_count_file" "$((count + 1))"
  if (( count == 0 )); then
    printf 'Test CI Run\037abc1234\037in_progress\037\0373\0371\0371\0371\0370\n'
    exit 0
  fi
  printf 'Test CI Run\037abc1234\037completed\037success\0373\0373\0370\0370\0370\n'
  exit 0
fi

printf 'unexpected gh invocation: %s\n' "$*" >&2
exit 1
`)

	cmd := exec.Command("bash", "scripts/watch-ci.sh", "--branch", "master", "--sha", "abc1234", "--interval", "0")
	cmd.Env = append(
		filteredEnvWithoutPath(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH_STATE_DIR="+stateDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("watch-ci script failed: %v\noutput:\n%s", err, string(output))
	}

	text := string(output)
	if !strings.Contains(text, "Tracking run 12345") {
		t.Fatalf("expected tracking message in output:\n%s", text)
	}
	if !strings.Contains(text, "status=in_progress") {
		t.Fatalf("expected in-progress snapshot in output:\n%s", text)
	}
	if !strings.Contains(text, "completed=1/3 running=1 queued=1") {
		t.Fatalf("expected stable job counters in output:\n%s", text)
	}
	if !strings.Contains(text, "Run 12345 finished with conclusion=success") {
		t.Fatalf("expected success conclusion in output:\n%s", text)
	}
}

func TestWatchCIScriptLatestBranchSwitchesToNewerRun(t *testing.T) {
	workspace := t.TempDir()
	fakeBin := filepath.Join(workspace, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}

	stateDir := filepath.Join(workspace, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	writeExecutable(t, filepath.Join(fakeBin, "gh"), `#!/usr/bin/env bash
set -euo pipefail

state_dir="${FAKE_GH_STATE_DIR:?}"
list_count_file="$state_dir/list-count"

read_count() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo 0
    return
  fi
  cat "$path"
}

write_count() {
  printf '%s' "$2" >"$1"
}

if [[ "${1:-}" == "run" && "${2:-}" == "list" ]]; then
  count="$(read_count "$list_count_file")"
  write_count "$list_count_file" "$((count + 1))"
  if (( count == 0 )); then
    printf '111\n'
    exit 0
  fi
  printf '222\n'
  exit 0
fi

if [[ "${1:-}" == "run" && "${2:-}" == "view" ]]; then
  if [[ "${3:-}" == "111" ]]; then
    printf 'Old CI Run\037abc1234\037in_progress\037\0374\0371\0371\0372\0370\n'
    exit 0
  fi
  if [[ "${3:-}" == "222" ]]; then
    printf 'New CI Run\037def5678\037completed\037success\0374\0374\0370\0370\0370\n'
    exit 0
  fi
fi

printf 'unexpected gh invocation: %s\n' "$*" >&2
exit 1
`)

	cmd := exec.Command("bash", "scripts/watch-ci.sh", "--branch", "master", "--latest-branch", "--interval", "0")
	cmd.Env = append(
		filteredEnvWithoutPath(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH_STATE_DIR="+stateDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("watch-ci script failed: %v\noutput:\n%s", err, string(output))
	}

	text := string(output)
	if !strings.Contains(text, "Tracking run 111") {
		t.Fatalf("expected initial run tracking in output:\n%s", text)
	}
	if !strings.Contains(text, "Tracking run 222") {
		t.Fatalf("expected watcher to switch to newer run in output:\n%s", text)
	}
	if !strings.Contains(text, "completed=1/4 running=1 queued=2") {
		t.Fatalf("expected stable counters for the first branch run:\n%s", text)
	}
	if !strings.Contains(text, "Run 222 finished with conclusion=success") {
		t.Fatalf("expected new run success conclusion in output:\n%s", text)
	}
}

func writeExecutable(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func filteredEnvWithoutPath() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			continue
		}
		if runtime.GOOS == "windows" && strings.HasPrefix(strings.ToUpper(entry), "PATH=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
