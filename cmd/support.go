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

func managedStackPrompt(cfg configpkg.Config) string {
	return fmt.Sprintf(
		"A managed stack can be created for you in:\n\n%s\n\nCreate and scaffold the default stack now?",
		cfg.Stack.Dir,
	)
}

func scaffoldManagedStack(cmd *cobra.Command, cfg configpkg.Config, force bool) error {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return nil
	}

	result, err := deps.scaffoldManagedStack(cfg, force)
	if err != nil {
		return err
	}

	if result.CreatedDir {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("created managed stack directory %s", result.StackDir)); err != nil {
			return err
		}
	}
	if result.WroteCompose {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("wrote managed compose file %s", result.ComposePath)); err != nil {
			return err
		}
	}
	if result.AlreadyPresent {
		return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("managed stack already exists at %s", result.ComposePath))
	}

	return nil
}
