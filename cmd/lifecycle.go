package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

type activeLocalStack struct {
	Name     string
	Path     string
	Services []string
}

func resolveTargetStackServices(cfg configpkg.Config, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}

	services := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		service, err := canonicalServiceName(arg)
		if err != nil {
			return nil, err
		}
		if err := ensureServiceEnabled(cfg, service); err != nil {
			return nil, err
		}
		if _, ok := seen[service]; ok {
			continue
		}
		seen[service] = struct{}{}
		services = append(services, service)
	}

	return services, nil
}

func displayServiceNames(serviceNames []string) []string {
	labels := make([]string, 0, len(serviceNames))
	for _, name := range serviceNames {
		definition, ok := serviceDefinitionByKey(name)
		if !ok {
			labels = append(labels, name)
			continue
		}
		labels = append(labels, definition.DisplayName)
	}
	return labels
}

func lifecycleTargetLabel(serviceNames []string) string {
	if len(serviceNames) == 0 {
		return "stack"
	}

	labels := displayServiceNames(serviceNames)
	switch len(labels) {
	case 1:
		return labels[0]
	case 2:
		return labels[0] + " and " + labels[1]
	default:
		return stringsJoin(labels[:len(labels)-1], ", ") + ", and " + labels[len(labels)-1]
	}
}

func waitForConfiguredServices(ctx context.Context, cfg configpkg.Config) error {
	return waitForSelectedServices(ctx, cfg, nil)
}

func waitForSelectedServices(ctx context.Context, cfg configpkg.Config, selected []string) error {
	definitions := make([]serviceDefinition, 0, len(waitableServiceDefinitions(cfg)))
	for _, definition := range selectedStackServiceDefinitions(cfg, selected) {
		if !definition.WaitOnStart || definition.PrimaryPort == nil {
			continue
		}
		definitions = append(definitions, definition)
	}

	for _, definition := range definitions {
		if err := waitForStackService(ctx, cfg, definition); err != nil {
			return err
		}
	}

	return nil
}

func ensureSelectedServicePortsAvailable(ctx context.Context, cfg configpkg.Config, selected []string) error {
	definitions := selectedStackServiceDefinitions(cfg, selected)
	if len(definitions) == 0 {
		return nil
	}

	states, err := stackServiceRuntimeStates(ctx, cfg, definitions)
	if err != nil {
		return err
	}

	conflicts := make([]string, 0, len(states))
	for _, state := range states {
		if state.PortState.CheckErr != nil {
			return fmt.Errorf("port %d check failed: %w", state.Port, state.PortState.CheckErr)
		}
		if state.PortState.Conflict {
			conflicts = append(conflicts, portConflictMessage(state.Definition.Key, state.Port))
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("cannot start %s: %s", strings.ToLower(lifecycleTargetLabel(selected)), stringsJoin(conflicts, "; "))
	}

	return nil
}

func verifySelectedServicesStarted(ctx context.Context, cfg configpkg.Config, selected []string) error {
	definitions := selectedStackServiceDefinitions(cfg, selected)
	if len(definitions) == 0 {
		return nil
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		states, err := stackServiceRuntimeStates(ctx, cfg, definitions)
		if err != nil {
			return err
		}
		if failure := firstServiceStartFailure(states); failure != nil {
			return failure
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func waitForStackService(ctx context.Context, cfg configpkg.Config, definition serviceDefinition) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		states, err := stackServiceRuntimeStates(ctx, cfg, []serviceDefinition{definition})
		if err != nil {
			return err
		}
		state := states[0]
		if failure := serviceStartFailure(state); failure != nil {
			return failure
		}
		if state.ContainerRunning && state.PortBound {
			if err := deps.waitForPort(ctx, state.Port, 500*time.Millisecond); err != nil {
				return fmt.Errorf("%s port %d did not become ready: %w", definition.Key, state.Port, err)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%s did not become ready: %s: %w", definition.Key, servicePendingReason(state), ctx.Err())
		case <-ticker.C:
		}
	}
}

func firstServiceStartFailure(states []stackServiceRuntimeState) error {
	for _, state := range states {
		if failure := serviceStartFailure(state); failure != nil {
			return failure
		}
	}

	return nil
}

func serviceStartFailure(state stackServiceRuntimeState) error {
	switch {
	case state.PortState.CheckErr != nil:
		return fmt.Errorf("port %d check failed: %w", state.Port, state.PortState.CheckErr)
	case state.PortState.Conflict:
		return fmt.Errorf("%s could not bind host port %d because it is already in use by another process or container", state.Definition.Key, state.Port)
	case state.ContainerFound && terminalContainerState(state.ContainerState):
		return fmt.Errorf("%s container failed to start (%s)", state.Definition.Key, displayContainerStatus(state))
	default:
		return nil
	}
}

func terminalContainerState(state string) bool {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "exited", "stopped", "dead", "removing":
		return true
	default:
		return false
	}
}

func displayContainerStatus(state stackServiceRuntimeState) string {
	if strings.TrimSpace(state.ContainerStatus) != "" {
		return strings.TrimSpace(state.ContainerStatus)
	}
	if strings.TrimSpace(state.ContainerState) != "" {
		return state.ContainerState
	}

	return "unknown"
}

func servicePendingReason(state stackServiceRuntimeState) string {
	switch {
	case !state.ContainerFound:
		return "container not found"
	case !state.ContainerRunning:
		return fmt.Sprintf("container status %s", displayContainerStatus(state))
	case !state.PortBound:
		return fmt.Sprintf("container is running but host port %d is not mapped", state.Port)
	default:
		return fmt.Sprintf("port %d is not listening yet", state.Port)
	}
}

func currentConfigPath() (string, error) {
	return deps.configFilePath()
}

func ensureNoOtherRunningStack(ctx context.Context) error {
	currentPath, err := currentConfigPath()
	if err != nil {
		return err
	}

	active, err := otherRunningLocalStack(ctx, currentPath)
	if err != nil {
		return err
	}
	if active == nil {
		return nil
	}

	command := fmt.Sprintf("stackctl --stack %s stop", active.Name)
	serviceList := ""
	if len(active.Services) > 0 {
		serviceList = fmt.Sprintf(" (%s)", stringsJoin(active.Services, ", "))
	}

	return fmt.Errorf(
		"another local stack is already running: %s%s; stop it first with `%s`",
		active.Name,
		serviceList,
		command,
	)
}

func otherRunningLocalStack(ctx context.Context, currentConfigPath string) (*activeLocalStack, error) {
	paths, err := deps.knownConfigPaths()
	if err != nil {
		return nil, err
	}

	currentConfigPath = filepath.Clean(currentConfigPath)
	for _, path := range paths {
		if filepath.Clean(path) == currentConfigPath {
			continue
		}

		cfg, err := deps.loadConfig(path)
		if err != nil {
			continue
		}

		services, err := runningStackServices(ctx, cfg)
		if err != nil || len(services) == 0 {
			continue
		}

		return &activeLocalStack{
			Name:     cfg.Stack.Name,
			Path:     path,
			Services: services,
		}, nil
	}

	return nil, nil
}

func runningStackServices(ctx context.Context, cfg configpkg.Config) ([]string, error) {
	services, err := runtimeServices(ctx, cfg)
	if err != nil {
		return nil, err
	}

	running := make([]string, 0, len(services))
	for _, service := range services {
		if strings.TrimSpace(service.ContainerName) == "" {
			continue
		}
		if !strings.EqualFold(service.Status, "running") {
			continue
		}
		running = append(running, service.DisplayName)
	}

	return running, nil
}
