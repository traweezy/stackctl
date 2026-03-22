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
		Stdin:  cmd.InOrStdin(),
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
	}
}

func defaultTerminalInteractive() bool {
	stdinFD, ok := fileDescriptor(os.Stdin)
	if !ok {
		return false
	}

	stdoutFD, ok := fileDescriptor(os.Stdout)
	if !ok {
		return false
	}

	return term.IsTerminal(stdinFD) && term.IsTerminal(stdoutFD)
}

func confirmWithPrompt(cmd *cobra.Command, message string, defaultYes bool) (bool, error) {
	if !deps.isTerminal() {
		return false, errors.New("interactive confirmation requested without a terminal")
	}

	return deps.promptYesNo(deps.stdin, cmd.OutOrStdout(), message, defaultYes)
}

func userCancelled(cmd *cobra.Command, message string) error {
	return output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, message)
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

func fileDescriptor(file *os.File) (int, bool) {
	fd := file.Fd()
	maxInt := ^uint(0) >> 1
	if fd > uintptr(maxInt) {
		return 0, false
	}

	// #nosec G115 -- fd is range-checked against the platform int size above.
	return int(fd), true
}
