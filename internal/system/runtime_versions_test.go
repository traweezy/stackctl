package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractRuntimeVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "podman", input: "podman version 4.9.3", want: "4.9.3", ok: true},
		{name: "podman compose provider", input: "podman-compose version: 1.0.6\nusing podman version: 4.9.3", want: "1.0.6", ok: true},
		{name: "docker compose provider", input: "Docker Compose version v2.33.1", want: "2.33.1", ok: true},
		{name: "no version", input: "not a version", want: "", ok: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := extractRuntimeVersion(tc.input)
			if ok != tc.ok {
				t.Fatalf("extractRuntimeVersion ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("extractRuntimeVersion = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		version string
		minimum string
		want    bool
	}{
		{name: "equal", version: "4.9.3", minimum: "4.9.3", want: true},
		{name: "newer patch", version: "4.9.4", minimum: "4.9.3", want: true},
		{name: "newer major", version: "5.0.0", minimum: "4.9.3", want: true},
		{name: "older patch", version: "4.9.2", minimum: "4.9.3", want: false},
		{name: "older minor", version: "4.8.9", minimum: "4.9.3", want: false},
		{name: "two-part version", version: "2.33", minimum: "1.0.6", want: true},
		{name: "suffix", version: "4.9.3+build1", minimum: "4.9.3", want: true},
		{name: "invalid", version: "unknown", minimum: "4.9.3", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := VersionAtLeast(tc.version, tc.minimum); got != tc.want {
				t.Fatalf("VersionAtLeast(%q, %q) = %v, want %v", tc.version, tc.minimum, got, tc.want)
			}
		})
	}
}

func TestPodmanVersionReadsCLIOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte("#!/bin/sh\nprintf 'podman version 4.9.3\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	version, err := PodmanVersion(context.Background())
	if err != nil {
		t.Fatalf("PodmanVersion returned error: %v", err)
	}
	if version != "4.9.3" {
		t.Fatalf("PodmanVersion = %q, want %q", version, "4.9.3")
	}
}

func TestPodmanComposeVersionUsesPodmanComposeProviderFallback(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "provider.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"${PODMAN_COMPOSE_PROVIDER:-}\" > \"" + logPath + "\"\ncat <<'EOF'\npodman-compose version: 1.0.6\nusing podman version: 4.9.3\nEOF\n"
	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
	writeExecutable(t, filepath.Join(dir, "podman-compose"))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "")

	version, err := PodmanComposeVersion(context.Background())
	if err != nil {
		t.Fatalf("PodmanComposeVersion returned error: %v", err)
	}
	if version != "1.0.6" {
		t.Fatalf("PodmanComposeVersion = %q, want %q", version, "1.0.6")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read provider log: %v", err)
	}
	if string(data) != "podman-compose\n" {
		t.Fatalf("unexpected provider env: %q", string(data))
	}
}

func TestRuntimeVersionCommandsCoverErrorPaths(t *testing.T) {
	t.Run("podman command failure", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "podman"), []byte("#!/bin/sh\nprintf 'permission denied\\n' >&2\nexit 3\n"), 0o755); err != nil {
			t.Fatalf("write fake podman: %v", err)
		}
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		_, err := PodmanVersion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "podman --version failed: permission denied") {
			t.Fatalf("expected podman version command error, got %v", err)
		}
	})

	t.Run("podman version parse failure", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "podman"), []byte("#!/bin/sh\nprintf 'podman build unknown\\n'\n"), 0o755); err != nil {
			t.Fatalf("write fake podman: %v", err)
		}
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		_, err := PodmanVersion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "could not determine podman version") {
			t.Fatalf("expected podman parse error, got %v", err)
		}
	})

	t.Run("podman compose respects explicit provider env", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "provider.log")
		script := "#!/bin/sh\nprintf '%s\\n' \"${PODMAN_COMPOSE_PROVIDER:-}\" > \"" + logPath + "\"\ncat <<'EOF'\npodman compose version 1.0.6\nEOF\n"
		if err := os.WriteFile(filepath.Join(dir, "podman"), []byte(script), 0o755); err != nil {
			t.Fatalf("write fake podman: %v", err)
		}
		writeExecutable(t, filepath.Join(dir, "podman-compose"))
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")

		version, err := PodmanComposeVersion(context.Background())
		if err != nil {
			t.Fatalf("PodmanComposeVersion returned error: %v", err)
		}
		if version != "1.0.6" {
			t.Fatalf("PodmanComposeVersion = %q, want %q", version, "1.0.6")
		}

		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read provider log: %v", err)
		}
		if string(data) != "docker-compose\n" {
			t.Fatalf("expected explicit provider env to be preserved, got %q", string(data))
		}
	})

	t.Run("podman compose command failure", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "podman"), []byte("#!/bin/sh\nprintf 'compose missing\\n' >&2\nexit 4\n"), 0o755); err != nil {
			t.Fatalf("write fake podman: %v", err)
		}
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		t.Setenv("PODMAN_COMPOSE_PROVIDER", "")

		_, err := PodmanComposeVersion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "podman compose version failed: compose missing") {
			t.Fatalf("expected compose version command error, got %v", err)
		}
	})
}

func TestVersionCommandErrorFallsBackAcrossOutputs(t *testing.T) {
	err := versionCommandError("podman --version", CommandResult{ExitCode: 5, Stderr: "broken"})
	if got := err.Error(); got != "podman --version failed: broken" {
		t.Fatalf("unexpected stderr-backed version error %q", got)
	}

	err = versionCommandError("podman --version", CommandResult{ExitCode: 6, Stdout: "fallback output"})
	if got := err.Error(); got != "podman --version failed: fallback output" {
		t.Fatalf("unexpected stdout-backed version error %q", got)
	}

	err = versionCommandError("podman --version", CommandResult{ExitCode: 7})
	if got := err.Error(); got != "podman --version failed: exit code 7" {
		t.Fatalf("unexpected exit-code-backed version error %q", got)
	}
}
