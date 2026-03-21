package compose

import (
	"context"
	"strconv"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

type Client struct {
	Runner system.Runner
}

func (c Client) Up(ctx context.Context, cfg configpkg.Config) error {
	return c.Runner.Run(
		ctx,
		cfg.Stack.Dir,
		"podman",
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"up",
		"-d",
	)
}

func (c Client) Down(ctx context.Context, cfg configpkg.Config, removeVolumes bool) error {
	args := []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"down",
	}
	if removeVolumes {
		args = append(args, "-v")
	}

	return c.Runner.Run(ctx, cfg.Stack.Dir, "podman", args...)
}

func (c Client) Logs(ctx context.Context, cfg configpkg.Config, tail int, follow bool, since string) error {
	args := []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"logs",
		"--tail",
		strconv.Itoa(tail),
	}
	if follow {
		args = append(args, "--follow")
	}
	if since != "" {
		args = append(args, "--since", since)
	}

	return c.Runner.Run(ctx, cfg.Stack.Dir, "podman", args...)
}

func (c Client) ContainerLogs(ctx context.Context, containerName string, tail int, follow bool, since string) error {
	args := []string{
		"logs",
		"--tail",
		strconv.Itoa(tail),
	}
	if follow {
		args = append(args, "--follow")
	}
	if since != "" {
		args = append(args, "--since", since)
	}
	args = append(args, containerName)

	return c.Runner.Run(ctx, "", "podman", args...)
}
