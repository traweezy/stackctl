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
		listContainers:     func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit:    func(context.Context) (system.OvercommitStatus, error) { return system.OvercommitStatus{}, nil },
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
		portInUse: func(int) (bool, error) { return false, nil },
		listContainers: func(context.Context) ([]system.Container, error) {
			return []system.Container{
				{
					Names:  []string{cfg.Services.PostgresContainer},
					State:  "running",
					Status: "Up",
					Ports:  []system.ContainerPort{{HostPort: cfg.Ports.Postgres, ContainerPort: 5432, Protocol: "tcp"}},
				},
				{
					Names:  []string{cfg.Services.RedisContainer},
					State:  "running",
					Status: "Up",
					Ports:  []system.ContainerPort{{HostPort: cfg.Ports.Redis, ContainerPort: 6379, Protocol: "tcp"}},
				},
				{
					Names:  []string{cfg.Services.PgAdminContainer},
					State:  "running",
					Status: "Up",
					Ports:  []system.ContainerPort{{HostPort: cfg.Ports.PgAdmin, ContainerPort: 80, Protocol: "tcp"}},
				},
			}, nil
		},
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{Supported: true, Value: 1}, nil
		},
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

func TestRunWithDepsWarnsWhenPortOwnedByUnknownProcess(t *testing.T) {
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
		cockpitStatus:      func(context.Context) system.CockpitState { return system.CockpitState{} },
		portInUse: func(port int) (bool, error) {
			return port == cfg.Ports.Postgres, nil
		},
		listContainers: func(context.Context) ([]system.Container, error) {
			return []system.Container{
				{
					Names:  []string{cfg.Services.RedisContainer},
					State:  "running",
					Status: "Up",
					Ports:  []system.ContainerPort{{HostPort: cfg.Ports.Redis, ContainerPort: 6379, Protocol: "tcp"}},
				},
			}, nil
		},
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{Supported: true, Value: 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	if !CheckPassed(report, "port 6379 is mapped by redis container") {
		t.Fatalf("expected redis port to be attributed to the stack: %+v", report.Checks)
	}

	for _, check := range report.Checks {
		if check.Message == "port 5432 is in use by another process or container, not postgres" && check.Status == output.StatusWarn {
			return
		}
	}

	t.Fatalf("expected warning about postgres port ownership, got %+v", report.Checks)
}

func TestRunWithDepsTreatsCockpitPortAsExpectedWhenCockpitIsActive(t *testing.T) {
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
		portInUse: func(port int) (bool, error) {
			return port == cfg.Ports.Cockpit, nil
		},
		listContainers: func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{Supported: true, Value: 1}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	if !CheckPassed(report, "port 9090 is in use by cockpit") {
		t.Fatalf("expected cockpit port ownership check, got %+v", report.Checks)
	}
}

func TestRunWithDepsWarnsWhenCockpitIsActiveButConfiguredPortIsNotListening(t *testing.T) {
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
		portInUse:      func(int) (bool, error) { return false, nil },
		listContainers: func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{Supported: true, Value: 1}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	for _, check := range report.Checks {
		if check.Message == "cockpit.socket active but port 9090 is not listening" && check.Status == output.StatusWarn {
			return
		}
	}

	t.Fatalf("expected warning about active cockpit without a listening configured port, got %+v", report.Checks)
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
