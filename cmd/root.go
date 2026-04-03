package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/logging"
)

type App struct {
	Version   string
	GitCommit string
	BuildDate string
}

func NewApp() *App {
	return &App{
		Version: "0.20.1",
	}
}

func NewRootCmd(app *App) *cobra.Command {
	var stackName string

	cmd := &cobra.Command{
		Use:   "stackctl",
		Short: "Manage a local Podman development stack",
		Example: "  stackctl setup\n" +
			"  stackctl start\n" +
			"  stackctl --stack staging start\n" +
			"  stackctl stack list\n" +
			"  stackctl tui\n" +
			"  stackctl services\n" +
			"  stackctl exec postgres -- psql -U app -d app",
		SilenceUsage:               true,
		SilenceErrors:              true,
		Version:                    app.Version,
		SuggestionsMinimumDistance: 2,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if rootOutput.Verbose && rootOutput.Quiet {
				return fmt.Errorf("--verbose and --quiet cannot be used together")
			}
			if rootOutput.Accessible && rootOutput.PlainWizard {
				return fmt.Errorf("--accessible and --plain cannot be used together")
			}
			if err := logging.ValidateLevel(rootOutput.LogLevel); err != nil {
				return fmt.Errorf("invalid --log-level %q", strings.TrimSpace(rootOutput.LogLevel))
			}
			if err := logging.ValidateFormat(rootOutput.LogFormat); err != nil {
				return fmt.Errorf("invalid --log-format %q: use text, json, or logfmt", strings.TrimSpace(rootOutput.LogFormat))
			}
			if err := applyRootEnvOverrides(cmd); err != nil {
				return err
			}
			logging.Reset()
			selected := strings.TrimSpace(stackName)
			if selected == "" {
				var err error
				selected, err = configpkg.ResolveSelectedStackName()
				if err != nil {
					return err
				}
			}
			if err := configpkg.ValidateStackName(selected); err != nil {
				return err
			}
			return os.Setenv(configpkg.StackNameEnvVar, selected)
		},
	}

	cmd.AddGroup(rootCommandGroups()...)
	cmd.SetHelpCommandGroupID(commandGroupUtility)
	cmd.SetCompletionCommandGroupID(commandGroupUtility)
	cmd.PersistentFlags().StringVar(&stackName, "stack", "", "Select a named stack config (overrides STACKCTL_STACK and the saved current stack)")
	cmd.PersistentFlags().BoolVarP(&rootOutput.Verbose, "verbose", "v", false, "Print extra lifecycle detail")
	cmd.PersistentFlags().BoolVarP(&rootOutput.Quiet, "quiet", "q", false, "Suppress non-essential progress output")
	cmd.PersistentFlags().BoolVar(&rootOutput.Accessible, "accessible", false, "Render interactive prompts and spinners in accessible mode")
	cmd.PersistentFlags().BoolVar(&rootOutput.PlainWizard, "plain", false, "Force the legacy plain-text config wizard instead of the form UI")
	cmd.PersistentFlags().StringVar(&rootOutput.LogLevel, "log-level", "", "Set the internal log level when --log-file is enabled")
	cmd.PersistentFlags().StringVar(&rootOutput.LogFormat, "log-format", "", "Set the internal log format for --log-file (text, json, logfmt)")
	cmd.PersistentFlags().StringVar(&rootOutput.LogFile, "log-file", "", "Write internal logs to this path ('-' writes to stderr)")
	mustRegisterFlagCompletion(cmd, "stack", completeConfiguredStackNames)

	startCmd := newStartCmd()
	startCmd.GroupID = commandGroupLifecycle
	stopCmd := newStopCmd()
	stopCmd.GroupID = commandGroupLifecycle
	restartCmd := newRestartCmd()
	restartCmd.GroupID = commandGroupLifecycle
	resetCmd := newResetCmd()
	resetCmd.GroupID = commandGroupLifecycle

	tuiCmd := newTUICmd(app)
	tuiCmd.GroupID = commandGroupInspect
	statusCmd := newStatusCmd()
	statusCmd.GroupID = commandGroupInspect
	servicesCmd := newServicesCmd()
	servicesCmd.GroupID = commandGroupInspect
	portsCmd := newPortsCmd()
	portsCmd.GroupID = commandGroupInspect
	logsCmd := newLogsCmd()
	logsCmd.GroupID = commandGroupInspect
	openCmd := newOpenCmd()
	openCmd.GroupID = commandGroupInspect
	healthCmd := newHealthCmd()
	healthCmd.GroupID = commandGroupInspect
	connectCmd := newConnectCmd()
	connectCmd.GroupID = commandGroupInspect
	envCmd := newEnvCmd()
	envCmd.GroupID = commandGroupInspect
	doctorCmd := newDoctorCmd()
	doctorCmd.GroupID = commandGroupInspect

	dbCmd := newDBCmd()
	dbCmd.GroupID = commandGroupOperate
	execCmd := newExecCmd()
	execCmd.GroupID = commandGroupOperate
	runCmd := newRunCmd()
	runCmd.GroupID = commandGroupOperate
	snapshotCmd := newSnapshotCmd()
	snapshotCmd.GroupID = commandGroupOperate

	setupCmd := newSetupCmd()
	setupCmd.GroupID = commandGroupConfig
	configCmd := newConfigCmd()
	configCmd.GroupID = commandGroupConfig
	stackCmd := newStackCmd()
	stackCmd.GroupID = commandGroupConfig
	factoryResetCmd := newFactoryResetCmd()
	factoryResetCmd.GroupID = commandGroupConfig

	versionCmd := newVersionCmd(app)
	versionCmd.GroupID = commandGroupUtility

	cmd.AddCommand(startCmd)
	cmd.AddCommand(stopCmd)
	cmd.AddCommand(restartCmd)
	cmd.AddCommand(resetCmd)
	cmd.AddCommand(tuiCmd)
	cmd.AddCommand(statusCmd)
	cmd.AddCommand(servicesCmd)
	cmd.AddCommand(portsCmd)
	cmd.AddCommand(dbCmd)
	cmd.AddCommand(execCmd)
	cmd.AddCommand(runCmd)
	cmd.AddCommand(snapshotCmd)
	cmd.AddCommand(logsCmd)
	cmd.AddCommand(openCmd)
	cmd.AddCommand(healthCmd)
	cmd.AddCommand(connectCmd)
	cmd.AddCommand(envCmd)
	cmd.AddCommand(factoryResetCmd)
	cmd.AddCommand(configCmd)
	cmd.AddCommand(stackCmd)
	cmd.AddCommand(doctorCmd)
	cmd.AddCommand(setupCmd)
	cmd.AddCommand(versionCmd)
	cmd.InitDefaultCompletionCmd()
	cmd.SetVersionTemplate(versionTemplate(app))

	return cmd
}

func versionTemplate(app *App) string {
	return fmt.Sprintf("stackctl %s\n", app.Version)
}

func applyRootEnvOverrides(cmd *cobra.Command) error {
	if err := applyBoolEnvOverride(cmd, "accessible", "ACCESSIBLE"); err != nil {
		return err
	}
	if err := applyBoolEnvOverride(cmd, "plain", "STACKCTL_WIZARD_PLAIN"); err != nil {
		return err
	}
	if err := applyStringEnvOverride(cmd, "log-level", logging.EnvLogLevel); err != nil {
		return err
	}
	if err := applyStringEnvOverride(cmd, "log-format", logging.EnvLogFormat); err != nil {
		return err
	}
	if err := applyStringEnvOverride(cmd, "log-file", logging.EnvLogFile); err != nil {
		return err
	}
	return nil
}

func applyBoolEnvOverride(cmd *cobra.Command, flagName, envVar string) error {
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil || !flag.Changed {
		return nil
	}
	value, err := cmd.Flags().GetBool(flagName)
	if err != nil {
		return err
	}
	if value {
		return os.Setenv(envVar, "1")
	}
	return os.Unsetenv(envVar)
}

func applyStringEnvOverride(cmd *cobra.Command, flagName, envVar string) error {
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil || !flag.Changed {
		return nil
	}
	value, err := cmd.Flags().GetString(flagName)
	if err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return os.Unsetenv(envVar)
	}
	return os.Setenv(envVar, value)
}
