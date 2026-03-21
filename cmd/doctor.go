package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run read-only diagnostics for the local stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := doctorpkg.Run(context.Background())
			if err != nil {
				return err
			}

			for _, check := range report.Checks {
				if err := output.StatusLine(cmd.OutOrStdout(), check.Status, check.Message); err != nil {
					return err
				}
			}

			if _, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Summary: %d ok, %d warn, %d miss, %d fail\n",
				report.OKCount,
				report.WarnCount,
				report.MissCount,
				report.FailCount,
			); err != nil {
				return err
			}

			if report.HasFailures() {
				return fmt.Errorf("doctor found issues that need attention")
			}

			return nil
		},
	}
}
