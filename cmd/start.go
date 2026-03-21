package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/compose"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the local development stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, true)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			client := compose.Client{Runner: runnerFor(cmd)}
			if err := client.Up(context.Background(), cfg); err != nil {
				return err
			}

			if cfg.Behavior.WaitForServicesStart {
				waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
				defer cancel()

				if err := waitForConfiguredServices(waitCtx, cfg); err != nil {
					return err
				}
			}

			if cfg.Behavior.OpenCockpitOnStart {
				if err := system.OpenURL(context.Background(), runnerFor(cmd), cfg.URLs.Cockpit); err != nil {
					return err
				}
			}
			if cfg.Setup.IncludePgAdmin && cfg.Behavior.OpenPgAdminOnStart {
				if err := system.OpenURL(context.Background(), runnerFor(cmd), cfg.URLs.PgAdmin); err != nil {
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
