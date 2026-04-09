package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestStackAdditionalCoverageBatchThree(t *testing.T) {
	t.Run("invalid stack names surface from stack subcommands", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
		}{
			{name: "use", args: []string{"stack", "use", "Bad"}},
			{name: "delete", args: []string{"stack", "delete", "Bad", "--force"}},
			{name: "rename source", args: []string{"stack", "rename", "Bad", "target"}},
			{name: "rename target", args: []string{"stack", "rename", "source", "Bad"}},
			{name: "clone source", args: []string{"stack", "clone", "Bad", "target"}},
			{name: "clone target", args: []string{"stack", "clone", "source", "Bad"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := executeRoot(t, tc.args...)
				if err == nil || !strings.Contains(err.Error(), "invalid stack name") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("stack delete covers command output and failure branches", func(t *testing.T) {
		stackCfg := configpkg.DefaultForStack("staging")
		stackCfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

		withDeleteDeps := func(t *testing.T, mutate func(*commandDeps)) {
			t.Helper()
			withTestDeps(t, func(d *commandDeps) {
				d.configFilePathForStack = func(name string) (string, error) {
					return "/tmp/stackctl/stacks/" + name + ".yaml", nil
				}
				d.stat = func(path string) (os.FileInfo, error) {
					switch path {
					case "/tmp/stackctl/stacks/staging.yaml":
						return fakeFileInfo{name: "staging.yaml"}, nil
					case stackCfg.Stack.Dir + "/compose.yaml":
						return nil, os.ErrNotExist
					default:
						return nil, os.ErrNotExist
					}
				}
				d.loadConfig = func(string) (configpkg.Config, error) { return stackCfg, nil }
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				if mutate != nil {
					mutate(d)
				}
			})
		}

		t.Run("warns when managed data remains", func(t *testing.T) {
			withDeleteDeps(t, func(d *commandDeps) {
				d.currentStackName = func() (string, error) { return "other", nil }
			})

			stdout, _, err := executeRoot(t, "stack", "delete", "staging", "--force")
			if err != nil {
				t.Fatalf("stack delete returned error: %v", err)
			}
			for _, fragment := range []string{
				"deleted stack config /tmp/stackctl/stacks/staging.yaml",
				"managed stack data remains at /tmp/stackctl-data/stacks/staging",
			} {
				if !strings.Contains(stdout, fragment) {
					t.Fatalf("stack delete output missing %q:\n%s", fragment, stdout)
				}
			}
		})

		t.Run("surfaces purge failures after delete target resolution", func(t *testing.T) {
			withDeleteDeps(t, func(d *commandDeps) {
				d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
				d.composePath = func(configpkg.Config) string { return stackCfg.Stack.Dir + "/compose.yaml" }
				d.removeAll = func(string) error { return errors.New("remove failed") }
			})

			_, _, err := executeRoot(t, "stack", "delete", "staging", "--purge-data", "--force")
			if err == nil || !strings.Contains(err.Error(), "remove managed stack dir") {
				t.Fatalf("unexpected purge failure: %v", err)
			}
		})

		t.Run("propagates individual delete status write failures", func(t *testing.T) {
			withDeleteDeps(t, func(d *commandDeps) {
				d.dataDirPath = func() (string, error) { return "/tmp/stackctl-data", nil }
				d.composePath = func(configpkg.Config) string { return stackCfg.Stack.Dir + "/compose.yaml" }
				d.removeAll = func(string) error { return nil }
				d.currentStackName = func() (string, error) { return "staging", nil }
				d.setCurrentStackName = func(string) error { return nil }
			})

			if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "deleted managed stack data"}, io.Discard, "stack", "delete", "staging", "--purge-data", "--force"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected managed-data status write failure, got %v", err)
			}
			if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "deleted stack config"}, io.Discard, "stack", "delete", "staging", "--purge-data", "--force"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected config status write failure, got %v", err)
			}
			if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "selected stack reset to"}, io.Discard, "stack", "delete", "staging", "--purge-data", "--force"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected selection reset write failure, got %v", err)
			}
		})
	})

	t.Run("stack rename covers remaining managed and success branches", func(t *testing.T) {
		sourcePath := "/tmp/stackctl/stacks/source.yaml"
		targetPath := "/tmp/stackctl/stacks/target.yaml"
		sourceCfg := configpkg.DefaultForStack("source")
		targetCfg := retargetStackConfig(sourceCfg, "target")

		withRenameDeps := func(t *testing.T, mutate func(*commandDeps)) {
			t.Helper()
			withTestDeps(t, func(d *commandDeps) {
				d.configFilePathForStack = func(name string) (string, error) {
					switch name {
					case "source":
						return sourcePath, nil
					case "target":
						return targetPath, nil
					default:
						return "/tmp/stackctl/stacks/" + name + ".yaml", nil
					}
				}
				d.loadConfig = func(string) (configpkg.Config, error) { return sourceCfg, nil }
				d.saveConfig = func(string, configpkg.Config) error { return nil }
				d.rename = func(string, string) error { return nil }
				d.removeFile = func(string) error { return nil }
				d.currentStackName = func() (string, error) { return "other", nil }
				d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
					return configpkg.ScaffoldResult{
						StackDir:     cfg.Stack.Dir,
						ComposePath:  configpkg.ComposePath(cfg),
						CreatedDir:   true,
						WroteCompose: true,
					}, nil
				}
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				d.stat = func(path string) (os.FileInfo, error) {
					switch path {
					case sourcePath:
						return fakeFileInfo{name: "source.yaml"}, nil
					case targetPath, targetCfg.Stack.Dir:
						return nil, os.ErrNotExist
					case sourceCfg.Stack.Dir:
						return fakeFileInfo{name: "source", dir: true}, nil
					default:
						return nil, os.ErrNotExist
					}
				}
				if mutate != nil {
					mutate(d)
				}
			})
		}

		t.Run("surfaces source target resolution failures", func(t *testing.T) {
			withRenameDeps(t, func(d *commandDeps) {
				d.stat = func(path string) (os.FileInfo, error) {
					if path == sourcePath {
						return nil, errors.New("source stat failed")
					}
					return nil, os.ErrNotExist
				}
			})

			_, _, err := executeRoot(t, "stack", "rename", "source", "target")
			if err == nil || !strings.Contains(err.Error(), "check stack config") {
				t.Fatalf("unexpected rename resolution error: %v", err)
			}
		})

		t.Run("removes the new config when moving the managed dir fails", func(t *testing.T) {
			removed := ""
			withRenameDeps(t, func(d *commandDeps) {
				d.stat = func(path string) (os.FileInfo, error) {
					switch path {
					case sourcePath:
						return fakeFileInfo{name: "source.yaml"}, nil
					case targetPath:
						return nil, os.ErrNotExist
					case sourceCfg.Stack.Dir, targetCfg.Stack.Dir:
						return fakeFileInfo{name: filepath.Base(path), dir: true}, nil
					default:
						return nil, os.ErrNotExist
					}
				}
				d.removeFile = func(path string) error {
					removed = path
					return nil
				}
			})

			_, _, err := executeRoot(t, "stack", "rename", "source", "target")
			if err == nil || !strings.Contains(err.Error(), "managed stack directory") {
				t.Fatalf("unexpected rename managed-dir error: %v", err)
			}
			if removed != targetPath {
				t.Fatalf("expected temporary target config cleanup, got %q", removed)
			}
		})

		t.Run("surfaces old config removal errors", func(t *testing.T) {
			withRenameDeps(t, func(d *commandDeps) {
				d.removeFile = func(path string) error {
					if path == sourcePath {
						return errors.New("remove old failed")
					}
					return nil
				}
			})

			_, _, err := executeRoot(t, "stack", "rename", "source", "target")
			if err == nil || !strings.Contains(err.Error(), "remove old stack config") {
				t.Fatalf("unexpected remove-old error: %v", err)
			}
		})

		t.Run("returns after a successful rename when selection does not change", func(t *testing.T) {
			withRenameDeps(t, nil)

			stdout, _, err := executeRoot(t, "stack", "rename", "source", "target")
			if err != nil {
				t.Fatalf("stack rename returned error: %v", err)
			}
			if !strings.Contains(stdout, "renamed stack source to target") {
				t.Fatalf("unexpected rename output: %s", stdout)
			}
			if strings.Contains(stdout, "selected stack updated") {
				t.Fatalf("did not expect a selection update in output: %s", stdout)
			}
		})

		t.Run("propagates rename status line write failures", func(t *testing.T) {
			withRenameDeps(t, nil)

			if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "renamed stack source to target"}, io.Discard, "stack", "rename", "source", "target"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected rename status write failure, got %v", err)
			}
		})
	})

	t.Run("stack clone covers remaining early error branches", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePathForStack = func(name string) (string, error) {
				switch name {
				case "source":
					return "/tmp/stackctl/stacks/source.yaml", nil
				case "target":
					return "/tmp/stackctl/stacks/target.yaml", nil
				default:
					return "/tmp/stackctl/stacks/" + name + ".yaml", nil
				}
			}
			d.stat = func(path string) (os.FileInfo, error) {
				if path == "/tmp/stackctl/stacks/source.yaml" {
					return nil, errors.New("source stat failed")
				}
				return nil, os.ErrNotExist
			}
		})

		_, _, err := executeRoot(t, "stack", "clone", "source", "target")
		if err == nil || !strings.Contains(err.Error(), "check stack config") {
			t.Fatalf("unexpected clone source resolution error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("source")
			d.configFilePathForStack = func(name string) (string, error) {
				if name == "target" {
					return "", errors.New("target path failed")
				}
				return "/tmp/stackctl/stacks/" + name + ".yaml", nil
			}
			d.stat = func(path string) (os.FileInfo, error) {
				if path == "/tmp/stackctl/stacks/source.yaml" {
					return fakeFileInfo{name: "source.yaml"}, nil
				}
				return nil, os.ErrNotExist
			}
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err = executeRoot(t, "stack", "clone", "source", "target")
		if err == nil || !strings.Contains(err.Error(), "target path failed") {
			t.Fatalf("unexpected clone target-path error: %v", err)
		}
	})
}

func TestStackHelperAdditionalCoverageBatchThree(t *testing.T) {
	t.Run("discoverStackEntries covers skipped paths, unknown runtime state, and missing-current errors", func(t *testing.T) {
		t.Setenv(configpkg.StackNameEnvVar, "staging")

		withTestDeps(t, func(d *commandDeps) {
			d.knownConfigPaths = func() ([]string, error) {
				return []string{
					"/tmp/stackctl/notes.txt",
					"/tmp/stackctl/stacks/staging.yaml",
				}, nil
			}
			d.loadConfig = func(string) (configpkg.Config, error) {
				cfg := configpkg.DefaultForStack("staging")
				return cfg, nil
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{}, errors.New("ps failed")
			}
		})

		entries, err := discoverStackEntries(context.Background())
		if err != nil {
			t.Fatalf("discoverStackEntries returned error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected only yaml-backed stacks to be listed, got %+v", entries)
		}
		if entries[0].Name != "staging" || entries[0].State != "unknown" || !entries[0].Current {
			t.Fatalf("unexpected discovered entry: %+v", entries[0])
		}

		t.Setenv(configpkg.StackNameEnvVar, "qa")
		withTestDeps(t, func(d *commandDeps) {
			d.knownConfigPaths = func() ([]string, error) { return []string{"/tmp/stackctl/notes.txt"}, nil }
			d.configFilePathForStack = func(string) (string, error) { return "", errors.New("path failed") }
		})

		if _, err := discoverStackEntries(context.Background()); err == nil || !strings.Contains(err.Error(), "path failed") {
			t.Fatalf("expected missing-current path failure, got %v", err)
		}
	})

	t.Run("deleteStackTarget surfaces current and selection errors", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("staging")
		cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

		withTestDeps(t, func(d *commandDeps) {
			d.removeFile = func(string) error { return nil }
			d.currentStackName = func() (string, error) { return "", errors.New("current failed") }
		})
		if _, err := deleteStackTarget(context.Background(), stackTarget{
			Name:       "staging",
			ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
			Config:     cfg,
		}, false); err == nil || !strings.Contains(err.Error(), "current failed") {
			t.Fatalf("expected current-stack failure, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.removeFile = func(string) error { return nil }
			d.currentStackName = func() (string, error) { return "staging", nil }
			d.setCurrentStackName = func(string) error { return errors.New("select failed") }
		})
		if _, err := deleteStackTarget(context.Background(), stackTarget{
			Name:       "staging",
			ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
			Config:     cfg,
		}, false); err == nil || !strings.Contains(err.Error(), "select failed") {
			t.Fatalf("expected set-current failure, got %v", err)
		}
	})

	t.Run("scaffoldManagedStackFiles propagates per-line write failures", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("staging")
		cfg.Stack.Dir = "/tmp/stackctl-data/stacks/staging"

		withTestDeps(t, func(d *commandDeps) {
			d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{
					CreatedDir: true,
					StackDir:   cfg.Stack.Dir,
				}, nil
			}
		})
		cmd := &cobra.Command{}
		cmd.SetOut(&substringWriteErrorWriter{target: "created managed stack directory"})
		cmd.SetErr(io.Discard)
		if err := scaffoldManagedStackFiles(cmd, cfg, false); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected created-dir write failure, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{
					WroteCompose: true,
					ComposePath:  configpkg.ComposePath(cfg),
				}, nil
			}
		})
		cmd = &cobra.Command{}
		cmd.SetOut(&substringWriteErrorWriter{target: "wrote managed compose file"})
		cmd.SetErr(io.Discard)
		if err := scaffoldManagedStackFiles(cmd, cfg, false); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected compose write status failure, got %v", err)
		}
	})

	t.Run("scaffoldManagedStackFiles is a no-op for unmanaged stacks", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("external")
		cfg.Stack.Managed = false

		if err := scaffoldManagedStackFiles(&cobra.Command{}, cfg, false); err != nil {
			t.Fatalf("expected unmanaged scaffold helper to return nil, got %v", err)
		}
	})
}

func TestDBAdditionalCoverageBatchThree(t *testing.T) {
	t.Run("loadRuntimeConfig failures surface across db commands", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
		}{
			{name: "shell", args: []string{"db", "shell"}},
			{name: "dump", args: []string{"db", "dump"}},
			{name: "restore", args: []string{"db", "restore", "-", "--force"}},
			{name: "reset", args: []string{"db", "reset", "--force"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") }
				})

				_, _, err := executeRootWithInput(t, strings.NewReader("select 1;\n"), tc.args...)
				if err == nil || !strings.Contains(err.Error(), "load failed") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("verbose compose output failures surface for dump, restore, and reset", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "stackdb"
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		cases := []struct {
			name string
			args []string
		}{
			{name: "dump", args: []string{"--verbose", "db", "dump"}},
			{name: "restore", args: []string{"--verbose", "db", "restore", "-", "--force"}},
			{name: "reset", args: []string{"--verbose", "db", "reset", "--force"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, tc.args...); err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected verbose compose write failure, got %v", err)
				}
			})
		}
	})

	t.Run("db dump without an output file returns after compose exec", func(t *testing.T) {
		composeCalled := false
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.composeExec = func(_ context.Context, runner system.Runner, _ configpkg.Config, _ string, _ []string, _ []string, _ bool) error {
				composeCalled = true
				_, err := io.WriteString(runner.Stdout, "-- dump to stdout --\n")
				return err
			}
		})

		stdout, _, err := executeRoot(t, "db", "dump")
		if err != nil {
			t.Fatalf("db dump returned error: %v", err)
		}
		if !composeCalled {
			t.Fatal("expected composeExec to run for stdout dumps")
		}
		if !strings.Contains(stdout, "-- dump to stdout --") {
			t.Fatalf("expected dump stdout content, got %q", stdout)
		}
		if strings.Contains(stdout, "wrote database dump to") {
			t.Fatalf("did not expect file-output status for stdout dump: %q", stdout)
		}
	})

	t.Run("signal cancellation suppresses db command exec errors", func(t *testing.T) {
		t.Run("shell", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
					_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
					time.Sleep(20 * time.Millisecond)
					return errors.New("ignored after signal")
				}
			})

			_, _, err := executeRoot(t, "db", "shell")
			if err != nil {
				t.Fatalf("expected shell signal cancellation to return nil, got %v", err)
			}
		})

		t.Run("dump", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
					_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
					time.Sleep(20 * time.Millisecond)
					return errors.New("ignored after signal")
				}
			})

			_, _, err := executeRoot(t, "db", "dump")
			if err != nil {
				t.Fatalf("expected dump signal cancellation to return nil, got %v", err)
			}
		})

		t.Run("restore", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
					_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
					time.Sleep(20 * time.Millisecond)
					return errors.New("ignored after signal")
				}
			})

			_, _, err := executeRootWithInput(t, strings.NewReader("select 1;\n"), "db", "restore", "-", "--force")
			if err != nil {
				t.Fatalf("expected restore signal cancellation to return nil, got %v", err)
			}
		})

		t.Run("reset terminate step", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "stackdb"

			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
					_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
					time.Sleep(20 * time.Millisecond)
					return errors.New("ignored after signal")
				}
			})

			_, _, err := executeRoot(t, "db", "reset", "--force")
			if err != nil {
				t.Fatalf("expected reset terminate-step cancellation to return nil, got %v", err)
			}
		})

		t.Run("reset drop step", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "stackdb"

			callCount := 0
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
					callCount++
					if callCount == 2 {
						_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
						time.Sleep(20 * time.Millisecond)
						return errors.New("ignored after signal")
					}
					return nil
				}
			})

			_, _, err := executeRoot(t, "db", "reset", "--force")
			if err != nil {
				t.Fatalf("expected reset drop-step cancellation to return nil, got %v", err)
			}
			if callCount != 2 {
				t.Fatalf("expected reset to stop on the drop step, got %d calls", callCount)
			}
		})
	})
}
