package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage persistent stack configuration",
		Example: "  stackctl config view\n" +
			"  stackctl config validate\n" +
			"  stackctl config scaffold --force",
	}

	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigValidateCmd())
	cmd.AddCommand(newConfigResetCmd())
	cmd.AddCommand(newConfigScaffoldCmd())

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var force bool
	var nonInteractive bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new stackctl config",
		Example: "  stackctl config init\n" +
			"  stackctl config init --non-interactive\n" +
			"  stackctl config init --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := deps.configFilePath()
			if err != nil {
				return err
			}

			current, exists, err := loadExistingConfig(path)
			if err != nil {
				return err
			}

			if exists && !force {
				ok, err := confirmWithPrompt(cmd, "A stackctl config already exists. Overwrite it?", false)
				if err != nil {
					return fmt.Errorf("config already exists at %s; rerun with --force or use an interactive terminal to confirm", path)
				}
				if !ok {
					return userCancelled(cmd, "config init cancelled")
				}
			}

			base := deps.defaultConfig()
			if exists {
				base = current
			}

			cfg, err := resolveConfigFromFlags(cmd, base, nonInteractive)
			if err != nil {
				return err
			}
			if err := scaffoldManagedStack(cmd, cfg, force); err != nil {
				return err
			}

			if err := deps.saveConfig(path, cfg); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved config to %s\n", path)
			return err
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing config without prompting")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Create the config from defaults without prompts")

	return cmd
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "view",
		Short:   "Print the current config in YAML format",
		Example: "  stackctl config view",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := deps.loadConfig("")
			if err != nil {
				return missingConfigHint(err)
			}

			data, err := deps.marshalConfig(cfg)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "path",
		Short:   "Print the resolved config path",
		Example: "  stackctl config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := deps.configFilePath()
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
}

func newConfigEditCmd() *cobra.Command {
	var nonInteractive bool

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit the current config using the interactive wizard",
		Example: "  stackctl config edit\n" +
			"  stackctl config edit --non-interactive",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := deps.configFilePath()
			if err != nil {
				return err
			}

			current, err := deps.loadConfig(path)
			if err != nil {
				return missingConfigHint(err)
			}

			cfg, err := resolveConfigFromFlags(cmd, current, nonInteractive)
			if err != nil {
				return err
			}
			if err := scaffoldManagedStack(cmd, cfg, false); err != nil {
				return err
			}

			if err := deps.saveConfig(path, cfg); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Updated config at %s\n", path)
			return err
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Save the current config after applying derived defaults")

	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Short:   "Validate the current config",
		Example: "  stackctl config validate",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := deps.loadConfig("")
			if err != nil {
				return missingConfigHint(err)
			}

			issues := deps.validateConfig(cfg)
			if len(issues) == 0 {
				return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "config is valid")
			}

			if err := printValidationIssues(cmd, issues); err != nil {
				return err
			}

			return fmt.Errorf("config validation failed with %d issue(s)", len(issues))
		},
	}
}

func newConfigResetCmd() *cobra.Command {
	var force bool
	var yes bool
	var deleteConfig bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset the config to defaults or delete it",
		Example: "  stackctl config reset\n" +
			"  stackctl config reset --delete --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := deps.configFilePath()
			if err != nil {
				return err
			}

			if deleteConfig {
				if !force && !yes {
					ok, err := confirmWithPrompt(cmd, "Delete the stackctl config file?", false)
					if err != nil {
						return fmt.Errorf("delete confirmation required; rerun with --force or --yes")
					}
					if !ok {
						return userCancelled(cmd, "config reset cancelled")
					}
				}

				if err := deps.removeFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("delete config %s: %w", path, err)
				}

				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted config at %s\n", path)
				return err
			}

			if !force && !yes {
				ok, err := confirmWithPrompt(cmd, "Reset the stackctl config to defaults?", false)
				if err != nil {
					return fmt.Errorf("reset confirmation required; rerun with --force or --yes")
				}
				if !ok {
					return userCancelled(cmd, "config reset cancelled")
				}
			}

			cfg := deps.defaultConfig()
			if err := scaffoldManagedStack(cmd, cfg, false); err != nil {
				return err
			}
			if err := deps.saveConfig(path, cfg); err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Reset config at %s to defaults\n", path)
			return err
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	cmd.Flags().BoolVar(&yes, "yes", false, "Assume yes for confirmation prompts")
	cmd.Flags().BoolVar(&deleteConfig, "delete", false, "Delete the config file instead of resetting it")

	return cmd
}

func newConfigScaffoldCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Create or refresh the managed stack files from embedded templates",
		Example: "  stackctl config scaffold\n" +
			"  stackctl config scaffold --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := deps.loadConfig("")
			if err != nil {
				return missingConfigHint(err)
			}
			if !cfg.Stack.Managed {
				return errors.New("config is using an external stack; switch to a managed stack before scaffolding")
			}

			return scaffoldManagedStack(cmd, cfg, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite managed stack files from embedded templates")

	return cmd
}

func resolveConfigFromFlags(cmd *cobra.Command, base configpkg.Config, nonInteractive bool) (configpkg.Config, error) {
	if nonInteractive {
		base.ApplyDerivedFields()
		return base, nil
	}

	if !deps.isTerminal() {
		return configpkg.Config{}, errors.New("interactive config requires a terminal; rerun with --non-interactive")
	}

	return deps.runWizard(deps.stdin, cmd.OutOrStdout(), base)
}

func loadExistingConfig(path string) (configpkg.Config, bool, error) {
	cfg, err := deps.loadConfig(path)
	if err == nil {
		return cfg, true, nil
	}
	if errors.Is(err, configpkg.ErrNotFound) {
		return configpkg.Config{}, false, nil
	}

	return configpkg.Config{}, false, err
}

func missingConfigHint(err error) error {
	if !errors.Is(err, configpkg.ErrNotFound) {
		return err
	}

	return errors.New("no stackctl config was found; run `stackctl setup` or `stackctl config init`")
}
