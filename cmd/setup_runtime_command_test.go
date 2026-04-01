package cmd

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSetupNonInteractiveCreatesConfigAndPrintsNextSteps(t *testing.T) {
	var saved bool
	scaffolded := false

	withTestDeps(t, func(d *commandDeps) {
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
			), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--non-interactive")
	if err != nil {
		t.Fatalf("setup returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected setup to save a config")
	}
	if !scaffolded {
		t.Fatal("expected setup to scaffold the default managed stack")
	}
	if !strings.Contains(stdout, "created default config") {
		t.Fatalf("stdout missing config creation message: %s", stdout)
	}
	if !strings.Contains(stdout, "Next steps:") {
		t.Fatalf("stdout missing next steps: %s", stdout)
	}
	for _, fragment := range []string{
		"stackctl setup --install",
		"stackctl start",
		"stackctl services",
		"stackctl env --export",
		"stackctl connect",
		"stackctl tui",
		"stackctl doctor",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stdout missing next-step hint %q:\n%s", fragment, stdout)
		}
	}
}

func TestSetupInteractiveDeclineContinuesWithoutSaving(t *testing.T) {
	saved := false

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"}), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup")
	if err != nil {
		t.Fatalf("setup returned error: %v", err)
	}
	if saved {
		t.Fatal("setup should not save config when interactive setup is declined")
	}
	if !strings.Contains(stdout, "config file not found") || !strings.Contains(stdout, "Next steps") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestSetupInteractiveRunsWizardWithoutPrompt(t *testing.T) {
	var savedCfg configpkg.Config

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Stack.Name = "custom-stack"
			return cfg, nil
		}
		d.saveConfig = func(_ string, cfg configpkg.Config) error {
			savedCfg = cfg
			return nil
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"}), nil
		}
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
			t.Fatal("prompt should not run when --interactive is set")
			return false, nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--interactive")
	if err != nil {
		t.Fatalf("setup --interactive returned error: %v", err)
	}
	if savedCfg.Stack.Name != "custom-stack" {
		t.Fatalf("unexpected saved config: %+v", savedCfg)
	}
	if !strings.Contains(stdout, "saved config to /tmp/stackctl/config.yaml") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestSetupInstallRunsPackageInstallAndCockpitEnable(t *testing.T) {
	var installPackages []string
	enabledCockpit := false

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.System.PackageManager = "dnf"
		d.platform = func() system.Platform {
			return system.Platform{
				GOOS:           "linux",
				PackageManager: "dnf",
				ServiceManager: system.ServiceManagerSystemd,
			}
		}
		d.loadConfig = func(string) (configpkg.Config, error) {
			return cfg, nil
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "skopeo installed"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "cockpit.socket installed"},
			), nil
		}
		d.installPackages = func(_ context.Context, _ system.Runner, _ string, requirements []system.Requirement) ([]string, error) {
			for _, requirement := range requirements {
				switch requirement {
				case system.RequirementPodman:
					installPackages = append(installPackages, "podman")
				case system.RequirementComposeProvider:
					installPackages = append(installPackages, "podman-compose")
				case system.RequirementBuildah:
					installPackages = append(installPackages, "buildah")
				case system.RequirementSkopeo:
					installPackages = append(installPackages, "skopeo")
				case system.RequirementCockpit:
					installPackages = append(installPackages, "cockpit", "cockpit-podman")
				}
			}
			return installPackages, nil
		}
		d.enableCockpit = func(context.Context, system.Runner) error {
			enabledCockpit = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--install", "--yes")
	if err != nil {
		t.Fatalf("setup --install returned error: %v", err)
	}
	if len(installPackages) == 0 {
		t.Fatal("expected installPackages to be called")
	}
	if !enabledCockpit {
		t.Fatal("expected cockpit enable to be called")
	}
	if !strings.Contains(stdout, "Installed:") {
		t.Fatalf("stdout missing installed summary: %s", stdout)
	}
	if strings.Contains(stdout, "stackctl setup --install") {
		t.Fatalf("stdout should not keep stale install guidance after install succeeds: %s", stdout)
	}
}

func TestSetupInstallPreparesPodmanMachineOnDarwin(t *testing.T) {
	var installPackages []string
	preparedMachine := false

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeCockpit = false
		cfg.Setup.InstallCockpit = false
		cfg.System.PackageManager = "brew"
		d.platform = func() system.Platform {
			return system.Platform{GOOS: "darwin", PackageManager: "brew", ServiceManager: system.ServiceManagerNone}
		}
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman machine not initialized"},
			), nil
		}
		d.installPackages = func(_ context.Context, _ system.Runner, _ string, requirements []system.Requirement) ([]string, error) {
			for _, requirement := range requirements {
				switch requirement {
				case system.RequirementPodman:
					installPackages = append(installPackages, "podman")
				case system.RequirementComposeProvider:
					installPackages = append(installPackages, "podman-compose")
				case system.RequirementSkopeo:
					installPackages = append(installPackages, "skopeo")
				}
			}
			return installPackages, nil
		}
		d.preparePodmanMachine = func(context.Context, system.Runner) error {
			preparedMachine = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--install", "--yes")
	if err != nil {
		t.Fatalf("setup --install returned error: %v", err)
	}
	if !preparedMachine {
		t.Fatal("expected podman machine preparation")
	}
	if got, want := strings.Join(installPackages, ","), "podman,podman-compose"; got != want {
		t.Fatalf("unexpected installed packages: got %q want %q", got, want)
	}
	if !strings.Contains(stdout, "podman machine is initialized and running") {
		t.Fatalf("stdout missing podman machine status: %s", stdout)
	}
}

func TestSetupInstallFallsBackToDetectedPackageManagerWhenConfiguredValueIsUnavailable(t *testing.T) {
	var usedPackageManager string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.System.PackageManager = "brew"
		d.platform = func() system.Platform {
			return system.Platform{
				GOOS:           "linux",
				DistroID:       "ubuntu",
				DistroLike:     []string{"debian"},
				PackageManager: "apt",
				ServiceManager: system.ServiceManagerSystemd,
			}
		}
		d.commandExists = func(name string) bool {
			return name == "apt-get" || name == "podman" || name == "podman-compose" || name == "buildah" || name == "skopeo" || name == "systemctl"
		}
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
			), nil
		}
		d.installPackages = func(_ context.Context, _ system.Runner, packageManager string, _ []system.Requirement) ([]string, error) {
			usedPackageManager = packageManager
			return []string{"podman"}, nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--install", "--yes")
	if err != nil {
		t.Fatalf("setup --install returned error: %v", err)
	}
	if usedPackageManager != "apt" {
		t.Fatalf("expected apt fallback, got %q", usedPackageManager)
	}
	if !strings.Contains(stdout, `configured package manager "brew" is not installed; using detected apt`) {
		t.Fatalf("expected package manager fallback notice, got:\n%s", stdout)
	}
}

func TestSetupInstallPromptDeclineCancels(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--install")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "ℹ️ setup install cancelled") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestSetupReturnsDoctorError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return doctorpkg.Report{}, errors.New("doctor failed")
		}
	})

	_, _, err := executeRoot(t, "setup")
	if err == nil || !strings.Contains(err.Error(), "doctor failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorCommandPrintsSummaryAndFailsOnMisses(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "config file found"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
			), nil
		}
	})

	stdout, _, err := executeRoot(t, "doctor")
	if err == nil {
		t.Fatal("expected doctor to return an error when misses exist")
	}
	if !strings.Contains(stdout, "Summary: 1 ok, 0 warn, 1 miss, 0 fail") {
		t.Fatalf("stdout missing summary: %s", stdout)
	}
}

func TestStartFirstRunDeclineReturnsGuidance(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
	})

	stdout, _, err := executeRoot(t, "start")
	if err == nil {
		t.Fatal("expected start to fail when setup is declined")
	}
	if !strings.Contains(stdout, "No stackctl config was found.") {
		t.Fatalf("stdout missing first-run preamble: %s", stdout)
	}
	if !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartFirstRunRunsWizardComposeWaitAndPrintsEndpoints(t *testing.T) {
	var saved bool
	scaffolded := false
	var waitPorts []int
	upCalled := false
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.runWizard = func(_ io.Reader, _ io.Writer, cfg configpkg.Config) (configpkg.Config, error) { return cfg, nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			upCalled = true
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitPorts = append(waitPorts, port)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "start")
	if err != nil {
		t.Fatalf("start returned error: %v", err)
	}
	if !saved || !upCalled || !scaffolded {
		t.Fatal("expected wizard save and compose up to run")
	}
	if !reflect.DeepEqual(waitPorts, []int{5432, 6379, 4222}) {
		t.Fatalf("wait ports = %v", waitPorts)
	}
	if !strings.Contains(stdout, "✅ stack started") {
		t.Fatalf("stdout missing success line: %s", stdout)
	}
	if !strings.Contains(stdout, "🚀 starting stack...") {
		t.Fatalf("stdout missing action line: %s", stdout)
	}
	if !strings.Contains(stdout, "Postgres\n  postgres://app:app@localhost:5432/app") {
		t.Fatalf("stdout missing postgres connection info: %s", stdout)
	}
	if !strings.Contains(stdout, "NATS\n  nats://stackctl@localhost:4222") {
		t.Fatalf("stdout missing nats connection info: %s", stdout)
	}
	if !strings.Contains(stdout, "Cockpit\n  https://localhost:9090") {
		t.Fatalf("stdout missing cockpit connection info: %s", stdout)
	}
}

func TestSetupOffersScaffoldingForExistingManagedConfig(t *testing.T) {
	scaffolded := false
	forced := false

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(_ io.Reader, _ io.Writer, question string, _ bool) (bool, error) {
			return strings.Contains(question, "Refresh the managed stack files now?"), nil
		}
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			forced = force
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "config file found"}), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup")
	if err != nil {
		t.Fatalf("setup returned error: %v", err)
	}
	if !scaffolded {
		t.Fatal("expected setup to scaffold the managed stack when prompted")
	}
	if !forced {
		t.Fatal("expected setup to force-refresh stale managed scaffold files")
	}
	if !strings.Contains(stdout, "wrote managed compose file") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestStartRefreshesManagedScaffoldBeforeComposeUp(t *testing.T) {
	scaffolded := false
	forced := false
	composeUpCalled := false
	cfg := configpkg.Default()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			forced = force
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			composeUpCalled = true
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
	})

	stdout, _, err := executeRoot(t, "start")
	if err != nil {
		t.Fatalf("start returned error: %v", err)
	}
	if !scaffolded || !forced {
		t.Fatalf("expected start to force-refresh managed scaffold before compose up, scaffolded=%v forced=%v", scaffolded, forced)
	}
	if !composeUpCalled {
		t.Fatal("expected start to continue with compose up after scaffold refresh")
	}
	if !strings.Contains(stdout, "wrote managed compose file") {
		t.Fatalf("stdout missing scaffold refresh message: %s", stdout)
	}
}

func TestLogsInvalidServiceReturnsError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	_, _, err := executeRoot(t, "logs", "-s", "bad")
	if err == nil {
		t.Fatal("expected logs to reject invalid service")
	}
	if !strings.Contains(err.Error(), "valid values: postgres, redis, nats, seaweedfs, meilisearch, pgadmin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogsServiceUsesComposeServiceFilter(t *testing.T) {
	var capturedService string
	var capturedTail int
	var follow bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeLogs = func(_ context.Context, _ system.Runner, _ configpkg.Config, tail int, watch bool, _ string, service string) error {
			capturedService = service
			capturedTail = tail
			follow = watch
			return nil
		}
	})

	_, _, err := executeRoot(t, "logs", "-s", "postgres", "-w", "-n", "200")
	if err != nil {
		t.Fatalf("logs returned error: %v", err)
	}
	if capturedService != "postgres" || capturedTail != 200 || !follow {
		t.Fatalf("unexpected log call: service=%s tail=%d follow=%v", capturedService, capturedTail, follow)
	}
}

func TestOpenAllOpensConfiguredTargets(t *testing.T) {
	var opened []string

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			opened = append(opened, target)
			return nil
		}
	})

	_, _, err := executeRoot(t, "open", "all")
	if err != nil {
		t.Fatalf("open all returned error: %v", err)
	}
	if len(opened) != 2 {
		t.Fatalf("opened urls = %v", opened)
	}
}

func TestHealthReportsPortAndContainerStatus(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.portListening = func(port int) bool { return port != 9090 }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[]},{"Names":["local-redis"],"Image":"redis:7","Status":"Up","State":"running","Ports":[]},{"Names":["local-pgadmin"],"Image":"dpage/pgadmin4:latest","Status":"Up","State":"running","Ports":[]}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "health")
	if err != nil {
		t.Fatalf("health returned error: %v", err)
	}
	if !strings.Contains(stdout, "⚠️ cockpit port not listening") {
		t.Fatalf("stdout missing cockpit warning: %s", stdout)
	}
	if !strings.Contains(stdout, "✅ postgres running") {
		t.Fatalf("stdout missing postgres running line: %s", stdout)
	}
	if !strings.Contains(stdout, "✅ redis running") {
		t.Fatalf("stdout missing redis running line: %s", stdout)
	}
	if !strings.Contains(stdout, "✅ pgadmin running") {
		t.Fatalf("stdout missing pgadmin running line: %s", stdout)
	}
}

func TestResetVolumesDeclineCancels(t *testing.T) {
	downCalled := false

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			downCalled = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "reset", "--volumes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downCalled {
		t.Fatal("compose down should not have been called")
	}
	if !strings.Contains(stdout, "ℹ️ reset cancelled") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestWaitForConfiguredServicesPropagatesTimeout(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error {
			return errors.New("timeout")
		}
	})

	err := waitForConfiguredServices(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("unexpected wait error: %v", err)
	}
}
