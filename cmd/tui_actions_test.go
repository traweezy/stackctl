package cmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func TestRunTUIActionStartUsesComposeUpAndWait(t *testing.T) {
	var composeUpCalls int
	var waitedPorts []int

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			composeUpCalls++
			return nil
		}
		value.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionStart)
	if err != nil {
		t.Fatalf("runTUIAction(start) returned error: %v", err)
	}
	if composeUpCalls != 1 {
		t.Fatalf("expected composeUp once, got %d", composeUpCalls)
	}
	if len(waitedPorts) != 3 || waitedPorts[0] != 5432 || waitedPorts[1] != 6379 || waitedPorts[2] != 4222 {
		t.Fatalf("unexpected waited ports: %+v", waitedPorts)
	}
	if !report.Refresh || report.Message != "stack started" {
		t.Fatalf("unexpected start report: %+v", report)
	}
}

func TestRunTUIActionUseStackPersistsSelection(t *testing.T) {
	var selected string
	t.Setenv(configpkg.StackNameEnvVar, configpkg.DefaultStackName)

	withTestDeps(t, func(value *commandDeps) {
		value.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionID("use-stack:staging"))
	if err != nil {
		t.Fatalf("runTUIAction(use-stack) returned error: %v", err)
	}
	if selected != "staging" {
		t.Fatalf("selected stack = %q", selected)
	}
	if current := os.Getenv(configpkg.StackNameEnvVar); current != "staging" {
		t.Fatalf("expected process stack env to update, got %q", current)
	}
	if !report.Refresh || report.Message != "selected stack staging" {
		t.Fatalf("unexpected use stack report: %+v", report)
	}
}

func TestRunTUIActionDeleteManagedStackPurgesData(t *testing.T) {
	var removedConfig string
	var removedData string
	var selected string
	composeDownCalled := false
	t.Setenv(configpkg.StackNameEnvVar, "staging")

	withTestDeps(t, func(value *commandDeps) {
		value.currentStackName = func() (string, error) { return "staging", nil }
		value.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
		value.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
		value.stat = func(path string) (os.FileInfo, error) {
			switch path {
			case "/tmp/stackctl/stacks/staging.yaml", "/tmp/stackctl-data/stacks/staging/compose.yaml":
				return fakeFileInfo{name: "existing"}, nil
			default:
				return nil, os.ErrNotExist
			}
		}
		value.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"
			return cfg, nil
		}
		value.composePath = func(cfg configpkg.Config) string {
			return cfg.Stack.Dir + "/compose.yaml"
		}
		value.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
		value.composeDownPath = func(_ context.Context, _ system.Runner, dir, composePath string, removeVolumes bool) error {
			composeDownCalled = true
			if dir != "/tmp/stackctl-data/stacks/staging" || composePath != "/tmp/stackctl-data/stacks/staging/compose.yaml" || !removeVolumes {
				t.Fatalf("unexpected compose down args: dir=%s compose=%s removeVolumes=%v", dir, composePath, removeVolumes)
			}
			return nil
		}
		value.removeAll = func(path string) error {
			removedData = path
			return nil
		}
		value.removeFile = func(path string) error {
			removedConfig = path
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionID("delete-stack:staging"))
	if err != nil {
		t.Fatalf("runTUIAction(delete-stack) returned error: %v", err)
	}
	if !composeDownCalled {
		t.Fatal("expected managed stack delete to stop the stack")
	}
	if removedConfig != "/tmp/stackctl/stacks/staging.yaml" {
		t.Fatalf("removed config = %q", removedConfig)
	}
	if removedData != "/tmp/stackctl-data/stacks/staging" {
		t.Fatalf("removed data = %q", removedData)
	}
	if selected != configpkg.DefaultStackName {
		t.Fatalf("selected stack reset = %q", selected)
	}
	if current := os.Getenv(configpkg.StackNameEnvVar); current != configpkg.DefaultStackName {
		t.Fatalf("expected process stack env reset, got %q", current)
	}
	if !report.Refresh || report.Message != "deleted stack staging" {
		t.Fatalf("unexpected delete stack report: %+v", report)
	}
}

func TestRunTUIActionStopUsesComposeDown(t *testing.T) {
	var removeVolumes bool
	var composeDownCalls int

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeDown = func(_ context.Context, _ system.Runner, _ configpkg.Config, volumes bool) error {
			composeDownCalls++
			removeVolumes = volumes
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionStop)
	if err != nil {
		t.Fatalf("runTUIAction(stop) returned error: %v", err)
	}
	if composeDownCalls != 1 {
		t.Fatalf("expected composeDown once, got %d", composeDownCalls)
	}
	if removeVolumes {
		t.Fatalf("expected stop action to keep volumes")
	}
	if report.Message != "stack stopped" || !report.Refresh {
		t.Fatalf("unexpected stop report: %+v", report)
	}
}

func TestRunTUIActionRestartUsesDownUpAndWait(t *testing.T) {
	var composeDownCalls int
	var composeUpCalls int
	var waitedPorts []int

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			composeDownCalls++
			return nil
		}
		value.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			composeUpCalls++
			return nil
		}
		value.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionRestart)
	if err != nil {
		t.Fatalf("runTUIAction(restart) returned error: %v", err)
	}
	if composeDownCalls != 1 || composeUpCalls != 1 {
		t.Fatalf("expected restart to call down and up once each, got down=%d up=%d", composeDownCalls, composeUpCalls)
	}
	if len(waitedPorts) != 3 {
		t.Fatalf("expected restart waits, got %+v", waitedPorts)
	}
	if report.Message != "stack restarted" || !report.Refresh {
		t.Fatalf("unexpected restart report: %+v", report)
	}
}

func TestRunTUIActionStartNamedStackUsesSelectedProfileConfig(t *testing.T) {
	var composeUpCalls int
	var waitedPorts []int
	t.Setenv(configpkg.StackNameEnvVar, configpkg.DefaultStackName)
	t.Setenv("XDG_DATA_HOME", "/tmp/stackctl-data")

	withTestDeps(t, func(value *commandDeps) {
		value.composePath = func(cfg configpkg.Config) string {
			return cfg.Stack.Dir + "/compose.yaml"
		}
		value.configFilePathForStack = func(name string) (string, error) {
			switch name {
			case "staging":
				return "/tmp/stackctl/stacks/staging.yaml", nil
			case configpkg.DefaultStackName:
				return "/tmp/stackctl/config.yaml", nil
			default:
				return "", errors.New("unexpected stack name")
			}
		}
		value.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/stackctl/stacks/staging.yaml" || path == "/tmp/stackctl-data/stackctl/stacks/staging/compose.yaml" {
				return fakeFileInfo{name: "staging.yaml"}, nil
			}
			return nil, os.ErrNotExist
		}
		value.loadConfig = func(path string) (configpkg.Config, error) {
			if path != "/tmp/stackctl/stacks/staging.yaml" {
				return configpkg.Config{}, errors.New("unexpected config path")
			}
			cfg := configpkg.DefaultForStack("staging")
			cfg.Ports.Postgres = 25432
			cfg.Ports.Redis = 26379
			cfg.Ports.NATS = 24222
			cfg.ApplyDerivedFields()
			return cfg, nil
		}
		value.composeUp = func(_ context.Context, _ system.Runner, cfg configpkg.Config) error {
			composeUpCalls++
			if cfg.Stack.Name != "staging" {
				t.Fatalf("expected staging config, got %q", cfg.Stack.Name)
			}
			return nil
		}
		value.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionID("start-stack:staging"))
	if err != nil {
		t.Fatalf("runTUIAction(start-stack) returned error: %v", err)
	}
	if composeUpCalls != 1 {
		t.Fatalf("expected composeUp once, got %d", composeUpCalls)
	}
	if len(waitedPorts) != 3 || waitedPorts[0] != 25432 || waitedPorts[1] != 26379 || waitedPorts[2] != 24222 {
		t.Fatalf("unexpected waited ports: %v", waitedPorts)
	}
	if report.Message != "stack staging started" || !report.Refresh {
		t.Fatalf("unexpected named stack start report: %+v", report)
	}
	for _, fragment := range []string{
		"Config: /tmp/stackctl/stacks/staging.yaml",
		"Selected stack remains dev-stack",
		"Use staging to inspect its config, services, and health in the rest of the dashboard.",
	} {
		if !strings.Contains(strings.Join(report.Details, " "), fragment) {
			t.Fatalf("expected report details to contain %q: %+v", fragment, report)
		}
	}
	if current := os.Getenv(configpkg.StackNameEnvVar); current != configpkg.DefaultStackName {
		t.Fatalf("expected selected stack to remain unchanged, got %q", current)
	}
}

func TestRunTUIActionStartServiceUsesComposeUpServicesAndWait(t *testing.T) {
	var calledServices []string
	var forceRecreate bool
	var waitedPorts []int

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, force bool, services []string) error {
			forceRecreate = force
			calledServices = append([]string(nil), services...)
			return nil
		}
		value.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionID("start-service:postgres"))
	if err != nil {
		t.Fatalf("runTUIAction(start-service) returned error: %v", err)
	}
	if forceRecreate {
		t.Fatal("service start should not force recreate")
	}
	if len(calledServices) != 1 || calledServices[0] != "postgres" {
		t.Fatalf("unexpected service selection: %v", calledServices)
	}
	if len(waitedPorts) != 1 || waitedPorts[0] != 5432 {
		t.Fatalf("unexpected waited ports: %v", waitedPorts)
	}
	if report.Message != "Postgres started" || !report.Refresh {
		t.Fatalf("unexpected start service report: %+v", report)
	}
}

func TestRunTUIActionStopServiceUsesComposeStopServices(t *testing.T) {
	var calledServices []string

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeStopServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, services []string) error {
			calledServices = append([]string(nil), services...)
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionID("stop-service:redis"))
	if err != nil {
		t.Fatalf("runTUIAction(stop-service) returned error: %v", err)
	}
	if len(calledServices) != 1 || calledServices[0] != "redis" {
		t.Fatalf("unexpected service selection: %v", calledServices)
	}
	if report.Message != "Redis stopped" || !report.Refresh {
		t.Fatalf("unexpected stop service report: %+v", report)
	}
}

func TestRunTUIActionRestartServiceUsesForceRecreate(t *testing.T) {
	var calledServices []string
	var forceRecreate bool
	var waitedPorts []int

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, force bool, services []string) error {
			forceRecreate = force
			calledServices = append([]string(nil), services...)
			return nil
		}
		value.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	report, err := runTUIAction(stacktui.ActionID("restart-service:nats"))
	if err != nil {
		t.Fatalf("runTUIAction(restart-service) returned error: %v", err)
	}
	if !forceRecreate {
		t.Fatal("service restart should force recreate")
	}
	if len(calledServices) != 1 || calledServices[0] != "nats" {
		t.Fatalf("unexpected service selection: %v", calledServices)
	}
	if len(waitedPorts) != 1 || waitedPorts[0] != 4222 {
		t.Fatalf("unexpected waited ports: %v", waitedPorts)
	}
	if report.Message != "NATS restarted" || !report.Refresh {
		t.Fatalf("unexpected restart service report: %+v", report)
	}
}

func TestRunTUIActionOpenCockpitReturnsFallbackURL(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.openURL = func(context.Context, system.Runner, string) error {
			return errors.New("browser unavailable")
		}
	})

	report, err := runTUIAction(stacktui.ActionOpenCockpit)
	if err != nil {
		t.Fatalf("runTUIAction(open cockpit) returned error: %v", err)
	}
	if report.Status != output.StatusWarn {
		t.Fatalf("expected warning report, got %+v", report)
	}
	for _, fragment := range []string{
		"browser launch is unavailable; use this cockpit URL",
		"Cockpit: https://localhost:9090",
	} {
		if !strings.Contains(report.Message+" "+strings.Join(report.Details, " "), fragment) {
			t.Fatalf("expected cockpit fallback report to contain %q: %+v", fragment, report)
		}
	}
}

func TestRunTUIActionOpenPgAdminReturnsFallbackURL(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.openURL = func(context.Context, system.Runner, string) error {
			return errors.New("browser unavailable")
		}
	})

	report, err := runTUIAction(stacktui.ActionOpenPgAdmin)
	if err != nil {
		t.Fatalf("runTUIAction(open pgadmin) returned error: %v", err)
	}
	if report.Status != output.StatusWarn {
		t.Fatalf("expected warning report, got %+v", report)
	}
	for _, fragment := range []string{
		"browser launch is unavailable; use this pgadmin URL",
		"pgAdmin: http://localhost:8081",
	} {
		if !strings.Contains(report.Message+" "+strings.Join(report.Details, " "), fragment) {
			t.Fatalf("expected pgadmin fallback report to contain %q: %+v", fragment, report)
		}
	}
}

func TestRunTUIActionDoctorSummarizesIssues(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			return cfg, nil
		}
		value.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "config valid"},
				doctorpkg.Check{Status: output.StatusWarn, Message: "port 9090 is busy"},
				doctorpkg.Check{Status: output.StatusMiss, Message: "buildah not installed"},
			), nil
		}
	})

	report, err := runTUIAction(stacktui.ActionDoctor)
	if err != nil {
		t.Fatalf("runTUIAction(doctor) returned error: %v", err)
	}
	if report.Status != output.StatusFail {
		t.Fatalf("expected failing doctor status, got %+v", report)
	}
	for _, fragment := range []string{
		"doctor finished: 1 ok, 1 warn, 1 miss, 0 fail",
		"warn port 9090 is busy",
		"miss buildah not installed",
	} {
		if !strings.Contains(report.Message+" "+strings.Join(report.Details, " "), fragment) {
			t.Fatalf("expected doctor report to contain %q: %+v", fragment, report)
		}
	}
}
