package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

func TestStackUseQuietSuppressesSelectionMessages(t *testing.T) {
	var selected string

	withTestDeps(t, func(d *commandDeps) {
		d.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
	})

	stdout, _, err := executeRoot(t, "--quiet", "stack", "use", "staging")
	if err != nil {
		t.Fatalf("stack use returned error: %v", err)
	}
	if selected != "staging" {
		t.Fatalf("selected stack = %q", selected)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected quiet stack use output to be empty, got: %s", stdout)
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
			case "/tmp/stackctl/stacks/staging.yaml", "/tmp/stackctl-data/stacks/staging/compose.yaml":
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
		d.composePath = func(cfg configpkg.Config) string {
			return cfg.Stack.Dir + "/compose.yaml"
		}
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
		d.composeDown = func(_ context.Context, _ system.Runner, cfg configpkg.Config, removeVolumes bool) error {
			composeDownCalled = true
			if cfg.Stack.Dir != "/tmp/stackctl-data/stacks/staging" || !removeVolumes {
				t.Fatalf("unexpected compose down args: cfg=%+v removeVolumes=%v", cfg.Stack, removeVolumes)
			}
			return nil
		}
		d.anyContainerExists = func(context.Context, []string) (bool, error) { return false, nil }
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
			cfg.Setup.IncludeMeilisearch = true
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
	if savedConfig.Services.MeilisearchContainer != "stackctl-qa-meilisearch" {
		t.Fatalf("expected meilisearch container to retarget, got %q", savedConfig.Services.MeilisearchContainer)
	}
	if savedConfig.Services.Meilisearch.DataVolume != "stackctl-qa-meilisearch-data" {
		t.Fatalf("expected meilisearch data volume to retarget, got %q", savedConfig.Services.Meilisearch.DataVolume)
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
			cfg.Setup.IncludeMeilisearch = true
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
	if savedConfig.Services.MeilisearchContainer != "stackctl-demo-meilisearch" {
		t.Fatalf("expected meilisearch container to retarget, got %q", savedConfig.Services.MeilisearchContainer)
	}
	if savedConfig.Services.Meilisearch.DataVolume != "stackctl-demo-meilisearch-data" {
		t.Fatalf("expected meilisearch data volume to retarget, got %q", savedConfig.Services.Meilisearch.DataVolume)
	}
	if !scaffolded {
		t.Fatal("expected managed stack clone to scaffold target files")
	}
	if !strings.Contains(stdout, "cloned stack dev-stack to demo") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestStackCurrentPrintsSelectedStack(t *testing.T) {
	t.Setenv(configpkg.StackNameEnvVar, "staging")

	stdout, _, err := executeRoot(t, "stack", "current")
	if err != nil {
		t.Fatalf("stack current returned error: %v", err)
	}
	if strings.TrimSpace(stdout) != "staging" {
		t.Fatalf("unexpected stack current output: %q", stdout)
	}
}

func TestStackCompletionHelpers(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.knownConfigPaths = func() ([]string, error) {
			return []string{
				"/tmp/stackctl/config.yaml",
				"/tmp/stackctl/stacks/staging.yaml",
			}, nil
		}
	})

	completions, directive := completeSingleConfiguredStackArg(nil, nil, "st")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected single-stack directive: %v", directive)
	}
	choices := completionChoices(completions)
	if len(choices) != 1 || choices[0] != "staging" {
		t.Fatalf("unexpected single-stack completions: %+v", choices)
	}

	renameCompletions, renameDirective := completeRenameStackArgs(nil, nil, "de")
	if renameDirective != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected rename directive: %v", renameDirective)
	}
	if !containsChoice(completionChoices(renameCompletions), configpkg.DefaultStackName) {
		t.Fatalf("expected default stack completion in %+v", completionChoices(renameCompletions))
	}

	cloneCompletions, cloneDirective := completeCloneStackArgs(nil, nil, "")
	if cloneDirective != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected clone directive: %v", cloneDirective)
	}
	if len(cloneCompletions) < 2 {
		t.Fatalf("expected configured stack completions, got %+v", cloneCompletions)
	}

	noMoreCompletions, _ := completeRenameStackArgs(nil, []string{"staging"}, "")
	if len(noMoreCompletions) != 0 {
		t.Fatalf("expected no rename completions after the first arg, got %+v", noMoreCompletions)
	}
}

func TestStackDeletePromptIncludesManagedPurgeDetails(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

	prompt := stackDeletePrompt(stackTarget{
		Name:       "staging",
		ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
		Config:     cfg,
	}, true)

	for _, fragment := range []string{
		"Delete stack staging?",
		"Mode: managed",
		"Data dir: /tmp/stackctl-data/stacks/staging",
		"Continue?",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("stack delete prompt missing %q:\n%s", fragment, prompt)
		}
	}

	invalidPrompt := stackDeletePrompt(stackTarget{
		Name:       "broken",
		ConfigPath: "/tmp/stackctl/stacks/broken.yaml",
		LoadErr:    errors.New("bad config"),
	}, false)
	if !strings.Contains(invalidPrompt, "Config status: invalid") {
		t.Fatalf("expected invalid-config note in prompt:\n%s", invalidPrompt)
	}
}

func TestResolveStackArgHandlesDefaultAndInvalidNames(t *testing.T) {
	name, err := resolveStackArg("   ")
	if err != nil {
		t.Fatalf("resolveStackArg returned error for blank input: %v", err)
	}
	if name != configpkg.DefaultStackName {
		t.Fatalf("blank stack name should resolve to default, got %q", name)
	}

	if _, err := resolveStackArg("bad name"); err == nil {
		t.Fatal("expected invalid stack name to fail")
	}
}

func TestResolveStackTargetCoversMissingInvalidAndStatError(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(name string) (string, error) {
			return "/tmp/stackctl/stacks/" + name + ".yaml", nil
		}
		d.stat = func(path string) (os.FileInfo, error) {
			switch path {
			case "/tmp/stackctl/stacks/missing.yaml":
				return nil, os.ErrNotExist
			case "/tmp/stackctl/stacks/invalid.yaml":
				return fakeFileInfo{name: "invalid.yaml"}, nil
			default:
				return nil, errors.New("boom")
			}
		}
		d.loadConfig = func(path string) (configpkg.Config, error) {
			if path == "/tmp/stackctl/stacks/invalid.yaml" {
				return configpkg.Config{}, errors.New("bad config")
			}
			t.Fatalf("unexpected loadConfig path: %s", path)
			return configpkg.Config{}, nil
		}
	})

	missing, err := resolveStackTarget("missing")
	if err != nil {
		t.Fatalf("missing stack target returned error: %v", err)
	}
	if missing.Exists {
		t.Fatalf("missing stack should not exist: %+v", missing)
	}

	invalid, err := resolveStackTarget("invalid")
	if err != nil {
		t.Fatalf("invalid stack target returned error: %v", err)
	}
	if !invalid.Exists || invalid.LoadErr == nil {
		t.Fatalf("expected invalid stack target with load error, got %+v", invalid)
	}

	if _, err := resolveStackTarget("broken"); err == nil || !strings.Contains(err.Error(), "check stack config") {
		t.Fatalf("expected stat failure to bubble up, got %v", err)
	}
}

func TestDeleteStackTargetTracksManagedDirAndSelectionReset(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

	var removedConfig string
	var selected string

	withTestDeps(t, func(d *commandDeps) {
		d.removeFile = func(path string) error {
			removedConfig = path
			return nil
		}
		d.currentStackName = func() (string, error) { return "staging", nil }
		d.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
	})

	result, err := deleteStackTarget(context.Background(), stackTarget{
		Name:       "staging",
		ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
		Config:     cfg,
		LoadErr:    nil,
	}, false)
	if err != nil {
		t.Fatalf("deleteStackTarget returned error: %v", err)
	}
	if removedConfig != "/tmp/stackctl/stacks/staging.yaml" {
		t.Fatalf("unexpected removed config path: %q", removedConfig)
	}
	if selected != configpkg.DefaultStackName {
		t.Fatalf("expected current stack to reset to default, got %q", selected)
	}
	if result.ManagedDataKept != cfg.Stack.Dir || !result.ResetToDefault {
		t.Fatalf("unexpected delete result: %+v", result)
	}
}

func TestMoveManagedStackDirCoversHelperBranches(t *testing.T) {
	withTestDeps(t, nil)

	if err := moveManagedStackDir("/tmp/source", "/tmp/source"); err != nil {
		t.Fatalf("same source/target should no-op: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/missing" {
				return nil, os.ErrNotExist
			}
			return fakeFileInfo{name: filepath.Base(path)}, nil
		}
	})
	if err := moveManagedStackDir("/tmp/missing", "/tmp/target"); err != nil {
		t.Fatalf("missing source should no-op: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/source" {
				return nil, errors.New("stat failed")
			}
			return fakeFileInfo{name: filepath.Base(path)}, nil
		}
	})
	if err := moveManagedStackDir("/tmp/source", "/tmp/target"); err == nil || !strings.Contains(err.Error(), "check managed stack directory") {
		t.Fatalf("expected source stat failure, got %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/source" || path == "/tmp/target" {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			return nil, os.ErrNotExist
		}
	})
	if err := moveManagedStackDir("/tmp/source", "/tmp/target"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected target exists failure, got %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/source" {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			if path == "/tmp/target" {
				return nil, errors.New("target stat failed")
			}
			return nil, os.ErrNotExist
		}
	})
	if err := moveManagedStackDir("/tmp/source", "/tmp/target"); err == nil || !strings.Contains(err.Error(), "check target managed stack directory") {
		t.Fatalf("expected target stat failure, got %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/source" {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			if path == "/tmp/target" {
				return nil, os.ErrNotExist
			}
			return nil, os.ErrNotExist
		}
		d.mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir failed") }
	})
	if err := moveManagedStackDir("/tmp/source", "/tmp/target"); err == nil || !strings.Contains(err.Error(), "create managed stack directory parent") {
		t.Fatalf("expected mkdir failure, got %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/source" {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			if path == "/tmp/target" {
				return nil, os.ErrNotExist
			}
			return nil, os.ErrNotExist
		}
		d.rename = func(string, string) error { return errors.New("rename failed") }
	})
	if err := moveManagedStackDir("/tmp/source", "/tmp/target"); err == nil || !strings.Contains(err.Error(), "rename managed stack directory") {
		t.Fatalf("expected rename failure, got %v", err)
	}

	renamed := false
	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/source" {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			if path == "/tmp/target" {
				return nil, os.ErrNotExist
			}
			return nil, os.ErrNotExist
		}
		d.rename = func(source, target string) error {
			renamed = source == "/tmp/source" && target == "/tmp/target"
			return nil
		}
	})
	if err := moveManagedStackDir("/tmp/source", "/tmp/target"); err != nil {
		t.Fatalf("expected moveManagedStackDir success, got %v", err)
	}
	if !renamed {
		t.Fatal("expected rename to be called for successful move")
	}
}

func TestScaffoldManagedStackFilesCoversResultModes(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

	withTestDeps(t, func(d *commandDeps) {
		d.scaffoldManagedStack = func(cfg configpkg.Config, _ bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{
				CreatedDir:   true,
				WroteCompose: true,
				StackDir:     cfg.Stack.Dir,
				ComposePath:  cfg.Stack.Dir + "/compose.yaml",
			}, nil
		}
	})

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := scaffoldManagedStackFiles(cmd, cfg, false); err != nil {
		t.Fatalf("scaffoldManagedStackFiles returned error: %v", err)
	}
	if !strings.Contains(out.String(), "created managed stack directory") || !strings.Contains(out.String(), "wrote managed compose file") {
		t.Fatalf("unexpected scaffold output: %s", out.String())
	}

	out.Reset()
	withTestDeps(t, func(d *commandDeps) {
		d.scaffoldManagedStack = func(cfg configpkg.Config, _ bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{
				AlreadyPresent: true,
				ComposePath:    cfg.Stack.Dir + "/compose.yaml",
			}, nil
		}
	})
	if err := scaffoldManagedStackFiles(cmd, cfg, false); err != nil {
		t.Fatalf("scaffoldManagedStackFiles returned error for already-present path: %v", err)
	}
	if !strings.Contains(out.String(), "managed stack already exists") {
		t.Fatalf("expected already-present scaffold message, got %q", out.String())
	}

	nonManaged := configpkg.Default()
	if err := scaffoldManagedStackFiles(cmd, nonManaged, false); err != nil {
		t.Fatalf("non-managed scaffold should no-op, got %v", err)
	}
}

func TestStackComposeFileExistsRejectsDirectoriesAndErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.stat = func(path string) (os.FileInfo, error) {
			switch path {
			case "/tmp/file":
				return fakeFileInfo{name: "compose.yaml"}, nil
			case "/tmp/dir":
				return fakeFileInfo{name: "compose", dir: true}, nil
			default:
				return nil, os.ErrNotExist
			}
		}
	})

	if !stackComposeFileExists("/tmp/file") {
		t.Fatal("expected regular file to exist")
	}
	if stackComposeFileExists("/tmp/dir") {
		t.Fatal("directories should not count as compose files")
	}
	if stackComposeFileExists("/tmp/missing") {
		t.Fatal("missing files should not count as compose files")
	}
}

func TestDeleteStackTargetHandlesConfigRemovalEdgeCases(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

	t.Run("ignore missing config file", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.removeFile = func(string) error { return os.ErrNotExist }
			d.currentStackName = func() (string, error) { return "other", nil }
		})

		result, err := deleteStackTarget(context.Background(), stackTarget{
			Name:       "staging",
			ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
			Config:     cfg,
		}, false)
		if err != nil {
			t.Fatalf("deleteStackTarget returned error: %v", err)
		}
		if result.ResetToDefault {
			t.Fatalf("unexpected reset result: %+v", result)
		}
	})

	t.Run("surface config removal errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.removeFile = func(string) error { return errors.New("rm failed") }
		})

		if _, err := deleteStackTarget(context.Background(), stackTarget{
			Name:       "staging",
			ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
			Config:     cfg,
		}, false); err == nil || !strings.Contains(err.Error(), "delete stack config") {
			t.Fatalf("expected delete error, got %v", err)
		}
	})
}

func TestPurgeManagedStackPreconditionsCoversGuardrails(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

	t.Run("requires managed stack", func(t *testing.T) {
		external := cfg
		external.Stack.Managed = false
		if err := purgeManagedStackPreconditions(context.Background(), external); err == nil || !strings.Contains(err.Error(), "managed stack") {
			t.Fatalf("expected managed-stack error, got %v", err)
		}
	})

	t.Run("surfaces data dir lookup errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "", errors.New("no data dir") }
		})

		if err := purgeManagedStackPreconditions(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "no data dir") {
			t.Fatalf("expected data dir error, got %v", err)
		}
	})

	t.Run("rejects paths outside the managed root", func(t *testing.T) {
		outside := cfg
		outside.Stack.Dir = "/tmp/elsewhere/staging"

		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
		})

		if err := purgeManagedStackPreconditions(context.Background(), outside); err == nil || !strings.Contains(err.Error(), "outside stackctl data dir") {
			t.Fatalf("expected outside-root error, got %v", err)
		}
	})

	t.Run("requires manual stop when running compose file is missing", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
			d.composePath = func(configpkg.Config) string { return "/tmp/stackctl-data/stacks/staging/compose.yaml" }
			d.stat = func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
		})

		if err := purgeManagedStackPreconditions(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "stop it manually before deleting") {
			t.Fatalf("expected missing compose error, got %v", err)
		}
	})

	t.Run("allows runtime detection errors when compose file exists", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
			d.composePath = func(configpkg.Config) string { return "/tmp/stackctl-data/stacks/staging/compose.yaml" }
			d.stat = func(path string) (os.FileInfo, error) {
				if path == "/tmp/stackctl-data/stacks/staging/compose.yaml" {
					return fakeFileInfo{name: "compose.yaml"}, nil
				}
				return nil, os.ErrNotExist
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{}, errors.New("podman ps failed")
			}
		})

		if err := purgeManagedStackPreconditions(context.Background(), cfg); err != nil {
			t.Fatalf("expected compose-presence fallback, got %v", err)
		}
	})
}

func TestPurgeManagedStackLocalStateQuietHandlesComposeAndRemovalPaths(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"
	composePath := "/tmp/stackctl-data/stacks/staging/compose.yaml"

	t.Run("surfaces precondition failures", func(t *testing.T) {
		external := cfg
		external.Stack.Managed = false
		if _, err := purgeManagedStackLocalStateQuiet(context.Background(), external); err == nil || !strings.Contains(err.Error(), "managed stack") {
			t.Fatalf("expected precondition error, got %v", err)
		}
	})

	t.Run("wraps compose down failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
			d.composePath = func(configpkg.Config) string { return composePath }
			d.stat = func(path string) (os.FileInfo, error) {
				if path == composePath {
					return fakeFileInfo{name: "compose.yaml"}, nil
				}
				return nil, os.ErrNotExist
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("compose down failed")
			}
		})

		if _, err := purgeManagedStackLocalStateQuiet(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "tear down managed stack") {
			t.Fatalf("expected compose down error, got %v", err)
		}
	})

	t.Run("wraps managed dir removal failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
			d.composePath = func(configpkg.Config) string { return composePath }
			d.stat = func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
			d.removeAll = func(string) error { return errors.New("remove failed") }
		})

		if _, err := purgeManagedStackLocalStateQuiet(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "remove managed stack dir") {
			t.Fatalf("expected removeAll error, got %v", err)
		}
	})

	t.Run("removes local state after successful teardown", func(t *testing.T) {
		composeDownCalled := false
		removed := ""

		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
			d.composePath = func(configpkg.Config) string { return composePath }
			d.stat = func(path string) (os.FileInfo, error) {
				if path == composePath {
					return fakeFileInfo{name: "compose.yaml"}, nil
				}
				return nil, os.ErrNotExist
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				composeDownCalled = true
				return nil
			}
			d.anyContainerExists = func(context.Context, []string) (bool, error) { return false, nil }
			d.removeAll = func(path string) error {
				removed = path
				return nil
			}
		})

		removedDir, err := purgeManagedStackLocalStateQuiet(context.Background(), cfg)
		if err != nil {
			t.Fatalf("purgeManagedStackLocalStateQuiet returned error: %v", err)
		}
		if !composeDownCalled {
			t.Fatal("expected compose down to run when compose file exists")
		}
		if removed != cfg.Stack.Dir || removedDir != cfg.Stack.Dir {
			t.Fatalf("unexpected removed dir values: removed=%q removedDir=%q", removed, removedDir)
		}
	})
}
