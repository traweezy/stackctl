package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
)

func newDoctorCmd() *cobra.Command {
	var fix bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics and optional fixes for the local stack",
		Example: "  stackctl doctor\n" +
			"  stackctl doctor --fix --yes",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fix {
				return runDoctorFixes(cmd, yes)
			}

			report, err := deps.runDoctor(context.Background())
			if err != nil {
				return err
			}

			if err := printDoctorReport(cmd, report); err != nil {
				return err
			}

			if report.HasFailures() {
				return fmt.Errorf("doctor found issues that need attention")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Try to apply supported fixes for doctor findings")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Assume yes for automatic fix prompts")

	return cmd
}

func runDoctorFixes(cmd *cobra.Command, yes bool) error {
	report, err := deps.runDoctor(context.Background())
	if err != nil {
		return err
	}
	if err := printDoctorReport(cmd, report); err != nil {
		return err
	}

	cfg := deps.defaultConfig()
	path, err := deps.configFilePath()
	if err != nil {
		return err
	}
	if loadedCfg, exists, err := loadExistingConfig(path); err != nil {
		return err
	} else if exists {
		cfg = loadedCfg
	}
	platform := deps.platform()

	appliedFix := false
	if cfg.Stack.Managed && cfg.Setup.ScaffoldDefaultStack {
		needsScaffold, err := deps.managedStackNeedsScaffold(cfg)
		if err != nil {
			return err
		}
		if needsScaffold {
			ok, err := confirmAutomaticFix(cmd, yes, managedStackPrompt(cfg))
			if err != nil {
				return err
			}
			if ok {
				if err := scaffoldManagedStack(cmd, cfg, true); err != nil {
					return err
				}
				appliedFix = true
			}
		}
	}

	missing := requiredRequirements(report, cfg, platform)
	if len(missing) > 0 {
		packageChoice, err := resolveInstallPackageManager(cfg.System.PackageManager)
		if err != nil {
			return fmt.Errorf("doctor cannot install missing packages automatically: %w", err)
		}
		if err := reportPackageManagerChoiceNotice(cmd, packageChoice); err != nil {
			return err
		}

		ok, err := confirmAutomaticFix(cmd, yes, fmt.Sprintf("Install missing packages with %s?", packageChoice.Name))
		if err != nil {
			return err
		}
		if ok {
			installed, err := deps.installPackages(context.Background(), runnerFor(cmd), packageChoice.Name, missing)
			if err != nil {
				return err
			}
			appliedFix = true
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\n", strings.Join(installed, ", ")); err != nil {
				return err
			}
		}
	}

	if shouldPreparePodmanMachine(report, platform) {
		ok, err := confirmAutomaticFix(cmd, yes, "Initialize and start the Podman machine now?")
		if err != nil {
			return err
		}
		if ok {
			if err := deps.preparePodmanMachine(context.Background(), runnerFor(cmd)); err != nil {
				return err
			}
			appliedFix = true
			if err := statusLine(cmd, output.StatusOK, "podman machine is initialized and running"); err != nil {
				return err
			}
		}
	}

	if shouldEnableCockpit(report, missing, platform) {
		ok, err := confirmAutomaticFix(cmd, yes, "Enable cockpit.socket now?")
		if err != nil {
			return err
		}
		if ok {
			if err := deps.enableCockpit(context.Background(), runnerFor(cmd)); err != nil {
				return err
			}
			appliedFix = true
			if err := statusLine(cmd, output.StatusOK, "enabled cockpit.socket"); err != nil {
				return err
			}
		}
	}

	if !appliedFix {
		if err := statusLine(cmd, output.StatusInfo, "no automatic fixes were applied"); err != nil {
			return err
		}
	}

	if err := plainLine(cmd, "\nPost-fix report:\n"); err != nil {
		return err
	}
	postReport, err := deps.runDoctor(context.Background())
	if err != nil {
		return err
	}
	if err := printDoctorReport(cmd, postReport); err != nil {
		return err
	}
	if postReport.HasFailures() {
		return fmt.Errorf("doctor still found issues that need attention")
	}

	return nil
}

func printDoctorReport(cmd *cobra.Command, report doctorpkg.Report) error {
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

	return nil
}

func confirmAutomaticFix(cmd *cobra.Command, yes bool, prompt string) (bool, error) {
	if yes {
		return true, nil
	}

	ok, err := confirmWithPrompt(cmd, prompt, false)
	if err != nil {
		return false, fmt.Errorf("automatic fix confirmation required; rerun with --yes")
	}

	return ok, nil
}
