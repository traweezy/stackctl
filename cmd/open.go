package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/system"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [cockpit|pgadmin|all]",
		Short: "Open configured web UIs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			target := "cockpit"
			if len(args) == 1 {
				target = strings.ToLower(args[0])
			}

			switch target {
			case "cockpit":
				return system.OpenURL(context.Background(), runnerFor(cmd), cfg.URLs.Cockpit)
			case "pgadmin":
				if !cfg.Setup.IncludePgAdmin {
					return fmt.Errorf("pgadmin is disabled in config")
				}
				return system.OpenURL(context.Background(), runnerFor(cmd), cfg.URLs.PgAdmin)
			case "all":
				if err := system.OpenURL(context.Background(), runnerFor(cmd), cfg.URLs.Cockpit); err != nil {
					return err
				}
				if cfg.Setup.IncludePgAdmin {
					return system.OpenURL(context.Background(), runnerFor(cmd), cfg.URLs.PgAdmin)
				}
				return nil
			default:
				return fmt.Errorf("invalid open target %q; valid values: cockpit, pgadmin, all", target)
			}
		},
	}
}
