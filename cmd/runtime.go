package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func loadRuntimeConfig(cmd *cobra.Command, allowFirstRun bool) (configpkg.Config, error) {
	path, err := deps.configFilePath()
	if err != nil {
		return configpkg.Config{}, err
	}

	cfg, err := deps.loadConfig(path)
	if err != nil {
		if !errors.Is(err, configpkg.ErrNotFound) {
			return configpkg.Config{}, err
		}
		if !allowFirstRun {
			return configpkg.Config{}, missingConfigHint(err)
		}
		if !deps.isTerminal() {
			return configpkg.Config{}, missingConfigHint(err)
		}

		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "No stackctl config was found."); err != nil {
			return configpkg.Config{}, err
		}
		ok, err := confirmWithPrompt(cmd, "Run interactive setup now?", true)
		if err != nil {
			return configpkg.Config{}, err
		}
		if !ok {
			return configpkg.Config{}, errors.New("run `stackctl setup` or `stackctl config init`")
		}

		cfg, err = deps.runWizard(deps.stdin, cmd.OutOrStdout(), deps.defaultConfig())
		if err != nil {
			return configpkg.Config{}, err
		}
		if err := scaffoldManagedStack(cmd, cfg, false); err != nil {
			return configpkg.Config{}, err
		}
		if err := deps.saveConfig(path, cfg); err != nil {
			return configpkg.Config{}, err
		}
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("saved config to %s", path)); err != nil {
			return configpkg.Config{}, err
		}
	}

	issues := deps.validateConfig(cfg)
	if len(issues) > 0 {
		if err := printValidationIssues(cmd, issues); err != nil {
			return configpkg.Config{}, err
		}
		return configpkg.Config{}, fmt.Errorf("config validation failed with %d issue(s)", len(issues))
	}

	return cfg, nil
}

func ensureComposeRuntime(cmd *cobra.Command, cfg configpkg.Config) error {
	if !deps.commandExists("podman") {
		return errors.New("podman is not installed; run `stackctl setup --install` or install it manually")
	}
	if !deps.podmanComposeAvail(context.Background()) {
		return errors.New("podman compose is not available; run `stackctl setup --install` or install podman-compose manually")
	}
	if _, err := deps.stat(deps.composePath(cfg)); err != nil {
		return fmt.Errorf("compose file %s is not available: %w", deps.composePath(cfg), err)
	}

	return nil
}

func serviceContainer(cfg configpkg.Config, service string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "postgres", "pg":
		return cfg.Services.PostgresContainer, nil
	case "redis", "rd":
		return cfg.Services.RedisContainer, nil
	case "pgadmin":
		return cfg.Services.PgAdminContainer, nil
	default:
		return "", fmt.Errorf("invalid service %q; valid values: postgres, redis, pgadmin", service)
	}
}

func stackContainerNames(cfg configpkg.Config) []string {
	names := []string{
		cfg.Services.PostgresContainer,
		cfg.Services.RedisContainer,
	}
	if cfg.Setup.IncludePgAdmin {
		names = append(names, cfg.Services.PgAdminContainer)
	}

	return names
}

func loadStackContainers(ctx context.Context, cfg configpkg.Config) ([]system.Container, error) {
	containers, err := system.ListContainers(ctx, deps.captureResult)
	if err != nil {
		return nil, err
	}

	return system.FilterContainersByName(containers, stackContainerNames(cfg)), nil
}

func formatPorts(ports []system.ContainerPort) string {
	if len(ports) == 0 {
		return "-"
	}

	values := make([]string, 0, len(ports))
	for _, port := range ports {
		values = append(values, fmt.Sprintf("%d->%d/%s", port.HostPort, port.ContainerPort, port.Protocol))
	}

	return strings.Join(values, ", ")
}

func printStatusTable(cmd *cobra.Command, containers []system.Container, verbose bool) error {
	if len(containers) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No containers from this stack were found.")
		return err
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
	if verbose {
		if _, err := fmt.Fprintln(writer, "NAME\tIMAGE\tSTATUS\tPORTS\tID\tCREATED"); err != nil {
			return err
		}
		for _, container := range containers {
			if _, err := fmt.Fprintf(
				writer,
				"%s\t%s\t%s\t%s\t%s\t%s\n",
				strings.Join(container.Names, ","),
				container.Image,
				container.Status,
				formatPorts(container.Ports),
				shortID(container.ID),
				container.CreatedAt,
			); err != nil {
				return err
			}
		}
	} else {
		if _, err := fmt.Fprintln(writer, "NAME\tIMAGE\tSTATUS\tPORTS"); err != nil {
			return err
		}
		for _, container := range containers {
			if _, err := fmt.Fprintf(
				writer,
				"%s\t%s\t%s\t%s\n",
				strings.Join(container.Names, ","),
				container.Image,
				container.Status,
				formatPorts(container.Ports),
			); err != nil {
				return err
			}
		}
	}

	return writer.Flush()
}

func printConnectionInfo(cmd *cobra.Command, cfg configpkg.Config) error {
	services, err := runtimeServices(context.Background(), cfg)
	if err != nil {
		return err
	}

	entries := make([]connectionEntry, 0, len(services))
	for _, service := range services {
		switch {
		case service.DSN != "":
			entries = append(entries, connectionEntry{Name: service.DisplayName, Value: service.DSN})
		case service.URL != "":
			entries = append(entries, connectionEntry{Name: service.DisplayName, Value: service.URL})
		}
	}

	return printConnectionEntries(cmd, entries)
}

func healthChecks(ctx context.Context, cfg configpkg.Config) ([]outputLine, error) {
	lines := []outputLine{
		checkPort(cfg.Ports.Postgres, "postgres port listening"),
		checkPort(cfg.Ports.Redis, "redis port listening"),
	}
	if cfg.Setup.IncludePgAdmin {
		lines = append(lines, checkPort(cfg.Ports.PgAdmin, "pgadmin port listening"))
	}
	lines = append(lines, checkPort(cfg.Ports.Cockpit, "cockpit port listening"))

	containers, err := loadStackContainers(ctx, cfg)
	if err != nil {
		lines = append(lines, outputLine{Status: output.StatusFail, Message: fmt.Sprintf("container status check failed: %v", err)})
		return lines, nil
	}

	containerByName := make(map[string]system.Container, len(containers))
	for _, container := range containers {
		for _, name := range container.Names {
			containerByName[name] = container
		}
	}

	for _, service := range configuredStackServices(cfg) {
		container, ok := containerByName[service.ContainerName]
		if !ok {
			lines = append(lines, outputLine{Status: output.StatusWarn, Message: fmt.Sprintf("%s container not found", service.Name)})
			continue
		}

		if container.State == "running" {
			lines = append(lines, outputLine{Status: output.StatusOK, Message: fmt.Sprintf("%s running", service.Name)})
			continue
		}

		lines = append(lines, outputLine{
			Status:  output.StatusWarn,
			Message: fmt.Sprintf("%s not running (%s)", service.Name, container.Status),
		})
	}

	return lines, nil
}

type outputLine struct {
	Status  string
	Message string
}

func checkPort(port int, label string) outputLine {
	if deps.portListening(port) {
		return outputLine{Status: output.StatusOK, Message: label}
	}

	return outputLine{Status: output.StatusWarn, Message: label}
}

type configuredService struct {
	Name          string
	ContainerName string
	Port          int
}

type runtimeService struct {
	Icon          string
	DisplayName   string
	Status        string
	ContainerName string
	Host          string
	ExternalPort  int
	InternalPort  int
	Database      string
	Email         string
	Username      string
	Password      string
	URL           string
	DSN           string
}

type connectionEntry struct {
	Name  string
	Value string
}

func configuredStackServices(cfg configpkg.Config) []configuredService {
	services := []configuredService{
		{Name: "postgres", ContainerName: cfg.Services.PostgresContainer, Port: cfg.Ports.Postgres},
		{Name: "redis", ContainerName: cfg.Services.RedisContainer, Port: cfg.Ports.Redis},
	}
	if cfg.Setup.IncludePgAdmin {
		services = append(services, configuredService{
			Name:          "pgadmin",
			ContainerName: cfg.Services.PgAdminContainer,
			Port:          cfg.Ports.PgAdmin,
		})
	}

	return services
}

func runtimeServices(ctx context.Context, cfg configpkg.Config) ([]runtimeService, error) {
	containers, err := loadStackContainers(ctx, cfg)
	if err != nil {
		return nil, err
	}

	containerByName := make(map[string]system.Container, len(containers))
	for _, container := range containers {
		for _, name := range container.Names {
			containerByName[name] = container
		}
	}

	services := []runtimeService{
		{
			Icon:          "🗄️",
			DisplayName:   "Postgres",
			Status:        containerStatus(containerByName, cfg.Services.PostgresContainer),
			ContainerName: cfg.Services.PostgresContainer,
			Host:          cfg.Connection.Host,
			ExternalPort:  cfg.Ports.Postgres,
			InternalPort:  containerInternalPort(containerByName, cfg.Services.PostgresContainer, cfg.Ports.Postgres),
			Database:      cfg.Connection.PostgresDatabase,
			Username:      cfg.Connection.PostgresUsername,
			Password:      cfg.Connection.PostgresPassword,
			DSN:           postgresDSN(cfg),
		},
		{
			Icon:          "⚡",
			DisplayName:   "Redis",
			Status:        containerStatus(containerByName, cfg.Services.RedisContainer),
			ContainerName: cfg.Services.RedisContainer,
			Host:          cfg.Connection.Host,
			ExternalPort:  cfg.Ports.Redis,
			InternalPort:  containerInternalPort(containerByName, cfg.Services.RedisContainer, cfg.Ports.Redis),
			Password:      cfg.Connection.RedisPassword,
			DSN:           redisDSN(cfg),
		},
	}

	if cfg.Setup.IncludePgAdmin {
		services = append(services, runtimeService{
			Icon:          "🌐",
			DisplayName:   "pgAdmin",
			Status:        containerStatus(containerByName, cfg.Services.PgAdminContainer),
			ContainerName: cfg.Services.PgAdminContainer,
			Host:          cfg.Connection.Host,
			ExternalPort:  cfg.Ports.PgAdmin,
			InternalPort:  containerInternalPort(containerByName, cfg.Services.PgAdminContainer, cfg.Ports.PgAdmin),
			Email:         cfg.Connection.PgAdminEmail,
			Password:      cfg.Connection.PgAdminPassword,
			URL:           cfg.URLs.PgAdmin,
		})
	}

	cockpit := deps.cockpitStatus(ctx)
	services = append(services, runtimeService{
		Icon:         "🖥️",
		DisplayName:  "Cockpit",
		Status:       cockpitStateLabel(cockpit),
		Host:         cfg.Connection.Host,
		ExternalPort: cfg.Ports.Cockpit,
		URL:          cfg.URLs.Cockpit,
	})

	return services, nil
}

func printServicesInfo(cmd *cobra.Command, cfg configpkg.Config) error {
	services, err := runtimeServices(context.Background(), cfg)
	if err != nil {
		return err
	}

	for idx, service := range services {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", service.Icon, service.DisplayName); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", service.Status); err != nil {
			return err
		}
		if service.ContainerName != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Container: %s\n", service.ContainerName); err != nil {
				return err
			}
		}
		if service.Host != "" && service.ExternalPort > 0 {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Host: %s\n", service.Host); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Port: %s\n", formatServicePort(service.ExternalPort, service.InternalPort)); err != nil {
				return err
			}
		}
		if service.Database != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Database: %s\n", service.Database); err != nil {
				return err
			}
		}
		if service.Email != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Email: %s\n", service.Email); err != nil {
				return err
			}
		}
		if service.Username != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Username: %s\n", service.Username); err != nil {
				return err
			}
		}
		if service.Password != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Password: %s\n", service.Password); err != nil {
				return err
			}
		}
		if service.URL != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  URL: %s\n", service.URL); err != nil {
				return err
			}
		}
		if service.DSN != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  DSN: %s\n", service.DSN); err != nil {
				return err
			}
		}

		if idx < len(services)-1 {
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}
		}
	}

	return nil
}

func printConnectionEntries(cmd *cobra.Command, entries []connectionEntry) error {
	for idx, entry := range entries {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n", entry.Name, entry.Value); err != nil {
			return err
		}
		if idx < len(entries)-1 {
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}
		}
	}

	return nil
}

func postgresDSN(cfg configpkg.Config) string {
	target := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Connection.PostgresUsername, cfg.Connection.PostgresPassword),
		Host:   fmt.Sprintf("%s:%d", cfg.Connection.Host, cfg.Ports.Postgres),
		Path:   cfg.Connection.PostgresDatabase,
	}

	return target.String()
}

func redisDSN(cfg configpkg.Config) string {
	target := &url.URL{
		Scheme: "redis",
		Host:   fmt.Sprintf("%s:%d", cfg.Connection.Host, cfg.Ports.Redis),
	}
	if cfg.Connection.RedisPassword != "" {
		target.User = url.UserPassword("", cfg.Connection.RedisPassword)
	}

	return target.String()
}

func containerStatus(containerByName map[string]system.Container, containerName string) string {
	container, ok := containerByName[containerName]
	if !ok {
		return "missing"
	}

	state := strings.TrimSpace(strings.ToLower(container.State))
	switch state {
	case "running":
		return "running"
	case "", "created", "configured", "exited", "stopped":
		return "stopped"
	default:
		return state
	}
}

func cockpitStateLabel(state system.CockpitState) string {
	switch {
	case state.Active:
		return "running"
	case !state.Installed:
		return "missing"
	case strings.TrimSpace(state.State) == "":
		return "stopped"
	default:
		return strings.TrimSpace(state.State)
	}
}

func containerInternalPort(containerByName map[string]system.Container, containerName string, hostPort int) int {
	container, ok := containerByName[containerName]
	if !ok {
		return 0
	}

	for _, port := range container.Ports {
		if port.HostPort == hostPort {
			return port.ContainerPort
		}
	}
	if len(container.Ports) == 0 {
		return 0
	}

	return container.Ports[0].ContainerPort
}

func formatServicePort(externalPort, internalPort int) string {
	switch {
	case externalPort > 0 && internalPort > 0:
		return fmt.Sprintf("%d -> %d", externalPort, internalPort)
	case externalPort > 0:
		return fmt.Sprintf("%d -> unknown", externalPort)
	case internalPort > 0:
		return fmt.Sprintf("unknown -> %d", internalPort)
	default:
		return "unknown"
	}
}

func shortID(value string) string {
	if len(value) <= 12 {
		return value
	}

	return value[:12]
}

func waitForConfiguredServices(ctx context.Context, cfg configpkg.Config) error {
	ports := []int{cfg.Ports.Postgres, cfg.Ports.Redis}
	if cfg.Setup.IncludePgAdmin {
		ports = append(ports, cfg.Ports.PgAdmin)
	}

	for _, port := range ports {
		if err := deps.waitForPort(ctx, port, 500*time.Millisecond); err != nil {
			return err
		}
	}

	return nil
}
