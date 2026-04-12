package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSystemCoverageReleaseBatchRuntimeAndErrorBranches(t *testing.T) {
	t.Run("open command name detects linux opener", func(t *testing.T) {
		if got := openCommandForRuntime(t); got == "" {
			t.Fatal("expected a supported opener for the current runtime")
		}

		if got := OpenCommandName(); got == "" {
			t.Fatal("expected OpenCommandName to resolve a supported opener on the host test runtime")
		}
	})

	t.Run("cockpit status detects command execution failure", func(t *testing.T) {
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "systemctl")
		if err := os.WriteFile(scriptPath, []byte("#!/no/such/interpreter\n"), 0o755); err != nil {
			t.Fatalf("write failing systemctl script: %v", err)
		}
		t.Setenv("PATH", dir)

		state := CockpitStatus(context.Background())
		if state.State != "detection failed" {
			t.Fatalf("expected detection failure, got %+v", state)
		}
	})

	t.Run("podman compose available returns false on execution error", func(t *testing.T) {
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "podman")
		if err := os.WriteFile(scriptPath, []byte("#!/no/such/interpreter\n"), 0o755); err != nil {
			t.Fatalf("write failing podman script: %v", err)
		}
		t.Setenv("PATH", dir)

		if PodmanComposeAvailable(context.Background()) {
			t.Fatal("expected compose availability to fail when podman cannot execute")
		}
	})

	t.Run("any container exists returns capture errors", func(t *testing.T) {
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "podman")
		if err := os.WriteFile(scriptPath, []byte("#!/no/such/interpreter\n"), 0o755); err != nil {
			t.Fatalf("write failing podman script: %v", err)
		}
		t.Setenv("PATH", dir)

		_, err := AnyContainerExists(context.Background(), []string{"postgres"})
		if err == nil {
			t.Fatal("expected AnyContainerExists to surface execution errors")
		}
	})

	t.Run("podman runtime version helpers surface execution and parse failures", func(t *testing.T) {
		t.Run("podman capture error", func(t *testing.T) {
			dir := t.TempDir()
			scriptPath := filepath.Join(dir, "podman")
			writeExecutable(t, scriptPath)
			if err := os.WriteFile(scriptPath, []byte("#!/no/such/interpreter\n"), 0o755); err != nil {
				t.Fatalf("write failing podman script: %v", err)
			}
			t.Setenv("PATH", dir)

			_, err := PodmanVersion(context.Background())
			if err == nil {
				t.Fatal("expected PodmanVersion to return the capture error")
			}
		})

		t.Run("podman compose capture error", func(t *testing.T) {
			dir := t.TempDir()
			scriptPath := filepath.Join(dir, "podman")
			writeExecutable(t, scriptPath)
			if err := os.WriteFile(scriptPath, []byte("#!/no/such/interpreter\n"), 0o755); err != nil {
				t.Fatalf("write failing podman script: %v", err)
			}
			t.Setenv("PATH", dir)
			t.Setenv("PODMAN_COMPOSE_PROVIDER", "")

			_, err := PodmanComposeVersion(context.Background())
			if err == nil {
				t.Fatal("expected PodmanComposeVersion to return the capture error")
			}
		})

		t.Run("podman compose parse failure", func(t *testing.T) {
			dir := t.TempDir()
			writeServicesTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf 'compose version unknown\\n'\n")
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
			t.Setenv("PODMAN_COMPOSE_PROVIDER", "")

			_, err := PodmanComposeVersion(context.Background())
			if err == nil || !strings.Contains(err.Error(), "could not determine podman compose provider version") {
				t.Fatalf("unexpected compose parse error: %v", err)
			}
		})

		if VersionAtLeast("4.9.3", "bogus") {
			t.Fatal("expected invalid minimum version to return false")
		}
		if _, ok := parseSemVersion("bogus"); ok {
			t.Fatal("expected invalid semver to fail")
		}
	})

	t.Run("list containers surfaces capture and fallback error details", func(t *testing.T) {
		_, err := ListContainers(context.Background(), func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{}, context.DeadlineExceeded
		})
		if err == nil {
			t.Fatal("expected capture error from ListContainers")
		}

		_, err = ListContainers(context.Background(), func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{ExitCode: 125, Stdout: "fallback stdout"}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "fallback stdout") {
			t.Fatalf("expected stdout fallback error, got %v", err)
		}

		_, err = ListContainers(context.Background(), func(context.Context, string, string, ...string) (CommandResult, error) {
			return CommandResult{ExitCode: 126}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "exit code 126") {
			t.Fatalf("expected exit-code fallback error, got %v", err)
		}

		filtered := FilterContainersByName([]Container{{Names: []string{"postgres"}}}, []string{"", "   "})
		if len(filtered) != 0 {
			t.Fatalf("expected blank filter names to be ignored, got %+v", filtered)
		}
	})

	t.Run("ports helpers cover unexpected errors and timeouts", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := WaitForPort(ctx, 65534, 5*time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "wait for port 65534") {
			t.Fatalf("expected WaitForPort timeout error, got %v", err)
		}
	})
}
