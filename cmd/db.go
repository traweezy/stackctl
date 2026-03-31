package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "db",
		Short:             "Run database-focused helpers",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}

	cmd.AddGroup(dbCommandGroups()...)
	cmd.SetHelpCommandGroupID(dbGroupAccess)

	shellCmd := noFileCompletion(newDBShellCmd())
	shellCmd.GroupID = dbGroupAccess
	dumpCmd := newDBDumpCmd()
	dumpCmd.GroupID = dbGroupBackupRestore
	restoreCmd := newDBRestoreCmd()
	restoreCmd.GroupID = dbGroupBackupRestore
	resetCmd := noArgsCommand(newDBResetCmd())
	resetCmd.GroupID = dbGroupMaintain

	cmd.AddCommand(shellCmd)
	cmd.AddCommand(dumpCmd)
	cmd.AddCommand(restoreCmd)
	cmd.AddCommand(resetCmd)

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
			if err := ensureServiceEnabled(cfg, "postgres"); err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}
			if err := verboseComposeFile(cmd, cfg); err != nil {
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

func newDBDumpCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "dump [output-path]",
		Short: "Dump the configured Postgres database as SQL",
		Example: "  stackctl db dump\n" +
			"  stackctl db dump dump.sql\n" +
			"  stackctl db dump --output dump.sql",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if outputPath != "" {
					return errors.New("specify the dump output with either a positional path or --output, not both")
				}
				outputPath = args[0]
			}

			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureServiceEnabled(cfg, "postgres"); err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}
			if err := verboseComposeFile(cmd, cfg); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			runner := runnerFor(cmd)
			if outputPath != "" {
				// #nosec G304 -- dump output paths are explicit user-selected CLI destinations.
				file, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("create dump file %s: %w", outputPath, err)
				}
				defer func() { _ = file.Close() }()
				runner.Stdout = file
			}

			if err := deps.composeExec(
				ctx,
				runner,
				cfg,
				"postgres",
				postgresPasswordEnv(cfg),
				append(
					[]string{"pg_dump"},
					append(postgresConnArgs(cfg, cfg.Connection.PostgresDatabase), "--format=plain", "--no-owner", "--no-privileges")...,
				),
				false,
			); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			if outputPath == "" {
				return nil
			}

			return statusLine(cmd, output.StatusOK, fmt.Sprintf("wrote database dump to %s", outputPath))
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write the SQL dump to a file instead of stdout")

	return cmd
}

func newDBRestoreCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restore <input-path|->",
		Short: "Restore the configured Postgres database from a SQL dump",
		Example: "  stackctl db restore dump.sql --force\n" +
			"  stackctl db restore - --force < dump.sql",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureServiceEnabled(cfg, "postgres"); err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}
			if err := verboseComposeFile(cmd, cfg); err != nil {
				return err
			}
			if !force {
				ok, err := confirmWithPrompt(
					cmd,
					fmt.Sprintf("This will apply a SQL dump to %s. Continue?", cfg.Connection.PostgresDatabase),
					false,
				)
				if err != nil {
					return fmt.Errorf("database restore confirmation required; rerun with --force")
				}
				if !ok {
					return userCancelled(cmd, "database restore cancelled")
				}
			}

			inputPath := args[0]
			source := inputPath
			input := cmd.InOrStdin()
			if inputPath != "-" {
				// #nosec G304 -- restore input paths are explicit user-selected CLI sources.
				file, err := os.Open(inputPath)
				if err != nil {
					return fmt.Errorf("open dump file %s: %w", inputPath, err)
				}
				defer func() { _ = file.Close() }()
				input = file
			} else {
				source = "stdin"
			}

			if err := statusLine(cmd, output.StatusInfo, fmt.Sprintf("restoring database from %s...", source)); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			runner := runnerFor(cmd)
			runner.Stdin = input
			if err := deps.composeExec(
				ctx,
				runner,
				cfg,
				"postgres",
				postgresPasswordEnv(cfg),
				append(
					[]string{"psql"},
					append(postgresConnArgs(cfg, cfg.Connection.PostgresDatabase), "-v", "ON_ERROR_STOP=1", "-f", "-")...,
				),
				false,
			); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			return statusLine(cmd, output.StatusOK, "database restore completed")
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation before restoring a database dump")

	return cmd
}

func newDBResetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Drop and recreate the configured Postgres database",
		Example: "  stackctl db reset --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}
			if err := ensureServiceEnabled(cfg, "postgres"); err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}
			if err := verboseComposeFile(cmd, cfg); err != nil {
				return err
			}
			if cfg.Connection.PostgresDatabase == "postgres" {
				return errors.New("stackctl db reset does not support resetting the postgres maintenance database")
			}
			if !force {
				ok, err := confirmWithPrompt(
					cmd,
					fmt.Sprintf("This will drop and recreate %s. Continue?", cfg.Connection.PostgresDatabase),
					false,
				)
				if err != nil {
					return fmt.Errorf("database reset confirmation required; rerun with --force")
				}
				if !ok {
					return userCancelled(cmd, "database reset cancelled")
				}
			}

			if err := statusLine(cmd, output.StatusReset, fmt.Sprintf("resetting database %s...", cfg.Connection.PostgresDatabase)); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := deps.composeExec(
				ctx,
				runnerFor(cmd),
				cfg,
				"postgres",
				postgresPasswordEnv(cfg),
				append(
					[]string{"psql"},
					append(
						postgresConnArgs(cfg, cfg.Services.Postgres.MaintenanceDatabase),
						"-v", "ON_ERROR_STOP=1",
						"-c", fmt.Sprintf(
							"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid()",
							sqlStringLiteral(cfg.Connection.PostgresDatabase),
						),
					)...,
				),
				false,
			); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			for _, sql := range []string{
				fmt.Sprintf("DROP DATABASE IF EXISTS %s", sqlIdentifier(cfg.Connection.PostgresDatabase)),
				fmt.Sprintf("CREATE DATABASE %s", sqlIdentifier(cfg.Connection.PostgresDatabase)),
			} {
				if err := deps.composeExec(
					ctx,
					runnerFor(cmd),
					cfg,
					"postgres",
					postgresPasswordEnv(cfg),
					append(
						[]string{"psql"},
						append(
							postgresConnArgs(cfg, cfg.Services.Postgres.MaintenanceDatabase),
							"-v", "ON_ERROR_STOP=1",
							"-c", sql,
						)...,
					),
					false,
				); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
			}

			return statusLine(cmd, output.StatusOK, fmt.Sprintf("database %s reset", cfg.Connection.PostgresDatabase))
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation before dropping and recreating the database")

	return cmd
}

func postgresPasswordEnv(cfg configpkg.Config) []string {
	return []string{"PGPASSWORD=" + cfg.Connection.PostgresPassword}
}

func postgresConnArgs(cfg configpkg.Config, database string) []string {
	return []string{
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(5432),
		"-U", cfg.Connection.PostgresUsername,
		"-d", database,
	}
}

func sqlIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func sqlStringLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}
