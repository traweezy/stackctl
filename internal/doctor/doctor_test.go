package doctor

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWithDepsReportsMissingEnvironment(t *testing.T) {
	report, err := runWithDeps(context.Background(), dependencies{
		configFilePath:     func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
		loadConfig:         func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound },
		validateConfig:     func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		composePath:        func(configpkg.Config) string { return "/tmp/compose.yaml" },
		stat:               func(string) (os.FileInfo, error) { return nil, errors.New("missing") },
		commandExists:      func(string) bool { return false },
		podmanComposeAvail: func(context.Context) bool { return false },
		openCommandName:    func() string { return "" },
		cockpitStatus:      func(context.Context) system.CockpitState { return system.CockpitState{State: "not installed"} },
		portInUse:          func(int) (bool, error) { return false, nil },
		anyContainerExists: func(context.Context, []string) (bool, error) { return false, nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if report.MissCount == 0 {
		t.Fatalf("expected misses, got %+v", report)
	}
	if report.Checks[0].Status != output.StatusMiss {
		t.Fatalf("expected missing config check, got %+v", report.Checks[0])
	}
}

func TestRunWithDepsReportsHealthyConfig(t *testing.T) {
	cfg := configpkg.Default()

	report, err := runWithDeps(context.Background(), dependencies{
		configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
		loadConfig:     func(string) (configpkg.Config, error) { return cfg, nil },
		validateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		composePath:    func(configpkg.Config) string { return "/tmp/compose.yaml" },
		stat: func(path string) (os.FileInfo, error) {
			switch path {
			case cfg.Stack.Dir:
				return fakeFileInfo{dir: true}, nil
			case "/tmp/compose.yaml":
				return fakeFileInfo{name: "compose.yaml"}, nil
			default:
				return nil, errors.New("missing")
			}
		},
		commandExists:      func(string) bool { return true },
		podmanComposeAvail: func(context.Context) bool { return true },
		openCommandName:    func() string { return "xdg-open" },
		cockpitStatus: func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		},
		portInUse:          func(int) (bool, error) { return false, nil },
		anyContainerExists: func(context.Context, []string) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if report.OKCount == 0 {
		t.Fatalf("expected healthy checks, got %+v", report)
	}
	if report.HasFailures() {
		t.Fatalf("expected no failures, got %+v", report)
	}
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
