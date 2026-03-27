package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func withTestDeps(t *testing.T, mutate func(*commandDeps)) {
	t.Helper()

	original := deps
	testDeps := defaultCommandDeps()
	testDeps.stdin = bytes.NewBuffer(nil)
	testDeps.isTerminal = func() bool { return false }
	testDeps.configDirPath = func() (string, error) { return "/tmp/stackctl", nil }
	testDeps.configFilePath = func() (string, error) { return "/tmp/stackctl/config.yaml", nil }
	testDeps.configFilePathForStack = func(name string) (string, error) {
		if name == configpkg.DefaultStackName {
			return "/tmp/stackctl/config.yaml", nil
		}
		return "/tmp/stackctl/stacks/" + name + ".yaml", nil
	}
	testDeps.knownConfigPaths = func() ([]string, error) { return []string{"/tmp/stackctl/config.yaml"}, nil }
	testDeps.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
	testDeps.currentStackName = func() (string, error) { return configpkg.DefaultStackName, nil }
	testDeps.setCurrentStackName = func(string) error { return nil }
	testDeps.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
	testDeps.saveConfig = func(string, configpkg.Config) error { return nil }
	testDeps.removeAll = func(string) error { return nil }
	testDeps.mkdirAll = func(string, os.FileMode) error { return nil }
	testDeps.rename = func(string, string) error { return nil }
	testDeps.marshalConfig = func(configpkg.Config) ([]byte, error) { return []byte("test: true\n"), nil }
	testDeps.defaultConfig = func() configpkg.Config { return configpkg.DefaultForStack(configpkg.DefaultStackName) }
	testDeps.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue { return nil }
	testDeps.runWizard = func(_ io.Reader, _ io.Writer, cfg configpkg.Config) (configpkg.Config, error) { return cfg, nil }
	testDeps.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
	testDeps.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, nil }
	testDeps.scaffoldManagedStack = func(cfg configpkg.Config, _ bool) (configpkg.ScaffoldResult, error) {
		return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg)}, nil
	}
	testDeps.composePath = func(configpkg.Config) string { return "/tmp/stackctl/compose.yaml" }
	testDeps.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "compose.yaml"}, nil }
	testDeps.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
	testDeps.commandExists = func(string) bool { return true }
	testDeps.podmanComposeAvail = func(context.Context) bool { return true }
	testDeps.openURL = func(context.Context, system.Runner, string) error { return nil }
	testDeps.copyToClipboard = func(context.Context, system.Runner, string) error { return nil }
	testDeps.installPackages = func(context.Context, system.Runner, string, []string) ([]string, error) { return nil, nil }
	testDeps.enableCockpit = func(context.Context, system.Runner) error { return nil }
	testDeps.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	testDeps.portListening = func(int) bool { return true }
	testDeps.portInUse = func(int) (bool, error) { return false, nil }
	testDeps.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
		return system.CommandResult{Stdout: "[]"}, nil
	}
	testDeps.anyContainerExists = func(context.Context, []string) (bool, error) { return false, nil }
	testDeps.cockpitStatus = func(context.Context) system.CockpitState { return system.CockpitState{} }
	testDeps.openCommandName = func() string { return "xdg-open" }
	testDeps.composeUp = func(context.Context, system.Runner, configpkg.Config) error { return nil }
	testDeps.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error { return nil }
	testDeps.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error { return nil }
	testDeps.composeStopServices = func(context.Context, system.Runner, configpkg.Config, []string) error { return nil }
	testDeps.composeLogs = func(context.Context, system.Runner, configpkg.Config, int, bool, string, string) error {
		return nil
	}
	testDeps.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
		return nil
	}
	testDeps.composeDownPath = func(context.Context, system.Runner, string, string, bool) error { return nil }
	testDeps.containerLogs = func(context.Context, system.Runner, string, int, bool, string) error { return nil }

	if mutate != nil {
		mutate(&testDeps)
	}

	deps = testDeps
	t.Cleanup(func() {
		deps = original
	})
}

func executeRoot(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	return executeAppRoot(t, NewApp(), args...)
}

func executeAppRoot(t *testing.T, app *App, args ...string) (string, string, error) {
	t.Helper()

	root := NewRootCmd(app)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	originalStack, hadStack := os.LookupEnv(configpkg.StackNameEnvVar)
	t.Cleanup(func() {
		if hadStack {
			_ = os.Setenv(configpkg.StackNameEnvVar, originalStack)
			return
		}
		_ = os.Unsetenv(configpkg.StackNameEnvVar)
	})

	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func newReport(checks ...doctorpkg.Check) doctorpkg.Report {
	report := doctorpkg.Report{Checks: make([]doctorpkg.Check, 0, len(checks))}
	for _, check := range checks {
		report.Checks = append(report.Checks, check)
		switch check.Status {
		case output.StatusOK:
			report.OKCount++
		case output.StatusWarn:
			report.WarnCount++
		case output.StatusMiss:
			report.MissCount++
		case output.StatusFail:
			report.FailCount++
		}
	}

	return report
}

func marshalContainersJSON(containers ...system.Container) string {
	data, err := json.Marshal(containers)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func runningContainerJSON(cfg configpkg.Config, services ...string) string {
	definitions := selectedStackServiceDefinitions(cfg, services)
	containers := make([]system.Container, 0, len(definitions))
	for _, definition := range definitions {
		if definition.ContainerName == nil || definition.PrimaryPort == nil {
			continue
		}
		containers = append(containers, system.Container{
			ID:     definition.Key + "123456",
			Image:  definition.Key + ":latest",
			Names:  []string{definition.ContainerName(cfg)},
			Status: "Up 5 minutes",
			State:  "running",
			Ports: []system.ContainerPort{
				{
					HostPort:      definition.PrimaryPort(cfg),
					ContainerPort: definition.DefaultInternalPort,
					Protocol:      "tcp",
				},
			},
			CreatedAt: "now",
		})
	}

	return marshalContainersJSON(containers...)
}

type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }
