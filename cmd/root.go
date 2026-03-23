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
		Version: "0.13.0",
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
			"  stackctl tui\n" +
			"  stackctl services\n" +
			"  stackctl exec postgres -- psql -U app -d app",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       app.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			selected := strings.TrimSpace(stackName)
			if selected == "" {
				selected = configpkg.SelectedStackName()
			}
			if err := configpkg.ValidateStackName(selected); err != nil {
				return err
			}
			return os.Setenv(configpkg.StackNameEnvVar, selected)
		},
	}

	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.PersistentFlags().StringVar(&stackName, "stack", "", "Select a named stack config (or use STACKCTL_STACK)")
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newRestartCmd())
	cmd.AddCommand(newTUICmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newServicesCmd())
	cmd.AddCommand(newPortsCmd())
	cmd.AddCommand(newDBCmd())
	cmd.AddCommand(newExecCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newOpenCmd())
	cmd.AddCommand(newHealthCmd())
	cmd.AddCommand(newConnectCmd())
	cmd.AddCommand(newResetCmd())
	cmd.AddCommand(newFactoryResetCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newSetupCmd())
	cmd.AddCommand(newVersionCmd(app))
	cmd.SetVersionTemplate(versionTemplate(app))

	return cmd
}

func versionTemplate(app *App) string {
	return fmt.Sprintf("stackctl %s\n", app.Version)
}
