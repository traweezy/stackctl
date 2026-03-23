package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

const tuiLogWatchTail = 100

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive stack dashboard",
		Long: "Open the interactive stack dashboard.\n\n" +
			"Use a full-screen operator view for overview, config, services,\n" +
			"health, and action history. The services pane includes host\n" +
			"ports, URLs, DSNs, and live-log handoff in one place. The dashboard\n" +
			"supports manual refresh, optional auto-refresh with a saved\n" +
			"TUI interval, compact mode,\n" +
			"masked secrets by default, split inspection panes, in-TUI\n" +
			"config editing with diff preview, save/reset/defaults/scaffold\n" +
			"flows, automatic managed-stack apply on save when it is safe,\n" +
			"and in-TUI actions for stack lifecycle tasks. Use\n" +
			"tab/shift+tab or h/l to\n" +
			"change sections, use j/k or [ and ] to switch the active\n" +
			"service inside split inspection panes, and press w from the\n" +
			"service and health panels to open live logs for the selected\n" +
			"compose service in the full terminal viewer.",
		Example: "  stackctl tui",
		RunE: func(cmd *cobra.Command, args []string) error {
			model := stacktui.NewFullModel(func() (stacktui.Snapshot, error) {
				return loadTUISnapshot()
			}, buildTUILogWatchCommand, runTUIAction, &stacktui.ConfigManager{
				DefaultConfig:             deps.defaultConfig,
				SaveConfig:                deps.saveConfig,
				ValidateConfig:            validateTUIConfig,
				MarshalConfig:             deps.marshalConfig,
				ManagedStackNeedsScaffold: deps.managedStackNeedsScaffold,
				ScaffoldManagedStack:      deps.scaffoldManagedStack,
			})

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

func loadTUIEditableConfig() (string, configpkg.Config, stacktui.ConfigSourceState, string, error) {
	configPath, err := deps.configFilePath()
	if err != nil {
		return "", configpkg.Config{}, stacktui.ConfigSourceUnavailable, "", err
	}

	cfg := deps.defaultConfig()
	source := stacktui.ConfigSourceMissing
	problem := "No stackctl config was found. Review the defaults in Config and save to create it."

	loaded, err := deps.loadConfig(configPath)
	switch {
	case err == nil:
		cfg = loaded
		source = stacktui.ConfigSourceLoaded
		problem = ""
	case errors.Is(err, configpkg.ErrNotFound):
		cfg = deps.defaultConfig()
	case err != nil:
		source = stacktui.ConfigSourceUnavailable
		problem = fmt.Sprintf("Current config could not be loaded: %v", err)
	}

	cfg.ApplyDerivedFields()

	return configPath, cfg, source, problem, nil
}

func loadTUISnapshot() (stacktui.Snapshot, error) {
	configPath, cfg, source, problem, err := loadTUIEditableConfig()
	if err != nil {
		return stacktui.Snapshot{}, err
	}

	return buildTUISnapshot(configPath, cfg, source, problem), nil
}

func buildTUISnapshot(configPath string, cfg configpkg.Config, source stacktui.ConfigSourceState, problem string) stacktui.Snapshot {
	ctx := context.Background()
	issues := validateTUIConfig(cfg)
	needsScaffold := false
	scaffoldProblem := ""
	if cfg.Stack.Managed && cfg.Setup.ScaffoldDefaultStack {
		var err error
		needsScaffold, err = deps.managedStackNeedsScaffold(cfg)
		if err != nil {
			scaffoldProblem = err.Error()
		}
	}

	snapshot := stacktui.Snapshot{
		ConfigPath:            configPath,
		ConfigData:            cfg,
		ConfigSource:          source,
		ConfigProblem:         problem,
		ConfigIssues:          append([]configpkg.ValidationIssue(nil), issues...),
		ConfigNeedsScaffold:   needsScaffold,
		ConfigScaffoldProblem: scaffoldProblem,
		StackName:             cfg.Stack.Name,
		StackDir:              cfg.Stack.Dir,
		ComposePath:           deps.composePath(cfg),
		Managed:               cfg.Stack.Managed,
		WaitForServices:       cfg.Behavior.WaitForServicesStart,
		StartupTimeoutSec:     cfg.Behavior.StartupTimeoutSec,
		LoadedAt:              time.Now(),
		DoctorChecks:          []stacktui.DoctorCheck{},
		Connections:           make([]stacktui.Connection, 0, len(connectionEntries(cfg))),
	}

	runtimeReady := source == stacktui.ConfigSourceLoaded && len(issues) == 0 && !needsScaffold && strings.TrimSpace(scaffoldProblem) == ""
	if runtimeReady {
		services, err := runtimeServices(ctx, cfg)
		if err != nil {
			snapshot.ServiceError = err.Error()
		}
		health, err := healthChecks(ctx, cfg)
		if err != nil {
			snapshot.HealthError = err.Error()
		}
		snapshot.Services = make([]stacktui.Service, 0, len(services))
		snapshot.Health = make([]stacktui.HealthLine, 0, len(health))

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

		for _, line := range health {
			snapshot.Health = append(snapshot.Health, stacktui.HealthLine{
				Status:  line.Status,
				Message: line.Message,
			})
		}
	} else if len(issues) > 0 {
		message := fmt.Sprintf("Config has %d validation issue(s). Review the Config section.", len(issues))
		snapshot.ServiceError = message
		snapshot.HealthError = message
		snapshot.DoctorError = message
	} else if source == stacktui.ConfigSourceLoaded && needsScaffold {
		message := "Managed stack scaffold is pending. Use g in Config to create the compose stack."
		snapshot.ServiceError = message
		snapshot.HealthError = message
		snapshot.DoctorError = message
	}
	if runtimeReady {
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
	}

	for _, entry := range connectionEntries(cfg) {
		snapshot.Connections = append(snapshot.Connections, stacktui.Connection{
			Name:  entry.Name,
			Value: entry.Value,
		})
	}

	return snapshot
}

func validateTUIConfig(cfg configpkg.Config) []configpkg.ValidationIssue {
	return filterTUIValidationIssues(cfg, deps.validateConfig(cfg))
}

func filterTUIValidationIssues(cfg configpkg.Config, issues []configpkg.ValidationIssue) []configpkg.ValidationIssue {
	if len(issues) == 0 {
		return nil
	}

	filtered := make([]configpkg.ValidationIssue, 0, len(issues))
	for _, issue := range issues {
		if pendingManagedScaffoldIssue(cfg, issue) {
			continue
		}
		filtered = append(filtered, issue)
	}

	return filtered
}

func pendingManagedScaffoldIssue(cfg configpkg.Config, issue configpkg.ValidationIssue) bool {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return false
	}

	normalized := cfg
	normalized.ApplyDerivedFields()

	expectedDir, err := configpkg.ManagedStackDir(normalized.Stack.Name)
	if err != nil {
		return false
	}
	if normalized.Stack.Dir != expectedDir || normalized.Stack.ComposeFile != configpkg.DefaultComposeFileName {
		return false
	}

	switch issue.Field {
	case "stack.dir":
		return issue.Message == fmt.Sprintf("directory does not exist: %s", normalized.Stack.Dir)
	case "stack.compose_file":
		return issue.Message == fmt.Sprintf("file does not exist: %s", configpkg.ComposePath(normalized))
	default:
		return false
	}
}

type tuiLogWatchCommand struct {
	cfg     configpkg.Config
	service string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

func buildTUILogWatchCommand(request stacktui.LogWatchRequest) (tea.ExecCommand, error) {
	_, cfg, err := loadTUIConfig()
	if err != nil {
		return nil, err
	}
	if err := ensureComposeRuntimeForConfig(cfg); err != nil {
		return nil, err
	}

	service := strings.TrimSpace(request.Service)
	if service == "" {
		return nil, errors.New("live logs require a selected service")
	}
	service, err = canonicalServiceName(service)
	if err != nil {
		return nil, err
	}

	return &tuiLogWatchCommand{
		cfg:     cfg,
		service: service,
	}, nil
}

func (c *tuiLogWatchCommand) SetStdin(reader io.Reader) {
	c.stdin = reader
}

func (c *tuiLogWatchCommand) SetStdout(writer io.Writer) {
	c.stdout = writer
}

func (c *tuiLogWatchCommand) SetStderr(writer io.Writer) {
	c.stderr = writer
}

func (c *tuiLogWatchCommand) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := system.Runner{
		Stdin:  c.stdin,
		Stdout: c.stdout,
		Stderr: c.stderr,
	}
	err := deps.composeLogs(ctx, runner, c.cfg, tuiLogWatchTail, true, "", c.service)
	if ctx.Err() != nil {
		return nil
	}
	return err
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
