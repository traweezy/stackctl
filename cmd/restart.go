package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "restart [service...]",
		Short:   "Restart the local development stack or selected services",
		Example: "  stackctl restart\n  stackctl restart postgres\n  stackctl restart redis nats",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			services, err := resolveTargetStackServices(cfg, args)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}
			if err := ensureNoOtherRunningStack(context.Background()); err != nil {
				return err
			}

			target := lifecycleTargetLabel(services)
			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusRestart, fmt.Sprintf("restarting %s...", strings.ToLower(target))); err != nil {
				return err
			}
			switch {
			case len(services) == 0:
				if err := deps.composeDown(context.Background(), runnerFor(cmd), cfg, false); err != nil {
					return err
				}
				err = deps.composeUp(context.Background(), runnerFor(cmd), cfg)
			default:
				err = deps.composeUpServices(context.Background(), runnerFor(cmd), cfg, true, services)
			}
			if err != nil {
				return err
			}

			if cfg.Behavior.WaitForServicesStart {
				waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
				defer cancel()

				if err := waitForSelectedServices(waitCtx, cfg, services); err != nil {
					return err
				}
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("%s restarted", target)); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}

			if len(services) > 0 {
				return printConnectionEntries(cmd, selectedConnectionEntries(cfg, services))
			}

			return printConnectionInfo(cmd, cfg)
		},
	}
}
