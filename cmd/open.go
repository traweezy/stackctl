package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [cockpit|pgadmin|all]",
		Short: "Open configured web UIs",
		Long:  "Open configured stack web UIs. If browser launch is unavailable, stackctl prints the URL instead.",
		Example: "  stackctl open\n" +
			"  stackctl open cockpit\n" +
			"  stackctl open pgadmin\n" +
			"  stackctl open all",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeOpenTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			target := "cockpit"
			if len(args) == 1 {
				target = strings.ToLower(args[0])
			} else if !cfg.CockpitEnabled() && cfg.PgAdminEnabled() {
				target = "pgadmin"
			}

			switch target {
			case "cockpit":
				if !cfg.CockpitEnabled() {
					return fmt.Errorf("cockpit is disabled in config")
				}
				return openConfiguredURL(cmd, "cockpit", cfg.URLs.Cockpit)
			case "pgadmin":
				if !cfg.PgAdminEnabled() {
					return fmt.Errorf("pgadmin is disabled in config")
				}
				return openConfiguredURL(cmd, "pgadmin", cfg.URLs.PgAdmin)
			case "all":
				if cfg.CockpitEnabled() {
					if err := openConfiguredURL(cmd, "cockpit", cfg.URLs.Cockpit); err != nil {
						return err
					}
				}
				if cfg.PgAdminEnabled() {
					return openConfiguredURL(cmd, "pgadmin", cfg.URLs.PgAdmin)
				}
				return nil
			default:
				return fmt.Errorf("invalid open target %q; valid values: cockpit, pgadmin, all", target)
			}
		},
	}
}

func openConfiguredURL(cmd *cobra.Command, name, target string) error {
	if err := deps.openURL(context.Background(), runnerFor(cmd), target); err == nil {
		return nil
	}

	if err := output.StatusLine(cmd.OutOrStdout(), output.StatusWarn, fmt.Sprintf("could not open %s automatically; use this URL", name)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n", target)
	return err
}
