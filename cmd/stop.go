package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "stop [service...]",
		Short:             "Stop the local development stack or selected services",
		Example:           "  stackctl stop\n  stackctl stop postgres\n  stackctl stop redis nats",
		ValidArgsFunction: completeStackServiceArgs,
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

			target := lifecycleTargetLabel(services)
			if err := verboseComposeFile(cmd, cfg); err != nil {
				return err
			}
			if err := statusLine(cmd, output.StatusStop, fmt.Sprintf("stopping %s...", strings.ToLower(target))); err != nil {
				return err
			}
			switch {
			case len(services) == 0:
				err = composeDownAndWait(context.Background(), runnerFor(cmd), cfg, false)
			default:
				err = deps.composeStopServices(context.Background(), runnerFor(cmd), cfg, services)
			}
			if err != nil {
				return err
			}

			return statusLine(cmd, output.StatusOK, fmt.Sprintf("%s stopped", target))
		},
	}
}
