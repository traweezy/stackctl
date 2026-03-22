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
	case stacktui.ActionOpen:
		return runTUIOpen(cfg)
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

func runTUIOpen(cfg configpkg.Config) (stacktui.ActionReport, error) {
	targets := []struct {
		name string
		url  string
	}{
		{name: "Cockpit", url: cfg.URLs.Cockpit},
	}
	if cfg.Setup.IncludePgAdmin && strings.TrimSpace(cfg.URLs.PgAdmin) != "" {
		targets = append(targets, struct {
			name string
			url  string
		}{name: "pgAdmin", url: cfg.URLs.PgAdmin})
	}

	if len(targets) == 0 {
		return stacktui.ActionReport{
			Status:  output.StatusWarn,
			Message: "no configured web UIs are available to open",
		}, nil
	}

	opened := make([]string, 0, len(targets))
	fallbacks := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := deps.openURL(context.Background(), quietRunner(), target.url); err != nil {
			fallbacks = append(fallbacks, fmt.Sprintf("%s: %s", target.name, target.url))
			continue
		}
		opened = append(opened, target.name)
	}

	switch {
	case len(fallbacks) == 0:
		return stacktui.ActionReport{
			Status:  output.StatusOK,
			Message: fmt.Sprintf("opened %s", strings.Join(opened, " and ")),
			Details: opened,
		}, nil
	case len(opened) == 0:
		return stacktui.ActionReport{
			Status:  output.StatusWarn,
			Message: "browser launch is unavailable; use the URLs below",
			Details: fallbacks,
		}, nil
	default:
		details := append([]string(nil), fallbacks...)
		return stacktui.ActionReport{
			Status:  output.StatusWarn,
			Message: fmt.Sprintf("opened %s; use these URLs for the rest", strings.Join(opened, " and ")),
			Details: details,
		}, nil
	}
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
