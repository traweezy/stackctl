package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func runnerFor(cmd *cobra.Command) system.Runner {
	return system.Runner{
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
	}
}

func defaultTerminalInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func confirmWithPrompt(cmd *cobra.Command, message string, defaultYes bool) (bool, error) {
	if !deps.isTerminal() {
		return false, errors.New("interactive confirmation requested without a terminal")
	}

	return deps.promptYesNo(deps.stdin, cmd.OutOrStdout(), message, defaultYes)
}

func printValidationIssues(cmd *cobra.Command, issues []configpkg.ValidationIssue) error {
	for _, issue := range issues {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusFail, fmt.Sprintf("%s: %s", issue.Field, issue.Message)); err != nil {
			return err
		}
	}

	return nil
}
