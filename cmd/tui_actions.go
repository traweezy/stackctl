package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func runTUIAction(action stacktui.ActionID) (stacktui.ActionReport, error) {
	if verb, stackName, ok := stacktuiStackAction(action); ok {
		switch verb {
		case "use":
			return runTUIUseStack(stackName)
		case "delete":
			return runTUIDeleteStack(stackName)
		case "start":
			return runTUIStartStack(stackName)
		case "stop":
			return runTUIStopStack(stackName)
		case "restart":
			return runTUIRestartStack(stackName)
		}
	}

	_, cfg, err := loadTUIConfig()
	if err != nil {
		return stacktui.ActionReport{}, err
	}
	if verb, service, ok := stacktuiServiceAction(action); ok {
		switch verb {
		case "start":
			return runTUIStart(cfg, []string{service})
		case "stop":
			return runTUIStop(cfg, []string{service})
		case "restart":
			return runTUIRestart(cfg, []string{service})
		}
	}

	switch action {
	case stacktui.ActionStart:
		return runTUIStart(cfg, nil)
	case stacktui.ActionStop:
		return runTUIStop(cfg, nil)
	case stacktui.ActionRestart:
		return runTUIRestart(cfg, nil)
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

func stacktuiServiceAction(action stacktui.ActionID) (string, string, bool) {
	return stacktui.ServiceActionTarget(action)
}

func stacktuiStackAction(action stacktui.ActionID) (string, string, bool) {
	return stacktui.StackActionTarget(action)
}

func runTUIUseStack(stackName string) (stacktui.ActionReport, error) {
	if err := deps.setCurrentStackName(stackName); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := os.Setenv(configpkg.StackNameEnvVar, stackName); err != nil {
		return stacktui.ActionReport{}, err
	}

	configPath, err := deps.configFilePathForStack(stackName)
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	report := stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: fmt.Sprintf("selected stack %s", stackName),
		Details: []string{fmt.Sprintf("Config: %s", configPath)},
		Refresh: true,
	}

	if _, exists, err := loadExistingConfig(configPath); err == nil && !exists {
		report.Details = append(report.Details, "No config exists yet. Open Config to create it.")
	}

	return report, nil
}

func runTUIDeleteStack(stackName string) (stacktui.ActionReport, error) {
	target, err := resolveStackTarget(stackName)
	if err != nil {
		return stacktui.ActionReport{}, err
	}
	if !target.Exists {
		return stacktui.ActionReport{}, fmt.Errorf("stack %s does not exist", stackName)
	}

	purgeData := target.LoadErr == nil && target.Config.Stack.Managed
	if !purgeData && target.LoadErr == nil {
		if services, err := runningStackServices(context.Background(), target.Config); err == nil && len(services) > 0 {
			return stacktui.ActionReport{}, fmt.Errorf("stack %s is running (%s); stop it before deleting the profile", stackName, strings.Join(services, ", "))
		}
	}

	result, err := deleteStackTarget(context.Background(), target, purgeData)
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	details := []string{fmt.Sprintf("Config: %s", result.ConfigPath)}
	if result.PurgedDataDir != "" {
		details = append(details, fmt.Sprintf("Purged managed data: %s", result.PurgedDataDir))
	}
	if result.ManagedDataKept != "" {
		details = append(details, fmt.Sprintf("Managed data still exists: %s", result.ManagedDataKept))
	}
	if result.ResetToDefault {
		if err := os.Setenv(configpkg.StackNameEnvVar, configpkg.DefaultStackName); err != nil {
			return stacktui.ActionReport{}, err
		}
		details = append(details, fmt.Sprintf("Selected stack reset to %s", configpkg.DefaultStackName))
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: fmt.Sprintf("deleted stack %s", stackName),
		Details: details,
		Refresh: true,
	}, nil
}

func loadTUIStackTargetConfig(stackName string) (stackTarget, error) {
	target, err := resolveStackTarget(stackName)
	if err != nil {
		return stackTarget{}, err
	}
	if !target.Exists {
		return stackTarget{}, fmt.Errorf("stack %s does not exist", stackName)
	}
	if target.LoadErr != nil {
		return stackTarget{}, fmt.Errorf("stack %s has an invalid config: %w", stackName, target.LoadErr)
	}
	if issues := deps.validateConfig(target.Config); len(issues) > 0 {
		return stackTarget{}, fmt.Errorf("stack %s config validation failed with %d issue(s)", stackName, len(issues))
	}

	return target, nil
}

func annotateStackLifecycleReport(stackName, configPath string, report stacktui.ActionReport) stacktui.ActionReport {
	report.Details = append([]string{fmt.Sprintf("Config: %s", configPath)}, report.Details...)

	selected := configpkg.SelectedStackName()
	if selected != stackName {
		report.Details = append(report.Details,
			fmt.Sprintf("Selected stack remains %s", selected),
			fmt.Sprintf("Use %s to inspect its config, services, and health in the rest of the dashboard.", stackName),
		)
	}

	return report
}

func runTUIStartStack(stackName string) (stacktui.ActionReport, error) {
	target, err := loadTUIStackTargetConfig(stackName)
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	report, err := runTUIStart(target.Config, nil)
	if err != nil {
		return stacktui.ActionReport{}, err
	}
	report.Message = fmt.Sprintf("stack %s started", stackName)

	return annotateStackLifecycleReport(stackName, target.ConfigPath, report), nil
}

func runTUIStopStack(stackName string) (stacktui.ActionReport, error) {
	target, err := loadTUIStackTargetConfig(stackName)
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	report, err := runTUIStop(target.Config, nil)
	if err != nil {
		return stacktui.ActionReport{}, err
	}
	report.Message = fmt.Sprintf("stack %s stopped", stackName)

	return annotateStackLifecycleReport(stackName, target.ConfigPath, report), nil
}

func runTUIRestartStack(stackName string) (stacktui.ActionReport, error) {
	target, err := loadTUIStackTargetConfig(stackName)
	if err != nil {
		return stacktui.ActionReport{}, err
	}

	report, err := runTUIRestart(target.Config, nil)
	if err != nil {
		return stacktui.ActionReport{}, err
	}
	report.Message = fmt.Sprintf("stack %s restarted", stackName)

	return annotateStackLifecycleReport(stackName, target.ConfigPath, report), nil
}

func runTUIStart(cfg configpkg.Config, services []string) (stacktui.ActionReport, error) {
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := ensureNoOtherRunningStack(context.Background()); err != nil {
		return stacktui.ActionReport{}, err
	}

	target := lifecycleTargetLabel(services)
	switch {
	case len(services) == 0:
		err := deps.composeUp(context.Background(), quietRunner(), cfg)
		if err != nil {
			return stacktui.ActionReport{}, err
		}
	default:
		err := deps.composeUpServices(context.Background(), quietRunner(), cfg, false, services)
		if err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	if cfg.Behavior.WaitForServicesStart {
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
		defer cancel()
		if err := waitForSelectedServices(waitCtx, cfg, services); err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	details := []string{
		fmt.Sprintf("Wait for services: %s", boolLabel(cfg.Behavior.WaitForServicesStart)),
		fmt.Sprintf("Startup timeout: %ds", cfg.Behavior.StartupTimeoutSec),
	}
	if len(services) > 0 {
		details = append(details, fmt.Sprintf("Service: %s", target))
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: fmt.Sprintf("%s started", target),
		Details: details,
		Refresh: true,
	}, nil
}

func runTUIStop(cfg configpkg.Config, services []string) (stacktui.ActionReport, error) {
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.ActionReport{}, err
	}

	target := lifecycleTargetLabel(services)
	switch {
	case len(services) == 0:
		err := deps.composeDown(context.Background(), quietRunner(), cfg, false)
		if err != nil {
			return stacktui.ActionReport{}, err
		}
	default:
		err := deps.composeStopServices(context.Background(), quietRunner(), cfg, services)
		if err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	details := []string{"Volumes were left intact."}
	if len(services) > 0 {
		details = append(details, fmt.Sprintf("Service: %s", target))
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: fmt.Sprintf("%s stopped", target),
		Details: details,
		Refresh: true,
	}, nil
}

func runTUIRestart(cfg configpkg.Config, services []string) (stacktui.ActionReport, error) {
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.ActionReport{}, err
	}
	if err := ensureNoOtherRunningStack(context.Background()); err != nil {
		return stacktui.ActionReport{}, err
	}

	target := lifecycleTargetLabel(services)
	switch {
	case len(services) == 0:
		if err := deps.composeDown(context.Background(), quietRunner(), cfg, false); err != nil {
			return stacktui.ActionReport{}, err
		}
		err := deps.composeUp(context.Background(), quietRunner(), cfg)
		if err != nil {
			return stacktui.ActionReport{}, err
		}
	default:
		err := deps.composeUpServices(context.Background(), quietRunner(), cfg, true, services)
		if err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	if cfg.Behavior.WaitForServicesStart {
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
		defer cancel()
		if err := waitForSelectedServices(waitCtx, cfg, services); err != nil {
			return stacktui.ActionReport{}, err
		}
	}

	details := []string{
		fmt.Sprintf("Wait for services: %s", boolLabel(cfg.Behavior.WaitForServicesStart)),
	}
	if len(services) > 0 {
		details = append(details, fmt.Sprintf("Service: %s", target))
	}

	return stacktui.ActionReport{
		Status:  output.StatusOK,
		Message: fmt.Sprintf("%s restarted", target),
		Details: details,
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
