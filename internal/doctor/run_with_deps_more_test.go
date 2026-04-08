package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWithDepsCoversCockpitContainerAndRedisBranchMatrix(t *testing.T) {
	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	}
	cfg := configpkg.DefaultForStackOnPlatform("dev-stack", platform)
	cfg.Setup.IncludeRedis = true
	cfg.Setup.IncludeNATS = true
	cfg.Setup.IncludePgAdmin = true
	cfg.Setup.IncludeCockpit = true
	cfg.Setup.InstallCockpit = false
	cfg.ApplyDerivedFields()

	composePath := "/tmp/stackctl/compose.yaml"
	report, err := runWithDeps(context.Background(), dependencies{
		configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
		loadConfig:     func(string) (configpkg.Config, error) { return cfg, nil },
		validateConfig: func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.name", Message: "broken"}}
		},
		composePath: func(configpkg.Config) string { return composePath },
		stat: func(path string) (os.FileInfo, error) {
			switch path {
			case cfg.Stack.Dir:
				return fakeFileInfo{dir: true}, nil
			case composePath:
				return fakeFileInfo{name: "compose.yaml"}, nil
			default:
				return nil, os.ErrNotExist
			}
		},
		platform: func() system.Platform { return platform },
		commandExists: func(name string) bool {
			switch name {
			case "podman":
				return true
			default:
				return false
			}
		},
		podmanVersion:        func(context.Context) (string, error) { return "5.1.0", nil },
		podmanComposeVersion: func(context.Context) (string, error) { return "2.39.0", nil },
		podmanComposeAvail:   func(context.Context) bool { return true },
		openCommandName:      func() string { return "" },
		cockpitStatus: func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: false, Active: false, State: "inactive"}
		},
		portInUse: func(port int) (bool, error) {
			switch port {
			case cfg.Ports.Postgres:
				return false, nil
			case cfg.Ports.Redis:
				return true, nil
			case cfg.Ports.NATS:
				return false, nil
			case cfg.Ports.PgAdmin:
				return false, errors.New("blocked")
			case cfg.Ports.Cockpit:
				return true, nil
			default:
				return false, nil
			}
		},
		listContainers: func(context.Context) ([]system.Container, error) {
			return []system.Container{
				{
					Names:  []string{cfg.Services.PostgresContainer},
					State:  "running",
					Status: "Up",
					Ports: []system.ContainerPort{
						{HostPort: cfg.Ports.Postgres, ContainerPort: 5432, Protocol: "tcp"},
					},
				},
				{
					Names:  []string{cfg.Services.RedisContainer},
					State:  "exited",
					Status: "Exited (1)",
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

	wantChecks := []struct {
		status  string
		message string
	}{
		{output.StatusFail, "config invalid (1 issue(s))"},
		{output.StatusMiss, "buildah not installed"},
		{output.StatusMiss, "skopeo not installed"},
		{output.StatusMiss, "ss not available"},
		{output.StatusMiss, "browser opener not available"},
		{output.StatusWarn, "cockpit helpers enabled but cockpit.socket must be installed manually on this platform"},
		{output.StatusOK, fmt.Sprintf("port %d is mapped by postgres container", cfg.Ports.Postgres)},
		{output.StatusOK, "postgres container running"},
		{output.StatusWarn, fmt.Sprintf("port %d is in use by another process or container, not redis", cfg.Ports.Redis)},
		{output.StatusWarn, "redis container not running (Exited (1))"},
		{output.StatusWarn, "nats container not found"},
		{output.StatusFail, fmt.Sprintf("port %d check failed: blocked", cfg.Ports.PgAdmin)},
		{output.StatusWarn, fmt.Sprintf("port %d is in use by another process, not cockpit", cfg.Ports.Cockpit)},
		{output.StatusWarn, "set vm.overcommit_memory=1 to avoid Redis memory overcommit warnings"},
	}
	for _, want := range wantChecks {
		if !doctorHasCheck(report, want.status, want.message) {
			t.Fatalf("expected doctor check %s %q, got %+v", want.status, want.message, report.Checks)
		}
	}
}

func TestRunWithDepsCoversPodmanMachineAndInspectionFailureBranches(t *testing.T) {
	platform := system.Platform{
		GOOS:           "darwin",
		PackageManager: "brew",
		ServiceManager: system.ServiceManagerNone,
	}
	cfg := configpkg.DefaultForStackOnPlatform("darwin-stack", platform)
	cfg.Setup.IncludeCockpit = false
	cfg.ApplyDerivedFields()

	report, err := runWithDeps(context.Background(), dependencies{
		configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
		loadConfig:     func(string) (configpkg.Config, error) { return cfg, nil },
		validateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		composePath:    func(configpkg.Config) string { return "/tmp/stackctl/compose.yaml" },
		stat: func(path string) (os.FileInfo, error) {
			switch path {
			case cfg.Stack.Dir:
				return fakeFileInfo{dir: true}, nil
			case "/tmp/stackctl/compose.yaml":
				return fakeFileInfo{name: "compose.yaml"}, nil
			default:
				return nil, os.ErrNotExist
			}
		},
		platform:             func() system.Platform { return platform },
		commandExists:        func(name string) bool { return name == "podman" },
		podmanVersion:        func(context.Context) (string, error) { return "5.1.0", nil },
		podmanComposeVersion: func(context.Context) (string, error) { return "", errors.New("provider missing") },
		podmanComposeAvail:   func(context.Context) bool { return true },
		podmanMachineStatus: func(context.Context) system.PodmanMachineState {
			return system.PodmanMachineState{Initialized: true, Running: false}
		},
		openCommandName: func() string { return "open" },
		cockpitStatus: func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
		},
		portInUse: func(int) (bool, error) { return false, nil },
		listContainers: func(context.Context) ([]system.Container, error) {
			return nil, errors.New("inspect boom")
		},
		redisOvercommit: func(context.Context) (system.OvercommitStatus, error) {
			return system.OvercommitStatus{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	for _, want := range []struct {
		status  string
		message string
	}{
		{output.StatusOK, "podman machine initialized"},
		{output.StatusMiss, "podman machine not running"},
		{output.StatusFail, "container inspection failed: inspect boom"},
		{output.StatusWarn, "podman compose provider version could not be determined; supported minimum is 1.0.6"},
		{output.StatusOK, "open available"},
	} {
		if !doctorHasCheck(report, want.status, want.message) {
			t.Fatalf("expected doctor check %s %q, got %+v", want.status, want.message, report.Checks)
		}
	}
}

func doctorHasCheck(report Report, status, message string) bool {
	for _, check := range report.Checks {
		if check.Status == status && check.Message == message {
			return true
		}
	}
	return false
}

func doctorMessages(report Report) []string {
	messages := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		messages = append(messages, check.Status+" "+check.Message)
	}
	return messages
}

func TestDoctorMessagesHelper(t *testing.T) {
	report := Report{}
	report.add(output.StatusWarn, "warn")
	report.add(output.StatusOK, "ok")

	messages := doctorMessages(report)
	if got := strings.Join(messages, ","); got != "WARN warn,OK ok" {
		t.Fatalf("unexpected doctor message summary: %q", got)
	}
}
