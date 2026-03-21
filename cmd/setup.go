package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
)

func newSetupCmd() *cobra.Command {
	var install bool
	var interactive bool
	var nonInteractive bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Prepare the local machine and stackctl config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactive && nonInteractive {
				return errors.New("--interactive and --non-interactive cannot be used together")
			}

			path, err := deps.configFilePath()
			if err != nil {
				return err
			}

			cfg, exists, err := loadExistingConfig(path)
			if err != nil {
				return err
			}

			if !exists {
				if err := output.StatusLine(cmd.OutOrStdout(), output.StatusMiss, "config file not found"); err != nil {
					return err
				}

				switch {
				case nonInteractive:
					cfg = deps.defaultConfig()
					if err := deps.saveConfig(path, cfg); err != nil {
						return err
					}
					exists = true
					if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("created default config at %s", path)); err != nil {
						return err
					}
				case interactive || deps.isTerminal():
					ok := true
					if !interactive {
						ok, err = confirmWithPrompt(cmd, "No stackctl config was found. Run interactive setup now?", true)
						if err != nil {
							return err
						}
					}
					if ok {
						cfg, err = deps.runWizard(deps.stdin, cmd.OutOrStdout(), deps.defaultConfig())
						if err != nil {
							return err
						}
						if err := deps.saveConfig(path, cfg); err != nil {
							return err
						}
						exists = true
						if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("saved config to %s", path)); err != nil {
							return err
						}
					}
				default:
					return errors.New("no config exists and setup cannot prompt without a terminal; rerun with --non-interactive")
				}
			}

			if !exists {
				cfg = deps.defaultConfig()
			}

			report, err := deps.runDoctor(context.Background())
			if err != nil {
				return err
			}

			for _, check := range report.Checks {
				if err := output.StatusLine(cmd.OutOrStdout(), check.Status, check.Message); err != nil {
					return err
				}
			}

			missing := requiredPackages(report, cfg)
			if len(missing) == 0 {
				if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "all required packages look available"); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Missing packages: %s\n", strings.Join(missing, ", ")); err != nil {
				return err
			}

			if install {
				if len(missing) == 0 {
					return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "nothing to install")
				}

				if !yes {
					ok, err := confirmWithPrompt(cmd, fmt.Sprintf("Install missing packages with %s?", cfg.System.PackageManager), false)
					if err != nil {
						return err
					}
					if !ok {
						return errors.New("setup install cancelled")
					}
				}

				installed, err := deps.installPackages(context.Background(), runnerFor(cmd), cfg.System.PackageManager, missing)
				if err != nil {
					return err
				}

				if cfg.Setup.InstallCockpit {
					if err := deps.enableCockpit(context.Background(), runnerFor(cmd)); err != nil {
						return err
					}
				}

				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\n", strings.Join(installed, ", ")); err != nil {
					return err
				}
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Next steps:"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "- run `stackctl doctor` to re-check the environment"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "- run `stackctl start` after the stack config and dependencies are ready"); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&install, "install", false, "Install supported missing dependencies")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "Force interactive config setup")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Skip prompts and use defaults where possible")
	cmd.Flags().BoolVar(&yes, "yes", false, "Assume yes for installation prompts")

	return cmd
}

func requiredPackages(report doctorpkg.Report, cfg configpkg.Config) []string {
	required := make([]string, 0, 6)

	if !doctorpkg.CheckPassed(report, "podman installed") {
		required = append(required, "podman")
	}
	if !doctorpkg.CheckPassed(report, "podman compose available") {
		required = append(required, "podman-compose")
	}
	if !doctorpkg.CheckPassed(report, "buildah installed") {
		required = append(required, "buildah")
	}
	if !doctorpkg.CheckPassed(report, "skopeo installed") {
		required = append(required, "skopeo")
	}
	if cfg.Setup.InstallCockpit {
		if !doctorpkg.CheckPassed(report, "cockpit.socket installed") {
			required = append(required, "cockpit", "cockpit-podman")
		}
	}

	return required
}
