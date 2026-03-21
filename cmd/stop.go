package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/compose"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the local development stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			client := compose.Client{Runner: runnerFor(cmd)}
			return client.Down(context.Background(), cfg, false)
		},
	}
}
