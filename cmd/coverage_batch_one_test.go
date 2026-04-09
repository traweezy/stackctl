package cmd

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

type writeErrorWriter struct {
	err error
}

func (w writeErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestDBCommandGuardBranches(t *testing.T) {
	t.Run("rejects postgres-disabled stacks", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePostgres = false
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()

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
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				_, _, err := executeRootWithInput(t, strings.NewReader("select 1;\n"), tc.args...)
				if err == nil || !strings.Contains(err.Error(), "postgres is not enabled in this stack") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("surfaces missing podman runtime errors", func(t *testing.T) {
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
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
					d.commandExists = func(string) bool { return false }
				})

				_, _, err := executeRootWithInput(t, strings.NewReader("select 1;\n"), tc.args...)
				if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})
}

func TestSetupAdditionalFailureBranches(t *testing.T) {
	t.Run("first-run non-interactive save failures stop setup", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.saveConfig = func(string, configpkg.Config) error { return errors.New("save failed") }
		})

		_, _, err := executeRoot(t, "setup", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("first-run interactive save failures stop setup", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
				cfg := configpkg.Default()
				cfg.Stack.Name = "interactive"
				return cfg, nil
			}
			d.saveConfig = func(string, configpkg.Config) error { return errors.New("save failed") }
		})

		_, _, err := executeRoot(t, "setup", "--interactive")
		if err == nil || !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("stale scaffold prompt failures are surfaced", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
				return false, errors.New("prompt failed")
			}
		})

		_, _, err := executeRoot(t, "setup")
		if err == nil || !strings.Contains(err.Error(), "prompt failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing scaffold warning write failures stop setup", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.isTerminal = func() bool { return false }
		})

		root := NewRootCmd(NewApp())
		root.SetOut(&failingWriteBuffer{failAfter: 1})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"setup"})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected scaffold warning write failure, got %v", err)
		}
	})

	t.Run("install prompt failures are surfaced", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
				return false, errors.New("prompt failed")
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		_, _, err := executeRoot(t, "setup", "--install")
		if err == nil || !strings.Contains(err.Error(), "prompt failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("install package failures are surfaced", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return nil, errors.New("install failed")
			}
		})

		_, _, err := executeRoot(t, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "install failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("podman machine preparation failures are surfaced", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = false
			cfg.Setup.InstallCockpit = false
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform {
				return system.Platform{GOOS: "darwin", PackageManager: "brew", ServiceManager: system.ServiceManagerNone}
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman machine not initialized"},
				), nil
			}
			d.preparePodmanMachine = func(context.Context, system.Runner) error {
				return errors.New("machine failed")
			}
		})

		_, _, err := executeRoot(t, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "machine failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cockpit enable failures are surfaced", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = true
			cfg.Setup.InstallCockpit = true
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform {
				return system.Platform{
					GOOS:           "linux",
					PackageManager: "apt",
					ServiceManager: system.ServiceManagerSystemd,
				}
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "cockpit.socket active"},
				), nil
			}
			d.enableCockpit = func(context.Context, system.Runner) error {
				return errors.New("enable failed")
			}
		})

		_, _, err := executeRoot(t, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "enable failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSnapshotHelperAdditionalBranches(t *testing.T) {
	t.Run("persistent volume specs include optional managed services and skip blank volumes", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludeMeilisearch = true
		cfg.Setup.IncludePgAdmin = true
		cfg.ApplyDerivedFields()
		cfg.Services.Redis.DataVolume = "   "

		specs := persistentVolumeSpecs(cfg)
		got := make([]string, 0, len(specs))
		for _, spec := range specs {
			got = append(got, spec.ServiceKey)
		}
		want := []string{"postgres", "seaweedfs", "meilisearch", "pgadmin"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected persistent volume specs: got %v want %v", got, want)
		}
	})

	t.Run("podman volume exists formats stdout and exit-code fallback details", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{ExitCode: 42, Stdout: "volume unavailable"}, nil
			}
		})
		_, err := podmanVolumeExists(context.Background(), "postgres_data")
		if err == nil || !strings.Contains(err.Error(), "volume unavailable") {
			t.Fatalf("unexpected stdout fallback error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{ExitCode: 99}, nil
			}
		})
		_, err = podmanVolumeExists(context.Background(), "postgres_data")
		if err == nil || !strings.Contains(err.Error(), "exit code 99") {
			t.Fatalf("unexpected exit-code fallback error: %v", err)
		}
	})

	t.Run("snapshot archive path helpers reject invalid input", func(t *testing.T) {
		if _, _, err := openSnapshotPathRoot(string(filepath.Separator)); err == nil || !strings.Contains(err.Error(), "invalid snapshot archive path") {
			t.Fatalf("expected invalid snapshot path error, got %v", err)
		}

		valid, err := normalizeSnapshotArchiveEntry(" ./volumes/postgres.tar ")
		if err != nil {
			t.Fatalf("expected normalized archive entry, got %v", err)
		}
		if valid != "volumes/postgres.tar" {
			t.Fatalf("unexpected normalized entry: %q", valid)
		}

		for _, tc := range []struct {
			name  string
			entry string
			want  string
		}{
			{name: "empty", entry: "   ", want: "entry name is empty"},
			{name: "absolute", entry: "/tmp/postgres.tar", want: "must be relative"},
			{name: "unclean", entry: "volumes/../postgres.tar", want: "must use a clean relative path"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := normalizeSnapshotArchiveEntry(tc.entry)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("tar helpers propagate write and source errors", func(t *testing.T) {
		headerErr := errors.New("header failed")
		writer := tar.NewWriter(writeErrorWriter{err: headerErr})
		t.Cleanup(func() { _ = writer.Close() })

		if err := writeTarEntry(writer, "manifest.json", []byte("{}")); !errors.Is(err, headerErr) {
			t.Fatalf("expected writeTarEntry to return %v, got %v", headerErr, err)
		}

		archiveWriter := tar.NewWriter(&bytes.Buffer{})
		t.Cleanup(func() { _ = archiveWriter.Close() })

		if err := writeTarFile(archiveWriter, "volumes/../postgres.tar", filepath.Join(t.TempDir(), "payload.tar")); err == nil || !strings.Contains(err.Error(), "must use a clean relative path") {
			t.Fatalf("unexpected invalid entry error: %v", err)
		}

		missingSource := filepath.Join(t.TempDir(), "missing.tar")
		if err := writeTarFile(archiveWriter, "volumes/postgres.tar", missingSource); err == nil {
			t.Fatal("expected missing source error")
		}
	})
}

func TestRunTUIActionAdditionalVariants(t *testing.T) {
	t.Run("named and service stop/restart actions route through their handlers", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.ApplyDerivedFields()
			cfg.Behavior.WaitForServicesStart = false
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
			d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error { return nil }
			d.composeStopServices = func(context.Context, system.Runner, configpkg.Config, []string) error { return nil }
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error { return nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
			}
		})

		cases := []struct {
			name string
			id   stacktui.ActionID
			want string
		}{
			{name: "stop named stack", id: stacktui.ActionID("stop-stack:staging"), want: "stack staging stopped"},
			{name: "restart named stack", id: stacktui.ActionID("restart-stack:staging"), want: "stack staging restarted"},
			{name: "stop service", id: stacktui.ActionID("stop-service:postgres"), want: "Postgres stopped"},
			{name: "restart service", id: stacktui.ActionID("restart-service:postgres"), want: "Postgres restarted"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				report, err := runTUIAction(tc.id)
				if err != nil {
					t.Fatalf("runTUIAction returned error: %v", err)
				}
				if report.Message != tc.want {
					t.Fatalf("unexpected report: %+v", report)
				}
			})
		}
	})

	t.Run("unsupported actions and open-target fallbacks return operator guidance", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, err := runTUIAction(stacktui.ActionID("unsupported"))
		if err == nil || !strings.Contains(err.Error(), `unsupported tui action "unsupported"`) {
			t.Fatalf("unexpected unsupported-action error: %v", err)
		}

		report, err := runTUIOpenTarget("Cockpit", "")
		if err != nil {
			t.Fatalf("runTUIOpenTarget returned error for blank URL: %v", err)
		}
		if report.Status != output.StatusWarn || report.Message != "no cockpit URL is configured" {
			t.Fatalf("unexpected blank-URL report: %+v", report)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.openURL = func(context.Context, system.Runner, string) error {
				return errors.New("open failed")
			}
		})

		report, err = runTUIOpenTarget("Cockpit", "http://127.0.0.1:9090")
		if err != nil {
			t.Fatalf("runTUIOpenTarget returned error for browser failure: %v", err)
		}
		if report.Status != output.StatusWarn || len(report.Details) != 1 || !strings.Contains(report.Details[0], "http://127.0.0.1:9090") {
			t.Fatalf("unexpected open-target fallback report: %+v", report)
		}
	})

	t.Run("doctor reports no issues when every check passes", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "compose available"},
				), nil
			}
		})

		report, err := runTUIDoctor()
		if err != nil {
			t.Fatalf("runTUIDoctor returned error: %v", err)
		}
		if report.Status != output.StatusOK || len(report.Details) != 1 || report.Details[0] != "No issues found." {
			t.Fatalf("unexpected doctor report: %+v", report)
		}
	})
}

func TestStackCommandAdditionalBranches(t *testing.T) {
	t.Run("stack list surfaces discovery errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.knownConfigPaths = func() ([]string, error) { return nil, errors.New("discover failed") }
		})

		_, _, err := executeRoot(t, "stack", "list")
		if err == nil || !strings.Contains(err.Error(), "discover failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("stack current propagates output failures", func(t *testing.T) {
		root := NewRootCmd(NewApp())
		root.SetOut(&failingWriteBuffer{failAfter: 1})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"stack", "current"})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected stack current write failure, got %v", err)
		}
	})

	t.Run("stack use surfaces selection persistence errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.setCurrentStackName = func(string) error { return errors.New("persist failed") }
		})

		_, _, err := executeRoot(t, "stack", "use", "staging")
		if err == nil || !strings.Contains(err.Error(), "persist failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("stack rename and clone reject invalid transitions", func(t *testing.T) {
		withTestDeps(t, nil)

		_, _, err := executeRoot(t, "stack", "rename", "same", "same")
		if err == nil || !strings.Contains(err.Error(), "source and destination stack names must be different") {
			t.Fatalf("unexpected rename error: %v", err)
		}

		_, _, err = executeRoot(t, "stack", "clone", "same", "same")
		if err == nil || !strings.Contains(err.Error(), "source and destination stack names must be different") {
			t.Fatalf("unexpected clone error: %v", err)
		}
	})

	t.Run("stack rename surfaces target path and selection state errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("source")
			cfg.ApplyDerivedFields()
			targetCfg := configpkg.DefaultForStack("target")
			targetCfg.ApplyDerivedFields()
			d.configFilePathForStack = func(name string) (string, error) {
				switch name {
				case "source":
					return "/tmp/stackctl/stacks/source.yaml", nil
				case "target":
					return "", errors.New("target path failed")
				default:
					return "/tmp/stackctl/stacks/" + name + ".yaml", nil
				}
			}
			d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "source.yaml"}, nil }
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "stack", "rename", "source", "target")
		if err == nil || !strings.Contains(err.Error(), "target path failed") {
			t.Fatalf("unexpected rename error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("source")
			cfg.ApplyDerivedFields()
			targetCfg := configpkg.DefaultForStack("target")
			targetCfg.ApplyDerivedFields()
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
				switch path {
				case "/tmp/stackctl/stacks/source.yaml":
					return fakeFileInfo{name: "source.yaml"}, nil
				case "/tmp/stackctl/stacks/target.yaml":
					return nil, os.ErrNotExist
				case targetCfg.Stack.Dir:
					return nil, os.ErrNotExist
				default:
					return fakeFileInfo{name: filepath.Base(path)}, nil
				}
			}
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.saveConfig = func(string, configpkg.Config) error { return nil }
			d.removeFile = func(string) error { return nil }
			d.currentStackName = func() (string, error) { return "", errors.New("current failed") }
		})

		_, _, err = executeRoot(t, "stack", "rename", "source", "target")
		if err == nil || !strings.Contains(err.Error(), "current failed") {
			t.Fatalf("unexpected rename current-stack error: %v", err)
		}
	})

	t.Run("stack clone surfaces managed target dir and status write failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("source")
			cfg.ApplyDerivedFields()
			targetCfg := configpkg.DefaultForStack("target")
			targetCfg.ApplyDerivedFields()
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
				switch path {
				case "/tmp/stackctl/stacks/source.yaml":
					return fakeFileInfo{name: "source.yaml"}, nil
				case "/tmp/stackctl/stacks/target.yaml":
					return nil, os.ErrNotExist
				case targetCfg.Stack.Dir:
					return fakeFileInfo{name: "target"}, nil
				default:
					return fakeFileInfo{name: filepath.Base(path)}, nil
				}
			}
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "stack", "clone", "source", "target")
		if err == nil || !strings.Contains(err.Error(), "managed stack directory") {
			t.Fatalf("unexpected clone dir error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("source")
			cfg.ApplyDerivedFields()
			targetCfg := configpkg.DefaultForStack("target")
			targetCfg.ApplyDerivedFields()
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
				switch path {
				case "/tmp/stackctl/stacks/source.yaml":
					return fakeFileInfo{name: "source.yaml"}, nil
				case "/tmp/stackctl/stacks/target.yaml", cfg.Stack.Dir, targetCfg.Stack.Dir:
					return nil, os.ErrNotExist
				default:
					return fakeFileInfo{name: filepath.Base(path)}, nil
				}
			}
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.saveConfig = func(string, configpkg.Config) error { return nil }
			d.scaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{ComposePath: configpkg.ComposePath(cfg)}, nil
			}
		})

		root := NewRootCmd(NewApp())
		root.SetOut(&failingWriteBuffer{failAfter: 1})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"stack", "clone", "source", "target"})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected stack clone status write failure, got %v", err)
		}
	})
}
