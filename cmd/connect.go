package cmd

import "github.com/spf13/cobra"

func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect",
		Short: "Print connection information for local services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			return printConnectionInfo(cmd, cfg)
		},
	}
}
