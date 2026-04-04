package cmd

import (
	"reflect"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRequirementLabels(t *testing.T) {
	got := requirementLabels([]system.Requirement{
		system.RequirementPodman,
		system.RequirementComposeProvider,
	})
	want := []string{"podman", "podman compose provider"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected requirement labels: got %v want %v", got, want)
	}
}

func TestDisplayRequirementLabels(t *testing.T) {
	originalDeps := deps
	t.Cleanup(func() { deps = originalDeps })

	t.Run("returns nil for empty requirements", func(t *testing.T) {
		got := displayRequirementLabels(nil, "", system.Platform{})
		if got != nil {
			t.Fatalf("expected nil labels, got %v", got)
		}
	})

	t.Run("uses detected install plan packages", func(t *testing.T) {
		deps.commandExists = func(name string) bool { return name == "apt-get" }

		got := displayRequirementLabels(
			[]system.Requirement{system.RequirementPodman, system.RequirementComposeProvider},
			"",
			system.Platform{GOOS: "linux", PackageManager: "apt", DistroID: "ubuntu"},
		)
		want := []string{"podman", "podman-compose"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected detected labels: got %v want %v", got, want)
		}
	})

	t.Run("falls back to platform plan when configured choice fails", func(t *testing.T) {
		deps.commandExists = func(string) bool { return false }

		got := displayRequirementLabels(
			[]system.Requirement{system.RequirementPodman},
			"",
			system.Platform{GOOS: "linux", PackageManager: "apt", DistroID: "ubuntu"},
		)
		if !reflect.DeepEqual(got, []string{"podman"}) {
			t.Fatalf("unexpected fallback plan labels: %v", got)
		}
	})

	t.Run("falls back to raw labels when no plan can be resolved", func(t *testing.T) {
		deps.commandExists = func(string) bool { return false }

		got := displayRequirementLabels(
			[]system.Requirement{system.RequirementPodman},
			"",
			system.Platform{GOOS: "linux", PackageManager: "unknown"},
		)
		if !reflect.DeepEqual(got, []string{"podman"}) {
			t.Fatalf("unexpected raw fallback labels: %v", got)
		}
	})
}

func TestPlanDisplayLabels(t *testing.T) {
	fallback := []string{"podman", "podman compose provider"}

	t.Run("returns fallback when plan is empty", func(t *testing.T) {
		got := planDisplayLabels(system.InstallPlan{}, fallback)
		if !reflect.DeepEqual(got, fallback) {
			t.Fatalf("unexpected empty-plan labels: got %v want %v", got, fallback)
		}
	})

	t.Run("combines packages and unsupported requirements", func(t *testing.T) {
		got := planDisplayLabels(system.InstallPlan{
			Packages:    []string{"podman", "podman-compose"},
			Unsupported: []system.Requirement{system.RequirementCockpit},
		}, fallback)
		want := []string{"podman", "podman-compose", "cockpit"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected plan labels: got %v want %v", got, want)
		}
	})
}
