package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/compose"
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
	return ensureComposeRuntimeForConfig(cfg)
}

func ensureComposeRuntimeForConfig(cfg configpkg.Config) error {
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
	definition, ok := serviceDefinitionByAlias(service)
	if !ok || definition.Kind != serviceKindStack {
		return "", fmt.Errorf("invalid service %q; valid values: %s", service, validStackServiceNames())
	}
	if definition.ContainerName == nil {
		return "", fmt.Errorf("service %q does not define a container name", definition.Key)
	}
	return definition.ContainerName(cfg), nil
}

func canonicalServiceName(service string) (string, error) {
	definition, ok := serviceDefinitionByAlias(service)
	if !ok || definition.Kind != serviceKindStack {
		return "", fmt.Errorf("invalid service %q; valid values: %s", service, validStackServiceNames())
	}
	return definition.Key, nil
}

func stackContainerNames(cfg configpkg.Config) []string {
	names := make([]string, 0, len(serviceDefinitions()))
	for _, definition := range enabledStackServiceDefinitions(cfg) {
		if definition.ContainerName == nil {
			continue
		}
		names = append(names, definition.ContainerName(cfg))
	}
	return names
}

func loadStackContainers(ctx context.Context, cfg configpkg.Config) ([]system.Container, error) {
	composePath := deps.composePath(cfg)
	if deps.podmanComposeAvail(ctx) && compose.SupportsPSJSON() {
		if _, err := deps.stat(composePath); err == nil {
			return compose.ListContainers(ctx, cfg.Stack.Dir, composePath, deps.captureResult)
		}
	}

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
	return printConnectionEntries(cmd, connectionEntries(cfg))
}

func healthChecks(ctx context.Context, cfg configpkg.Config) ([]outputLine, error) {
	lines := make([]outputLine, 0, len(serviceDefinitions())*2)
	for _, definition := range enabledServiceDefinitions(cfg) {
		if definition.PrimaryPort != nil && definition.PrimaryPortLabel != "" {
			lines = append(lines, checkPort(definition.PrimaryPort(cfg), definition.PrimaryPortLabel))
		}
	}

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

	return outputLine{Status: output.StatusWarn, Message: missingPortLabel(label)}
}

func missingPortLabel(label string) string {
	if strings.HasSuffix(label, " listening") {
		return strings.TrimSuffix(label, " listening") + " not listening"
	}

	return label
}

type configuredService struct {
	Name          string
	ContainerName string
	Port          int
}

type runtimeService struct {
	Name            string `json:"name"`
	Icon            string `json:"-"`
	DisplayName     string `json:"display_name"`
	Status          string `json:"status"`
	ContainerName   string `json:"container_name,omitempty"`
	Image           string `json:"image,omitempty"`
	DataVolume      string `json:"data_volume,omitempty"`
	Host            string `json:"host,omitempty"`
	ExternalPort    int    `json:"external_port,omitempty"`
	InternalPort    int    `json:"internal_port,omitempty"`
	Database        string `json:"database,omitempty"`
	MaintenanceDB   string `json:"maintenance_database,omitempty"`
	Email           string `json:"email,omitempty"`
	Token           string `json:"-"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"-"`
	AppendOnly      *bool  `json:"appendonly,omitempty"`
	SavePolicy      string `json:"save_policy,omitempty"`
	MaxMemoryPolicy string `json:"maxmemory_policy,omitempty"`
	ServerMode      string `json:"server_mode,omitempty"`
	URL             string `json:"url,omitempty"`
	DSN             string `json:"dsn,omitempty"`
}

type connectionEntry struct {
	Name  string
	Value string
}

func configuredStackServices(cfg configpkg.Config) []configuredService {
	services := make([]configuredService, 0, len(serviceDefinitions()))
	for _, definition := range enabledStackServiceDefinitions(cfg) {
		if definition.ContainerName == nil || definition.PrimaryPort == nil {
			continue
		}
		services = append(services, configuredService{
			Name:          definition.Key,
			ContainerName: definition.ContainerName(cfg),
			Port:          definition.PrimaryPort(cfg),
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

	services := make([]runtimeService, 0, len(serviceDefinitions()))
	for _, definition := range runtimeServiceDefinitions(cfg) {
		services = append(services, runtimeServiceForDefinition(ctx, cfg, definition, containerByName))
	}

	return services, nil
}

func printServicesJSON(cmd *cobra.Command, cfg configpkg.Config) error {
	services, err := runtimeServices(context.Background(), cfg)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return err
	}
	if _, err := cmd.OutOrStdout().Write(data); err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write([]byte("\n"))
	return err
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
		if service.Image != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", service.Image); err != nil {
				return err
			}
		}
		if service.DataVolume != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Data volume: %s\n", service.DataVolume); err != nil {
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
		if service.MaintenanceDB != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Maintenance DB: %s\n", service.MaintenanceDB); err != nil {
				return err
			}
		}
		if service.Email != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Email: %s\n", service.Email); err != nil {
				return err
			}
		}
		if service.Token != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Token: %s\n", service.Token); err != nil {
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
		if service.AppendOnly != nil {
			appendOnlyValue := "disabled"
			if *service.AppendOnly {
				appendOnlyValue = "enabled"
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Appendonly: %s\n", appendOnlyValue); err != nil {
				return err
			}
		}
		if service.SavePolicy != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Save policy: %s\n", service.SavePolicy); err != nil {
				return err
			}
		}
		if service.MaxMemoryPolicy != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Maxmemory policy: %s\n", service.MaxMemoryPolicy); err != nil {
				return err
			}
		}
		if service.ServerMode != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Server mode: %s\n", service.ServerMode); err != nil {
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

func connectionEntries(cfg configpkg.Config) []connectionEntry {
	entries := make([]connectionEntry, 0, len(serviceDefinitions()))
	for _, definition := range enabledServiceDefinitions(cfg) {
		if definition.ConnectionEntries == nil {
			continue
		}
		entries = append(entries, definition.ConnectionEntries(cfg)...)
	}
	return entries
}

func selectedConnectionEntries(cfg configpkg.Config, services []string) []connectionEntry {
	if len(services) == 0 {
		return connectionEntries(cfg)
	}

	entries := make([]connectionEntry, 0, len(services))
	for _, service := range services {
		definition, ok := serviceDefinitionByKey(service)
		if !ok || definition.ConnectionEntries == nil {
			continue
		}
		entries = append(entries, definition.ConnectionEntries(cfg)...)
	}

	return entries
}

func serviceCopyTarget(cfg configpkg.Config, target string) (string, string, error) {
	spec, ok := copyTargetSpec(cfg, target)
	if !ok {
		return "", "", fmt.Errorf("invalid copy target %q; valid values: %s", target, validCopyTargetNames())
	}
	value, err := spec.Resolve(cfg)
	if err != nil {
		return "", "", err
	}
	return spec.Label, value, nil
}

func normalizedCopyTarget(target string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(target)))
}

func boolPointer(value bool) *bool {
	return &value
}

func pgAdminModeLabel(serverMode bool) string {
	if serverMode {
		return "enabled"
	}

	return "disabled"
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

func natsDSN(cfg configpkg.Config) string {
	target := &url.URL{
		Scheme: "nats",
		Host:   fmt.Sprintf("%s:%d", cfg.Connection.Host, cfg.Ports.NATS),
	}
	if cfg.Connection.NATSToken != "" {
		target.User = url.User(cfg.Connection.NATSToken)
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
