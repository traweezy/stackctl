package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

type App struct {
	Version   string
	GitCommit string
	BuildDate string
}

func NewApp() *App {
	return &App{
		Version: "0.14.0",
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
	mustRegisterFlagCompletion(cmd, "stack", completeConfiguredStackNames)

	startCmd := newStartCmd()
	startCmd.GroupID = commandGroupLifecycle
	stopCmd := newStopCmd()
	stopCmd.GroupID = commandGroupLifecycle
	restartCmd := newRestartCmd()
	restartCmd.GroupID = commandGroupLifecycle
	resetCmd := newResetCmd()
	resetCmd.GroupID = commandGroupLifecycle

	tuiCmd := newTUICmd()
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
	doctorCmd := newDoctorCmd()
	doctorCmd.GroupID = commandGroupInspect

	dbCmd := newDBCmd()
	dbCmd.GroupID = commandGroupOperate
	execCmd := newExecCmd()
	execCmd.GroupID = commandGroupOperate

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
	cmd.AddCommand(logsCmd)
	cmd.AddCommand(openCmd)
	cmd.AddCommand(healthCmd)
	cmd.AddCommand(connectCmd)
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
