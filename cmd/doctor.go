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

	missing := requiredPackages(report, cfg)
	if len(missing) > 0 {
		packageManager := strings.TrimSpace(cfg.System.PackageManager)
		if packageManager == "" {
			return fmt.Errorf("doctor cannot install missing packages automatically because no package manager is configured")
		}

		ok, err := confirmAutomaticFix(cmd, yes, fmt.Sprintf("Install missing packages with %s?", packageManager))
		if err != nil {
			return err
		}
		if ok {
			installed, err := deps.installPackages(context.Background(), runnerFor(cmd), packageManager, missing)
			if err != nil {
				return err
			}
			appliedFix = true
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\n", strings.Join(installed, ", ")); err != nil {
				return err
			}
		}
	}

	if cfg.CockpitEnabled() && cfg.Setup.InstallCockpit && shouldEnableCockpit(report, missing) {
		ok, err := confirmAutomaticFix(cmd, yes, "Enable cockpit.socket now?")
		if err != nil {
			return err
		}
		if ok {
			if err := deps.enableCockpit(context.Background(), runnerFor(cmd)); err != nil {
				return err
			}
			appliedFix = true
			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "enabled cockpit.socket"); err != nil {
				return err
			}
		}
	}

	if !appliedFix {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, "no automatic fixes were applied"); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nPost-fix report:"); err != nil {
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

func shouldEnableCockpit(report doctorpkg.Report, missing []string) bool {
	for _, pkg := range missing {
		if pkg == "cockpit" || pkg == "cockpit-podman" {
			return true
		}
	}

	return doctorpkg.CheckPassed(report, "cockpit.socket installed") && !doctorpkg.CheckPassed(report, "cockpit.socket active")
}
