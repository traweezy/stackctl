package system

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveInstallPlanByBackend(t *testing.T) {
	t.Run("brew skips unsupported requirements", func(t *testing.T) {
		plan, err := ResolveInstallPlan("brew", []Requirement{
			RequirementPodman,
			RequirementComposeProvider,
			RequirementBuildah,
			RequirementCockpit,
		})
		if err != nil {
			t.Fatalf("ResolveInstallPlan returned error: %v", err)
		}
		if got, want := strings.Join(plan.Packages, ","), "podman,podman-compose"; got != want {
			t.Fatalf("unexpected packages: got %q want %q", got, want)
		}
		if got, want := len(plan.Unsupported), 2; got != want {
			t.Fatalf("unexpected unsupported requirements: %+v", plan.Unsupported)
		}
	})

	t.Run("dnf resolves cockpit packages", func(t *testing.T) {
		plan, err := ResolveInstallPlan("dnf", []Requirement{RequirementCockpit})
		if err != nil {
			t.Fatalf("ResolveInstallPlan returned error: %v", err)
		}
		if got, want := strings.Join(plan.Packages, ","), "cockpit,cockpit-podman"; got != want {
			t.Fatalf("unexpected packages: got %q want %q", got, want)
		}
	})
}

func TestRecommendPackageManagerUsesDetectedPlatform(t *testing.T) {
	recommendation := RecommendPackageManager(
		Platform{
			GOOS:           "linux",
			DistroID:       "ubuntu",
			DistroLike:     []string{"debian"},
			PackageManager: "apt",
		},
		func(name string) bool { return name == "apt-get" },
	)

	if recommendation.Name != "apt" || recommendation.Command != "apt-get" || !recommendation.Available {
		t.Fatalf("unexpected recommendation: %+v", recommendation)
	}
	if !strings.Contains(recommendation.Reason, "Debian/Ubuntu") {
		t.Fatalf("unexpected recommendation reason: %+v", recommendation)
	}
}

func TestResolvePackageManagerChoiceFallsBackToDetectedValue(t *testing.T) {
	choice, err := ResolvePackageManagerChoice(
		"",
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
		t.Fatalf("unexpected choice: %+v", choice)
	}
	if !strings.Contains(choice.Notice, "using detected apt") {
		t.Fatalf("expected fallback notice, got %+v", choice)
	}
}

func TestResolvePackageManagerChoiceFallsBackWhenConfiguredCommandMissing(t *testing.T) {
	choice, err := ResolvePackageManagerChoice(
		"brew",
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
	if choice.Name != "apt" {
		t.Fatalf("unexpected fallback choice: %+v", choice)
	}
	if !strings.Contains(choice.Notice, `"brew"`) || !strings.Contains(choice.Notice, "apt") {
		t.Fatalf("expected stale-config notice, got %+v", choice)
	}
}

func TestResolvePackageManagerChoiceErrorsWhenRecommendedCommandMissing(t *testing.T) {
	_, err := ResolvePackageManagerChoice(
		"",
		Platform{GOOS: "darwin", PackageManager: "brew"},
		func(string) bool { return false },
	)
	if err == nil || !strings.Contains(err.Error(), "brew") || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatPackageManagerRecommendationMessages(t *testing.T) {
	available := FormatPackageManagerRecommendation(PackageManagerRecommendation{
		Name:      "apt",
		Command:   "apt-get",
		Available: true,
		Reason:    "recommended for this Debian/Ubuntu-family Linux host",
	})
	if !strings.Contains(available, "available via apt-get") {
		t.Fatalf("unexpected available recommendation: %q", available)
	}

	unavailable := FormatPackageManagerRecommendation(PackageManagerRecommendation{
		Name:      "brew",
		Command:   "brew",
		Available: false,
		Reason:    "recommended for this macOS host",
	})
	if !strings.Contains(unavailable, "install brew first") {
		t.Fatalf("unexpected unavailable recommendation: %q", unavailable)
	}

	empty := FormatPackageManagerRecommendation(PackageManagerRecommendation{})
	if !strings.Contains(empty, "No supported package manager") {
		t.Fatalf("unexpected empty recommendation: %q", empty)
	}
}

func TestCurrentPlatformHelpersReflectRuntime(t *testing.T) {
	platform := CurrentPlatform()

	if platform.GOOS != runtime.GOOS {
		t.Fatalf("unexpected runtime GOOS: %+v", platform)
	}
	if DetectPackageManager() != platform.PackageManager {
		t.Fatalf("DetectPackageManager did not match CurrentPlatform: %+v", platform)
	}

	recommendation := CurrentPackageManagerRecommendation()
	if recommendation.Name != "" && recommendation.Command != PackageManagerCommand(recommendation.Name) {
		t.Fatalf("unexpected current recommendation: %+v", recommendation)
	}
}

func TestPlatformCapabilityHelpers(t *testing.T) {
	linux := Platform{
		GOOS:           "linux",
		PackageManager: "dnf",
		ServiceManager: ServiceManagerSystemd,
	}
	if !linux.SupportsCockpitAutoEnable() {
		t.Fatalf("expected systemd dnf host to support cockpit auto-enable: %+v", linux)
	}
	if !linux.SupportsSSCheck() {
		t.Fatalf("expected linux host to support ss checks: %+v", linux)
	}

	darwin := Platform{GOOS: "darwin", PackageManager: "brew"}
	if darwin.SupportsCockpitAutoEnable() {
		t.Fatalf("expected darwin host to skip cockpit auto-enable: %+v", darwin)
	}
	if darwin.SupportsSSCheck() {
		t.Fatalf("expected darwin host to skip ss checks: %+v", darwin)
	}
}

func TestRecommendPackageManagerFallsBackToDetectedCommand(t *testing.T) {
	recommendation := RecommendPackageManager(
		Platform{GOOS: "linux", DistroID: "unknown", PackageManager: "unsupported"},
		func(name string) bool { return name == "pacman" },
	)

	if recommendation.Name != "pacman" || recommendation.Command != "pacman" || !recommendation.Available {
		t.Fatalf("unexpected fallback recommendation: %+v", recommendation)
	}
	if !strings.Contains(recommendation.Reason, "available package-manager commands") {
		t.Fatalf("unexpected fallback recommendation reason: %+v", recommendation)
	}
}

func TestPlatformPackageManagerReasonCoversKnownFamilies(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     string
	}{
		{name: "darwin", platform: Platform{GOOS: "darwin"}, want: "macOS"},
		{name: "fedora", platform: Platform{GOOS: "linux", DistroID: "fedora"}, want: "Fedora/RHEL-family"},
		{name: "opensuse", platform: Platform{GOOS: "linux", DistroID: "opensuse-tumbleweed"}, want: "openSUSE-family"},
		{name: "generic", platform: Platform{GOOS: "linux", DistroID: "gentoo"}, want: "gentoo Linux host"},
	}

	for _, tc := range tests {
		if got := platformPackageManagerReason(tc.platform); !strings.Contains(got, tc.want) {
			t.Fatalf("%s: unexpected package-manager reason %q", tc.name, got)
		}
	}
}

func TestInstallPackagesRunsBackendCommands(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")

	writeScript := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write script %s: %v", name, err)
		}
	}

	writeScript("sudo", "#!/bin/sh\necho sudo \"$@\" >> \""+logPath+"\"\n")
	writeScript("brew", "#!/bin/sh\necho brew \"$@\" >> \""+logPath+"\"\n")
	writeScript("pacman", "#!/bin/sh\necho pacman \"$@\" >> \""+logPath+"\"\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	runner := Runner{Stdout: io.Discard, Stderr: io.Discard}

	if _, err := InstallPackages(context.Background(), runner, "apt", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("apt install returned error: %v", err)
	}
	if _, err := InstallPackages(context.Background(), runner, "pacman", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("pacman install returned error: %v", err)
	}
	if _, err := InstallPackages(context.Background(), runner, "brew", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("brew install returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	for _, fragment := range []string{
		"sudo apt-get update",
		"sudo apt-get install -y podman",
		"sudo pacman -Syu --noconfirm --needed podman",
		"brew install podman",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("command log missing %q:\n%s", fragment, output)
		}
	}
}

func TestInstallPackagesErrorsForUnsupportedRequirement(t *testing.T) {
	dir := t.TempDir()
	writePath := filepath.Join(dir, "brew")
	if err := os.WriteFile(writePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write brew stub: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	runner := Runner{Stdout: io.Discard, Stderr: io.Discard}

	_, err := InstallPackages(context.Background(), runner, "brew", []Requirement{RequirementBuildah})
	if err == nil || !strings.Contains(err.Error(), "does not support automatic installation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallPackagesRetriesZypperInstalls(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	refreshCountPath := filepath.Join(dir, "refresh-count")

	writeScript := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write script %s: %v", name, err)
		}
	}

	writeScript("sudo", "#!/bin/sh\nprintf 'sudo %s\\n' \"$*\" >> \""+logPath+"\"\nexec \"$@\"\n")
	writeScript("zypper", "#!/bin/sh\nset -eu\nprintf 'zypper %s\\n' \"$*\" >> \""+logPath+"\"\ncase \"$*\" in\n  *' clean --all')\n    exit 0\n    ;;\n  *' refresh --force')\n    count=0\n    if [ -f \""+refreshCountPath+"\" ]; then\n      count=$(cat \""+refreshCountPath+"\")\n    fi\n    count=$((count + 1))\n    printf '%s' \"$count\" > \""+refreshCountPath+"\"\n    if [ \"$count\" -eq 1 ]; then\n      echo 'repository metadata missing' >&2\n      exit 104\n    fi\n    exit 0\n    ;;\n  *' install '* )\n    exit 0\n    ;;\n  *)\n    echo \"unexpected zypper args: $*\" >&2\n    exit 1\n    ;;\nesac\n")
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	if _, err := InstallPackages(context.Background(), runner, "zypper", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("zypper install returned error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	if !strings.Contains(stdout.String(), "zypper install attempt 1/3") || !strings.Contains(stdout.String(), "zypper install attempt 2/3") {
		t.Fatalf("expected zypper attempt notices in stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "zypper install attempt 1 failed; cleaning metadata and retrying") {
		t.Fatalf("expected zypper retry notice in stderr:\n%s", stderr.String())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	for _, fragment := range []string{
		"sudo zypper --non-interactive clean --all",
		"sudo zypper --non-interactive --gpg-auto-import-keys refresh --force",
		"sudo zypper --non-interactive install podman",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("command log missing %q:\n%s", fragment, output)
		}
	}
}
