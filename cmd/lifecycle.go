package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
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
	for _, definition := range waitableServiceDefinitions(cfg) {
		if len(selected) > 0 && !slices.Contains(selected, definition.Key) {
			continue
		}
		if definition.PrimaryPort == nil {
			continue
		}
		port := definition.PrimaryPort(cfg)
		if err := deps.waitForPort(ctx, port, 500*time.Millisecond); err != nil {
			return fmt.Errorf("%s port %d did not become ready: %w", definition.Key, port, err)
		}
	}

	return nil
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
