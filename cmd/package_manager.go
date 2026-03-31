package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func resolveInstallPackageManager(configured string) (system.PackageManagerChoice, error) {
	choice, err := system.ResolvePackageManagerChoice(configured, deps.platform(), deps.commandExists)
	if err != nil {
		return system.PackageManagerChoice{}, fmt.Errorf("resolve package manager: %w", err)
	}
	return choice, nil
}

func reportPackageManagerChoiceNotice(cmd *cobra.Command, choice system.PackageManagerChoice) error {
	if choice.Notice == "" {
		return nil
	}
	return output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, choice.Notice)
}
