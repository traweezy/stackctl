package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start",
		Short:   "Start the local development stack",
		Example: "  stackctl start",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, true)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusStart, "starting stack..."); err != nil {
				return err
			}
			if err := deps.composeUp(context.Background(), runnerFor(cmd), cfg); err != nil {
				return err
			}

			if cfg.Behavior.WaitForServicesStart {
				waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
				defer cancel()

				if err := waitForConfiguredServices(waitCtx, cfg); err != nil {
					return err
				}
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "stack started"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}

			return printConnectionInfo(cmd, cfg)
		},
	}
}
