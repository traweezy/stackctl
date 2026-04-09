package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var marshalVersionJSON = json.MarshalIndent

func newVersionCmd(app *App) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:               "version",
		Short:             "Print version information",
		Example:           "  stackctl version\n  stackctl version --json",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := versionOutput{
				Version:   app.Version,
				GitCommit: app.GitCommit,
				BuildDate: app.BuildDate,
			}
			if jsonOutput {
				data, err := marshalVersionJSON(info, "", "  ")
				if err != nil {
					return err
				}
				if _, err := cmd.OutOrStdout().Write(data); err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write([]byte("\n"))
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", info.Version); err != nil {
				return err
			}
			if info.GitCommit != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "git_commit: %s\n", info.GitCommit); err != nil {
					return err
				}
			}
			if info.BuildDate != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "build_date: %s\n", info.BuildDate); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Print version details as JSON")
	return cmd
}

type versionOutput struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}
