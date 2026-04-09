package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

var runNotifyContext = signal.NotifyContext
var runSelectedStackServiceDefinitions = selectedStackServiceDefinitions

func newRunCmd() *cobra.Command {
	var noStart bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "run [service...] -- <command...>",
		Short: "Run a host command with stack environment variables",
		Long: "Run a host command with the same app-ready environment variables exposed by\n" +
			"`stackctl env`, optionally ensuring the selected services are running first.",
		Example: "  stackctl run -- go test ./...\n" +
			"  stackctl run postgres redis -- air\n" +
			"  stackctl run --dry-run -- npm run dev",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeRunArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceArgs, commandArgs, err := parseRunInvocation(cmd, args)
			if err != nil {
				return err
			}

			cfg, err := loadRuntimeConfig(cmd, true)
			if err != nil {
				return err
			}

			selectedServices, err := resolveRunTargetServices(cfg, serviceArgs)
			if err != nil {
				return err
			}

			groups, err := envGroups(cfg, selectedServices)
			if err != nil {
				return err
			}
			env := flattenEnvGroups(groups)

			if dryRun {
				return printRunDryRun(cmd, cfg, selectedServices, commandArgs, groups, noStart)
			}

			target := lifecycleTargetLabel(selectedServices)
			if noStart {
				if err := ensureSelectedRunServicesReady(context.Background(), cfg, selectedServices); err != nil {
					return err
				}
				if err := verboseLine(cmd, fmt.Sprintf("Using running %s for %s", strings.ToLower(target), formatShellCommand(commandArgs))); err != nil {
					return err
				}
			} else {
				if err := syncManagedScaffoldIfNeeded(cmd, cfg); err != nil {
					return err
				}
				if err := ensureComposeRuntime(cmd, cfg); err != nil {
					return err
				}
				if err := verboseComposeFile(cmd, cfg); err != nil {
					return err
				}
				if err := ensureNoOtherRunningStack(context.Background()); err != nil {
					return err
				}
				if err := ensureSelectedServicePortsAvailable(context.Background(), cfg, selectedServices); err != nil {
					return err
				}
				if err := statusLine(cmd, output.StatusStart, fmt.Sprintf("starting %s for %s...", strings.ToLower(target), formatShellCommand(commandArgs))); err != nil {
					return err
				}
				if err := startRunServices(cmd, cfg, selectedServices); err != nil {
					return err
				}

				waitCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Behavior.StartupTimeoutSec)*time.Second)
				defer cancel()
				if err := waitForRunServices(waitCtx, cfg, selectedServices); err != nil {
					return err
				}
			}

			ctx, stop := runNotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			runner := runnerFor(cmd)
			runner.Env = append(runner.Env, mapEnvToAssignments(env)...)
			err = deps.runExternalCommand(ctx, runner, "", commandArgs)
			if ctx.Err() != nil {
				return nil
			}
			return err
		},
	}

	cmd.Flags().BoolVar(&noStart, "no-start", false, "Fail instead of starting services when they are not already ready")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the selected services, command, and injected environment without running anything")

	return cmd
}

func parseRunInvocation(cmd *cobra.Command, args []string) ([]string, []string, error) {
	dash := cmd.ArgsLenAtDash()
	if dash < 0 || dash >= len(args) {
		return nil, nil, errors.New("usage: stackctl run [service...] -- <command...>")
	}

	serviceArgs := append([]string(nil), args[:dash]...)
	commandArgs := append([]string(nil), args[dash:]...)

	return serviceArgs, commandArgs, nil
}

func resolveRunTargetServices(cfg configpkg.Config, args []string) ([]string, error) {
	services, err := resolveTargetStackServices(cfg, args)
	if err != nil {
		return nil, err
	}
	if len(services) > 0 {
		return services, nil
	}
	return enabledStackServiceKeys(cfg), nil
}

func enabledStackServiceKeys(cfg configpkg.Config) []string {
	definitions := enabledStackServiceDefinitions(cfg)
	keys := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		keys = append(keys, definition.Key)
	}
	return keys
}

func startRunServices(cmd *cobra.Command, cfg configpkg.Config, services []string) error {
	switch {
	case len(services) == 0, len(services) == len(enabledStackServiceDefinitions(cfg)):
		return deps.composeUp(context.Background(), runnerFor(cmd), cfg)
	default:
		return deps.composeUpServices(context.Background(), runnerFor(cmd), cfg, false, services)
	}
}

func waitForRunServices(ctx context.Context, cfg configpkg.Config, services []string) error {
	for _, definition := range runSelectedStackServiceDefinitions(cfg, services) {
		if definition.PrimaryPort == nil {
			continue
		}
		if err := waitForStackService(ctx, cfg, definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureSelectedRunServicesReady(ctx context.Context, cfg configpkg.Config, services []string) error {
	states, err := stackServiceRuntimeStates(ctx, cfg, selectedStackServiceDefinitions(cfg, services))
	if err != nil {
		return err
	}
	for _, state := range states {
		if failure := serviceStartFailure(state); failure != nil {
			return failure
		}
		if !state.ContainerRunning || !state.PortBound || !state.PortState.Listening {
			return fmt.Errorf("%s is not ready: %s", state.Definition.Key, servicePendingReason(state))
		}
	}
	return nil
}

func printRunDryRun(cmd *cobra.Command, cfg configpkg.Config, services []string, commandArgs []string, groups []envGroup, noStart bool) error {
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Stack: %s\n", cfg.Stack.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Services: %s\n", strings.Join(displayServiceNames(services), ", ")); err != nil {
		return err
	}
	startMode := "ensure running"
	if noStart {
		startMode = "require already running"
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Service mode: %s\n", startMode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Command: %s\n\n", formatShellCommand(commandArgs)); err != nil {
		return err
	}
	return writeEnvGroups(cmd.OutOrStdout(), groups, true)
}

func formatShellCommand(commandArgs []string) string {
	quoted := make([]string, 0, len(commandArgs))
	for _, value := range commandArgs {
		quoted = append(quoted, quoteShellValue(value))
	}
	return strings.Join(quoted, " ")
}

func mapEnvToAssignments(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	assignments := make([]string, 0, len(values))
	for _, key := range keys {
		value := values[key]
		assignments = append(assignments, key+"="+value)
	}
	return assignments
}
