package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestTUIStackAndLifecycleErrorPaths(t *testing.T) {
	t.Run("pendingManagedScaffoldIssue covers managed-stack-dir failures", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", "")
		cfg := configpkg.Default()
		cfg.Stack.Name = ""
		if issue := pendingManagedScaffoldIssue(cfg, configpkg.ValidationIssue{Field: "stack.dir", Message: "directory does not exist"}); issue {
			t.Fatal("expected pendingManagedScaffoldIssue to return false when the managed stack dir cannot be resolved")
		}

		cfg = configpkg.Default()
		cfg.Stack.Dir = "/tmp/custom-stack"
		if issue := pendingManagedScaffoldIssue(cfg, configpkg.ValidationIssue{Field: "stack.dir", Message: "directory does not exist: /tmp/custom-stack"}); issue {
			t.Fatal("expected pendingManagedScaffoldIssue to ignore stacks outside the managed default path")
		}
	})

	t.Run("tui command paths cover debug log failures and canceled handoffs", func(t *testing.T) {
		if _, _, err := executeRoot(t, "tui", "--debug-log-file", t.TempDir()); err == nil {
			t.Fatal("expected stackctl tui to surface debug-log open failures")
		}

		originalTuiNotifyContext := tuiNotifyContext
		t.Cleanup(func() { tuiNotifyContext = originalTuiNotifyContext })
		tuiNotifyContext = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, func() {}
		}

		withTestDeps(t, func(d *commandDeps) {
			d.composeLogs = func(context.Context, system.Runner, configpkg.Config, int, bool, string, string) error {
				return errors.New("logs failed")
			}
			d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
				return syscall.EINTR
			}
		})

		if err := (&tuiLogWatchCommand{cfg: configpkg.Default(), service: "postgres"}).Run(); err != nil {
			t.Fatalf("expected canceled tui log watch to suppress errors, got %v", err)
		}
		if err := (&tuiServiceShellCommand{cfg: configpkg.Default(), service: "postgres"}).Run(); err != nil {
			t.Fatalf("expected canceled tui service shell to suppress errors, got %v", err)
		}
		if err := (&tuiDBShellCommand{cfg: configpkg.Default()}).Run(); err != nil {
			t.Fatalf("expected canceled tui db shell to suppress errors, got %v", err)
		}
	})

	t.Run("default command deps closures cover compose helper branches", func(t *testing.T) {
		defaults := defaultCommandDeps()
		if err := defaults.composeLogs(context.Background(), system.Runner{}, configpkg.Default(), 10, false, "", "bogus"); err == nil {
			t.Fatal("expected default composeLogs helper to reject invalid services")
		}
		_ = defaults.composeDownPath(context.Background(), system.Runner{}, "", "compose.yaml", false)
	})

	t.Run("stack delete and lifecycle waiting cover remaining branch paths", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "external.yaml"}, nil }
			d.loadConfig = func(path string) (configpkg.Config, error) {
				if strings.Contains(path, "external") {
					cfg := configpkg.DefaultForStack("external")
					cfg.Stack.Managed = false
					cfg.ApplyDerivedFields()
					return cfg, nil
				}
				return configpkg.Default(), nil
			}
		})

		_, _, err := executeRoot(t, "stack", "delete", "external", "--purge-data", "--force")
		if err == nil || !strings.Contains(err.Error(), "external stack") {
			t.Fatalf("expected external purge-data rejection, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.configFilePathForStack = func(name string) (string, error) { return "/tmp/stackctl/stacks/" + name + ".yaml", nil }
			d.stat = func(path string) (os.FileInfo, error) { return fakeFileInfo{name: filepath.Base(path)}, nil }
			d.loadConfig = func(string) (configpkg.Config, error) {
				cfg := configpkg.DefaultForStack("staging")
				cfg.Stack.Managed = true
				cfg.ApplyDerivedFields()
				return cfg, nil
			}
			d.dataDirPath = func() (string, error) { return "", errors.New("data dir failed") }
		})
		_, _, err = executeRoot(t, "stack", "delete", "staging", "--purge-data", "--force")
		if err == nil || !strings.Contains(err.Error(), "data dir failed") {
			t.Fatalf("expected stack delete purge-data precondition error, got %v", err)
		}

		cfg := configpkg.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := waitForStackService(ctx, cfg, serviceDefinition{
			Key:         "postgres",
			Kind:        serviceKindStack,
			PrimaryPort: func(configpkg.Config) int { return 5432 },
			ContainerName: func(configpkg.Config) string {
				return "stack-postgres"
			},
		}); err == nil || !strings.Contains(err.Error(), "did not become ready") {
			t.Fatalf("expected waitForStackService timeout, got %v", err)
		}
	})

	t.Run("runTUIRestart and runTUIDeleteStack cover remaining error branches", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		withTestDeps(t, func(value *commandDeps) {
			value.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
				return false, errors.New("scaffold failed")
			}
		})
		if _, err := runTUIRestart(cfg, nil); err == nil || !strings.Contains(err.Error(), "scaffold failed") {
			t.Fatalf("expected scaffold sync error, got %v", err)
		}

		withTestDeps(t, func(value *commandDeps) {
			value.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, nil }
			value.configFilePath = func() (string, error) { return "", errors.New("config path failed") }
		})
		if _, err := runTUIRestart(cfg, nil); err == nil || !strings.Contains(err.Error(), "config path failed") {
			t.Fatalf("expected running-stack guard error, got %v", err)
		}

		originalSetStackNameEnv := setStackNameEnv
		t.Cleanup(func() { setStackNameEnv = originalSetStackNameEnv })
		setStackNameEnv = func(string, string) error { return errors.New("setenv failed") }
		withTestDeps(t, func(value *commandDeps) {
			value.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
			value.loadConfig = func(string) (configpkg.Config, error) {
				stack := configpkg.DefaultForStack("staging")
				stack.Stack.Managed = true
				stack.ApplyDerivedFields()
				return stack, nil
			}
			value.currentStackName = func() (string, error) { return "staging", nil }
			value.dataDirPath = func() (string, error) { return "/home/tylers/.local/share/stackctl", nil }
			value.removeFile = func(string) error { return nil }
			value.removeAll = func(string) error { return nil }
		})
		if _, err := runTUIDeleteStack("staging"); err == nil || !strings.Contains(err.Error(), "setenv failed") {
			t.Fatalf("expected setenv failure, got %v", err)
		}
	})
}
