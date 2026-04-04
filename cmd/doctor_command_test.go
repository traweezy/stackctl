package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDoctorFixAppliesSupportedRemediations(t *testing.T) {
	var doctorCalls int
	var installed []string
	var enabledCockpit bool
	var scaffolded bool
	var forcedScaffold bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.InstallCockpit = true
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			forcedScaffold = force
			return configpkg.ScaffoldResult{
				CreatedDir:   true,
				WroteCompose: true,
				StackDir:     cfg.Stack.Dir,
				ComposePath:  configpkg.ComposePath(cfg),
			}, nil
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			doctorCalls++
			if doctorCalls == 1 {
				return newReport(
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
					doctorpkg.Check{Status: output.StatusWarn, Message: "cockpit.socket inactive"},
				), nil
			}
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket active"},
			), nil
		}
		d.installPackages = func(_ context.Context, _ system.Runner, _ string, requirements []system.Requirement) ([]string, error) {
			for _, requirement := range requirements {
				switch requirement {
				case system.RequirementPodman:
					installed = append(installed, "podman")
				case system.RequirementComposeProvider:
					installed = append(installed, "podman-compose")
				default:
					installed = append(installed, string(requirement))
				}
			}
			return installed, nil
		}
		d.enableCockpit = func(context.Context, system.Runner) error {
			enabledCockpit = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "doctor", "--fix", "--yes")
	if err != nil {
		t.Fatalf("doctor --fix returned error: %v", err)
	}
	if doctorCalls != 2 {
		t.Fatalf("expected doctor to run twice, got %d", doctorCalls)
	}
	if !scaffolded {
		t.Fatal("expected doctor --fix to scaffold the managed stack")
	}
	if !forcedScaffold {
		t.Fatal("expected doctor --fix to force-refresh stale managed scaffold files")
	}
	if !enabledCockpit {
		t.Fatal("expected doctor --fix to enable cockpit")
	}
	if want := []string{"podman", "podman-compose"}; strings.Join(installed, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected installed packages: %q", installed)
	}
	for _, fragment := range []string{"Installed: podman, podman-compose", "Post-fix report:", "enabled cockpit.socket"} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("doctor --fix output missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestDoctorFixFallsBackToDetectedPackageManagerWhenConfigBlank(t *testing.T) {
	var usedPackageManager string
	var doctorRuns int

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.System.PackageManager = ""
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			doctorRuns++
			if doctorRuns == 1 {
				return newReport(
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				), nil
			}
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
			), nil
		}
		d.installPackages = func(_ context.Context, _ system.Runner, packageManager string, requirements []system.Requirement) ([]string, error) {
			usedPackageManager = packageManager
			return []string{"podman"}, nil
		}
	})

	stdout, _, err := executeRoot(t, "doctor", "--fix", "--yes")
	if err != nil {
		t.Fatalf("doctor --fix returned error: %v", err)
	}
	if usedPackageManager != "apt" {
		t.Fatalf("expected apt fallback, got %q", usedPackageManager)
	}
	if !strings.Contains(stdout, "using detected apt for this run") {
		t.Fatalf("expected fallback notice, got:\n%s", stdout)
	}
}

func TestDoctorFixRequiresYesWhenPromptUnavailable(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
			), nil
		}
	})

	_, _, err := executeRoot(t, "doctor", "--fix")
	if err == nil || !strings.Contains(err.Error(), "automatic fix confirmation required; rerun with --yes") {
		t.Fatalf("unexpected doctor --fix error: %v", err)
	}
}

func TestDoctorFixPromptDeclineLeavesSystemUntouched(t *testing.T) {
	var doctorRuns int
	var installCalled bool

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			doctorRuns++
			if doctorRuns == 1 {
				return newReport(
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
				), nil
			}
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
			), nil
		}
		d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
			installCalled = true
			return nil, errors.New("install should not run")
		}
	})

	stdout, _, err := executeRoot(t, "doctor", "--fix")
	if err != nil {
		t.Fatalf("doctor --fix returned error: %v", err)
	}
	if doctorRuns != 2 {
		t.Fatalf("expected doctor to run twice, got %d", doctorRuns)
	}
	if installCalled {
		t.Fatal("expected package installation to be skipped when prompt is declined")
	}
	if !strings.Contains(stdout, "no automatic fixes were applied") {
		t.Fatalf("expected no-fixes message, got:\n%s", stdout)
	}
}

func TestDoctorRemediationMarkdownIncludesVersionUpgradeGuidance(t *testing.T) {
	report := newReport(
		doctorpkg.Check{Status: output.StatusWarn, Message: "podman 4.3.1 is below supported minimum 4.9.3"},
	)

	markdown := doctorRemediationMarkdown(report)
	if !strings.Contains(markdown, "Upgrade Podman and the selected compose provider") {
		t.Fatalf("expected version upgrade guidance, got:\n%s", markdown)
	}
}

func TestDoctorRemediationMarkdownIncludesPodmanMachineGuidance(t *testing.T) {
	report := newReport(
		doctorpkg.Check{Status: output.StatusMiss, Message: "podman machine not initialized"},
	)

	markdown := doctorRemediationMarkdown(report)
	if !strings.Contains(markdown, "podman machine init") || !strings.Contains(markdown, "podman machine start") {
		t.Fatalf("expected podman machine guidance, got:\n%s", markdown)
	}
}

func TestDoctorRemediationMarkdownIncludesManualCockpitGuidance(t *testing.T) {
	report := newReport(
		doctorpkg.Check{Status: output.StatusWarn, Message: "cockpit helpers enabled but cockpit.socket must be installed manually on this platform"},
	)

	markdown := doctorRemediationMarkdown(report)
	if !strings.Contains(markdown, "Install Cockpit manually on this host") {
		t.Fatalf("expected manual cockpit guidance, got:\n%s", markdown)
	}
}

func TestDoctorRemediationMarkdownIncludesUnsupportedCockpitGuidance(t *testing.T) {
	report := newReport(
		doctorpkg.Check{Status: output.StatusWarn, Message: "cockpit helpers are not supported on brew"},
	)

	markdown := doctorRemediationMarkdown(report)
	if !strings.Contains(markdown, "Disable `setup.include_cockpit` and `setup.install_cockpit`") {
		t.Fatalf("expected unsupported cockpit guidance, got:\n%s", markdown)
	}
}
