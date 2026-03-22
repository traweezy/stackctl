package cmd

import "github.com/spf13/cobra"

func newServicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "services",
		Short:   "Show full connection details for configured services",
		Example: "  stackctl services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			return printServicesInfo(cmd, cfg)
		},
	}
}
