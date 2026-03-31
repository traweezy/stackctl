package system

import (
	"context"
	"io"
	"os"
	"path/filepath"
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
