package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var watch bool
	var tail int
	var service string
	var since string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show logs for the stack or a single service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			if service == "" {
				return deps.composeLogs(context.Background(), runnerFor(cmd), cfg, tail, watch, since)
			}

			containerName, err := serviceContainer(cfg, service)
			if err != nil {
				return err
			}

			return deps.containerLogs(context.Background(), runnerFor(cmd), containerName, tail, watch, since)
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Follow logs")
	cmd.Flags().IntVarP(&tail, "tail", "n", 100, "Number of log lines to show")
	cmd.Flags().StringVarP(&service, "service", "s", "", "Filter logs to a single service")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since a relative time or timestamp")

	return cmd
}
