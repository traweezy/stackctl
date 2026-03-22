package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive stack dashboard",
		Long: "Open the interactive stack dashboard.\n\n" +
			"Use a full-screen operator view for overview, services, ports,\n" +
			"health, connections, logs, and action history. The dashboard\n" +
			"supports manual refresh, optional auto-refresh, compact mode,\n" +
			"masked secrets by default, split inspection panes, and in-TUI\n" +
			"actions for stack lifecycle tasks. Use tab/shift+tab or h/l to\n" +
			"change sections, use j/k or [ and ] to switch the active\n" +
			"service/filter inside split inspection panes, and press f in\n" +
			"Logs to toggle follow mode with a conservative poll interval.",
		Example: "  stackctl tui",
		RunE: func(cmd *cobra.Command, args []string) error {
			model := stacktui.NewInspectionModel(func() (stacktui.Snapshot, error) {
				return loadTUISnapshot()
			}, loadTUILogs, runTUIAction)

			program := tea.NewProgram(model)
			_, err := program.Run()
			return err
		},
	}
}

func loadTUIConfig() (string, configpkg.Config, error) {
	configPath, err := deps.configFilePath()
	if err != nil {
		return "", configpkg.Config{}, err
	}

	cfg, err := deps.loadConfig(configPath)
	if err != nil {
		return "", configpkg.Config{}, missingConfigHint(err)
	}

	issues := deps.validateConfig(cfg)
	if len(issues) > 0 {
		return "", configpkg.Config{}, validationIssuesError(issues)
	}

	return configPath, cfg, nil
}

func loadTUISnapshot() (stacktui.Snapshot, error) {
	configPath, cfg, err := loadTUIConfig()
	if err != nil {
		return stacktui.Snapshot{}, err
	}

	return buildTUISnapshot(configPath, cfg), nil
}

func buildTUISnapshot(configPath string, cfg configpkg.Config) stacktui.Snapshot {
	ctx := context.Background()

	services, err := runtimeServices(ctx, cfg)
	serviceError := ""
	if err != nil {
		serviceError = err.Error()
	}

	health, err := healthChecks(ctx, cfg)
	healthError := ""
	if err != nil {
		healthError = err.Error()
	}

	snapshot := stacktui.Snapshot{
		ConfigPath:        configPath,
		StackName:         cfg.Stack.Name,
		StackDir:          cfg.Stack.Dir,
		ComposePath:       deps.composePath(cfg),
		Managed:           cfg.Stack.Managed,
		WaitForServices:   cfg.Behavior.WaitForServicesStart,
		StartupTimeoutSec: cfg.Behavior.StartupTimeoutSec,
		LoadedAt:          time.Now(),
		ServiceError:      serviceError,
		HealthError:       healthError,
		DoctorChecks:      []stacktui.DoctorCheck{},
		Services:          make([]stacktui.Service, 0, len(services)),
		Health:            make([]stacktui.HealthLine, 0, len(health)),
		Connections:       make([]stacktui.Connection, 0, len(connectionEntries(cfg))),
	}

	for _, service := range services {
		snapshot.Services = append(snapshot.Services, stacktui.Service{
			Name:            service.Name,
			DisplayName:     service.DisplayName,
			Status:          service.Status,
			ContainerName:   service.ContainerName,
			Image:           service.Image,
			DataVolume:      service.DataVolume,
			Host:            service.Host,
			ExternalPort:    service.ExternalPort,
			InternalPort:    service.InternalPort,
			PortListening:   service.ExternalPort > 0 && deps.portListening(service.ExternalPort),
			Database:        service.Database,
			MaintenanceDB:   service.MaintenanceDB,
			Email:           service.Email,
			Username:        service.Username,
			Password:        service.Password,
			AppendOnly:      service.AppendOnly,
			SavePolicy:      service.SavePolicy,
			MaxMemoryPolicy: service.MaxMemoryPolicy,
			ServerMode:      service.ServerMode,
			URL:             service.URL,
			DSN:             service.DSN,
		})
	}

	doctorReport, err := deps.runDoctor(ctx)
	if err != nil {
		snapshot.DoctorError = err.Error()
	} else {
		snapshot.DoctorSummary = stacktui.DoctorSummary{
			OK:   doctorReport.OKCount,
			Warn: doctorReport.WarnCount,
			Miss: doctorReport.MissCount,
			Fail: doctorReport.FailCount,
		}
		for _, check := range doctorReport.Checks {
			snapshot.DoctorChecks = append(snapshot.DoctorChecks, stacktui.DoctorCheck{
				Status:  check.Status,
				Message: check.Message,
			})
		}
	}

	for _, line := range health {
		snapshot.Health = append(snapshot.Health, stacktui.HealthLine{
			Status:  line.Status,
			Message: line.Message,
		})
	}

	for _, entry := range connectionEntries(cfg) {
		snapshot.Connections = append(snapshot.Connections, stacktui.Connection{
			Name:  entry.Name,
			Value: entry.Value,
		})
	}

	return snapshot
}

func loadTUILogs(request stacktui.LogRequest) (stacktui.LogSnapshot, error) {
	_, cfg, err := loadTUIConfig()
	if err != nil {
		return stacktui.LogSnapshot{}, err
	}
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return stacktui.LogSnapshot{}, err
	}

	service := strings.TrimSpace(request.Service)
	if service != "" {
		service, err = canonicalServiceName(service)
		if err != nil {
			return stacktui.LogSnapshot{}, err
		}
	}

	tail := request.Tail
	if tail <= 0 {
		tail = 200
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := system.Runner{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	err = deps.composeLogs(context.Background(), runner, cfg, tail, false, "", service)
	output := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(stdout.String()),
		strings.TrimSpace(stderr.String()),
	}, "\n"))
	if err != nil {
		if output != "" {
			return stacktui.LogSnapshot{
				Service:  service,
				Output:   output,
				LoadedAt: time.Now(),
			}, fmt.Errorf("%v", err)
		}
		return stacktui.LogSnapshot{}, err
	}

	return stacktui.LogSnapshot{
		Service:  service,
		Output:   output,
		LoadedAt: time.Now(),
	}, nil
}

func validationIssuesError(issues []configpkg.ValidationIssue) error {
	if len(issues) == 0 {
		return nil
	}

	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Field, issue.Message))
	}

	return errors.New("config validation failed: " + strings.Join(parts, "; "))
}
