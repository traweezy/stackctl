package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestStackHelpGroupsCommands(t *testing.T) {
	stdout, _, err := executeRoot(t, "stack", "--help")
	if err != nil {
		t.Fatalf("stack help returned error: %v", err)
	}

	for _, fragment := range []string{
		"Inspect Commands",
		"Selection Commands",
		"Maintenance Commands",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stack help missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestStackUseUpdatesSelection(t *testing.T) {
	var selected string

	withTestDeps(t, func(d *commandDeps) {
		d.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
	})

	stdout, _, err := executeRoot(t, "stack", "use", "staging")
	if err != nil {
		t.Fatalf("stack use returned error: %v", err)
	}
	if selected != "staging" {
		t.Fatalf("selected stack = %q", selected)
	}
	if !strings.Contains(stdout, "selected stack staging") {
		t.Fatalf("stdout missing selection message: %s", stdout)
	}
	if !strings.Contains(stdout, "run `stackctl config init`") {
		t.Fatalf("stdout missing init hint: %s", stdout)
	}
}

func TestStackListShowsCurrentMissingStack(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.knownConfigPaths = func() ([]string, error) {
			return []string{"/tmp/stackctl/config.yaml"}, nil
		}
		d.loadConfig = func(path string) (configpkg.Config, error) {
			if path == "/tmp/stackctl/config.yaml" {
				return configpkg.Default(), nil
			}
			return configpkg.Config{}, configpkg.ErrNotFound
		}
	})

	stdout, _, err := executeRoot(t, "--stack", "staging", "stack", "list")
	if err != nil {
		t.Fatalf("stack list returned error: %v", err)
	}
	if !strings.Contains(stdout, "staging") || !strings.Contains(stdout, "not configured") {
		t.Fatalf("unexpected stack list output:\n%s", stdout)
	}
}

func TestStackListShowsConfiguredServicesForStoppedStacks(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.knownConfigPaths = func() ([]string, error) {
			return []string{"/tmp/stackctl/stacks/staging.yaml"}, nil
		}
		d.loadConfig = func(path string) (configpkg.Config, error) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.Setup.IncludeNATS = false
			cfg.Setup.IncludePgAdmin = false
			cfg.ApplyDerivedFields()
			return cfg, nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	stdout, _, err := executeRoot(t, "stack", "list")
	if err != nil {
		t.Fatalf("stack list returned error: %v", err)
	}
	for _, fragment := range []string{
		"staging",
		"stopped",
		"Postgres, Redis",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("stack list output missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestStackDeleteRefusesRunningStackWithoutPurgeData(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/stackctl/stacks/staging.yaml" {
				return fakeFileInfo{name: "staging.yaml"}, nil
			}
			return nil, os.ErrNotExist
		}
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"
			return cfg, nil
		}
		d.captureResult = func(_ context.Context, _ string, name string, _ ...string) (system.CommandResult, error) {
			if name != "podman" {
				return system.CommandResult{}, errors.New("unexpected command")
			}
			return system.CommandResult{
				Stdout: `[{"Id":"abcdef","Names":["stackctl-staging-postgres"],"Image":"postgres:16","Status":"Up","State":"running","Ports":[],"CreatedAt":"now"}]`,
			}, nil
		}
	})

	_, _, err := executeRoot(t, "stack", "delete", "staging", "--force")
	if err == nil || !strings.Contains(err.Error(), "--purge-data") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStackDeletePurgeManagedStackRemovesDataAndResetsSelection(t *testing.T) {
	var removedConfig string
	var removedData string
	var resetSelection string
	composeDownCalled := false

	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
		d.currentStackName = func() (string, error) { return "staging", nil }
		d.setCurrentStackName = func(name string) error {
			resetSelection = name
			return nil
		}
		d.stat = func(path string) (os.FileInfo, error) {
			switch path {
			case "/tmp/stackctl/stacks/staging.yaml", "/tmp/stackctl/compose.yaml":
				return fakeFileInfo{name: filepath.Base(path)}, nil
			default:
				return nil, os.ErrNotExist
			}
		}
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"
			return cfg, nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
		d.composeDownPath = func(_ context.Context, _ system.Runner, dir, composePath string, removeVolumes bool) error {
			composeDownCalled = true
			if dir != "/tmp/stackctl-data/stacks/staging" || composePath != "/tmp/stackctl/compose.yaml" || !removeVolumes {
				t.Fatalf("unexpected compose down args: dir=%s compose=%s removeVolumes=%v", dir, composePath, removeVolumes)
			}
			return nil
		}
		d.removeAll = func(path string) error {
			removedData = path
			return nil
		}
		d.removeFile = func(path string) error {
			removedConfig = path
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "stack", "delete", "staging", "--purge-data", "--force")
	if err != nil {
		t.Fatalf("stack delete returned error: %v", err)
	}
	if !composeDownCalled {
		t.Fatal("expected stack delete to stop the managed stack")
	}
	if removedData != "/tmp/stackctl-data/stacks/staging" {
		t.Fatalf("removed data dir = %q", removedData)
	}
	if removedConfig != "/tmp/stackctl/stacks/staging.yaml" {
		t.Fatalf("removed config path = %q", removedConfig)
	}
	if resetSelection != configpkg.DefaultStackName {
		t.Fatalf("reset selection = %q", resetSelection)
	}
	if !strings.Contains(stdout, "selected stack reset to dev-stack") {
		t.Fatalf("stdout missing selection reset: %s", stdout)
	}
}

func TestStackRenameManagedStackUpdatesSelection(t *testing.T) {
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	sourcePath := filepath.Join(configRoot, "stackctl", "stacks", "staging.yaml")
	targetPath := filepath.Join(configRoot, "stackctl", "stacks", "qa.yaml")
	sourceDir := filepath.Join(dataRoot, "stackctl", "stacks", "staging")
	targetDir := filepath.Join(dataRoot, "stackctl", "stacks", "qa")

	var savedPath string
	var savedConfig configpkg.Config
	var removedPath string
	var renamedFrom string
	var renamedTo string
	var selected string

	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(name string) (string, error) {
			switch name {
			case "staging":
				return sourcePath, nil
			case "qa":
				return targetPath, nil
			default:
				return "", errors.New("unexpected stack")
			}
		}
		d.currentStackName = func() (string, error) { return "staging", nil }
		d.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
		d.stat = func(path string) (os.FileInfo, error) {
			switch path {
			case sourcePath:
				return fakeFileInfo{name: "staging.yaml"}, nil
			case targetPath:
				return nil, os.ErrNotExist
			case sourceDir:
				return fakeFileInfo{name: "staging", dir: true}, nil
			case targetDir:
				return nil, os.ErrNotExist
			default:
				return nil, os.ErrNotExist
			}
		}
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.Setup.IncludeSeaweedFS = true
			cfg.ApplyDerivedFields()
			cfg.Stack.Dir = sourceDir
			return cfg, nil
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
		d.saveConfig = func(path string, cfg configpkg.Config) error {
			savedPath = path
			savedConfig = cfg
			return nil
		}
		d.rename = func(from, to string) error {
			renamedFrom = from
			renamedTo = to
			return nil
		}
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			if !force {
				t.Fatal("expected managed stack rename to force scaffold refresh")
			}
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
		d.removeFile = func(path string) error {
			removedPath = path
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "stack", "rename", "staging", "qa")
	if err != nil {
		t.Fatalf("stack rename returned error: %v", err)
	}
	if savedPath != targetPath {
		t.Fatalf("saved path = %q", savedPath)
	}
	if savedConfig.Stack.Name != "qa" || savedConfig.Stack.Dir != targetDir {
		t.Fatalf("unexpected saved config: %+v", savedConfig.Stack)
	}
	if savedConfig.Services.SeaweedFSContainer != "stackctl-qa-seaweedfs" {
		t.Fatalf("expected seaweedfs container to retarget, got %q", savedConfig.Services.SeaweedFSContainer)
	}
	if savedConfig.Services.SeaweedFS.DataVolume != "stackctl-qa-seaweedfs-data" {
		t.Fatalf("expected seaweedfs data volume to retarget, got %q", savedConfig.Services.SeaweedFS.DataVolume)
	}
	if renamedFrom != sourceDir || renamedTo != targetDir {
		t.Fatalf("unexpected dir rename: %s -> %s", renamedFrom, renamedTo)
	}
	if removedPath != sourcePath {
		t.Fatalf("removed path = %q", removedPath)
	}
	if selected != "qa" {
		t.Fatalf("selected stack = %q", selected)
	}
	if !strings.Contains(stdout, "renamed stack staging to qa") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestStackCloneManagedStackCreatesNewConfig(t *testing.T) {
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	sourcePath := filepath.Join(configRoot, "stackctl", "config.yaml")
	targetPath := filepath.Join(configRoot, "stackctl", "stacks", "demo.yaml")
	targetDir := filepath.Join(dataRoot, "stackctl", "stacks", "demo")

	var savedPath string
	var savedConfig configpkg.Config
	scaffolded := false

	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(name string) (string, error) {
			switch name {
			case configpkg.DefaultStackName:
				return sourcePath, nil
			case "demo":
				return targetPath, nil
			default:
				return "", errors.New("unexpected stack")
			}
		}
		d.stat = func(path string) (os.FileInfo, error) {
			switch path {
			case sourcePath:
				return fakeFileInfo{name: "config.yaml"}, nil
			case targetPath, targetDir:
				return nil, os.ErrNotExist
			default:
				return nil, os.ErrNotExist
			}
		}
		d.loadConfig = func(string) (configpkg.Config, error) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeSeaweedFS = true
			cfg.ApplyDerivedFields()
			cfg.Stack.Dir = filepath.Join(dataRoot, "stackctl", "stacks", "dev-stack")
			return cfg, nil
		}
		d.saveConfig = func(path string, cfg configpkg.Config) error {
			savedPath = path
			savedConfig = cfg
			return nil
		}
		d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			scaffolded = true
			if force {
				t.Fatal("expected clone scaffold to avoid force")
			}
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		}
	})

	stdout, _, err := executeRoot(t, "stack", "clone", "dev-stack", "demo")
	if err != nil {
		t.Fatalf("stack clone returned error: %v", err)
	}
	if savedPath != targetPath {
		t.Fatalf("saved path = %q", savedPath)
	}
	if savedConfig.Stack.Name != "demo" || savedConfig.Stack.Dir != targetDir {
		t.Fatalf("unexpected saved config: %+v", savedConfig.Stack)
	}
	if savedConfig.Services.SeaweedFSContainer != "stackctl-demo-seaweedfs" {
		t.Fatalf("expected seaweedfs container to retarget, got %q", savedConfig.Services.SeaweedFSContainer)
	}
	if savedConfig.Services.SeaweedFS.DataVolume != "stackctl-demo-seaweedfs-data" {
		t.Fatalf("expected seaweedfs data volume to retarget, got %q", savedConfig.Services.SeaweedFS.DataVolume)
	}
	if !scaffolded {
		t.Fatal("expected managed stack clone to scaffold target files")
	}
	if !strings.Contains(stdout, "cloned stack dev-stack to demo") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}
