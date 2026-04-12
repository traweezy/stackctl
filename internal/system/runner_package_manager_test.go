package system

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePackageManagerChoiceCoversConfiguredAndUnsupportedPaths(t *testing.T) {
	t.Run("unsupported configured manager falls back to detected recommendation", func(t *testing.T) {
		choice, err := ResolvePackageManagerChoice(
			"pkgsrc",
			Platform{
				GOOS:           "linux",
				DistroID:       "ubuntu",
				DistroLike:     []string{"debian"},
				PackageManager: "apt",
			},
			func(name string) bool { return name == "apt-get" },
		)
		if err != nil {
			t.Fatalf("ResolvePackageManagerChoice returned error: %v", err)
		}
		if choice.Name != "apt" || choice.Command != "apt-get" {
			t.Fatalf("unexpected fallback choice: %+v", choice)
		}
		if !strings.Contains(choice.Notice, `configured package manager "pkgsrc" is unsupported`) {
			t.Fatalf("expected unsupported-manager notice, got %+v", choice)
		}
	})

	t.Run("unsupported configured manager errors without a recommendation", func(t *testing.T) {
		_, err := ResolvePackageManagerChoice(
			"pkgsrc",
			Platform{GOOS: "linux", DistroID: "unknown", PackageManager: "unsupported"},
			func(string) bool { return false },
		)
		if err == nil || !strings.Contains(err.Error(), `unsupported package manager "pkgsrc"`) {
			t.Fatalf("unexpected unsupported-manager error: %v", err)
		}
	})

	t.Run("uses configured package manager when installed", func(t *testing.T) {
		choice, err := ResolvePackageManagerChoice(
			"apt",
			Platform{
				GOOS:           "linux",
				DistroID:       "ubuntu",
				DistroLike:     []string{"debian"},
				PackageManager: "apt",
			},
			func(name string) bool { return name == "apt-get" },
		)
		if err != nil {
			t.Fatalf("ResolvePackageManagerChoice returned error: %v", err)
		}
		if choice.Name != "apt" || choice.Command != "apt-get" || choice.Notice != "" {
			t.Fatalf("unexpected configured choice: %+v", choice)
		}
	})

	t.Run("configured recommended manager errors when its command is missing", func(t *testing.T) {
		_, err := ResolvePackageManagerChoice(
			"brew",
			Platform{GOOS: "darwin", PackageManager: "brew"},
			func(string) bool { return false },
		)
		if err == nil || !strings.Contains(err.Error(), `package manager "brew" is recommended for this machine`) {
			t.Fatalf("unexpected missing-recommended-manager error: %v", err)
		}
	})

	t.Run("configured manager errors when no alternative is installed", func(t *testing.T) {
		_, err := ResolvePackageManagerChoice(
			"apt",
			Platform{GOOS: "linux", DistroID: "unknown", PackageManager: "unsupported"},
			func(string) bool { return false },
		)
		if err == nil || !strings.Contains(err.Error(), `configured package manager "apt" is not installed; install apt-get or update system.package_manager`) {
			t.Fatalf("unexpected missing-configured-manager error: %v", err)
		}
	})

	t.Run("empty config errors when nothing is detected", func(t *testing.T) {
		_, err := ResolvePackageManagerChoice(
			"",
			Platform{GOOS: "linux", DistroID: "unknown", PackageManager: "unsupported"},
			func(string) bool { return false },
		)
		if err == nil || !strings.Contains(err.Error(), "no supported package manager was detected on this machine") {
			t.Fatalf("unexpected no-manager error: %v", err)
		}
	})
}

func TestRecommendPackageManagerAndReasonFallbacks(t *testing.T) {
	t.Run("returns empty recommendation when nothing is supported or installed", func(t *testing.T) {
		recommendation := RecommendPackageManager(
			Platform{GOOS: "linux", DistroID: "unknown", PackageManager: "unsupported"},
			func(string) bool { return false },
		)
		if recommendation != (PackageManagerRecommendation{}) {
			t.Fatalf("expected empty recommendation, got %+v", recommendation)
		}
	})

	t.Run("prefers the earliest detected package-manager command", func(t *testing.T) {
		recommendation := RecommendPackageManager(
			Platform{GOOS: "linux", DistroID: "unknown", PackageManager: "unsupported"},
			func(name string) bool { return name == "dnf" || name == "apk" },
		)
		if recommendation.Name != "dnf" || recommendation.Command != "dnf" || !recommendation.Available {
			t.Fatalf("unexpected detected recommendation: %+v", recommendation)
		}
	})

	t.Run("covers more platform-family reasons", func(t *testing.T) {
		cases := []struct {
			name     string
			platform Platform
			want     string
		}{
			{
				name:     "debian family",
				platform: Platform{GOOS: "linux", DistroID: "ubuntu"},
				want:     "Debian/Ubuntu-family",
			},
			{
				name:     "arch family",
				platform: Platform{GOOS: "linux", DistroID: "manjaro"},
				want:     "Arch-family",
			},
			{
				name:     "alpine",
				platform: Platform{GOOS: "linux", DistroID: "alpine"},
				want:     "Alpine Linux",
			},
			{
				name:     "fallback host reason",
				platform: Platform{GOOS: "linux"},
				want:     "detected from the current host",
			},
		}

		for _, tc := range cases {
			if got := platformPackageManagerReason(tc.platform); !strings.Contains(got, tc.want) {
				t.Fatalf("%s: unexpected package-manager reason %q", tc.name, got)
			}
		}
	})
}

func TestRunnerCaptureAndExternalExecErrorFormatting(t *testing.T) {
	t.Run("capture returns stdout on success", func(t *testing.T) {
		dir := t.TempDir()
		writeExecTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf 'compose ok\\n'\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		stdout, err := (Runner{}).Capture(context.Background(), "", "podman", "compose", "version")
		if err != nil {
			t.Fatalf("Runner.Capture returned error: %v", err)
		}
		if strings.TrimSpace(stdout) != "compose ok" {
			t.Fatalf("unexpected stdout %q", stdout)
		}
	})

	t.Run("capture falls back to stdout details on non-zero exit", func(t *testing.T) {
		dir := t.TempDir()
		writeExecTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nprintf 'compose failed from stdout\\n'\nexit 8\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		_, err := (Runner{}).Capture(context.Background(), "", "podman", "compose", "version")
		if err == nil || !strings.Contains(err.Error(), "compose failed from stdout") {
			t.Fatalf("unexpected stdout-detail capture error: %v", err)
		}
	})

	t.Run("capture reports the exit code when no output is available", func(t *testing.T) {
		dir := t.TempDir()
		writeExecTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\nexit 9\n")
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		_, err := (Runner{}).Capture(context.Background(), "", "podman", "compose", "version")
		if err == nil || !strings.Contains(err.Error(), "run podman compose version exited with code 9") {
			t.Fatalf("unexpected exit-code capture error: %v", err)
		}
	})

	t.Run("external exec surfaces missing executables", func(t *testing.T) {
		err := RunExternalCommand(context.Background(), Runner{}, "", []string{"missing-external-tool", "hello"})
		if err == nil || !strings.Contains(err.Error(), `run missing-external-tool hello`) {
			t.Fatalf("unexpected external command error: %v", err)
		}
	})
}

func TestWriteRunnerNoticeHandlesNilAndWritableOutputs(t *testing.T) {
	writeRunnerNotice(nil, "ignored %d", 1)

	var out bytes.Buffer
	writeRunnerNotice(&out, "attempt %d/%d", 2, 3)
	if out.String() != "attempt 2/3" {
		t.Fatalf("unexpected runner notice %q", out.String())
	}
}

func TestPackageManagerCommandAndInstallPackageEdgeCases(t *testing.T) {
	t.Run("unknown package manager has no command", func(t *testing.T) {
		if got := PackageManagerCommand("pkgsrc"); got != "" {
			t.Fatalf("expected unknown package manager command to be empty, got %q", got)
		}
	})

	t.Run("install packages returns nil for empty requirements", func(t *testing.T) {
		packages, err := InstallPackages(context.Background(), Runner{}, "apt", nil)
		if err != nil {
			t.Fatalf("InstallPackages returned error: %v", err)
		}
		if packages != nil {
			t.Fatalf("expected nil package list, got %+v", packages)
		}
	})

	t.Run("install packages rejects unsupported package managers", func(t *testing.T) {
		_, err := InstallPackages(context.Background(), Runner{}, "pkgsrc", []Requirement{RequirementPodman})
		if err == nil || !strings.Contains(err.Error(), `unsupported package manager "pkgsrc"`) {
			t.Fatalf("unexpected unsupported-package-manager error: %v", err)
		}
	})

	t.Run("install packages errors when the configured command is missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		_, err := InstallPackages(context.Background(), Runner{}, "apt", []Requirement{RequirementPodman})
		if err == nil || !strings.Contains(err.Error(), `package manager "apt" is configured but the apt-get command is not installed on this machine`) {
			t.Fatalf("unexpected missing-command error: %v", err)
		}
	})
}

func TestRunZypperInstallWithRetryReturnsFinalFailure(t *testing.T) {
	originalEUID := currentEUID
	currentEUID = func() int { return 0 }
	t.Cleanup(func() { currentEUID = originalEUID })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")

	writeExecTestScript(t, filepath.Join(dir, "zypper"), "#!/bin/sh\nprintf 'zypper %s\\n' \"$*\" >> \""+logPath+"\"\necho 'metadata failed' >&2\nexit 104\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runZypperInstallWithRetry(context.Background(), Runner{Stdout: &stdout, Stderr: &stderr}, []string{"podman"})
	if err == nil || !strings.Contains(err.Error(), "zypper install failed after 3 attempts") {
		t.Fatalf("unexpected zypper retry error: %v", err)
	}
	if !strings.Contains(stdout.String(), "zypper install attempt 3/3") {
		t.Fatalf("expected final zypper attempt notice in stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "zypper install attempt 2 failed; cleaning metadata and retrying") {
		t.Fatalf("expected retry notice in stderr:\n%s", stderr.String())
	}
}

func TestRunnerRunSurfacesCommandFailures(t *testing.T) {
	dir := t.TempDir()
	writeExecTestScript(t, filepath.Join(dir, "podman"), "#!/bin/sh\necho 'compose up failed' >&2\nexit 12\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	err := (Runner{}).Run(context.Background(), "", "podman", "compose", "up")
	if err == nil || !strings.Contains(err.Error(), "run podman compose up") {
		t.Fatalf("unexpected runner run error: %v", err)
	}
}

func TestCaptureResultWithEnvSurfacesExecutionErrors(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "podman")
	writeExecTestScript(t, scriptPath, "#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(scriptPath, []byte("#!/no/such/interpreter\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	_, err := CaptureResultWithEnv(context.Background(), "", nil, "podman", "compose", "version")
	if err == nil || !strings.Contains(err.Error(), "run podman compose version") {
		t.Fatalf("unexpected execution error: %v", err)
	}
}
