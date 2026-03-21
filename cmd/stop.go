package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
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

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusAction, "stopping containers..."); err != nil {
				return err
			}
			if err := deps.composeDown(context.Background(), runnerFor(cmd), cfg, false); err != nil {
				return err
			}

			return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "stack stopped")
		},
	}
}
