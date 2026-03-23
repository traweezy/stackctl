package cmd

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/traweezy/stackctl/internal/compose"
	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/system"
)

type commandDeps struct {
	stdin                     io.Reader
	isTerminal                func() bool
	configDirPath             func() (string, error)
	configFilePath            func() (string, error)
	knownConfigPaths          func() ([]string, error)
	dataDirPath               func() (string, error)
	loadConfig                func(string) (configpkg.Config, error)
	saveConfig                func(string, configpkg.Config) error
	removeFile                func(string) error
	removeAll                 func(string) error
	marshalConfig             func(configpkg.Config) ([]byte, error)
	defaultConfig             func() configpkg.Config
	validateConfig            func(configpkg.Config) []configpkg.ValidationIssue
	runWizard                 func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error)
	promptYesNo               func(io.Reader, io.Writer, string, bool) (bool, error)
	managedStackNeedsScaffold func(configpkg.Config) (bool, error)
	scaffoldManagedStack      func(configpkg.Config, bool) (configpkg.ScaffoldResult, error)
	composePath               func(configpkg.Config) string
	stat                      func(string) (os.FileInfo, error)
	runDoctor                 func(context.Context) (doctorpkg.Report, error)
	commandExists             func(string) bool
	podmanComposeAvail        func(context.Context) bool
	openURL                   func(context.Context, system.Runner, string) error
	copyToClipboard           func(context.Context, system.Runner, string) error
	installPackages           func(context.Context, system.Runner, string, []string) ([]string, error)
	enableCockpit             func(context.Context, system.Runner) error
	waitForPort               func(context.Context, int, time.Duration) error
	portListening             func(int) bool
	portInUse                 func(int) (bool, error)
	captureResult             func(context.Context, string, string, ...string) (system.CommandResult, error)
	anyContainerExists        func(context.Context, []string) (bool, error)
	cockpitStatus             func(context.Context) system.CockpitState
	openCommandName           func() string
	composeUp                 func(context.Context, system.Runner, configpkg.Config) error
	composeUpServices         func(context.Context, system.Runner, configpkg.Config, bool, []string) error
	composeDown               func(context.Context, system.Runner, configpkg.Config, bool) error
	composeStopServices       func(context.Context, system.Runner, configpkg.Config, []string) error
	composeLogs               func(context.Context, system.Runner, configpkg.Config, int, bool, string, string) error
	composeExec               func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error
	composeDownPath           func(context.Context, system.Runner, string, string, bool) error
	containerLogs             func(context.Context, system.Runner, string, int, bool, string) error
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		stdin:                     os.Stdin,
		isTerminal:                defaultTerminalInteractive,
		configDirPath:             configpkg.ConfigDirPath,
		configFilePath:            configpkg.ConfigFilePath,
		knownConfigPaths:          configpkg.KnownConfigPaths,
		dataDirPath:               configpkg.DataDirPath,
		loadConfig:                configpkg.Load,
		saveConfig:                configpkg.Save,
		removeFile:                os.Remove,
		removeAll:                 os.RemoveAll,
		marshalConfig:             configpkg.Marshal,
		defaultConfig: func() configpkg.Config {
			return configpkg.DefaultForStack(configpkg.SelectedStackName())
		},
		validateConfig:            configpkg.Validate,
		runWizard:                 configpkg.RunWizard,
		promptYesNo:               configpkg.PromptYesNo,
		managedStackNeedsScaffold: configpkg.ManagedStackNeedsScaffold,
		scaffoldManagedStack:      configpkg.ScaffoldManagedStack,
		composePath:               configpkg.ComposePath,
		stat:                      os.Stat,
		runDoctor:                 doctorpkg.Run,
		commandExists:             system.CommandExists,
		podmanComposeAvail:        system.PodmanComposeAvailable,
		openURL:                   system.OpenURL,
		copyToClipboard:           system.CopyToClipboard,
		installPackages:           system.InstallPackages,
		enableCockpit:             system.EnableCockpit,
		waitForPort:               system.WaitForPort,
		portListening:             system.PortListening,
		portInUse:                 system.PortInUse,
		captureResult:             system.CaptureResult,
		anyContainerExists:        system.AnyContainerExists,
		cockpitStatus:             system.CockpitStatus,
		openCommandName:           system.OpenCommandName,
		composeUp: func(ctx context.Context, runner system.Runner, cfg configpkg.Config) error {
			return compose.Client{Runner: runner}.Up(ctx, cfg)
		},
		composeUpServices: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, forceRecreate bool, services []string) error {
			return compose.Client{Runner: runner}.UpServices(ctx, cfg, forceRecreate, services...)
		},
		composeDown: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, removeVolumes bool) error {
			return compose.Client{Runner: runner}.Down(ctx, cfg, removeVolumes)
		},
		composeStopServices: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, services []string) error {
			return compose.Client{Runner: runner}.StopServices(ctx, cfg, services...)
		},
		composeLogs: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, tail int, follow bool, since, service string) error {
			return compose.Client{Runner: runner}.Logs(ctx, cfg, tail, follow, since, service)
		},
		composeExec: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, service string, env []string, commandArgs []string, tty bool) error {
			return compose.Client{Runner: runner}.Exec(ctx, cfg, service, env, commandArgs, tty)
		},
		composeDownPath: func(ctx context.Context, runner system.Runner, dir, composePath string, removeVolumes bool) error {
			return compose.Client{Runner: runner}.DownPath(ctx, dir, composePath, removeVolumes)
		},
		containerLogs: func(ctx context.Context, runner system.Runner, containerName string, tail int, follow bool, since string) error {
			return compose.Client{Runner: runner}.ContainerLogs(ctx, containerName, tail, follow, since)
		},
	}
}

var deps = defaultCommandDeps()
