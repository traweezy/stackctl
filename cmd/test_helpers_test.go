package cmd

import (
	"bytes"
	"context"
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
	testDeps.configFilePath = func() (string, error) { return "/tmp/stackctl/config.yaml", nil }
	testDeps.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
	testDeps.saveConfig = func(string, configpkg.Config) error { return nil }
	testDeps.marshalConfig = func(configpkg.Config) ([]byte, error) { return []byte("test: true\n"), nil }
	testDeps.defaultConfig = configpkg.Default
	testDeps.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue { return nil }
	testDeps.runWizard = func(_ io.Reader, _ io.Writer, cfg configpkg.Config) (configpkg.Config, error) { return cfg, nil }
	testDeps.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
	testDeps.composePath = func(configpkg.Config) string { return "/tmp/stackctl/compose.yaml" }
	testDeps.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "compose.yaml"}, nil }
	testDeps.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
	testDeps.commandExists = func(string) bool { return true }
	testDeps.podmanComposeAvail = func(context.Context) bool { return true }
	testDeps.openURL = func(context.Context, system.Runner, string) error { return nil }
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
	testDeps.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error { return nil }
	testDeps.composeLogs = func(context.Context, system.Runner, configpkg.Config, int, bool, string) error { return nil }
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
