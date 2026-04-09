package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

var (
	healthNotifyContext = func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
		return signal.NotifyContext(parent, signals...)
	}
	newHealthTicker = func(interval time.Duration) (<-chan time.Time, func()) {
		ticker := time.NewTicker(interval)
		return ticker.C, ticker.Stop
	}
)

func newHealthCmd() *cobra.Command {
	var watch bool
	var interval int

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check whether the local stack is reachable",
		Example: "  stackctl health\n" +
			"  stackctl health --watch --interval 2",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			printOnce := func(ctx context.Context) error {
				lines, _ := healthChecks(ctx, cfg)
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

			ctx, stop := healthNotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			tickC, stopTicker := newHealthTicker(time.Duration(interval) * time.Second)
			defer stopTicker()

			for {
				if err := printOnce(ctx); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return nil
				case <-tickC:
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
