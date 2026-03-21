package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDefaultCommandDepsProvidesHooks(t *testing.T) {
	value := defaultCommandDeps()
	if value.configFilePath == nil || value.composeUp == nil || value.removeFile == nil {
		t.Fatal("default command deps should initialize function hooks")
	}
}

func TestDefaultCommandDepsClosuresExecuteAgainstFakeBinaries(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")

	writeScript := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write script %s: %v", name, err)
		}
	}

	writeScript("podman", "#!/bin/sh\necho podman \"$@\" >> \""+logPath+"\"\n")
	writeScript("xdg-open", "#!/bin/sh\necho xdg-open \"$@\" >> \""+logPath+"\"\n")
	writeScript("sudo", "#!/bin/sh\necho sudo \"$@\" >> \""+logPath+"\"\n")

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.Stack.Dir = dir
	cfg.Stack.ComposeFile = "compose.yaml"
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	value := defaultCommandDeps()
	runner := system.Runner{Stdout: io.Discard, Stderr: io.Discard}

	if err := value.openURL(context.Background(), runner, "http://example.com"); err != nil {
		t.Fatalf("openURL returned error: %v", err)
	}
	if _, err := value.installPackages(context.Background(), runner, "apt", []string{"podman"}); err != nil {
		t.Fatalf("installPackages returned error: %v", err)
	}
	if err := value.enableCockpit(context.Background(), runner); err != nil {
		t.Fatalf("enableCockpit returned error: %v", err)
	}
	if err := value.composeUp(context.Background(), runner, cfg); err != nil {
		t.Fatalf("composeUp returned error: %v", err)
	}
	if err := value.composeDown(context.Background(), runner, cfg, true); err != nil {
		t.Fatalf("composeDown returned error: %v", err)
	}
	if err := value.composeLogs(context.Background(), runner, cfg, 10, true, "1m"); err != nil {
		t.Fatalf("composeLogs returned error: %v", err)
	}
	if err := value.containerLogs(context.Background(), runner, "local-postgres", 5, false, ""); err != nil {
		t.Fatalf("containerLogs returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "xdg-open") || !strings.Contains(output, "podman compose") || !strings.Contains(output, "sudo apt-get install -y podman") {
		t.Fatalf("unexpected command log: %s", output)
	}
}

func TestDefaultTerminalInteractive(t *testing.T) {
	_ = defaultTerminalInteractive()
}

func TestConfirmWithPromptRequiresTerminal(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	_, err := confirmWithPrompt(NewRootCmd(NewApp()), "Continue?", true)
	if err == nil {
		t.Fatal("expected confirmation to require a terminal")
	}
}

func TestConfigPathPrintsResolvedPath(t *testing.T) {
	withTestDeps(t, nil)

	stdout, _, err := executeRoot(t, "config", "path")
	if err != nil {
		t.Fatalf("config path returned error: %v", err)
	}
	if strings.TrimSpace(stdout) != "/tmp/stackctl/config.yaml" {
		t.Fatalf("unexpected config path output: %q", stdout)
	}
}

func TestConfigEditNonInteractiveSavesUpdatedConfig(t *testing.T) {
	var saved bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "edit", "--non-interactive")
	if err != nil {
		t.Fatalf("config edit returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected config edit to save config")
	}
	if !strings.Contains(stdout, "Updated config at /tmp/stackctl/config.yaml") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigInitForceOverwritesExistingConfig(t *testing.T) {
	var saved bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "config", "init", "--non-interactive", "--force")
	if err != nil {
		t.Fatalf("config init --force returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected config init to save config")
	}
}

func TestConfigViewMissingConfigReturnsGuidance(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "config", "view")
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigResetForceWritesDefaults(t *testing.T) {
	var saved configpkg.Config

	withTestDeps(t, func(d *commandDeps) {
		d.saveConfig = func(_ string, cfg configpkg.Config) error {
			saved = cfg
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "reset", "--force")
	if err != nil {
		t.Fatalf("config reset returned error: %v", err)
	}
	if saved.Stack.Name != "dev-stack" {
		t.Fatalf("unexpected saved config: %+v", saved)
	}
	if !strings.Contains(stdout, "Reset config at /tmp/stackctl/config.yaml to defaults") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigResetDeleteWithoutTerminalNeedsForce(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "config", "reset", "--delete")
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectPrintsConnectionInfo(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	stdout, _, err := executeRoot(t, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !strings.Contains(stdout, "postgres://app:app@localhost:5432/app") {
		t.Fatalf("unexpected connect output: %s", stdout)
	}
}

func TestStatusPrintsTable(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef1234567890","Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[{"host_port":5432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if !strings.Contains(stdout, "local-postgres") || !strings.Contains(stdout, "5432->5432/tcp") {
		t.Fatalf("unexpected status output: %s", stdout)
	}
}

func TestStatusJSONPrintsContainers(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef1234567890","Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json returned error: %v", err)
	}
	if !strings.Contains(stdout, "\"local-postgres\"") {
		t.Fatalf("unexpected json status output: %s", stdout)
	}
}

func TestStatusVerboseShowsExtraColumns(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef1234567890","Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[{"host_port":5432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "status", "--verbose")
	if err != nil {
		t.Fatalf("status --verbose returned error: %v", err)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "abcdef123456") {
		t.Fatalf("unexpected verbose status output: %s", stdout)
	}
}

func TestStopRunsComposeDown(t *testing.T) {
	downCalled := false

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			downCalled = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "stop")
	if err != nil {
		t.Fatalf("stop returned error: %v", err)
	}
	if !downCalled {
		t.Fatal("expected stop to call compose down")
	}
}

func TestStatusRequiresPodman(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.commandExists = func(string) bool { return false }
	})

	_, _, err := executeRoot(t, "status")
	if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetupRejectsConflictingModes(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "setup", "--interactive", "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetupWithoutTerminalRequiresNonInteractive(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	_, _, err := executeRoot(t, "setup")
	if err == nil || !strings.Contains(err.Error(), "cannot prompt without a terminal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetupInstallWithNothingMissingExitsCleanly(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
			), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--install")
	if err != nil {
		t.Fatalf("setup --install returned error: %v", err)
	}
	if !strings.Contains(stdout, "nothing to install") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestRestartRunsDownUpWaitAndOpen(t *testing.T) {
	calls := make([]string, 0, 4)

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Behavior.OpenPgAdminOnStart = true
			return cfg, nil
		}
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			calls = append(calls, "down")
			return nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			calls = append(calls, "up")
			return nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			calls = append(calls, target)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "restart")
	if err != nil {
		t.Fatalf("restart returned error: %v", err)
	}
	if len(calls) < 4 || calls[0] != "down" || calls[1] != "up" {
		t.Fatalf("unexpected restart call order: %v", calls)
	}
	if !strings.Contains(stdout, "[OK  ] stack restarted") {
		t.Fatalf("unexpected restart output: %s", stdout)
	}
}

func TestOpenRejectsInvalidTarget(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	_, _, err := executeRoot(t, "open", "bad")
	if err == nil {
		t.Fatal("expected invalid open target error")
	}
	if !strings.Contains(err.Error(), "valid values: cockpit, pgadmin, all") {
		t.Fatalf("unexpected open error: %v", err)
	}
}

func TestOpenDefaultsToCockpit(t *testing.T) {
	opened := ""

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			opened = target
			return nil
		}
	})

	_, _, err := executeRoot(t, "open")
	if err != nil {
		t.Fatalf("open returned error: %v", err)
	}
	if opened != configpkg.Default().URLs.Cockpit {
		t.Fatalf("unexpected opened url: %s", opened)
	}
}

func TestOpenPgAdminDisabledReturnsError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Setup.IncludePgAdmin = false
			return cfg, nil
		}
	})

	_, _, err := executeRoot(t, "open", "pgadmin")
	if err == nil || !strings.Contains(err.Error(), "pgadmin is disabled") {
		t.Fatalf("unexpected open error: %v", err)
	}
}

func TestVersionIncludesOptionalMetadata(t *testing.T) {
	app := NewApp()
	app.GitCommit = "abc123"
	app.BuildDate = "2026-03-21"

	stdout, _, err := executeAppRoot(t, app, "version")
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	if !strings.Contains(stdout, "git_commit: abc123") || !strings.Contains(stdout, "build_date: 2026-03-21") {
		t.Fatalf("unexpected version output: %s", stdout)
	}
}

func TestResolveConfigFromFlagsRejectsNonInteractiveWithoutTerminal(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	_, err := resolveConfigFromFlags(NewRootCmd(NewApp()), configpkg.Default(), false)
	if err == nil || !strings.Contains(err.Error(), "requires a terminal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorCommandSucceedsWithCleanReport(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "config file found"}), nil
		}
	})

	stdout, _, err := executeRoot(t, "doctor")
	if err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	if !strings.Contains(stdout, "Summary: 1 ok, 0 warn, 0 miss, 0 fail") {
		t.Fatalf("unexpected doctor output: %s", stdout)
	}
}

func TestEnsureComposeRuntimeErrorsWhenPrerequisitesMissing(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.commandExists = func(string) bool { return false }
	})

	err := ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
		t.Fatalf("unexpected error: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.commandExists = func(string) bool { return true }
		d.podmanComposeAvail = func(context.Context) bool { return false }
	})

	err = ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "podman compose is not available") {
		t.Fatalf("unexpected error: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.commandExists = func(string) bool { return true }
		d.podmanComposeAvail = func(context.Context) bool { return true }
		d.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	})

	err = ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "compose file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeConfigWithoutFirstRunReturnsHint(t *testing.T) {
	withTestDeps(t, nil)

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeConfigPassesThroughUnexpectedLoadError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("boom") }
	})

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeConfigFailsValidation(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.dir", Message: "invalid"}}
		}
	})

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadStackContainersReturnsParseError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "{"}, nil
		}
	})

	_, err := loadStackContainers(context.Background(), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "parse podman status output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthChecksWarnWhenContainersMissing(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	lines, err := healthChecks(context.Background(), configpkg.Default())
	if err != nil {
		t.Fatalf("healthChecks returned error: %v", err)
	}
	if got := lines[len(lines)-1].Message; got != "no containers from this stack were found" {
		t.Fatalf("unexpected final health message: %s", got)
	}
}

func TestHealthChecksWarnWhenContainersNotRunning(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Names":["local-postgres"],"Image":"postgres:16","Status":"Exited","State":"exited","Ports":[]}]`,
			}, nil
		}
	})

	lines, err := healthChecks(context.Background(), configpkg.Default())
	if err != nil {
		t.Fatalf("healthChecks returned error: %v", err)
	}
	if got := lines[len(lines)-1].Message; got != "some stack containers are not running" {
		t.Fatalf("unexpected final health message: %s", got)
	}
}

func TestPrintStatusTableNoContainers(t *testing.T) {
	root := NewRootCmd(NewApp())
	var out strings.Builder
	root.SetOut(&out)

	if err := printStatusTable(root, nil, false); err != nil {
		t.Fatalf("printStatusTable returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No containers from this stack were found.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestShortIDLeavesShortValuesUntouched(t *testing.T) {
	if got := shortID("12345"); got != "12345" {
		t.Fatalf("unexpected short id: %s", got)
	}
}
