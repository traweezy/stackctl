package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive stack dashboard",
		Long: "Open the interactive read-only stack dashboard.\n\n" +
			"Use a read-only operator view for overview, services, health, and\n" +
			"connections. The dashboard supports manual refresh, optional\n" +
			"auto-refresh, compact mode, and masked secrets by default.",
		Example: "  stackctl tui",
		RunE: func(cmd *cobra.Command, args []string) error {
			model := stacktui.NewModel(func() (stacktui.Snapshot, error) {
				return loadTUISnapshot()
			})

			program := tea.NewProgram(model)
			_, err := program.Run()
			return err
		},
	}
}

func loadTUISnapshot() (stacktui.Snapshot, error) {
	configPath, err := deps.configFilePath()
	if err != nil {
		return stacktui.Snapshot{}, err
	}

	cfg, err := deps.loadConfig(configPath)
	if err != nil {
		return stacktui.Snapshot{}, missingConfigHint(err)
	}

	issues := deps.validateConfig(cfg)
	if len(issues) > 0 {
		return stacktui.Snapshot{}, validationIssuesError(issues)
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
		Services:          make([]stacktui.Service, 0, len(services)),
		Health:            make([]stacktui.HealthLine, 0, len(health)),
		Connections:       make([]stacktui.Connection, 0, len(connectionEntries(cfg))),
	}

	for _, service := range services {
		snapshot.Services = append(snapshot.Services, stacktui.Service{
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
