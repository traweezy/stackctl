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
)

func TestRunTUIStackLifecycleCommandsReturnNamedStackLoadErrors(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		value.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	})

	for name, run := range map[string]func(string) (any, error){
		"start":   func(stackName string) (any, error) { return runTUIStartStack(stackName) },
		"stop":    func(stackName string) (any, error) { return runTUIStopStack(stackName) },
		"restart": func(stackName string) (any, error) { return runTUIRestartStack(stackName) },
	} {
		t.Run(name, func(t *testing.T) {
			_, err := run("staging")
			if err == nil || !strings.Contains(err.Error(), "stack staging does not exist") {
				t.Fatalf("expected missing-stack error, got %v", err)
			}
		})
	}
}

func TestRunTUIRestartStackOmitsSelectionReminderWhenAlreadySelected(t *testing.T) {
	t.Setenv(configpkg.StackNameEnvVar, "staging")

	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.DefaultForStack("staging")
		cfg.ApplyDerivedFields()
		value.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
		value.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error { return nil }
		value.composeUp = func(context.Context, system.Runner, configpkg.Config) error { return nil }
		value.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		value.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	})

	report, err := runTUIRestartStack("staging")
	if err != nil {
		t.Fatalf("runTUIRestartStack returned error: %v", err)
	}
	if report.Message != "stack staging restarted" {
		t.Fatalf("unexpected restart stack report: %+v", report)
	}
	for _, detail := range report.Details {
		if strings.Contains(detail, "Selected stack remains") {
			t.Fatalf("did not expect selection reminder in %+v", report.Details)
		}
	}
}

func TestRunTUIDoctorWarnAndErrorPaths(t *testing.T) {
	t.Run("warn-only report", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "config valid"},
					doctorpkg.Check{Status: output.StatusWarn, Message: "port 5432 is busy"},
				), nil
			}
		})

		report, err := runTUIDoctor()
		if err != nil {
			t.Fatalf("runTUIDoctor returned error: %v", err)
		}
		if report.Status != output.StatusWarn {
			t.Fatalf("expected warning doctor status, got %+v", report)
		}
		if !strings.Contains(report.Message, "1 ok, 1 warn, 0 miss, 0 fail") {
			t.Fatalf("unexpected doctor summary: %+v", report)
		}
	})

	t.Run("doctor execution failure", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return doctorpkg.Report{}, errors.New("doctor failed")
			}
		})

		_, err := runTUIDoctor()
		if err == nil || !strings.Contains(err.Error(), "doctor failed") {
			t.Fatalf("expected doctor error, got %v", err)
		}
	})
}
