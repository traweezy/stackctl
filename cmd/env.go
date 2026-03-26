package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newEnvCmd() *cobra.Command {
	var jsonOutput bool
	var exportOutput bool

	cmd := &cobra.Command{
		Use:   "env [service...]",
		Short: "Print app-ready environment variables",
		Long: "Print app-ready environment variables from the current stack config.\n\n" +
			"By default this prints shell-safe KEY=value assignments. Use --export\n" +
			"when you want export-prefixed lines for eval/source workflows, or --json\n" +
			"for tooling.",
		Example: "  stackctl env\n" +
			"  stackctl env --export\n" +
			"  stackctl env postgres redis\n" +
			"  stackctl env --json",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeEnvArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if jsonOutput && exportOutput {
				return errors.New("--json and --export cannot be used together")
			}
			if jsonOutput {
				return printEnvJSON(cmd, cfg, args)
			}
			return printEnvInfo(cmd, cfg, args, exportOutput)
		},
	}

	cmd.Flags().BoolVar(&exportOutput, "export", false, "Prefix assignments with export for shell eval workflows")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Print environment variables as JSON")

	return cmd
}
