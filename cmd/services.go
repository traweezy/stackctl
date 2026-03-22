package cmd

import "github.com/spf13/cobra"

func newServicesCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "services",
		Short: "Show full connection details for configured services",
		Example: "  stackctl services\n" +
			"  stackctl services --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printServicesJSON(cmd, cfg)
			}

			return printServicesInfo(cmd, cfg)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Print service details as JSON")

	return cmd
}
