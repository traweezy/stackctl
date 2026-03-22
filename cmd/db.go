package cmd

import (
	"context"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Run database-focused helpers",
	}

	cmd.AddCommand(newDBShellCmd())

	return cmd
}

func newDBShellCmd() *cobra.Command {
	var noTTY bool

	cmd := &cobra.Command{
		Use:   "shell [-- <psql args...>]",
		Short: "Open psql against the configured Postgres database",
		Example: "  stackctl db shell\n" +
			"  stackctl db shell -- -c \"select version()\"\n" +
			"  stackctl db shell -- -tAc \"select current_user\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			commandArgs := []string{
				"psql",
				"-h", "127.0.0.1",
				"-p", strconv.Itoa(5432),
				"-U", cfg.Connection.PostgresUsername,
				"-d", cfg.Connection.PostgresDatabase,
			}
			commandArgs = append(commandArgs, args...)

			err = deps.composeExec(
				ctx,
				runnerFor(cmd),
				cfg,
				"postgres",
				[]string{"PGPASSWORD=" + cfg.Connection.PostgresPassword},
				commandArgs,
				deps.isTerminal() && !noTTY,
			)
			if ctx.Err() != nil {
				return nil
			}
			return err
		},
	}

	cmd.Flags().BoolVar(&noTTY, "no-tty", false, "Disable TTY allocation for the psql session")

	return cmd
}
