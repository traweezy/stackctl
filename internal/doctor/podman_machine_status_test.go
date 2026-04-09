package doctor

import (
	"context"
	"os"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWithDepsReportsInitializedButStoppedPodmanMachine(t *testing.T) {
	report, err := runWithDeps(context.Background(), dependencies{
		configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
		loadConfig:     func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound },
		validateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		composePath:    configpkg.ComposePath,
		stat:           func(string) (os.FileInfo, error) { return nil, nil },
		platform: func() system.Platform {
			return system.Platform{GOOS: "darwin"}
		},
		commandExists:        func(name string) bool { return name == "podman" },
		podmanVersion:        func(context.Context) (string, error) { return system.SupportedPodmanVersion, nil },
		podmanComposeVersion: func(context.Context) (string, error) { return system.SupportedComposeProviderVersion, nil },
		podmanComposeAvail:   func(context.Context) bool { return true },
		podmanMachineStatus: func(context.Context) system.PodmanMachineState {
			return system.PodmanMachineState{Supported: true, Initialized: true, Running: false}
		},
		openCommandName: func() string { return "open" },
		cockpitStatus: func(context.Context) system.CockpitState {
			return system.CockpitState{}
		},
		portInUse:      func(int) (bool, error) { return false, nil },
		listContainers: func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	found := false
	for _, check := range report.Checks {
		if check.Message == "podman machine initialized" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected initialized podman machine check, got %+v", report.Checks)
	}
}

func TestRunWithDepsReportsRunningPodmanMachine(t *testing.T) {
	report, err := runWithDeps(context.Background(), dependencies{
		configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
		loadConfig:     func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound },
		validateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		composePath:    configpkg.ComposePath,
		stat:           func(string) (os.FileInfo, error) { return nil, nil },
		platform: func() system.Platform {
			return system.Platform{GOOS: "darwin"}
		},
		commandExists:        func(name string) bool { return name == "podman" },
		podmanVersion:        func(context.Context) (string, error) { return system.SupportedPodmanVersion, nil },
		podmanComposeVersion: func(context.Context) (string, error) { return system.SupportedComposeProviderVersion, nil },
		podmanComposeAvail:   func(context.Context) bool { return true },
		podmanMachineStatus: func(context.Context) system.PodmanMachineState {
			return system.PodmanMachineState{Supported: true, Initialized: true, Running: true}
		},
		openCommandName: func() string { return "open" },
		cockpitStatus: func(context.Context) system.CockpitState {
			return system.CockpitState{}
		},
		portInUse:      func(int) (bool, error) { return false, nil },
		listContainers: func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	found := false
	for _, check := range report.Checks {
		if check.Message == "podman machine running" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected running podman machine check, got %+v", report.Checks)
	}
}
