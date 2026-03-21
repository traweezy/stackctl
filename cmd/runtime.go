package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

type podmanContainer struct {
	ID        string       `json:"Id"`
	Image     string       `json:"Image"`
	Names     []string     `json:"Names"`
	Status    string       `json:"Status"`
	State     string       `json:"State"`
	Ports     []podmanPort `json:"Ports"`
	CreatedAt string       `json:"CreatedAt"`
}

type podmanPort struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func loadRuntimeConfig(cmd *cobra.Command, allowFirstRun bool) (configpkg.Config, error) {
	path, err := configpkg.ConfigFilePath()
	if err != nil {
		return configpkg.Config{}, err
	}

	cfg, err := configpkg.Load(path)
	if err != nil {
		if !errors.Is(err, configpkg.ErrNotFound) {
			return configpkg.Config{}, err
		}
		if !allowFirstRun {
			return configpkg.Config{}, missingConfigHint(err)
		}
		if !terminalInteractive() {
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

		cfg, err = configpkg.RunWizard(os.Stdin, cmd.OutOrStdout(), configpkg.Default())
		if err != nil {
			return configpkg.Config{}, err
		}
		if err := configpkg.Save(path, cfg); err != nil {
			return configpkg.Config{}, err
		}
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("saved config to %s", path)); err != nil {
			return configpkg.Config{}, err
		}
	}

	issues := configpkg.Validate(cfg)
	if len(issues) > 0 {
		if err := printValidationIssues(cmd, issues); err != nil {
			return configpkg.Config{}, err
		}
		return configpkg.Config{}, fmt.Errorf("config validation failed with %d issue(s)", len(issues))
	}

	return cfg, nil
}

func ensureComposeRuntime(cmd *cobra.Command, cfg configpkg.Config) error {
	if !system.CommandExists("podman") {
		return errors.New("podman is not installed; run `stackctl setup --install` or install it manually")
	}
	if !system.PodmanComposeAvailable(context.Background()) {
		return errors.New("podman compose is not available; run `stackctl setup --install` or install podman-compose manually")
	}
	if _, err := os.Stat(configpkg.ComposePath(cfg)); err != nil {
		return fmt.Errorf("compose file %s is not available: %w", configpkg.ComposePath(cfg), err)
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

func loadStackContainers(ctx context.Context, cfg configpkg.Config) ([]podmanContainer, error) {
	result, err := system.CaptureResult(ctx, "", "podman", "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("podman ps failed: %s", strings.TrimSpace(result.Stderr))
	}

	containers := make([]podmanContainer, 0)
	if err := json.Unmarshal([]byte(result.Stdout), &containers); err != nil {
		return nil, fmt.Errorf("parse podman status output: %w", err)
	}

	nameSet := make(map[string]struct{}, len(stackContainerNames(cfg)))
	for _, name := range stackContainerNames(cfg) {
		nameSet[name] = struct{}{}
	}

	filtered := make([]podmanContainer, 0, len(nameSet))
	for _, container := range containers {
		for _, name := range container.Names {
			if _, ok := nameSet[name]; ok {
				filtered = append(filtered, container)
				break
			}
		}
	}

	return filtered, nil
}

func formatPorts(ports []podmanPort) string {
	if len(ports) == 0 {
		return "-"
	}

	values := make([]string, 0, len(ports))
	for _, port := range ports {
		values = append(values, fmt.Sprintf("%d->%d/%s", port.HostPort, port.ContainerPort, port.Protocol))
	}

	return strings.Join(values, ", ")
}

func printStatusTable(cmd *cobra.Command, containers []podmanContainer, verbose bool) error {
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

	if len(containers) == 0 {
		lines = append(lines, outputLine{Status: output.StatusWarn, Message: "no containers from this stack were found"})
		return lines, nil
	}

	allRunning := true
	for _, container := range containers {
		if container.State != "running" {
			allRunning = false
			break
		}
	}
	if allRunning {
		lines = append(lines, outputLine{Status: output.StatusOK, Message: "containers are running"})
	} else {
		lines = append(lines, outputLine{Status: output.StatusWarn, Message: "some stack containers are not running"})
	}

	return lines, nil
}

type outputLine struct {
	Status  string
	Message string
}

func checkPort(port int, label string) outputLine {
	if system.PortListening(port) {
		return outputLine{Status: output.StatusOK, Message: label}
	}

	return outputLine{Status: output.StatusWarn, Message: label}
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
		if err := system.WaitForPort(ctx, port, 500*time.Millisecond); err != nil {
			return err
		}
	}

	return nil
}
