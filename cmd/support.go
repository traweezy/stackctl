package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

type rootOutputOptions struct {
	Verbose bool
	Quiet   bool
}

var rootOutput rootOutputOptions

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
	return statusLine(cmd, output.StatusInfo, message)
}

func printValidationIssues(cmd *cobra.Command, issues []configpkg.ValidationIssue) error {
	for _, issue := range issues {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusFail, fmt.Sprintf("%s: %s", issue.Field, issue.Message)); err != nil {
			return err
		}
	}

	return nil
}

func filterAutoScaffoldValidationIssues(cfg configpkg.Config, issues []configpkg.ValidationIssue) []configpkg.ValidationIssue {
	if len(issues) == 0 {
		return nil
	}

	filtered := make([]configpkg.ValidationIssue, 0, len(issues))
	for _, issue := range issues {
		if pendingManagedScaffoldIssue(cfg, issue) {
			continue
		}
		filtered = append(filtered, issue)
	}

	return filtered
}

func pendingManagedScaffoldIssue(cfg configpkg.Config, issue configpkg.ValidationIssue) bool {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return false
	}

	normalized := cfg
	normalized.ApplyDerivedFields()

	expectedDir, err := configpkg.ManagedStackDir(normalized.Stack.Name)
	if err != nil {
		return false
	}
	if normalized.Stack.Dir != expectedDir || normalized.Stack.ComposeFile != configpkg.DefaultComposeFileName {
		return false
	}

	switch issue.Field {
	case "stack.dir":
		return issue.Message == fmt.Sprintf("directory does not exist: %s", normalized.Stack.Dir)
	case "stack.compose_file":
		return issue.Message == fmt.Sprintf("file does not exist: %s", configpkg.ComposePath(normalized))
	default:
		return false
	}
}

func managedStackPrompt(cfg configpkg.Config) string {
	return fmt.Sprintf(
		"A managed stack can be created or refreshed for you in:\n\n%s\n\nRefresh the managed stack files now?",
		cfg.Stack.Dir,
	)
}

func scaffoldManagedStack(cmd *cobra.Command, cfg configpkg.Config, force bool) error {
	if !cfg.Stack.Managed {
		return nil
	}

	result, err := deps.scaffoldManagedStack(cfg, force)
	if err != nil {
		return err
	}

	if result.CreatedDir {
		if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("created managed stack directory %s", result.StackDir)); err != nil {
			return err
		}
	}
	if result.WroteCompose {
		if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("wrote managed compose file %s", result.ComposePath)); err != nil {
			return err
		}
	}
	if result.WroteNATSConfig {
		if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("wrote managed nats config file %s", result.NATSConfigPath)); err != nil {
			return err
		}
	}
	if result.WroteRedisACL {
		if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("wrote managed redis ACL file %s", result.RedisACLPath)); err != nil {
			return err
		}
	}
	if result.WrotePgAdminServers {
		if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("wrote managed pgAdmin server bootstrap file %s", result.PgAdminServersPath)); err != nil {
			return err
		}
	}
	if result.WrotePGPass {
		if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("wrote managed pgpass file %s", result.PGPassPath)); err != nil {
			return err
		}
	}
	if result.AlreadyPresent {
		return statusLine(cmd, output.StatusOK, fmt.Sprintf("managed stack already exists at %s", result.ComposePath))
	}

	return nil
}

func syncManagedScaffoldIfNeeded(cmd *cobra.Command, cfg configpkg.Config) error {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return nil
	}

	needsScaffold, err := deps.managedStackNeedsScaffold(cfg)
	if err != nil {
		return err
	}
	if !needsScaffold {
		return nil
	}

	return scaffoldManagedStack(cmd, cfg, true)
}

func syncManagedScaffoldIfNeededForConfig(cfg configpkg.Config) error {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return nil
	}

	needsScaffold, err := deps.managedStackNeedsScaffold(cfg)
	if err != nil {
		return err
	}
	if !needsScaffold {
		return nil
	}

	_, err = deps.scaffoldManagedStack(cfg, true)
	return err
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

func quietRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		return rootOutput.Quiet
	}
	value, err := cmd.Flags().GetBool("quiet")
	if err == nil {
		return value
	}
	return rootOutput.Quiet
}

func verboseRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		return rootOutput.Verbose
	}
	value, err := cmd.Flags().GetBool("verbose")
	if err == nil {
		return value
	}
	return rootOutput.Verbose
}

func statusLine(cmd *cobra.Command, status, message string) error {
	if quietRequested(cmd) {
		return nil
	}
	return output.StatusLine(cmd.OutOrStdout(), status, message)
}

func blankLine(cmd *cobra.Command) error {
	if quietRequested(cmd) {
		return nil
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout())
	return err
}

func plainLine(cmd *cobra.Command, format string, args ...any) error {
	if quietRequested(cmd) {
		return nil
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), format, args...)
	return err
}

func verboseLine(cmd *cobra.Command, message string) error {
	if quietRequested(cmd) || !verboseRequested(cmd) {
		return nil
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(message))
	return err
}

func verboseComposeFile(cmd *cobra.Command, cfg configpkg.Config) error {
	return verboseLine(cmd, fmt.Sprintf("Using compose file %s", deps.composePath(cfg)))
}
