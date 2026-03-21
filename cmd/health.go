package cmd

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newHealthCmd() *cobra.Command {
	var watch bool
	var interval int

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check whether the local stack is reachable",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			printOnce := func(ctx context.Context) error {
				lines, err := healthChecks(ctx, cfg)
				if err != nil {
					return err
				}
				for _, line := range lines {
					if err := output.StatusLine(cmd.OutOrStdout(), line.Status, line.Message); err != nil {
						return err
					}
				}
				return nil
			}

			if !watch {
				return printOnce(context.Background())
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			ticker := time.NewTicker(time.Duration(interval) * time.Second)
			defer ticker.Stop()

			for {
				if err := printOnce(ctx); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if _, err := cmd.OutOrStdout().Write([]byte("\n")); err != nil {
						return err
					}
				}
			}
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Continuously rerun health checks")
	cmd.Flags().IntVarP(&interval, "interval", "i", 5, "Watch interval in seconds")

	return cmd
}
