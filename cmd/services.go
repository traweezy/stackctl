package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newServicesCmd() *cobra.Command {
	var jsonOutput bool
	var copyTarget string

	cmd := &cobra.Command{
		Use:   "services",
		Short: "Show full connection details for configured services",
		Example: "  stackctl services\n" +
			"  stackctl services --json\n" +
			"  stackctl services --copy meilisearch-api-key\n" +
			"  stackctl services --copy seaweedfs\n" +
			"  stackctl services --copy seaweedfs-secret-key",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if jsonOutput && copyTarget != "" {
				return errors.New("--json and --copy cannot be used together")
			}
			if copyTarget != "" {
				label, value, err := serviceCopyTarget(cfg, copyTarget)
				if err != nil {
					return err
				}
				if err := deps.copyToClipboard(context.Background(), runnerFor(cmd), value); err != nil {
					return err
				}
				return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("copied %s to clipboard", label))
			}
			if jsonOutput {
				return printServicesJSON(cmd, cfg)
			}

			return printServicesInfo(cmd, cfg)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Print service details as JSON")
	cmd.Flags().StringVar(&copyTarget, "copy", "", "Copy a service value such as postgres, meilisearch-api-key, seaweedfs, seaweedfs-secret-key, pgadmin, or cockpit")
	mustRegisterFlagCompletion(cmd, "copy", completeServiceCopyTargets)

	return cmd
}
