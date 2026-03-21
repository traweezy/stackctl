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
