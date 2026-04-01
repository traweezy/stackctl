package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDefaultCommandDepsProvidesHooks(t *testing.T) {
	value := defaultCommandDeps()
	if value.configDirPath == nil || value.configFilePath == nil || value.configFilePathForStack == nil || value.knownConfigPaths == nil || value.currentStackName == nil || value.setCurrentStackName == nil || value.composeUp == nil || value.composeUpServices == nil || value.composeStopServices == nil || value.composeExec == nil || value.runExternalCommand == nil || value.removeFile == nil || value.removeAll == nil || value.mkdirAll == nil || value.rename == nil || value.scaffoldManagedStack == nil || value.managedStackNeedsScaffold == nil || value.podmanMachineStatus == nil {
		t.Fatal("default command deps should initialize function hooks")
	}
}

func TestDefaultCommandDepsClosuresExecuteAgainstFakeBinaries(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")

	writeScript := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write script %s: %v", name, err)
		}
	}

	writeScript("podman", "#!/bin/sh\necho podman \"$@\" >> \""+logPath+"\"\n")
	writeScript("xdg-open", "#!/bin/sh\necho xdg-open \"$@\" >> \""+logPath+"\"\n")
	writeScript("sudo", "#!/bin/sh\necho sudo \"$@\" >> \""+logPath+"\"\n")

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.Stack.Dir = dir
	cfg.Stack.ComposeFile = "compose.yaml"
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	value := defaultCommandDeps()
	runner := system.Runner{Stdout: io.Discard, Stderr: io.Discard}

	if err := value.openURL(context.Background(), runner, "http://example.com"); err != nil {
		t.Fatalf("openURL returned error: %v", err)
	}
	if _, err := value.installPackages(context.Background(), runner, "apt", []system.Requirement{system.RequirementPodman}); err != nil {
		t.Fatalf("installPackages returned error: %v", err)
	}
	if err := value.enableCockpit(context.Background(), runner); err != nil {
		t.Fatalf("enableCockpit returned error: %v", err)
	}
	if err := value.composeUp(context.Background(), runner, cfg); err != nil {
		t.Fatalf("composeUp returned error: %v", err)
	}
	if err := value.composeUpServices(context.Background(), runner, cfg, true, []string{"postgres"}); err != nil {
		t.Fatalf("composeUpServices returned error: %v", err)
	}
	if err := value.composeDown(context.Background(), runner, cfg, true); err != nil {
		t.Fatalf("composeDown returned error: %v", err)
	}
	if err := value.composeStopServices(context.Background(), runner, cfg, []string{"postgres"}); err != nil {
		t.Fatalf("composeStopServices returned error: %v", err)
	}
	if err := value.composeLogs(context.Background(), runner, cfg, 10, true, "1m", "postgres"); err != nil {
		t.Fatalf("composeLogs returned error: %v", err)
	}
	if err := value.composeExec(context.Background(), runner, cfg, "postgres", []string{"PGPASSWORD=stackpass"}, []string{"printenv", "PGDATA"}, false); err != nil {
		t.Fatalf("composeExec returned error: %v", err)
	}
	if err := value.containerLogs(context.Background(), runner, "local-postgres", 5, false, ""); err != nil {
		t.Fatalf("containerLogs returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "xdg-open") || !strings.Contains(output, "podman compose") || !strings.Contains(output, "sudo apt-get install -y podman") {
		t.Fatalf("unexpected command log: %s", output)
	}
}

func TestDefaultTerminalInteractive(t *testing.T) {
	_ = defaultTerminalInteractive()
}

func TestConfirmWithPromptRequiresTerminal(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	_, err := confirmWithPrompt(NewRootCmd(NewApp()), "Continue?", true)
	if err == nil {
		t.Fatal("expected confirmation to require a terminal")
	}
}

func TestConfigPathPrintsResolvedPath(t *testing.T) {
	withTestDeps(t, nil)

	stdout, _, err := executeRoot(t, "config", "path")
	if err != nil {
		t.Fatalf("config path returned error: %v", err)
	}
	if strings.TrimSpace(stdout) != "/tmp/stackctl/config.yaml" {
		t.Fatalf("unexpected config path output: %q", stdout)
	}
}

func TestConfigEditNonInteractiveSavesUpdatedConfig(t *testing.T) {
	var saved bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "edit", "--non-interactive")
	if err != nil {
		t.Fatalf("config edit returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected config edit to save config")
	}
	if !strings.Contains(stdout, "Updated config at /tmp/stackctl/config.yaml") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigInitForceOverwritesExistingConfig(t *testing.T) {
	var saved bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.saveConfig = func(string, configpkg.Config) error {
			saved = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "config", "init", "--non-interactive", "--force")
	if err != nil {
		t.Fatalf("config init --force returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected config init to save config")
	}
}

func TestConfigViewMissingConfigReturnsGuidance(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "config", "view")
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigResetForceWritesDefaults(t *testing.T) {
	var saved configpkg.Config
	scaffolded := false

	withTestDeps(t, func(d *commandDeps) {
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
		d.saveConfig = func(_ string, cfg configpkg.Config) error {
			saved = cfg
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "config", "reset", "--force")
	if err != nil {
		t.Fatalf("config reset returned error: %v", err)
	}
	if saved.Stack.Name != "dev-stack" {
		t.Fatalf("unexpected saved config: %+v", saved)
	}
	if !scaffolded {
		t.Fatal("expected config reset to scaffold the default managed stack")
	}
	if !strings.Contains(stdout, "Reset config at /tmp/stackctl/config.yaml to defaults") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestConfigResetDeleteWithoutTerminalNeedsForce(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "config", "reset", "--delete")
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectPrintsConnectionInfo(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.RedisPassword = "redispass"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !strings.Contains(stdout, "Postgres\n  postgres://app:app@devbox:5432/app") {
		t.Fatalf("unexpected connect output: %s", stdout)
	}
	if !strings.Contains(stdout, "Redis\n  redis://:redispass@devbox:6379") {
		t.Fatalf("expected redis auth DSN in connect output: %s", stdout)
	}
	if strings.Contains(stdout, "DSN:") || strings.Contains(stdout, "URL:") {
		t.Fatalf("connect should stay minimal, got: %s", stdout)
	}
}

func TestConnectUsesConfigWithoutRuntimeInspection(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.RedisPassword = "redispass"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			t.Fatal("connect should not inspect podman runtime")
			return system.CommandResult{}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			t.Fatal("connect should not inspect cockpit runtime")
			return system.CockpitState{}
		}
	})

	stdout, _, err := executeRoot(t, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !strings.Contains(stdout, "Cockpit\n  https://devbox:9090") {
		t.Fatalf("expected cockpit URL in connect output: %s", stdout)
	}
	if !strings.Contains(stdout, "pgAdmin\n  http://devbox:8081") {
		t.Fatalf("expected pgadmin URL in connect output: %s", stdout)
	}
}

func TestConnectIncludesSeaweedFSEndpointAndCredentialsWhenEnabled(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Connection.SeaweedFSAccessKey = "seaweed-access"
		cfg.Connection.SeaweedFSSecretKey = "seaweed-secret"
		cfg.Ports.SeaweedFS = 18333
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !strings.Contains(stdout, "SeaweedFS S3 endpoint\n  http://devbox:18333") {
		t.Fatalf("expected seaweedfs endpoint in connect output: %s", stdout)
	}
	if !strings.Contains(stdout, "SeaweedFS access key\n  seaweed-access") {
		t.Fatalf("expected seaweedfs access key in connect output: %s", stdout)
	}
	if !strings.Contains(stdout, "SeaweedFS secret key\n  seaweed-secret") {
		t.Fatalf("expected seaweedfs secret key in connect output: %s", stdout)
	}
}

func TestConnectIncludesMeilisearchURLAndAPIKeyWhenEnabled(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeMeilisearch = true
		cfg.Connection.MeilisearchMasterKey = "meili-master-key-123"
		cfg.Ports.Meilisearch = 17700
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !strings.Contains(stdout, "Meilisearch\n  http://devbox:17700") {
		t.Fatalf("expected meilisearch url in connect output: %s", stdout)
	}
	if !strings.Contains(stdout, "Meilisearch API key\n  meili-master-key-123") {
		t.Fatalf("expected meilisearch api key in connect output: %s", stdout)
	}
}

func TestConnectAllowsExternalStackWithoutComposeFile(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Stack.Managed = false
		cfg.Setup.ScaffoldDefaultStack = false
		cfg.Stack.Dir = t.TempDir()
		cfg.Connection.Host = "devbox"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !strings.Contains(stdout, "postgres://app:app@devbox:5432/app") {
		t.Fatalf("unexpected connect output: %s", stdout)
	}
}

func TestStatusPrintsTable(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef1234567890","Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[{"host_port":5432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "status")
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	if !strings.Contains(stdout, "local-postgres") || !strings.Contains(stdout, "5432->5432/tcp") {
		t.Fatalf("unexpected status output: %s", stdout)
	}
}

func TestStatusJSONPrintsContainers(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef1234567890","Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json returned error: %v", err)
	}
	if !strings.Contains(stdout, "\"local-postgres\"") {
		t.Fatalf("unexpected json status output: %s", stdout)
	}
}

func TestStatusVerboseShowsExtraColumns(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef1234567890","Names":["local-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[{"host_port":5432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "status", "--verbose")
	if err != nil {
		t.Fatalf("status --verbose returned error: %v", err)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "abcdef123456") {
		t.Fatalf("unexpected verbose status output: %s", stdout)
	}
}

func TestStopRunsComposeDown(t *testing.T) {
	downCalled := false

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			downCalled = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "stop")
	if err != nil {
		t.Fatalf("stop returned error: %v", err)
	}
	if !downCalled {
		t.Fatal("expected stop to call compose down")
	}
}

func TestStopVerboseShowsComposeFile(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	stdout, _, err := executeRoot(t, "--verbose", "stop")
	if err != nil {
		t.Fatalf("stop returned error: %v", err)
	}
	if !strings.Contains(stdout, "Using compose file /tmp/stackctl/compose.yaml") {
		t.Fatalf("stdout missing verbose compose detail:\n%s", stdout)
	}
}

func TestStopQuietSuppressesProgressOutput(t *testing.T) {
	var downCalled bool

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			downCalled = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "--quiet", "stop")
	if err != nil {
		t.Fatalf("stop returned error: %v", err)
	}
	if !downCalled {
		t.Fatal("expected stop to still run compose down")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected quiet stop output to be empty, got:\n%s", stdout)
	}
}

func TestStartServiceRunsComposeUpServicesAndWaitsSelectedPort(t *testing.T) {
	var calledServices []string
	var forceRecreate bool
	var waitedPorts []int

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, force bool, services []string) error {
			forceRecreate = force
			calledServices = append([]string(nil), services...)
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
		}
		d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "start", "postgres")
	if err != nil {
		t.Fatalf("start postgres returned error: %v", err)
	}
	if !reflect.DeepEqual(calledServices, []string{"postgres"}) {
		t.Fatalf("unexpected services: %v", calledServices)
	}
	if forceRecreate {
		t.Fatal("service start should not force recreate")
	}
	if !reflect.DeepEqual(waitedPorts, []int{5432}) {
		t.Fatalf("unexpected waited ports: %v", waitedPorts)
	}
	for _, fragment := range []string{
		"🚀 starting postgres...",
		"✅ Postgres started",
		"Postgres\n  postgres://app:app@localhost:5432/app",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stdout missing %q:\n%s", fragment, stdout)
		}
	}
	if strings.Contains(stdout, "Redis\n  redis://localhost:6379") {
		t.Fatalf("service start should only print selected connections:\n%s", stdout)
	}
}

func TestStartFailsWhenHostPortIsAlreadyBusy(t *testing.T) {
	composeUpCalled := false
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: marshalContainersJSON()}, nil
		}
		d.portInUse = func(port int) (bool, error) {
			return port == cfg.Ports.Postgres, nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			composeUpCalled = true
			return nil
		}
	})

	_, _, err := executeRoot(t, "start")
	if err == nil {
		t.Fatal("expected start to fail when a host port is already busy")
	}
	if composeUpCalled {
		t.Fatal("compose up should not run when port preflight fails")
	}
	for _, fragment := range []string{
		"cannot start stack",
		"port 5432 is in use by another process or container, not postgres",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q: %v", fragment, err)
		}
	}
}

func TestStartAllowsPendingManagedScaffoldValidationIssues(t *testing.T) {
	var scaffolded bool
	var started bool

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Behavior.WaitForServicesStart = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{
				{Field: "stack.dir", Message: fmt.Sprintf("directory does not exist: %s", cfg.Stack.Dir)},
				{Field: "stack.compose_file", Message: fmt.Sprintf("file does not exist: %s", configpkg.ComposePath(cfg))},
			}
		}
		d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
		d.scaffoldManagedStack = func(cfg configpkg.Config, _ bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			return configpkg.ScaffoldResult{
				StackDir:     cfg.Stack.Dir,
				ComposePath:  configpkg.ComposePath(cfg),
				WroteCompose: true,
			}, nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			started = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "start")
	if err != nil {
		t.Fatalf("start returned error: %v", err)
	}
	if !scaffolded {
		t.Fatal("expected start to scaffold pending managed stack files")
	}
	if !started {
		t.Fatal("expected start to continue into compose up")
	}
	if !strings.Contains(stdout, "wrote managed compose file") {
		t.Fatalf("expected start output to include scaffold status:\n%s", stdout)
	}
}

func TestStartServiceWithWaitDisabledFailsWhenContainerExitsImmediately(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Behavior.WaitForServicesStart = false
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ bool, _ []string) error {
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: marshalContainersJSON(system.Container{
				ID:     "postgres123456",
				Image:  "postgres:latest",
				Names:  []string{cfg.Services.PostgresContainer},
				Status: "Exited (1)",
				State:  "exited",
			})}, nil
		}
	})

	_, _, err := executeRoot(t, "start", "postgres")
	if err == nil {
		t.Fatal("expected start to fail when the selected container exits immediately")
	}
	if !strings.Contains(err.Error(), "postgres container failed to start (Exited (1))") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopServiceRunsComposeStopServices(t *testing.T) {
	var calledServices []string

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.composeStopServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, services []string) error {
			calledServices = append([]string(nil), services...)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "stop", "redis")
	if err != nil {
		t.Fatalf("stop redis returned error: %v", err)
	}
	if !reflect.DeepEqual(calledServices, []string{"redis"}) {
		t.Fatalf("unexpected services: %v", calledServices)
	}
	for _, fragment := range []string{
		"🛑 stopping redis...",
		"✅ Redis stopped",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stdout missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestRestartServiceRunsComposeUpServicesWithForceRecreate(t *testing.T) {
	var calledServices []string
	var forceRecreate bool
	var waitedPorts []int

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeUpServices = func(_ context.Context, _ system.Runner, _ configpkg.Config, force bool, services []string) error {
			forceRecreate = force
			calledServices = append([]string(nil), services...)
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "nats")}, nil
		}
		d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			waitedPorts = append(waitedPorts, port)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "restart", "nats")
	if err != nil {
		t.Fatalf("restart nats returned error: %v", err)
	}
	if !reflect.DeepEqual(calledServices, []string{"nats"}) {
		t.Fatalf("unexpected services: %v", calledServices)
	}
	if !forceRecreate {
		t.Fatal("service restart should force recreate")
	}
	if !reflect.DeepEqual(waitedPorts, []int{4222}) {
		t.Fatalf("unexpected waited ports: %v", waitedPorts)
	}
	for _, fragment := range []string{
		"🔄 restarting nats...",
		"✅ NATS restarted",
		"NATS\n  nats://stackctl@localhost:4222",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stdout missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestStartRefusesWhenAnotherLocalStackIsRunning(t *testing.T) {
	currentPath := "/tmp/stackctl/config.yaml"
	otherPath := "/tmp/stackctl/stacks/staging.yaml"
	currentCfg := configpkg.Default()
	otherCfg := configpkg.DefaultForStack("staging")

	withTestDeps(t, func(d *commandDeps) {
		d.configFilePath = func() (string, error) { return currentPath, nil }
		d.knownConfigPaths = func() ([]string, error) { return []string{currentPath, otherPath}, nil }
		d.loadConfig = func(path string) (configpkg.Config, error) {
			switch path {
			case currentPath:
				return currentCfg, nil
			case otherPath:
				return otherCfg, nil
			default:
				return configpkg.Config{}, configpkg.ErrNotFound
			}
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Names":["stackctl-staging-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[]}]`,
			}, nil
		}
	})

	_, _, err := executeRoot(t, "start")
	if err == nil {
		t.Fatal("expected start to refuse when another stack is running")
	}
	for _, fragment := range []string{
		"another local stack is already running: staging",
		"`stackctl --stack staging stop`",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q: %v", fragment, err)
		}
	}
}

func TestStatusRequiresPodman(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.commandExists = func(string) bool { return false }
	})

	_, _, err := executeRoot(t, "status")
	if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootRejectsVerboseAndQuietTogether(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "--verbose", "--quiet", "status")
	if err == nil || !strings.Contains(err.Error(), "--verbose and --quiet cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootRejectsAccessibleAndPlainTogether(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "--accessible", "--plain", "version")
	if err == nil || !strings.Contains(err.Error(), "--accessible and --plain cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootAppliesInteractiveAndLoggingFlagOverrides(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t,
		"--accessible",
		"--log-level", "debug",
		"--log-format", "json",
		"--log-file", "-",
		"version",
	)
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	if got := os.Getenv("ACCESSIBLE"); got != "1" {
		t.Fatalf("expected ACCESSIBLE override, got %q", got)
	}
	if got := os.Getenv("STACKCTL_LOG_LEVEL"); got != "debug" {
		t.Fatalf("expected STACKCTL_LOG_LEVEL override, got %q", got)
	}
	if got := os.Getenv("STACKCTL_LOG_FORMAT"); got != "json" {
		t.Fatalf("expected STACKCTL_LOG_FORMAT override, got %q", got)
	}
	if got := os.Getenv("STACKCTL_LOG_FILE"); got != "-" {
		t.Fatalf("expected STACKCTL_LOG_FILE override, got %q", got)
	}
}

func TestSetupRejectsConflictingModes(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "setup", "--interactive", "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetupWithoutTerminalRequiresNonInteractive(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	_, _, err := executeRoot(t, "setup")
	if err == nil || !strings.Contains(err.Error(), "cannot prompt without a terminal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetupInstallWithNothingMissingExitsCleanly(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket active"},
			), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup", "--install")
	if err != nil {
		t.Fatalf("setup --install returned error: %v", err)
	}
	if !strings.Contains(stdout, "nothing to install") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestSetupIncludesManualCockpitGuidanceWhenAutoInstallIsUnsupported(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.System.PackageManager = "apt"
		d.platform = func() system.Platform {
			return system.Platform{
				GOOS:           "linux",
				PackageManager: "apt",
				ServiceManager: system.ServiceManagerSystemd,
			}
		}
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
				doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
				doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				doctorpkg.Check{Status: output.StatusWarn, Message: "cockpit helpers enabled but cockpit.socket must be installed manually on this platform"},
			), nil
		}
	})

	stdout, _, err := executeRoot(t, "setup")
	if err != nil {
		t.Fatalf("setup returned error: %v", err)
	}
	if !strings.Contains(stdout, "install cockpit manually on this platform if you want the Cockpit web UI") {
		t.Fatalf("expected manual cockpit guidance, got: %s", stdout)
	}
}

func TestRestartRunsDownUpWaitAndPrintsEndpoints(t *testing.T) {
	calls := make([]string, 0, 2)

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
			calls = append(calls, "down")
			return nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			calls = append(calls, "up")
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	})

	stdout, _, err := executeRoot(t, "restart")
	if err != nil {
		t.Fatalf("restart returned error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "down" || calls[1] != "up" {
		t.Fatalf("unexpected restart call order: %v", calls)
	}
	if !strings.Contains(stdout, "✅ stack restarted") {
		t.Fatalf("unexpected restart output: %s", stdout)
	}
	if !strings.Contains(stdout, "🔄 restarting stack...") {
		t.Fatalf("unexpected restart action output: %s", stdout)
	}
	if !strings.Contains(stdout, "Cockpit\n  https://localhost:9090") {
		t.Fatalf("restart should print connection info: %s", stdout)
	}
}

func TestResetWaitsForContainersToBeRemoved(t *testing.T) {
	originalTimeout := downWaitTimeout
	originalInterval := downWaitInterval
	downWaitTimeout = 50 * time.Millisecond
	downWaitInterval = time.Millisecond
	t.Cleanup(func() {
		downWaitTimeout = originalTimeout
		downWaitInterval = originalInterval
	})

	checks := 0

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.anyContainerExists = func(context.Context, []string) (bool, error) {
			checks++
			return checks < 2, nil
		}
	})

	stdout, _, err := executeRoot(t, "reset", "--force")
	if err != nil {
		t.Fatalf("reset returned error: %v", err)
	}
	if checks < 2 {
		t.Fatalf("expected reset to poll for container removal, got %d checks", checks)
	}
	if !strings.Contains(stdout, "✅ stack reset") {
		t.Fatalf("unexpected reset output: %s", stdout)
	}
}

func TestRestartWaitsForContainersToBeRemovedBeforeStartingAgain(t *testing.T) {
	originalTimeout := downWaitTimeout
	originalInterval := downWaitInterval
	downWaitTimeout = 50 * time.Millisecond
	downWaitInterval = time.Millisecond
	t.Cleanup(func() {
		downWaitTimeout = originalTimeout
		downWaitInterval = originalInterval
	})

	existsChecks := 0
	upCalled := false

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.anyContainerExists = func(context.Context, []string) (bool, error) {
			existsChecks++
			return existsChecks < 2, nil
		}
		d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
			upCalled = true
			if existsChecks < 2 {
				t.Fatalf("composeUp called before containers were removed")
			}
			return nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	})

	if _, _, err := executeRoot(t, "restart"); err != nil {
		t.Fatalf("restart returned error: %v", err)
	}
	if !upCalled {
		t.Fatal("expected restart to start the stack again")
	}
	if existsChecks < 2 {
		t.Fatalf("expected restart to poll for container removal, got %d checks", existsChecks)
	}
}

func TestOpenRejectsInvalidTarget(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	_, _, err := executeRoot(t, "open", "bad")
	if err == nil {
		t.Fatal("expected invalid open target error")
	}
	if !strings.Contains(err.Error(), "valid values: cockpit, meilisearch, pgadmin, all") {
		t.Fatalf("unexpected open error: %v", err)
	}
}

func TestOpenDefaultsToCockpit(t *testing.T) {
	opened := ""

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			opened = target
			return nil
		}
	})

	_, _, err := executeRoot(t, "open")
	if err != nil {
		t.Fatalf("open returned error: %v", err)
	}
	if opened != configpkg.Default().URLs.Cockpit {
		t.Fatalf("unexpected opened url: %s", opened)
	}
}

func TestOpenDefaultsToMeilisearchWhenCockpitDisabled(t *testing.T) {
	opened := ""

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = false
			cfg.Setup.IncludePgAdmin = false
			cfg.Setup.IncludeMeilisearch = true
			cfg.ApplyDerivedFields()
			return cfg, nil
		}
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			opened = target
			return nil
		}
	})

	_, _, err := executeRoot(t, "open")
	if err != nil {
		t.Fatalf("open returned error: %v", err)
	}
	if opened != "http://localhost:7700" {
		t.Fatalf("unexpected opened url: %s", opened)
	}
}

func TestOpenDefaultsToPgAdminWhenCockpitAndMeilisearchDisabled(t *testing.T) {
	opened := ""

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = false
			cfg.Setup.IncludeMeilisearch = false
			cfg.Setup.IncludePgAdmin = true
			cfg.ApplyDerivedFields()
			return cfg, nil
		}
		d.openURL = func(_ context.Context, _ system.Runner, target string) error {
			opened = target
			return nil
		}
	})

	_, _, err := executeRoot(t, "open")
	if err != nil {
		t.Fatalf("open returned error: %v", err)
	}
	if opened != "http://localhost:8081" {
		t.Fatalf("unexpected opened url: %s", opened)
	}
}

func TestOpenPgAdminDisabledReturnsError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Setup.IncludePgAdmin = false
			return cfg, nil
		}
	})

	_, _, err := executeRoot(t, "open", "pgadmin")
	if err == nil || !strings.Contains(err.Error(), "pgadmin is disabled") {
		t.Fatalf("unexpected open error: %v", err)
	}
}

func TestVersionIncludesOptionalMetadata(t *testing.T) {
	app := NewApp()
	app.GitCommit = "abc123"
	app.BuildDate = "2026-03-21"

	stdout, _, err := executeAppRoot(t, app, "version")
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	if !strings.Contains(stdout, "git_commit: abc123") || !strings.Contains(stdout, "build_date: 2026-03-21") {
		t.Fatalf("unexpected version output: %s", stdout)
	}
}

func TestVersionJSONIncludesOptionalMetadata(t *testing.T) {
	app := NewApp()
	app.GitCommit = "abc123"
	app.BuildDate = "2026-03-21"

	stdout, _, err := executeAppRoot(t, app, "version", "--json")
	if err != nil {
		t.Fatalf("version --json returned error: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal version json: %v\n%s", err, stdout)
	}
	if payload["version"] != app.Version || payload["git_commit"] != "abc123" || payload["build_date"] != "2026-03-21" {
		t.Fatalf("unexpected version json payload: %+v", payload)
	}
}

func TestResolveConfigFromFlagsRejectsNonInteractiveWithoutTerminal(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	_, err := resolveConfigFromFlags(NewRootCmd(NewApp()), configpkg.Default(), false)
	if err == nil || !strings.Contains(err.Error(), "requires a terminal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorCommandSucceedsWithCleanReport(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "config file found"}), nil
		}
	})

	stdout, _, err := executeRoot(t, "doctor")
	if err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	if !strings.Contains(stdout, "Summary: 1 ok, 0 warn, 0 miss, 0 fail") {
		t.Fatalf("unexpected doctor output: %s", stdout)
	}
}

func TestEnsureComposeRuntimeErrorsWhenPrerequisitesMissing(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.commandExists = func(string) bool { return false }
	})

	err := ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
		t.Fatalf("unexpected error: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.commandExists = func(string) bool { return true }
		d.podmanComposeAvail = func(context.Context) bool { return false }
	})

	err = ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "podman compose is not available") {
		t.Fatalf("unexpected error: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.commandExists = func(string) bool { return true }
		d.podmanComposeAvail = func(context.Context) bool { return true }
		d.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	})

	err = ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "compose file") {
		t.Fatalf("unexpected error: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.platform = func() system.Platform {
			return system.Platform{GOOS: "darwin", PackageManager: "brew", ServiceManager: system.ServiceManagerNone}
		}
		d.commandExists = func(string) bool { return true }
		d.podmanComposeAvail = func(context.Context) bool { return true }
		d.podmanMachineStatus = func(context.Context) system.PodmanMachineState {
			return system.PodmanMachineState{Supported: true, Initialized: false, Running: false}
		}
	})

	err = ensureComposeRuntime(NewRootCmd(NewApp()), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "podman machine is not initialized") {
		t.Fatalf("unexpected darwin machine error: %v", err)
	}
}

func TestLoadRuntimeConfigWithoutFirstRunReturnsHint(t *testing.T) {
	withTestDeps(t, nil)

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeConfigPassesThroughUnexpectedLoadError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("boom") }
	})

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRuntimeConfigFailsValidation(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.dir", Message: "invalid"}}
		}
	})

	_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadStackContainersReturnsParseError(t *testing.T) {
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "{"}, nil
		}
	})

	_, err := loadStackContainers(context.Background(), configpkg.Default())
	if err == nil || !strings.Contains(err.Error(), "parse compose status output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadStackContainersUsesComposePSWhenAvailable(t *testing.T) {
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(_ context.Context, dir, name string, args ...string) (system.CommandResult, error) {
			if dir != configpkg.Default().Stack.Dir {
				t.Fatalf("unexpected working dir: %q", dir)
			}
			if name != "podman" {
				t.Fatalf("unexpected executable: %q", name)
			}
			want := []string{"compose", "-f", "/tmp/stackctl/compose.yaml", "ps", "--format", "json"}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("unexpected args:\n got: %q\nwant: %q", args, want)
			}
			return system.CommandResult{
				Stdout: "{\"ID\":\"postgres123\",\"Names\":\"local-postgres\",\"Status\":\"Up\",\"State\":\"running\"}\n",
			}, nil
		}
	})

	containers, err := loadStackContainers(context.Background(), configpkg.Default())
	if err != nil {
		t.Fatalf("loadStackContainers returned error: %v", err)
	}
	if len(containers) != 1 || containers[0].Names[0] != "local-postgres" {
		t.Fatalf("unexpected containers: %+v", containers)
	}
}

func TestHealthChecksWarnWhenContainersMissing(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	lines, err := healthChecks(context.Background(), configpkg.Default())
	if err != nil {
		t.Fatalf("healthChecks returned error: %v", err)
	}
	if got := lines[len(lines)-1].Message; got != "pgadmin container not found" {
		t.Fatalf("unexpected final health message: %s", got)
	}
}

func TestHealthChecksWarnWhenContainersNotRunning(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Names":["local-postgres"],"Image":"postgres:16","Status":"Exited","State":"exited","Ports":[]}]`,
			}, nil
		}
	})

	lines, err := healthChecks(context.Background(), configpkg.Default())
	if err != nil {
		t.Fatalf("healthChecks returned error: %v", err)
	}
	found := false
	for _, line := range lines {
		if line.Message == "postgres not running (Exited)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected postgres health warning, got %+v", lines)
	}
}

func TestHealthChecksWarnWhenPortIsBusyOutsideTheStack(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: marshalContainersJSON(system.Container{
				ID:     "postgres123456",
				Image:  "postgres:latest",
				Names:  []string{cfg.Services.PostgresContainer},
				Status: "Created",
				State:  "created",
			})}, nil
		}
		d.portInUse = func(port int) (bool, error) {
			return port == cfg.Ports.Postgres, nil
		}
	})

	lines, err := healthChecks(context.Background(), cfg)
	if err != nil {
		t.Fatalf("healthChecks returned error: %v", err)
	}

	foundConflict := false
	for _, line := range lines {
		if line.Message == "postgres port listening" && line.Status == output.StatusOK {
			t.Fatalf("expected health check to avoid a false-positive postgres listener: %+v", lines)
		}
		if line.Message == "port 5432 is in use by another process or container, not postgres" {
			foundConflict = true
		}
	}
	if !foundConflict {
		t.Fatalf("expected external port-conflict warning, got %+v", lines)
	}
}

func TestWaitForConfiguredServicesOnlyWaitsForCoreServicePorts(t *testing.T) {
	var ports []int
	cfg := configpkg.Default()
	cfg.Ports.Postgres = 15432
	cfg.Ports.Redis = 16379
	cfg.Ports.NATS = 14222
	cfg.Ports.PgAdmin = 18081
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			ports = append(ports, port)
			return nil
		}
	})

	if err := waitForConfiguredServices(context.Background(), cfg); err != nil {
		t.Fatalf("waitForConfiguredServices returned error: %v", err)
	}

	if !reflect.DeepEqual(ports, []int{15432, 16379, 14222}) {
		t.Fatalf("unexpected ports: %v", ports)
	}
}

func TestWaitForConfiguredServicesNamesTheFailingService(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Ports.Postgres = 15432
	cfg.Ports.Redis = 16379
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(_ context.Context, port int, _ time.Duration) error {
			if port == 16379 {
				return errors.New("context deadline exceeded")
			}
			return nil
		}
	})

	err := waitForConfiguredServices(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "redis port 16379 did not become ready") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintStatusTableNoContainers(t *testing.T) {
	root := NewRootCmd(NewApp())
	var out strings.Builder
	root.SetOut(&out)

	if err := printStatusTable(root, nil, false); err != nil {
		t.Fatalf("printStatusTable returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No containers from this stack were found.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestShortIDLeavesShortValuesUntouched(t *testing.T) {
	if got := shortID("12345"); got != "12345" {
		t.Fatalf("unexpected short id: %s", got)
	}
}
