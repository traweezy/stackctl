package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/system"
)

func TestResolveInstallPackageManagerReturnsDetectedChoice(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.platform = func() system.Platform {
			return system.Platform{GOOS: "linux", PackageManager: "apt"}
		}
		d.commandExists = func(name string) bool { return name == "apt-get" }
	})

	choice, err := resolveInstallPackageManager("")
	if err != nil {
		t.Fatalf("resolveInstallPackageManager returned error: %v", err)
	}
	if choice.Name != "apt" || choice.Command != "apt-get" {
		t.Fatalf("unexpected choice %+v", choice)
	}
	if !strings.Contains(choice.Notice, "using detected apt for this run") {
		t.Fatalf("expected fallback notice, got %q", choice.Notice)
	}
}

func TestResolveInstallPackageManagerWrapsErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.platform = func() system.Platform {
			return system.Platform{GOOS: "linux", PackageManager: "apt"}
		}
		d.commandExists = func(string) bool { return false }
	})

	_, err := resolveInstallPackageManager("")
	if err == nil {
		t.Fatal("expected resolveInstallPackageManager to fail")
	}
	if !strings.Contains(err.Error(), "resolve package manager:") {
		t.Fatalf("expected wrapped package-manager error, got %v", err)
	}
}

func TestReportPackageManagerChoiceNoticeHandlesEmptyAndPresentMessages(t *testing.T) {
	t.Run("empty notice", func(t *testing.T) {
		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := reportPackageManagerChoiceNotice(cmd, system.PackageManagerChoice{}); err != nil {
			t.Fatalf("reportPackageManagerChoiceNotice returned error: %v", err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected no output, got %q", stdout.String())
		}
	})

	t.Run("present notice", func(t *testing.T) {
		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		err := reportPackageManagerChoiceNotice(cmd, system.PackageManagerChoice{
			Name:    "apt",
			Command: "apt-get",
			Notice:  "using detected apt for this run",
		})
		if err != nil {
			t.Fatalf("reportPackageManagerChoiceNotice returned error: %v", err)
		}
		if got := stdout.String(); got != "ℹ️ using detected apt for this run\n" {
			t.Fatalf("unexpected notice output %q", got)
		}
	})
}
