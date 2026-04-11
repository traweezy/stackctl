package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderVHSDemoScriptOverridesOutputPath(t *testing.T) {
	repo := repoRoot(t)
	workspace := t.TempDir()
	fakeBin := filepath.Join(workspace, "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}

	safeName := strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(t.Name())
	tmpDir := filepath.Join(repo, "tmp", "vhs")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("create repo tmp dir: %v", err)
	}

	tapeRel := filepath.Join("tmp", "vhs", safeName+".tape")
	outputRel := filepath.Join("tmp", "vhs", safeName+".gif")
	binaryRel := filepath.Join("tmp", "vhs", safeName+"-stackctl")
	tapeAbs := filepath.Join(repo, tapeRel)
	outputAbs := filepath.Join(repo, outputRel)
	binaryAbs := filepath.Join(repo, binaryRel)
	logPath := filepath.Join(workspace, "podman.log")

	t.Cleanup(func() {
		_ = os.Remove(tapeAbs)
		_ = os.Remove(outputAbs)
		_ = os.Remove(binaryAbs)
	})

	writeTestScript(t, filepath.Join(fakeBin, "podman"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "$STACKCTL_VHS_TEST_LOG"
tape_path=""
for arg in "$@"; do
  if [[ "$arg" == /vhs/*.tape ]]; then
    tape_path="$arg"
  fi
done
if [[ -z "$tape_path" ]]; then
  echo "missing tape path" >&2
  exit 1
fi
host_tape="${STACKCTL_VHS_TEST_REPO_ROOT}${tape_path#/vhs}"
output_rel="$(awk '$1 == "Output" { print $2; exit }' "$host_tape")"
mkdir -p "${STACKCTL_VHS_TEST_REPO_ROOT}/$(dirname "$output_rel")"
printf 'rendered from %s\n' "$tape_path" > "${STACKCTL_VHS_TEST_REPO_ROOT}/${output_rel}"
`)

	if err := os.WriteFile(binaryAbs, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	if err := os.WriteFile(tapeAbs, []byte(strings.Join([]string{
		"# test tape",
		"Output tmp/vhs/original.gif",
		"Type \"./" + binaryRel + " --help\"",
		"Enter",
		"Sleep 1s",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write test tape: %v", err)
	}

	cmd := exec.Command(
		"bash",
		"scripts/render-vhs-demo.sh",
		"--engine", "podman",
		"--binary", binaryRel,
		"--tape", tapeRel,
		"--output", outputRel,
	)
	cmd.Dir = repo
	cmd.Env = append(
		os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"STACKCTL_VHS_TEST_LOG="+logPath,
		"STACKCTL_VHS_TEST_REPO_ROOT="+repo,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("render script returned error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	if _, err := os.Stat(outputAbs); err != nil {
		t.Fatalf("expected rendered output %s: %v", outputAbs, err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read engine log: %v", err)
	}
	logText := string(logData)
	if !strings.Contains(logText, "ghcr.io/charmbracelet/vhs:v0.11.0") {
		t.Fatalf("expected pinned image in engine log:\n%s", logText)
	}
	if !strings.Contains(logText, "--userns keep-id") {
		t.Fatalf("expected podman keep-id args in engine log:\n%s", logText)
	}
	if !strings.Contains(logText, "-e HOME=/tmp/vhs-home") {
		t.Fatalf("expected ephemeral container HOME in engine log:\n%s", logText)
	}
	if !strings.Contains(logText, "/vhs/tmp/vhs/") {
		t.Fatalf("expected repo-mounted tape path in engine log:\n%s", logText)
	}
	if !strings.Contains(stdout.String(), "Rendered VHS demo: "+outputAbs) {
		t.Fatalf("expected rendered output message in stdout:\n%s", stdout.String())
	}
}

func TestRenderVHSDemoScriptRejectsOutputOutsideRepo(t *testing.T) {
	repo := repoRoot(t)
	outsideOutput := filepath.Join(t.TempDir(), "outside.gif")

	cmd := exec.Command("bash", "scripts/render-vhs-demo.sh", "--output", outsideOutput)
	cmd.Dir = repo

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected outside-repo output to fail\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Path must stay under the repo root") {
		t.Fatalf("expected repo-root guard in stderr:\n%s", stderr.String())
	}
}
