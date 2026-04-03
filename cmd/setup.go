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
	"github.com/traweezy/stackctl/internal/system"
)

func newSetupCmd() *cobra.Command {
	var install bool
	var interactive bool
	var nonInteractive bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Prepare the local machine and stackctl config",
		Example: "  stackctl setup\n" +
			"  stackctl setup --non-interactive\n" +
			"  stackctl setup --install --yes",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			createdConfig := false

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
				if err := statusLine(cmd, output.StatusInfo, "config file not found"); err != nil {
					return err
				}

				switch {
				case nonInteractive:
					cfg = deps.defaultConfig()
					if err := runSpinnerAction(cmd, "Preparing managed stack scaffold", func(context.Context) error {
						return syncManagedScaffoldIfNeeded(cmd, cfg)
					}); err != nil {
						return err
					}
					if err := deps.saveConfig(path, cfg); err != nil {
						return err
					}
					exists = true
					createdConfig = true
					if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("created default config at %s", path)); err != nil {
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
						if err := runSpinnerAction(cmd, "Preparing managed stack scaffold", func(context.Context) error {
							return syncManagedScaffoldIfNeeded(cmd, cfg)
						}); err != nil {
							return err
						}
						if err := deps.saveConfig(path, cfg); err != nil {
							return err
						}
						exists = true
						createdConfig = true
						if err := statusLine(cmd, output.StatusOK, fmt.Sprintf("saved config to %s", path)); err != nil {
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
			if exists && !createdConfig && cfg.Stack.Managed && cfg.Setup.ScaffoldDefaultStack {
				needsScaffold, err := deps.managedStackNeedsScaffold(cfg)
				if err != nil {
					return err
				}
				if needsScaffold {
					shouldScaffold := nonInteractive || yes
					if !shouldScaffold && deps.isTerminal() {
						shouldScaffold, err = confirmWithPrompt(cmd, managedStackPrompt(cfg), true)
						if err != nil {
							return err
						}
					}
					if shouldScaffold {
						if err := runSpinnerAction(cmd, "Refreshing managed stack scaffold", func(context.Context) error {
							return scaffoldManagedStack(cmd, cfg, true)
						}); err != nil {
							return err
						}
					} else if err := output.StatusLine(cmd.OutOrStdout(), output.StatusWarn, "managed stack files are missing"); err != nil {
						return err
					}
				}
			}

			var report doctorpkg.Report
			if err := runSpinnerAction(cmd, "Running environment diagnostics", func(ctx context.Context) error {
				var runErr error
				report, runErr = deps.runDoctor(ctx)
				return runErr
			}); err != nil {
				return err
			}
			platform := deps.platform()

			for _, check := range report.Checks {
				if err := output.StatusLine(cmd.OutOrStdout(), check.Status, check.Message); err != nil {
					return err
				}
			}

			missing := requiredRequirements(report, cfg, platform)
			needsMachine := shouldPreparePodmanMachine(report, platform)
			needsManualCockpit := requiresManualCockpitInstall(report, cfg, platform)
			needsUnsupportedCockpit := requiresUnsupportedCockpitGuidance(cfg, platform)
			missingLabels := displayRequirementLabels(missing, cfg.System.PackageManager, platform)
			if len(missing) == 0 {
				if !needsMachine && !needsManualCockpit && !needsUnsupportedCockpit {
					if err := statusLine(cmd, output.StatusOK, "all required dependencies look available"); err != nil {
						return err
					}
				}
			} else if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Missing requirements: %s\n", strings.Join(missingLabels, ", ")); err != nil {
				return err
			}
			if needsMachine {
				if err := statusLine(cmd, output.StatusWarn, "podman machine still needs initialization or startup"); err != nil {
					return err
				}
			}
			if needsManualCockpit {
				if err := statusLine(cmd, output.StatusWarn, "cockpit helpers are enabled but cockpit must be installed manually on this platform"); err != nil {
					return err
				}
			}

			if install {
				if len(missing) == 0 && !needsMachine && !shouldEnableCockpit(report, missing, platform) && !needsManualCockpit && !needsUnsupportedCockpit {
					return statusLine(cmd, output.StatusOK, "nothing to install")
				}

				packageChoice := system.PackageManagerChoice{}
				if len(missing) > 0 {
					packageChoice, err = resolveInstallPackageManager(cfg.System.PackageManager)
					if err != nil {
						return err
					}
					if err := reportPackageManagerChoiceNotice(cmd, packageChoice); err != nil {
						return err
					}
				}

				if len(missing) > 0 && !yes {
					ok, err := confirmWithPrompt(cmd, fmt.Sprintf("Install missing packages with %s?", packageChoice.Name), false)
					if err != nil {
						return err
					}
					if !ok {
						return userCancelled(cmd, "setup install cancelled")
					}
				}

				if len(missing) > 0 {
					installed, err := deps.installPackages(context.Background(), runnerFor(cmd), packageChoice.Name, missing)
					if err != nil {
						return err
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Installed: %s\n", strings.Join(installed, ", ")); err != nil {
						return err
					}
				}

				if platform.UsesPodmanMachine() {
					if err := runSpinnerAction(cmd, "Preparing Podman machine", func(ctx context.Context) error {
						return deps.preparePodmanMachine(ctx, runnerFor(cmd))
					}); err != nil {
						return err
					}
					if err := statusLine(cmd, output.StatusOK, "podman machine is initialized and running"); err != nil {
						return err
					}
				}

				if shouldEnableCockpit(report, missing, platform) {
					if err := runSpinnerAction(cmd, "Enabling cockpit.socket", func(ctx context.Context) error {
						return deps.enableCockpit(ctx, runnerFor(cmd))
					}); err != nil {
						return err
					}
					if err := statusLine(cmd, output.StatusOK, "enabled cockpit.socket"); err != nil {
						return err
					}
				}

				missingLabels = nil
				needsMachine = false
			}

			if err := printSetupNextSteps(cmd, cfg, missingLabels, needsMachine, needsManualCockpit, needsUnsupportedCockpit); err != nil {
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

func requiredRequirements(report doctorpkg.Report, cfg configpkg.Config, platform system.Platform) []system.Requirement {
	required := make([]system.Requirement, 0, 5)

	if !doctorpkg.CheckPassed(report, "podman installed") {
		required = append(required, system.RequirementPodman)
	}
	if !doctorpkg.CheckPassed(report, "podman compose available") {
		required = append(required, system.RequirementComposeProvider)
	}
	if platform.SupportsBuildah() && !doctorpkg.CheckPassed(report, "buildah installed") {
		required = append(required, system.RequirementBuildah)
	}
	if !doctorpkg.CheckPassed(report, "skopeo installed") {
		required = append(required, system.RequirementSkopeo)
	}
	if cfg.CockpitEnabled() && cfg.Setup.InstallCockpit && platform.SupportsCockpitAutoInstall() {
		if !doctorpkg.CheckPassed(report, "cockpit.socket installed") {
			required = append(required, system.RequirementCockpit)
		}
	}

	return required
}

func printSetupNextSteps(cmd *cobra.Command, cfg configpkg.Config, missing []string, needsPodmanMachine bool, needsManualCockpit bool, needsUnsupportedCockpit bool) error {
	if deps.isTerminal() {
		return renderMarkdownBlock(cmd, setupNextStepsMarkdown(cfg, missing, needsPodmanMachine, needsManualCockpit, needsUnsupportedCockpit))
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Next steps:"); err != nil {
		return err
	}
	for _, step := range setupNextSteps(cfg, missing, needsPodmanMachine, needsManualCockpit, needsUnsupportedCockpit) {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", step); err != nil {
			return err
		}
	}
	return nil
}

func setupNextStepsMarkdown(cfg configpkg.Config, missing []string, needsPodmanMachine bool, needsManualCockpit bool, needsUnsupportedCockpit bool) string {
	steps := setupNextSteps(cfg, missing, needsPodmanMachine, needsManualCockpit, needsUnsupportedCockpit)
	lines := []string{
		"## Next steps",
		"",
	}
	for _, step := range steps {
		lines = append(lines, "- "+step)
	}
	return strings.Join(lines, "\n")
}

func setupNextSteps(cfg configpkg.Config, missing []string, needsPodmanMachine bool, needsManualCockpit bool, needsUnsupportedCockpit bool) []string {
	steps := make([]string, 0, 9)
	if len(missing) > 0 {
		steps = append(steps, fmt.Sprintf(
			"run `stackctl setup --install` or install %s manually first",
			strings.Join(missing, ", "),
		))
	}
	if needsPodmanMachine {
		steps = append(steps, "run `podman machine init` and `podman machine start` before launching the stack")
	}
	if needsManualCockpit {
		steps = append(steps, "install cockpit manually on this platform if you want the Cockpit web UI")
	}
	if needsUnsupportedCockpit {
		steps = append(steps, "disable `setup.include_cockpit` and `setup.install_cockpit` on this host, or manage Cockpit separately outside stackctl")
	}

	startHint := "run `stackctl start` after the stack config and dependencies are ready"
	if !cfg.Stack.Managed {
		startHint = "run `stackctl start` when the external stack is ready to be launched from this config"
	}

	steps = append(steps,
		startHint,
		"run `stackctl services` to inspect status, ports, and credentials",
		"run `stackctl env --export` to load app-ready environment variables",
		"run `stackctl connect` for minimal DSNs, URLs, and endpoints",
		"run `stackctl tui` for the interactive dashboard",
		"run `stackctl doctor` to re-check the environment",
	)

	return steps
}

func requirementLabels(requirements []system.Requirement) []string {
	labels := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		labels = append(labels, string(requirement))
	}
	return labels
}

func displayRequirementLabels(requirements []system.Requirement, configuredPackageManager string, platform system.Platform) []string {
	if len(requirements) == 0 {
		return nil
	}

	labels := requirementLabels(requirements)
	choice, err := system.ResolvePackageManagerChoice(configuredPackageManager, platform, deps.commandExists)
	if err != nil {
		fallbackPlan, fallbackErr := system.ResolveInstallPlan(platform.PackageManager, requirements)
		if fallbackErr != nil {
			return labels
		}
		return planDisplayLabels(fallbackPlan, labels)
	}

	plan, err := system.ResolveInstallPlan(choice.Name, requirements)
	if err != nil {
		return labels
	}

	return planDisplayLabels(plan, labels)
}

func planDisplayLabels(plan system.InstallPlan, fallback []string) []string {
	if len(plan.Packages) == 0 && len(plan.Unsupported) == 0 {
		return fallback
	}

	labels := append([]string(nil), plan.Packages...)
	for _, requirement := range plan.Unsupported {
		labels = append(labels, string(requirement))
	}
	if len(labels) == 0 {
		return fallback
	}

	return labels
}

func shouldPreparePodmanMachine(report doctorpkg.Report, platform system.Platform) bool {
	if !platform.UsesPodmanMachine() {
		return false
	}

	return !doctorpkg.CheckPassed(report, "podman machine initialized") || !doctorpkg.CheckPassed(report, "podman machine running")
}

func containsRequirement(requirements []system.Requirement, target system.Requirement) bool {
	for _, requirement := range requirements {
		if requirement == target {
			return true
		}
	}
	return false
}

func shouldEnableCockpit(report doctorpkg.Report, requirements []system.Requirement, platform system.Platform) bool {
	if !platform.SupportsCockpitAutoEnable() {
		return false
	}
	if containsRequirement(requirements, system.RequirementCockpit) {
		return true
	}

	return doctorpkg.CheckPassed(report, "cockpit.socket installed") && !doctorpkg.CheckPassed(report, "cockpit.socket active")
}

func requiresManualCockpitInstall(report doctorpkg.Report, cfg configpkg.Config, platform system.Platform) bool {
	if !cfg.CockpitEnabled() || !cfg.Setup.InstallCockpit || !platform.SupportsCockpit() || platform.SupportsCockpitAutoInstall() {
		return false
	}

	return !doctorpkg.CheckPassed(report, "cockpit.socket installed")
}

func requiresUnsupportedCockpitGuidance(cfg configpkg.Config, platform system.Platform) bool {
	return cfg.CockpitEnabled() && !platform.SupportsCockpit()
}
