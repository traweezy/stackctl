package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type CaptureFunc func(context.Context, string, string, ...string) (CommandResult, error)

type Container struct {
	ID        string          `json:"Id"`
	Image     string          `json:"Image"`
	Names     []string        `json:"Names"`
	Status    string          `json:"Status"`
	State     string          `json:"State"`
	Ports     []ContainerPort `json:"Ports"`
	CreatedAt string          `json:"CreatedAt"`
}

type ContainerPort struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func ListContainers(ctx context.Context, capture CaptureFunc) ([]Container, error) {
	result, err := capture(ctx, "", "podman", "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return nil, fmt.Errorf("podman ps failed: %s", detail)
	}

	stdout := strings.TrimSpace(result.Stdout)
	if stdout == "" {
		return []Container{}, nil
	}

	containers := make([]Container, 0)
	if err := json.Unmarshal([]byte(stdout), &containers); err != nil {
		return nil, fmt.Errorf("parse podman status output: %w", err)
	}

	return containers, nil
}

func FilterContainersByName(containers []Container, names []string) []Container {
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		nameSet[trimmed] = struct{}{}
	}

	filtered := make([]Container, 0, len(nameSet))
	for _, container := range containers {
		for _, name := range container.Names {
			if _, ok := nameSet[name]; ok {
				filtered = append(filtered, container)
				break
			}
		}
	}

	return filtered
}
