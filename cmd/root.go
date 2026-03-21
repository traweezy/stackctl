package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type App struct {
	Version   string
	GitCommit string
	BuildDate string
}

func NewApp() *App {
	return &App{
		Version: "0.1.0",
	}
}

func NewRootCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "stackctl",
		Short:         "Manage a local Podman development stack",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       app.Version,
	}

	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newRestartCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newOpenCmd())
	cmd.AddCommand(newHealthCmd())
	cmd.AddCommand(newConnectCmd())
	cmd.AddCommand(newResetCmd())
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
