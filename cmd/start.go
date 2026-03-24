package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "start [service...]",
		Short:             "Start the local development stack or selected services",
		Example:           "  stackctl start\n  stackctl start postgres\n  stackctl start redis nats",
		ValidArgsFunction: completeStackServiceArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, true)
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
			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusStart, fmt.Sprintf("starting %s...", strings.ToLower(target))); err != nil {
				return err
			}
			switch {
			case len(services) == 0:
				err = deps.composeUp(context.Background(), runnerFor(cmd), cfg)
			default:
				err = deps.composeUpServices(context.Background(), runnerFor(cmd), cfg, false, services)
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

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("%s started", target)); err != nil {
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
