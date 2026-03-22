package cmd

import (
	"context"
	"errors"
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
	if len(waitedPorts) != 2 || waitedPorts[0] != 5432 || waitedPorts[1] != 6379 {
		t.Fatalf("unexpected waited ports: %+v", waitedPorts)
	}
	if !report.Refresh || report.Message != "stack started" {
		t.Fatalf("unexpected start report: %+v", report)
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
	if len(waitedPorts) != 2 {
		t.Fatalf("expected restart waits, got %+v", waitedPorts)
	}
	if report.Message != "stack restarted" || !report.Refresh {
		t.Fatalf("unexpected restart report: %+v", report)
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
