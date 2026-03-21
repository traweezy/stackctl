package cmd

import (
	"context"
	"errors"
	"io"
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

	withTestDeps(t, func(d *commandDeps) {
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
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
	if !strings.Contains(stdout, "created default config") {
		t.Fatalf("stdout missing config creation message: %s", stdout)
	}
	if !strings.Contains(stdout, "Next steps:") {
		t.Fatalf("stdout missing next steps: %s", stdout)
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
	if !strings.Contains(stdout, "config file not found") || !strings.Contains(stdout, "Next steps:") {
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
		d.loadConfig = func(string) (configpkg.Config, error) {
			return configpkg.Default(), nil
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
		d.installPackages = func(_ context.Context, _ system.Runner, _ string, packages []string) ([]string, error) {
			installPackages = append([]string(nil), packages...)
			return packages, nil
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

	_, _, err := executeRoot(t, "setup", "--install")
	if err == nil || !strings.Contains(err.Error(), "setup install cancelled") {
		t.Fatalf("unexpected error: %v", err)
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

func TestStartFirstRunRunsWizardComposeWaitAndOpen(t *testing.T) {
	var saved bool
	var waitPorts []int
	var opened []string
	upCalled := false

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
		d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Behavior.OpenPgAdminOnStart = true
			return cfg, nil
		}
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			upCalled = true
			return nil
		}
		d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitPorts = append(waitPorts, port)
			return nil
		}
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			opened = append(opened, target)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "start")
	if err != nil {
		t.Fatalf("start returned error: %v", err)
	}
	if !saved || !upCalled {
		t.Fatal("expected wizard save and compose up to run")
	}
	if len(waitPorts) != 3 {
		t.Fatalf("wait ports = %v", waitPorts)
	}
	if len(opened) != 2 {
		t.Fatalf("opened urls = %v", opened)
	}
	if !strings.Contains(stdout, "[OK  ] stack started") {
		t.Fatalf("stdout missing success line: %s", stdout)
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
	if !strings.Contains(err.Error(), "valid values: postgres, redis, pgadmin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogsServiceUsesMappedContainer(t *testing.T) {
	var capturedContainer string
	var capturedTail int
	var follow bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.containerLogs = func(_ context.Context, _ system.Runner, container string, tail int, watch bool, _ string) error {
			capturedContainer = container
			capturedTail = tail
			follow = watch
			return nil
		}
	})

	_, _, err := executeRoot(t, "logs", "-s", "postgres", "-w", "-n", "200")
	if err != nil {
		t.Fatalf("logs returned error: %v", err)
	}
	if capturedContainer != "local-postgres" || capturedTail != 200 || !follow {
		t.Fatalf("unexpected log call: container=%s tail=%d follow=%v", capturedContainer, capturedTail, follow)
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
				Stdout: `[{"Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[]}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "health")
	if err != nil {
		t.Fatalf("health returned error: %v", err)
	}
	if !strings.Contains(stdout, "[WARN] cockpit port listening") {
		t.Fatalf("stdout missing cockpit warning: %s", stdout)
	}
	if !strings.Contains(stdout, "[OK  ] containers are running") {
		t.Fatalf("stdout missing container ok line: %s", stdout)
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

	_, _, err := executeRoot(t, "reset", "--volumes")
	if err == nil {
		t.Fatal("expected reset to cancel")
	}
	if !strings.Contains(err.Error(), "reset cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if downCalled {
		t.Fatal("compose down should not have been called")
	}
}

func TestWaitForConfiguredServicesPropagatesTimeout(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.waitForPort = func(context.Context, int, time.Duration) error {
			return errors.New("timeout")
		}
	})

	err := waitForConfiguredServices(context.Background(), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("unexpected wait error: %v", err)
	}
}
