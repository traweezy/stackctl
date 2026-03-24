package cmd

import (
	"context"
	"errors"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var noTTY bool

	cmd := &cobra.Command{
		Use:   "exec <service> -- <command...>",
		Short: "Run a command inside a stack service",
		Long: "Run a command inside a configured service container. " +
			"Use -- before the target command so stackctl stops parsing flags.",
		Example: "  stackctl exec postgres -- psql -U app -d app\n" +
			"  stackctl exec redis -- redis-cli -a secret PING\n" +
			"  stackctl exec pgadmin -- printenv PGADMIN_DEFAULT_EMAIL",
		ValidArgsFunction: completeExecArgs,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("usage: stackctl exec <service> -- <command...>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			serviceName, err := canonicalServiceName(args[0])
			if err != nil {
				return err
			}
			if err := ensureServiceEnabled(cfg, serviceName); err != nil {
				return err
			}
			if err := ensureComposeRuntime(cmd, cfg); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			err = deps.composeExec(ctx, runnerFor(cmd), cfg, serviceName, nil, args[1:], deps.isTerminal() && !noTTY)
			if ctx.Err() != nil {
				return nil
			}
			return err
		},
	}

	cmd.Flags().BoolVar(&noTTY, "no-tty", false, "Disable TTY allocation for the exec session")

	return cmd
}
