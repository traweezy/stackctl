package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
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

func TestRunUsesDefaultDependenciesWithoutConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(configpkg.StackNameEnvVar, "")

	report, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(report.Checks) == 0 {
		t.Fatalf("expected doctor report checks, got %+v", report)
	}
}

func TestRunWithOptionsUsesInjectedDependencies(t *testing.T) {
	previous := doctorDependencies
	doctorDependencies = func() dependencies {
		return dependencies{
			configFilePath:     func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
			loadConfig:         func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound },
			validateConfig:     func(configpkg.Config) []configpkg.ValidationIssue { return nil },
			composePath:        func(configpkg.Config) string { return "/tmp/compose.yaml" },
			stat:               func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
			commandExists:      func(string) bool { return false },
			podmanComposeAvail: func(context.Context) bool { return false },
			openCommandName:    func() string { return "" },
			cockpitStatus:      func(context.Context) system.CockpitState { return system.CockpitState{} },
			portInUse:          func(int) (bool, error) { return false, nil },
			listContainers:     func(context.Context) ([]system.Container, error) { return nil, nil },
			redisOvercommit:    func(context.Context) (system.OvercommitStatus, error) { return system.OvercommitStatus{}, nil },
		}
	}
	defer func() { doctorDependencies = previous }()

	report, err := RunWithOptions(context.Background(), Options{CheckImages: true})
	if err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}
	if len(report.Checks) == 0 {
		t.Fatalf("expected doctor report checks, got %+v", report)
	}
}

func TestDefaultDependenciesPopulatesRequiredCallbacks(t *testing.T) {
	deps := defaultDependencies()

	callbacks := map[string]any{
		"configFilePath":       deps.configFilePath,
		"loadConfig":           deps.loadConfig,
		"validateConfig":       deps.validateConfig,
		"composePath":          deps.composePath,
		"stat":                 deps.stat,
		"platform":             deps.platform,
		"commandExists":        deps.commandExists,
		"podmanVersion":        deps.podmanVersion,
		"podmanComposeVersion": deps.podmanComposeVersion,
		"podmanComposeAvail":   deps.podmanComposeAvail,
		"podmanMachineStatus":  deps.podmanMachineStatus,
		"openCommandName":      deps.openCommandName,
		"cockpitStatus":        deps.cockpitStatus,
		"portInUse":            deps.portInUse,
		"listContainers":       deps.listContainers,
		"redisOvercommit":      deps.redisOvercommit,
		"checkImageReference":  deps.checkImageReference,
	}
	for name, callback := range callbacks {
		if callback == nil {
			t.Fatalf("expected %s callback to be populated", name)
		}
	}
}

func TestAddRuntimeVersionCheckHandlesNilErrorsAndVersionComparisons(t *testing.T) {
	t.Run("nil detector", func(t *testing.T) {
		var report Report
		addRuntimeVersionCheck(&report, "podman", "4.9.3", nil, context.Background())
		if len(report.Checks) != 0 {
			t.Fatalf("expected nil detector to add no checks, got %+v", report.Checks)
		}
	})

	t.Run("detector error", func(t *testing.T) {
		var report Report
		addRuntimeVersionCheck(&report, "podman", "4.9.3", func(context.Context) (string, error) {
			return "", errors.New("boom")
		}, context.Background())

		if report.WarnCount != 1 {
			t.Fatalf("expected warning for version detection failure, got %+v", report)
		}
		if got := report.Checks[0].Message; got != "podman version could not be determined; supported minimum is 4.9.3" {
			t.Fatalf("unexpected warning message %q", got)
		}
	})

	t.Run("below supported minimum", func(t *testing.T) {
		var report Report
		addRuntimeVersionCheck(&report, "podman", "4.9.3", func(context.Context) (string, error) {
			return "4.3.1", nil
		}, context.Background())

		if report.WarnCount != 1 {
			t.Fatalf("expected warning count 1, got %+v", report)
		}
		if got := report.Checks[0].Message; got != "podman 4.3.1 is below supported minimum 4.9.3" {
			t.Fatalf("unexpected warning message %q", got)
		}
	})

	t.Run("meets supported minimum", func(t *testing.T) {
		var report Report
		addRuntimeVersionCheck(&report, "podman", "4.9.3", func(context.Context) (string, error) {
			return "5.0.0", nil
		}, context.Background())

		if report.OKCount != 1 {
			t.Fatalf("expected ok count 1, got %+v", report)
		}
		if got := report.Checks[0].Message; got != "podman 5.0.0 meets supported minimum 4.9.3" {
			t.Fatalf("unexpected ok message %q", got)
		}
	})
}

func TestConfigResultHandlesNotFoundBlankAndErrors(t *testing.T) {
	path := "/tmp/stackctl/config.yaml"

	cfg := configpkg.Default()
	loaded, ok, err := configResult(dependencies{
		loadConfig: func(string) (configpkg.Config, error) { return cfg, nil },
	}, path)
	if err != nil || !ok || loaded.Stack.Dir != cfg.Stack.Dir {
		t.Fatalf("expected loaded config, got cfg=%+v ok=%t err=%v", loaded, ok, err)
	}

	loaded, ok, err = configResult(dependencies{
		loadConfig: func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound },
	}, path)
	if err != nil || ok || loaded != (configpkg.Config{}) {
		t.Fatalf("expected missing config result, got cfg=%+v ok=%t err=%v", loaded, ok, err)
	}

	loaded, ok, err = configResult(dependencies{
		loadConfig: func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("   ") },
	}, path)
	if err != nil || ok || loaded != (configpkg.Config{}) {
		t.Fatalf("expected blank error to be ignored, got cfg=%+v ok=%t err=%v", loaded, ok, err)
	}

	expectedErr := errors.New("parse failed")
	loaded, ok, err = configResult(dependencies{
		loadConfig: func(string) (configpkg.Config, error) { return configpkg.Config{}, expectedErr },
	}, path)
	if !errors.Is(err, expectedErr) || ok || loaded != (configpkg.Config{}) {
		t.Fatalf("expected parse failure to surface, got cfg=%+v ok=%t err=%v", loaded, ok, err)
	}
}

func TestContainerBindsHostPortMatchesMappedPorts(t *testing.T) {
	container := system.Container{
		Ports: []system.ContainerPort{
			{HostPort: 5432, ContainerPort: 5432, Protocol: "tcp"},
			{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"},
		},
	}

	if !containerBindsHostPort(container, 5432) {
		t.Fatal("expected host port 5432 to be detected")
	}
	if containerBindsHostPort(container, 9090) {
		t.Fatal("expected unmapped host port 9090 to be rejected")
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

func TestRunWithDepsWarnsWhenRuntimeIsBelowSupportedMinimum(t *testing.T) {
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
		commandExists: func(string) bool { return true },
		podmanVersion: func(context.Context) (string, error) { return "4.3.1", nil },
		podmanComposeVersion: func(context.Context) (string, error) {
			return "1.0.3", nil
		},
		podmanComposeAvail: func(context.Context) bool { return true },
		openCommandName:    func() string { return "xdg-open" },
		cockpitStatus:      func(context.Context) system.CockpitState { return system.CockpitState{} },
		portInUse:          func(int) (bool, error) { return false, nil },
		listContainers:     func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit:    func(context.Context) (system.OvercommitStatus, error) { return system.OvercommitStatus{}, nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	if !CheckPassed(report, "podman installed") {
		t.Fatalf("expected podman installed check, got %+v", report.Checks)
	}
	if !CheckPassed(report, "podman compose available") {
		t.Fatalf("expected podman compose check, got %+v", report.Checks)
	}

	warnings := []string{
		"podman 4.3.1 is below supported minimum 4.9.3",
		"podman compose provider 1.0.3 is below supported minimum 1.0.6",
	}
	for _, warning := range warnings {
		found := false
		for _, check := range report.Checks {
			if check.Status == output.StatusWarn && check.Message == warning {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected warning %q, got %+v", warning, report.Checks)
		}
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

func TestRunWithDepsOnDarwinReportsPodmanMachineAndSkipsLinuxOnlyChecks(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Setup.IncludeCockpit = false
	cfg.Setup.InstallCockpit = false

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
		platform: func() system.Platform {
			return system.Platform{GOOS: "darwin", PackageManager: "brew", ServiceManager: system.ServiceManagerNone}
		},
		commandExists:      func(name string) bool { return name == "podman" },
		podmanComposeAvail: func(context.Context) bool { return true },
		podmanMachineStatus: func(context.Context) system.PodmanMachineState {
			return system.PodmanMachineState{Supported: true, Initialized: false, State: "not initialized"}
		},
		openCommandName: func() string { return "open" },
		cockpitStatus:   func(context.Context) system.CockpitState { return system.CockpitState{} },
		portInUse:       func(int) (bool, error) { return false, nil },
		listContainers:  func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) { return system.OvercommitStatus{}, nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !CheckPassed(report, "podman installed") {
		t.Fatalf("expected podman to be reported as installed: %+v", report.Checks)
	}
	if CheckPassed(report, "buildah installed") {
		t.Fatalf("buildah should not be required on darwin: %+v", report.Checks)
	}
	if CheckPassed(report, "ss available") {
		t.Fatalf("ss should not be checked on darwin: %+v", report.Checks)
	}
	foundMachineMiss := false
	for _, check := range report.Checks {
		if check.Message == "podman machine not initialized" && check.Status == output.StatusMiss {
			foundMachineMiss = true
		}
		if strings.Contains(check.Message, "cockpit.socket") {
			t.Fatalf("darwin report should not include cockpit checks when disabled: %+v", report.Checks)
		}
	}
	if !foundMachineMiss {
		t.Fatalf("expected missing podman machine check: %+v", report.Checks)
	}
}

func TestRunWithDepsOptionsChecksReachableServiceImages(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Setup.IncludeCockpit = false
	cfg.Setup.InstallCockpit = false
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.ApplyDerivedFields()

	checked := make([]string, 0, 6)
	report, err := runWithDepsOptions(context.Background(), dependencies{
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
		commandExists:        func(string) bool { return true },
		podmanVersion:        func(context.Context) (string, error) { return system.SupportedPodmanVersion, nil },
		podmanComposeVersion: func(context.Context) (string, error) { return system.SupportedComposeProviderVersion, nil },
		podmanComposeAvail:   func(context.Context) bool { return true },
		openCommandName:      func() string { return "xdg-open" },
		cockpitStatus:        func(context.Context) system.CockpitState { return system.CockpitState{} },
		portInUse:            func(int) (bool, error) { return false, nil },
		listContainers:       func(context.Context) ([]system.Container, error) { return nil, nil },
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{Supported: true, Value: 1}, nil
		},
		checkImageReference: func(ctx context.Context, image string) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) <= 0 {
				t.Fatalf("expected image check context deadline for %s", image)
			}
			checked = append(checked, image)
			if image == cfg.Services.NATS.Image || image == cfg.Services.Meilisearch.Image {
				return errors.New("registry denied")
			}
			return nil
		},
	}, Options{CheckImages: true})
	if err != nil {
		t.Fatalf("runWithDepsOptions returned error: %v", err)
	}

	if len(checked) != 6 {
		t.Fatalf("expected 6 image checks, got %d (%v)", len(checked), checked)
	}
	for _, want := range []struct {
		status  string
		message string
	}{
		{output.StatusOK, "postgres image is reachable: " + cfg.Services.Postgres.Image},
		{output.StatusWarn, "nats image could not be resolved: " + cfg.Services.NATS.Image + " (registry denied)"},
		{output.StatusOK, fmt.Sprintf("port %d is free for seaweedfs", cfg.Ports.SeaweedFS)},
		{output.StatusWarn, "seaweedfs container not found"},
		{output.StatusWarn, "meilisearch image could not be resolved: " + cfg.Services.Meilisearch.Image + " (registry denied)"},
		{output.StatusWarn, "meilisearch container not found"},
	} {
		if !doctorHasCheck(report, want.status, want.message) {
			t.Fatalf("expected doctor check %s %q, got %+v", want.status, want.message, report.Checks)
		}
	}
}

func TestCheckRemoteImageReferenceBranches(t *testing.T) {
	ref, err := name.ParseReference("docker.io/library/postgres:16")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	previousParse := parseDoctorImageReference
	previousHead := doctorRemoteHead
	previousGet := doctorRemoteGet
	defer func() {
		parseDoctorImageReference = previousParse
		doctorRemoteHead = previousHead
		doctorRemoteGet = previousGet
	}()

	t.Run("parse errors are returned", func(t *testing.T) {
		parseDoctorImageReference = func(string) (name.Reference, error) { return nil, errors.New("bad image") }
		doctorRemoteHead = previousHead
		doctorRemoteGet = previousGet

		if err := checkRemoteImageReference(context.Background(), "%%%bad%%%"); err == nil || !strings.Contains(err.Error(), "bad image") {
			t.Fatalf("expected parse failure, got %v", err)
		}
	})

	t.Run("head success returns nil", func(t *testing.T) {
		parseDoctorImageReference = func(string) (name.Reference, error) { return ref, nil }
		doctorRemoteHead = func(context.Context, name.Reference) error { return nil }
		doctorRemoteGet = func(context.Context, name.Reference) error {
			t.Fatal("expected get fallback to be skipped after head success")
			return nil
		}

		if err := checkRemoteImageReference(context.Background(), "docker.io/library/postgres:16"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("get fallback is used after head failure", func(t *testing.T) {
		parseDoctorImageReference = func(string) (name.Reference, error) { return ref, nil }
		doctorRemoteHead = func(context.Context, name.Reference) error { return errors.New("head failed") }
		doctorRemoteGet = func(context.Context, name.Reference) error { return nil }

		if err := checkRemoteImageReference(context.Background(), "docker.io/library/postgres:16"); err != nil {
			t.Fatalf("expected nil error after get fallback, got %v", err)
		}
	})

	t.Run("get fallback errors are returned", func(t *testing.T) {
		parseDoctorImageReference = func(string) (name.Reference, error) { return ref, nil }
		doctorRemoteHead = func(context.Context, name.Reference) error { return errors.New("head failed") }
		doctorRemoteGet = func(context.Context, name.Reference) error { return errors.New("get failed") }

		if err := checkRemoteImageReference(context.Background(), "docker.io/library/postgres:16"); err == nil || !strings.Contains(err.Error(), "get failed") {
			t.Fatalf("expected get failure, got %v", err)
		}
	})
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
