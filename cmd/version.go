package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:               "version",
		Short:             "Print version information",
		Example:           "  stackctl version",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", app.Version)
			if err != nil {
				return err
			}

			if app.GitCommit != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "git_commit: %s\n", app.GitCommit); err != nil {
					return err
				}
			}

			if app.BuildDate != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "build_date: %s\n", app.BuildDate); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
