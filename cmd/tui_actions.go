package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func runTUIAction(action stacktui.ActionID) (stacktui.ActionReport, error) {
	_, cfg, err := loadTUIConfig()
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	switch action {
	case stacktui.ActionStart:
		return runTUIStart(cfg)
	case stacktui.ActionStop:
		return runTUIStop(cfg)
	case stacktui.ActionRestart:
		return runTUIRestart(cfg)
	case stacktui.ActionOpenCockpit:
		return runTUIOpenTarget("Cockpit", cfg.URLs.Cockpit)
	case stacktui.ActionOpenPgAdmin:
		return runTUIOpenTarget("pgAdmin", cfg.URLs.PgAdmin)
	case stacktui.ActionDoctor:
		return runTUIDoctor()
	default:
		return stacktui.ActionReport{}, fmt.Errorf("unsupported tui action %q", action)
	}
}

func quietRunner() system.Runner {
	return system.Runner{
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
}

func runTUIStart(cfg configpkg.Config) (stacktui.ActionReport, error) {
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := deps.composeUp(context.Background(), quietRunner(), cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if cfg.Behavior.WaitForServicesStart {
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
		defer cancel()
		if err := waitForConfiguredServices(waitCtx, cfg); err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	details := []string{
		fmt.Sprintf("Wait for services: %s", boolLabel(cfg.Behavior.WaitForServicesStart)),
		fmt.Sprintf("Startup timeout: %ds", cfg.Behavior.StartupTimeoutSec),
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: "stack started",
		Details: details,
		Refresh: true,
	}, nil
}

func runTUIStop(cfg configpkg.Config) (stacktui.ActionReport, error) {
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := deps.composeDown(context.Background(), quietRunner(), cfg, false); err != nil {
		return stacktui.ActionReport{}, err
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: "stack stopped",
		Details: []string{"Volumes were left intact."},
		Refresh: true,
	}, nil
}

func runTUIRestart(cfg configpkg.Config) (stacktui.ActionReport, error) {
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := deps.composeDown(context.Background(), quietRunner(), cfg, false); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := deps.composeUp(context.Background(), quietRunner(), cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if cfg.Behavior.WaitForServicesStart {
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
		defer cancel()
		if err := waitForConfiguredServices(waitCtx, cfg); err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: "stack restarted",
		Details: []string{
			fmt.Sprintf("Wait for services: %s", boolLabel(cfg.Behavior.WaitForServicesStart)),
		},
		Refresh: true,
	}, nil
}

func runTUIOpenTarget(name, targetURL string) (stacktui.ActionReport, error) {
	if strings.TrimSpace(targetURL) == "" {
		return stacktui.ActionReport{
			Status:  output.StatusWarn,
			Message: fmt.Sprintf("no %s URL is configured", strings.ToLower(name)),
		}, nil
	}

	if err := deps.openURL(context.Background(), quietRunner(), targetURL); err != nil {
		return stacktui.ActionReport{
			Status:  output.StatusWarn,
			Message: fmt.Sprintf("browser launch is unavailable; use this %s URL", strings.ToLower(name)),
			Details: []string{fmt.Sprintf("%s: %s", name, targetURL)},
		}, nil
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: fmt.Sprintf("opened %s", name),
		Details: []string{fmt.Sprintf("%s: %s", name, targetURL)},
	}, nil
}

func runTUIDoctor() (stacktui.ActionReport, error) {
	report, err := deps.runDoctor(context.Background())
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	status := output.StatusOK
	switch {
	case report.FailCount > 0 || report.MissCount > 0:
		status = output.StatusFail
	case report.WarnCount > 0:
		status = output.StatusWarn
	}

	return stacktui.ActionReport{
		Status:  status,
		Message: doctorSummary(report),
		Details: doctorDetails(report),
	}, nil
}

func doctorSummary(report doctorpkg.Report) string {
	return fmt.Sprintf(
		"doctor finished: %d ok, %d warn, %d miss, %d fail",
		report.OKCount,
		report.WarnCount,
		report.MissCount,
		report.FailCount,
	)
}

func doctorDetails(report doctorpkg.Report) []string {
	details := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		if check.Status == output.StatusOK {
			continue
		}
		details = append(details, fmt.Sprintf("%s %s", strings.ToLower(check.Status), check.Message))
	}
	if len(details) == 0 {
		return []string{"No issues found."}
	}

	return details
}

func boolLabel(value bool) string {
	if value {
		return "on"
	}

	return "off"
}
