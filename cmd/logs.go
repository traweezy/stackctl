package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var watch bool
	var tail int
	var service string
	var since string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show recent stack logs or follow them live",
		Long: "Show recent stack logs. By default this prints the last 100 lines and exits. " +
			"Use --watch to keep streaming log output.",
		Example: "  stackctl logs\n" +
			"  stackctl logs --watch\n" +
			"  stackctl logs --service postgres\n" +
			"  stackctl logs --service pg --tail 200 --watch",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			var serviceName string
			if service != "" {
				serviceName, err = canonicalServiceName(service)
				if err != nil {
					return err
				}
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			ctx := context.Background()
			if watch {
				watchCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
				defer stop()
				ctx = watchCtx
			}

			err = deps.composeLogs(ctx, runnerFor(cmd), cfg, tail, watch, since, serviceName)
			if watch && ctx.Err() != nil {
				return nil
			}
			return err
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Follow logs")
	cmd.Flags().IntVarP(&tail, "tail", "n", 100, "Number of log lines to show")
	cmd.Flags().StringVarP(&service, "service", "s", "", "Filter logs to a single service (postgres|pg, redis|rd, pgadmin)")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since a relative time or timestamp")

	return cmd
}
