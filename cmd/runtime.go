package cmd

import (
	"context"
	"errors"
	"fmt"
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
	lines := []string{
		fmt.Sprintf("Postgres\n  DSN: postgres://app:app@localhost:%d/app\n", cfg.Ports.Postgres),
		fmt.Sprintf("Redis\n  DSN: redis://localhost:%d\n", cfg.Ports.Redis),
		fmt.Sprintf("Cockpit\n  URL: %s\n", cfg.URLs.Cockpit),
	}
	if cfg.Setup.IncludePgAdmin {
		lines = append(lines, fmt.Sprintf("pgAdmin\n  URL: %s\n", cfg.URLs.PgAdmin))
	}

	_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
	return err
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
