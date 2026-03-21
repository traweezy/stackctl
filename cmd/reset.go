package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newResetCmd() *cobra.Command {
	var volumes bool
	var force bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Bring the stack down and optionally wipe volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			if volumes && !force {
				ok, err := confirmWithPrompt(cmd, "This will remove stack volumes and delete database data. Continue?", false)
				if err != nil {
					return fmt.Errorf("volume wipe confirmation required; rerun with --force")
				}
				if !ok {
					return errors.New("reset cancelled")
				}
			}

			return deps.composeDown(context.Background(), runnerFor(cmd), cfg, volumes)
		},
	}

	cmd.Flags().BoolVarP(&volumes, "volumes", "v", false, "Remove volumes while stopping the stack")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation for destructive reset")

	return cmd
}
