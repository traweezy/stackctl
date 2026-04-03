package system

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenURLRunsPlatformOpener(t *testing.T) {
	opener := openCommandForRuntime(t)
	dir := t.TempDir()
	logPath := filepath.Join(dir, "open.log")
	writeServicesTestScript(t, filepath.Join(dir, opener), "#!/bin/sh\nprintf '%s\\n' \"$*\" > \""+logPath+"\"\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	if err := OpenURL(context.Background(), Runner{}, "https://example.com/docs"); err != nil {
		t.Fatalf("OpenURL returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read opener log: %v", err)
	}
	if strings.TrimSpace(string(data)) != "https://example.com/docs" {
		t.Fatalf("unexpected opener arguments: %q", string(data))
	}
}

func TestOpenURLReturnsErrorWithoutSupportedOpener(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := OpenURL(context.Background(), Runner{}, "https://example.com")
	if err == nil {
		t.Fatal("expected OpenURL to return an error when no opener exists")
	}
	if !strings.Contains(err.Error(), "no supported browser opener found") {
		t.Fatalf("unexpected OpenURL error: %v", err)
	}
}

func TestCockpitStatusDetectsActiveSocket(t *testing.T) {
	dir := t.TempDir()
	writeServicesTestScript(t, filepath.Join(dir, "systemctl"), `#!/bin/sh
set -eu
case "$1" in
  list-unit-files)
    printf 'cockpit.socket enabled\n'
    ;;
  is-active)
    printf 'active\n'
    ;;
  *)
    echo "unexpected systemctl args: $*" >&2
    exit 1
    ;;
esac
`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	state := CockpitStatus(context.Background())
	if !state.Installed || !state.Active || state.State != "active" {
		t.Fatalf("unexpected cockpit state: %+v", state)
	}
}

func TestPodmanComposeAvailableUsesPodmanComposeProviderFallback(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman-compose-provider.log")
	writeServicesTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf '%s\\n' \"${PODMAN_COMPOSE_PROVIDER:-}\" > \""+logPath+"\"\nexit 0\n")
	writeExecutable(t, filepath.Join(dir, "podman-compose"))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "")

	if !PodmanComposeAvailable(context.Background()) {
		t.Fatal("expected PodmanComposeAvailable to succeed")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read podman compose log: %v", err)
	}
	if strings.TrimSpace(string(data)) != "podman-compose" {
		t.Fatalf("unexpected compose provider env: %q", string(data))
	}
}

func TestAnyContainerExistsFindsExistingContainer(t *testing.T) {
	dir := t.TempDir()
	writeServicesTestScript(t, filepath.Join(dir, "podman"), `#!/bin/sh
set -eu
if [ "$1" != "container" ] || [ "$2" != "exists" ]; then
  echo "unexpected podman args: $*" >&2
  exit 1
fi
if [ "$3" = "redis" ]; then
  exit 0
fi
exit 1
`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	exists, err := AnyContainerExists(context.Background(), []string{"postgres", "redis"})
	if err != nil {
		t.Fatalf("AnyContainerExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected AnyContainerExists to detect the redis container")
	}
}

func TestEnableCockpitRunsEnableNowCommand(t *testing.T) {
	originalEUID := currentEUID
	currentEUID = func() int { return 1000 }
	t.Cleanup(func() { currentEUID = originalEUID })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "sudo.log")
	writeServicesTestScript(t, filepath.Join(dir, "sudo"), "#!/bin/sh\nprintf '%s\\n' \"$*\" > \""+logPath+"\"\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	if err := EnableCockpit(context.Background(), Runner{}); err != nil {
		t.Fatalf("EnableCockpit returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read sudo log: %v", err)
	}
	if strings.TrimSpace(string(data)) != "systemctl enable --now cockpit.socket" {
		t.Fatalf("unexpected sudo arguments: %q", string(data))
	}
}

func openCommandForRuntime(t *testing.T) string {
	t.Helper()

	switch runtime.GOOS {
	case "linux":
		return "xdg-open"
	case "darwin":
		return "open"
	default:
		t.Skipf("OpenURL test does not support GOOS=%s", runtime.GOOS)
		return ""
	}
}

func writeServicesTestScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write services test script %s: %v", path, err)
	}
}
