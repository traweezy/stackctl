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

func TestCockpitStatusHandlesUnavailableAndMissingStates(t *testing.T) {
	t.Run("systemctl unavailable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		state := CockpitStatus(context.Background())
		if state != (CockpitState{State: "systemctl unavailable"}) {
			t.Fatalf("unexpected cockpit state: %+v", state)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		dir := t.TempDir()
		writeServicesTestScript(t, filepath.Join(dir, "systemctl"), `#!/bin/sh
set -eu
case "$1" in
  list-unit-files)
    printf '0 unit files listed.\n'
    ;;
  *)
    echo "unexpected systemctl args: $*" >&2
    exit 1
    ;;
esac
`)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		state := CockpitStatus(context.Background())
		if state != (CockpitState{State: "not installed"}) {
			t.Fatalf("unexpected cockpit state: %+v", state)
		}
	})
}

func TestCockpitStatusHandlesStateErrors(t *testing.T) {
	t.Run("state unknown when is-active cannot execute", func(t *testing.T) {
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "systemctl")
		writeServicesTestScript(t, scriptPath, `#!/bin/sh
set -eu
case "$1" in
  list-unit-files)
    printf 'cockpit.socket enabled\n'
    cat > "$0" <<'EOF'
#!/no/such/interpreter
EOF
    chmod +x "$0"
    ;;
  *)
    echo "unexpected systemctl args: $*" >&2
    exit 1
    ;;
esac
`)
		t.Setenv("PATH", dir)

		state := CockpitStatus(context.Background())
		if !state.Installed || state.Active || state.State != "state unknown" {
			t.Fatalf("unexpected cockpit state: %+v", state)
		}
	})

	t.Run("inactive state falls back to stderr", func(t *testing.T) {
		dir := t.TempDir()
		writeServicesTestScript(t, filepath.Join(dir, "systemctl"), `#!/bin/sh
set -eu
case "$1" in
  list-unit-files)
    printf 'cockpit.socket enabled\n'
    ;;
  is-active)
    printf 'inactive (dead)\n' >&2
    exit 3
    ;;
  *)
    echo "unexpected systemctl args: $*" >&2
    exit 1
    ;;
esac
`)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		state := CockpitStatus(context.Background())
		if !state.Installed || state.Active || state.State != "inactive (dead)" {
			t.Fatalf("unexpected cockpit state: %+v", state)
		}
	})
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

func TestPodmanComposeAvailableReturnsFalseWhenUnavailable(t *testing.T) {
	t.Run("missing podman", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		if PodmanComposeAvailable(context.Background()) {
			t.Fatal("expected PodmanComposeAvailable to fail without podman")
		}
	})

	t.Run("non-zero compose version", func(t *testing.T) {
		dir := t.TempDir()
		writeServicesTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nexit 1\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		if PodmanComposeAvailable(context.Background()) {
			t.Fatal("expected PodmanComposeAvailable to fail on non-zero exit")
		}
	})
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

func TestAnyContainerExistsReturnsFalseForMissingOrEmptyNames(t *testing.T) {
	t.Run("missing podman", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		exists, err := AnyContainerExists(context.Background(), []string{"redis"})
		if err != nil {
			t.Fatalf("AnyContainerExists returned error: %v", err)
		}
		if exists {
			t.Fatal("expected no container detection without podman")
		}
	})

	t.Run("skips blank names and returns false", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "podman.log")
		writeServicesTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \""+logPath+"\"\nexit 1\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		exists, err := AnyContainerExists(context.Background(), []string{"", "  ", "postgres"})
		if err != nil {
			t.Fatalf("AnyContainerExists returned error: %v", err)
		}
		if exists {
			t.Fatal("expected no existing containers to be detected")
		}

		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read podman log: %v", err)
		}
		if got := strings.TrimSpace(string(data)); got != "container exists postgres" {
			t.Fatalf("unexpected podman invocation log: %q", got)
		}
	})
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
