package cmd

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
)

var marshalStatusJSON = json.MarshalIndent

func newStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status for containers in this stack",
		Example: "  stackctl status\n" +
			"  stackctl status --verbose\n" +
			"  stackctl status --json",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensurePodmanRuntimeReady(); err != nil {
				return err
			}

			containers, err := loadStackContainers(context.Background(), cfg)
			if err != nil {
				return err
			}

			if jsonOutput {
				data, err := marshalStatusJSON(containers, "", "  ")
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(data)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write([]byte("\n"))
				return err
			}

			return printStatusTable(cmd, containers, verboseRequested(cmd))
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Print container status as JSON")

	return cmd
}
