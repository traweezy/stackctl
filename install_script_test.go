package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallScriptVerifiesChecksumAndInstallsBinary(t *testing.T) {
	workspace := t.TempDir()
	fixture := newInstallScriptFixture(t, workspace, true)
	installDir := filepath.Join(workspace, "bin")

	cmd := exec.Command("bash", "scripts/install.sh", "--dir", installDir, "--repo", "traweezy/stackctl")
	cmd.Env = fixture.env()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("install script returned error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	installedPath := filepath.Join(installDir, "stackctl")
	data, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(data) != fixture.binaryContent {
		t.Fatalf("unexpected installed binary contents: %q", string(data))
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Verified archive checksum.")) {
		t.Fatalf("expected checksum verification message in stdout:\n%s", stdout.String())
	}
}

func TestInstallScriptFailsOnChecksumMismatch(t *testing.T) {
	workspace := t.TempDir()
	fixture := newInstallScriptFixture(t, workspace, false)
	installDir := filepath.Join(workspace, "bin")

	cmd := exec.Command("bash", "scripts/install.sh", "--dir", installDir, "--repo", "traweezy/stackctl")
	cmd.Env = fixture.env()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected checksum mismatch to fail\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Checksum verification failed")) {
		t.Fatalf("expected checksum failure in stderr:\n%s", stderr.String())
	}
}

type installScriptFixture struct {
	workspace     string
	fakeBin       string
	archivePath   string
	checksumsPath string
	binaryPath    string
	binaryContent string
}

func newInstallScriptFixture(t *testing.T, workspace string, validChecksum bool) installScriptFixture {
	t.Helper()

	fakeBin := filepath.Join(workspace, "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}

	archivePath := filepath.Join(workspace, "stackctl_Linux_x86_64.tar.gz")
	archiveContent := []byte("stackctl-release-archive")
	if err := os.WriteFile(archivePath, archiveContent, 0o644); err != nil {
		t.Fatalf("write fake archive: %v", err)
	}

	binaryPath := filepath.Join(workspace, "stackctl")
	binaryContent := "#!/bin/sh\necho stackctl\n"
	if err := os.WriteFile(binaryPath, []byte(binaryContent), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	checksumBytes := sha256.Sum256(archiveContent)
	checksumValue := hex.EncodeToString(checksumBytes[:])
	if !validChecksum {
		checksumValue = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	checksumsPath := filepath.Join(workspace, "checksums.txt")
	checksumsContent := checksumValue + "  stackctl_Linux_x86_64.tar.gz\n"
	if err := os.WriteFile(checksumsPath, []byte(checksumsContent), 0o644); err != nil {
		t.Fatalf("write checksums file: %v", err)
	}

	writeTestScript(t, filepath.Join(fakeBin, "uname"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-m" ]]; then
  printf 'x86_64\n'
  exit 0
fi
printf 'Linux\n'
`)

	writeTestScript(t, filepath.Join(fakeBin, "curl"), `#!/usr/bin/env bash
set -euo pipefail
output=""
url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

case "$url" in
  */releases/latest)
    printf '{"tag_name":"v0.16.0"}\n'
    ;;
  */checksums.txt)
    cp "$FAKE_CHECKSUMS_PATH" "$output"
    ;;
  *.tar.gz)
    cp "$FAKE_ARCHIVE_PATH" "$output"
    ;;
  *)
    echo "unexpected url: $url" >&2
    exit 1
    ;;
esac
`)

	writeTestScript(t, filepath.Join(fakeBin, "tar"), `#!/usr/bin/env bash
set -euo pipefail
dest=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -C)
      dest="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
cp "$FAKE_BINARY_PATH" "$dest/stackctl"
`)

	writeTestScript(t, filepath.Join(fakeBin, "install"), `#!/usr/bin/env bash
set -euo pipefail
args=("$@")
src="${args[$(( $# - 2 ))]}"
dst="${args[$(( $# - 1 ))]}"
cp "$src" "$dst"
chmod 0755 "$dst"
`)

	return installScriptFixture{
		workspace:     workspace,
		fakeBin:       fakeBin,
		archivePath:   archivePath,
		checksumsPath: checksumsPath,
		binaryPath:    binaryPath,
		binaryContent: binaryContent,
	}
}

func (f installScriptFixture) env() []string {
	return append(os.Environ(),
		"PATH="+f.fakeBin+":"+os.Getenv("PATH"),
		"FAKE_ARCHIVE_PATH="+f.archivePath,
		"FAKE_CHECKSUMS_PATH="+f.checksumsPath,
		"FAKE_BINARY_PATH="+f.binaryPath,
	)
}

func writeTestScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write test script %s: %v", path, err)
	}
}
