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
	stdin              io.Reader
	isTerminal         func() bool
	configFilePath     func() (string, error)
	loadConfig         func(string) (configpkg.Config, error)
	saveConfig         func(string, configpkg.Config) error
	removeFile         func(string) error
	marshalConfig      func(configpkg.Config) ([]byte, error)
	defaultConfig      func() configpkg.Config
	validateConfig     func(configpkg.Config) []configpkg.ValidationIssue
	runWizard          func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error)
	promptYesNo        func(io.Reader, io.Writer, string, bool) (bool, error)
	composePath        func(configpkg.Config) string
	stat               func(string) (os.FileInfo, error)
	runDoctor          func(context.Context) (doctorpkg.Report, error)
	commandExists      func(string) bool
	podmanComposeAvail func(context.Context) bool
	openURL            func(context.Context, system.Runner, string) error
	installPackages    func(context.Context, system.Runner, string, []string) ([]string, error)
	enableCockpit      func(context.Context, system.Runner) error
	waitForPort        func(context.Context, int, time.Duration) error
	portListening      func(int) bool
	portInUse          func(int) (bool, error)
	captureResult      func(context.Context, string, string, ...string) (system.CommandResult, error)
	anyContainerExists func(context.Context, []string) (bool, error)
	cockpitStatus      func(context.Context) system.CockpitState
	openCommandName    func() string
	composeUp          func(context.Context, system.Runner, configpkg.Config) error
	composeDown        func(context.Context, system.Runner, configpkg.Config, bool) error
	composeLogs        func(context.Context, system.Runner, configpkg.Config, int, bool, string) error
	containerLogs      func(context.Context, system.Runner, string, int, bool, string) error
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		stdin:              os.Stdin,
		isTerminal:         defaultTerminalInteractive,
		configFilePath:     configpkg.ConfigFilePath,
		loadConfig:         configpkg.Load,
		saveConfig:         configpkg.Save,
		removeFile:         os.Remove,
		marshalConfig:      configpkg.Marshal,
		defaultConfig:      configpkg.Default,
		validateConfig:     configpkg.Validate,
		runWizard:          configpkg.RunWizard,
		promptYesNo:        configpkg.PromptYesNo,
		composePath:        configpkg.ComposePath,
		stat:               os.Stat,
		runDoctor:          doctorpkg.Run,
		commandExists:      system.CommandExists,
		podmanComposeAvail: system.PodmanComposeAvailable,
		openURL:            system.OpenURL,
		installPackages:    system.InstallPackages,
		enableCockpit:      system.EnableCockpit,
		waitForPort:        system.WaitForPort,
		portListening:      system.PortListening,
		portInUse:          system.PortInUse,
		captureResult:      system.CaptureResult,
		anyContainerExists: system.AnyContainerExists,
		cockpitStatus:      system.CockpitStatus,
		openCommandName:    system.OpenCommandName,
		composeUp: func(ctx context.Context, runner system.Runner, cfg configpkg.Config) error {
			return compose.Client{Runner: runner}.Up(ctx, cfg)
		},
		composeDown: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, removeVolumes bool) error {
			return compose.Client{Runner: runner}.Down(ctx, cfg, removeVolumes)
		},
		composeLogs: func(ctx context.Context, runner system.Runner, cfg configpkg.Config, tail int, follow bool, since string) error {
			return compose.Client{Runner: runner}.Logs(ctx, cfg, tail, follow, since)
		},
		containerLogs: func(ctx context.Context, runner system.Runner, containerName string, tail int, follow bool, since string) error {
			return compose.Client{Runner: runner}.ContainerLogs(ctx, containerName, tail, follow, since)
		},
	}
}

var deps = defaultCommandDeps()
