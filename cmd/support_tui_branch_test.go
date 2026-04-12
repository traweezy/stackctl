package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func runTeaInitForCoverage(model tea.Model) tea.Model {
	cmd := model.Init()
	if cmd == nil {
		return model
	}

	msg := cmd()
	if msg == nil {
		return model
	}

	value := reflect.ValueOf(msg)
	if value.IsValid() && value.Kind() == reflect.Slice {
		if value.Len() == 0 {
			return model
		}
		nested, ok := value.Index(0).Interface().(tea.Cmd)
		if !ok || nested == nil {
			return model
		}
		nestedMsg := nested()
		nextModel, _ := model.Update(nestedMsg)
		return nextModel
	}

	nextModel, _ := model.Update(msg)
	return nextModel
}

func overflowFDFile() *os.File {
	return os.NewFile(^uintptr(0), "overflow")
}

func TestSupportAndTUICoverageBatchSix(t *testing.T) {
	t.Run("defaultTerminalInteractive returns false for invalid descriptors", func(t *testing.T) {
		originalStdin := os.Stdin
		originalStdout := os.Stdout
		t.Cleanup(func() {
			os.Stdin = originalStdin
			os.Stdout = originalStdout
		})

		tempFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
		if err != nil {
			t.Fatalf("CreateTemp returned error: %v", err)
		}
		defer func() { _ = tempFile.Close() }()

		os.Stdin = overflowFDFile()
		os.Stdout = tempFile
		if defaultTerminalInteractive() {
			t.Fatal("expected invalid stdin descriptor to disable interactive mode")
		}

		os.Stdin = tempFile
		os.Stdout = overflowFDFile()
		if defaultTerminalInteractive() {
			t.Fatal("expected invalid stdout descriptor to disable interactive mode")
		}
	})

	t.Run("fileDescriptor rejects overflowing descriptors", func(t *testing.T) {
		if _, ok := fileDescriptor(overflowFDFile()); ok {
			t.Fatal("expected overflowing descriptor to be rejected")
		}
	})

	t.Run("quietRequested and verboseRequested fall back for nil commands", func(t *testing.T) {
		originalQuiet := rootOutput.Quiet
		originalVerbose := rootOutput.Verbose
		t.Cleanup(func() {
			rootOutput.Quiet = originalQuiet
			rootOutput.Verbose = originalVerbose
		})

		rootOutput.Quiet = true
		rootOutput.Verbose = true

		if !quietRequested(nil) {
			t.Fatal("expected nil quietRequested call to use root output state")
		}
		if !verboseRequested(nil) {
			t.Fatal("expected nil verboseRequested call to use root output state")
		}
	})

	t.Run("scaffoldManagedStack surfaces write failures for every emitted status line", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		result := configpkg.ScaffoldResult{
			StackDir:            cfg.Stack.Dir,
			ComposePath:         configpkg.ComposePath(cfg),
			NATSConfigPath:      configpkg.NATSConfigPath(cfg),
			RedisACLPath:        configpkg.RedisACLPath(cfg),
			PgAdminServersPath:  configpkg.PgAdminServersPath(cfg),
			PGPassPath:          configpkg.PGPassPath(cfg),
			CreatedDir:          true,
			WroteCompose:        true,
			WroteNATSConfig:     true,
			WroteRedisACL:       true,
			WrotePgAdminServers: true,
			WrotePGPass:         true,
		}

		for _, failAfter := range []int{1, 2, 3, 4, 5, 6} {
			t.Run("fail-after-"+string(rune('0'+failAfter)), func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
						return result, nil
					}
				})

				cmd := &cobra.Command{Use: "scaffold"}
				cmd.SetOut(&failingWriteBuffer{failAfter: failAfter})
				if err := scaffoldManagedStack(cmd, cfg, true); err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected write failure after %d writes, got %v", failAfter, err)
				}
			})
		}
	})

	t.Run("runTUIDeleteStack propagates delete errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.Stack.Managed = false
			cfg.ApplyDerivedFields()
			d.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
			d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.removeFile = func(string) error { return errors.New("delete failed") }
		})

		_, err := runTUIDeleteStack("staging")
		if err == nil || !strings.Contains(err.Error(), "delete failed") {
			t.Fatalf("expected delete failure, got %v", err)
		}
	})

	t.Run("loadTUIStackTargetConfig propagates resolution errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePathForStack = func(string) (string, error) { return "", errors.New("resolve failed") }
		})

		_, err := loadTUIStackTargetConfig("staging")
		if err == nil || !strings.Contains(err.Error(), "resolve failed") {
			t.Fatalf("expected resolution failure, got %v", err)
		}
	})

	t.Run("named stack lifecycle actions propagate inner command errors", func(t *testing.T) {
		for name, run := range map[string]func(string) (stacktui.ActionReport, error){
			"start":   runTUIStartStack,
			"stop":    runTUIStopStack,
			"restart": runTUIRestartStack,
		} {
			t.Run(name, func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.DefaultForStack("staging")
					cfg.ApplyDerivedFields()
					d.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
					d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.commandExists = func(string) bool { return false }
				})

				_, err := run("staging")
				if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
					t.Fatalf("expected named stack action to surface runtime readiness error, got %v", err)
				}
			})
		}
	})

	t.Run("runTUIStart covers scaffold, runtime, and running-stack guard failures", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("scaffold check failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
					return false, errors.New("scaffold check failed")
				}
			})

			_, err := runTUIStart(cfg, nil)
			if err == nil || !strings.Contains(err.Error(), "scaffold check failed") {
				t.Fatalf("expected scaffold failure, got %v", err)
			}
		})

		t.Run("compose runtime failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.commandExists = func(string) bool { return false }
			})

			_, err := runTUIStart(cfg, nil)
			if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
				t.Fatalf("expected runtime readiness failure, got %v", err)
			}
		})

		t.Run("another stack is already running", func(t *testing.T) {
			otherCfg := configpkg.DefaultForStack("staging")
			otherCfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.knownConfigPaths = func() ([]string, error) {
					return []string{"/tmp/stackctl/config.yaml", "/tmp/stackctl/stacks/staging.yaml"}, nil
				}
				d.loadConfig = func(path string) (configpkg.Config, error) {
					if strings.Contains(path, "staging") {
						return otherCfg, nil
					}
					return cfg, nil
				}
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(otherCfg)}, nil
				}
			})

			_, err := runTUIStart(cfg, nil)
			if err == nil || !strings.Contains(err.Error(), "another local stack is already running") {
				t.Fatalf("expected running-stack guard failure, got %v", err)
			}
		})
	})

	t.Run("runTUIStop propagates compose runtime failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.commandExists = func(string) bool { return false }
		})

		_, err := runTUIStop(configpkg.Default(), nil)
		if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
			t.Fatalf("expected runtime readiness failure, got %v", err)
		}
	})

	t.Run("runTUIRestart covers additional restart guard branches", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("all services port conflict after compose down", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				d.portListening = func(int) bool { return false }
				d.portInUse = func(port int) (bool, error) { return port == cfg.Ports.Postgres, nil }
			})

			_, err := runTUIRestart(cfg, nil)
			if err == nil || !strings.Contains(err.Error(), "cannot start stack") {
				t.Fatalf("expected port-conflict restart failure, got %v", err)
			}
		})

		t.Run("all services compose up failure after checks", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				d.portListening = func(int) bool { return false }
				d.portInUse = func(int) (bool, error) { return false, nil }
				d.composeUp = func(context.Context, system.Runner, configpkg.Config) error {
					return errors.New("compose up after down failed")
				}
			})

			_, err := runTUIRestart(cfg, nil)
			if err == nil || !strings.Contains(err.Error(), "compose up after down failed") {
				t.Fatalf("expected compose-up failure, got %v", err)
			}
		})

		t.Run("selected services port conflict", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				d.portListening = func(int) bool { return false }
				d.portInUse = func(port int) (bool, error) { return port == cfg.Ports.Postgres, nil }
			})

			_, err := runTUIRestart(cfg, []string{"postgres"})
			if err == nil || !strings.Contains(err.Error(), "cannot start postgres") {
				t.Fatalf("expected selected-service port conflict, got %v", err)
			}
		})

		t.Run("wait failure after full restart", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
				}
				d.waitForPort = func(context.Context, int, time.Duration) error {
					return errors.New("restart wait failed")
				}
			})

			_, err := runTUIRestart(cfg, nil)
			if err == nil || !strings.Contains(err.Error(), "restart wait failed") {
				t.Fatalf("expected restart wait failure, got %v", err)
			}
		})
	})

	t.Run("newTeaProgram executes the default factory", func(t *testing.T) {
		program := newTeaProgram(testTeaModel{})
		if program == nil {
			t.Fatal("expected default tea program factory to return a program")
		}
	})

	t.Run("newTUICmd run initializes the loader and debug log when configured", func(t *testing.T) {
		original := newTeaProgram
		t.Cleanup(func() { newTeaProgram = original })

		debugPath := filepath.Join(t.TempDir(), "stackctl-tui.log")
		var initModel tea.Model
		newTeaProgram = func(model tea.Model, options ...tea.ProgramOption) teaProgram {
			return stubTeaProgram{run: func() (tea.Model, error) {
				initModel = runTeaInitForCoverage(model)
				return initModel, nil
			}}
		}

		cmd := newTUICmd(&App{Version: "1.2.3"})
		if err := cmd.Flags().Set("debug-log-file", debugPath); err != nil {
			t.Fatalf("set debug-log-file flag: %v", err)
		}
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("newTUICmd RunE returned error: %v", err)
		}
		if _, ok := initModel.(stacktui.Model); !ok {
			t.Fatalf("expected initialized model to remain a stacktui.Model, got %T", initModel)
		}
		if _, err := os.Stat(debugPath); err != nil {
			t.Fatalf("expected debug log file to be created, got %v", err)
		}
	})

	t.Run("loadTUISnapshot propagates editable config errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("snapshot config failed") }
		})

		_, err := loadTUISnapshot()
		if err == nil || !strings.Contains(err.Error(), "snapshot config failed") {
			t.Fatalf("expected editable config failure, got %v", err)
		}
	})

	t.Run("buildTUISnapshot records scaffold and doctor errors", func(t *testing.T) {
		t.Run("managed scaffold problem", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
					return false, errors.New("scaffold problem")
				}
			})

			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			snapshot := buildTUISnapshot("/tmp/stackctl/config.yaml", cfg, stacktui.ConfigSourceLoaded, "")
			if !strings.Contains(snapshot.ConfigScaffoldProblem, "scaffold problem") {
				t.Fatalf("expected scaffold problem in snapshot, got %+v", snapshot)
			}
		})

		t.Run("doctor failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.ApplyDerivedFields()
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
				}
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					return doctorpkg.Report{}, errors.New("doctor snapshot failed")
				}
			})

			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			snapshot := buildTUISnapshot("/tmp/stackctl/config.yaml", cfg, stacktui.ConfigSourceLoaded, "")
			if !strings.Contains(snapshot.DoctorError, "doctor snapshot failed") {
				t.Fatalf("expected doctor error in snapshot, got %+v", snapshot)
			}
		})
	})

	t.Run("buildTUI shell commands propagate load config errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("config load failed") }
		})

		if _, err := buildTUIServiceShellCommand(stacktui.ServiceShellRequest{Service: "postgres"}); err == nil || !strings.Contains(err.Error(), "config load failed") {
			t.Fatalf("expected service shell config error, got %v", err)
		}
		if _, err := buildTUIDBShellCommand(stacktui.DBShellRequest{Service: "postgres"}); err == nil || !strings.Contains(err.Error(), "config load failed") {
			t.Fatalf("expected db shell config error, got %v", err)
		}
	})

	t.Run("snapshot helpers surface temp-dir and restore errors", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		specs := []persistentVolumeSpec{{
			ServiceKey:   "postgres",
			DisplayName:  "Postgres",
			VolumeName:   "stackctl-postgres",
			ArchiveEntry: "volumes/postgres.tar",
		}}

		t.Run("saveSnapshotArchive temp dir failure", func(t *testing.T) {
			tempFile := filepath.Join(t.TempDir(), "tmp-as-file")
			if err := os.WriteFile(tempFile, []byte("not a directory"), 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			t.Setenv("TMPDIR", tempFile)

			err := saveSnapshotArchive(&cobra.Command{Use: "snapshot"}, cfg, specs, filepath.Join(t.TempDir(), "snapshot.tar"))
			if err == nil || !strings.Contains(err.Error(), "create snapshot temp dir") {
				t.Fatalf("expected temp-dir save failure, got %v", err)
			}
		})

		t.Run("readSnapshotArchive extraction dir failure", func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "snapshot.tar")
			payloadPath := filepath.Join(t.TempDir(), "payload.tar")
			if err := os.WriteFile(payloadPath, []byte("payload"), 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			manifest := snapshotManifest{
				Version:   1,
				StackName: cfg.Stack.Name,
				Volumes: []snapshotVolumeRecord{{
					Service:    "postgres",
					SourceName: "stackctl-postgres",
					Archive:    "volumes/postgres.tar",
				}},
			}
			if err := writeSnapshotArchive(archivePath, manifest, map[string]string{"volumes/postgres.tar": payloadPath}); err != nil {
				t.Fatalf("writeSnapshotArchive returned error: %v", err)
			}

			tempFile := filepath.Join(t.TempDir(), "tmp-as-file")
			if err := os.WriteFile(tempFile, []byte("not a directory"), 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			t.Setenv("TMPDIR", tempFile)

			_, _, cleanup, err := readSnapshotArchive(archivePath)
			if cleanup != nil {
				t.Fatal("expected cleanup to stay nil on extraction-dir failure")
			}
			if err == nil || !strings.Contains(err.Error(), "create snapshot extraction dir") {
				t.Fatalf("expected extraction-dir failure, got %v", err)
			}
		})

		t.Run("restoreSnapshotArchive surfaces remove failures", func(t *testing.T) {
			dir := t.TempDir()
			scriptPath := filepath.Join(dir, "podman")
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = \"volume\" ] && [ \"$2\" = \"rm\" ]; then\n" +
				"  echo \"remove failed\" >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"exit 0\n"
			if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
				t.Fatalf("write failing podman: %v", err)
			}
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{ExitCode: 0}, nil
				}
			})

			err := restoreSnapshotArchive(&cobra.Command{Use: "snapshot"}, specs, snapshotManifest{
				Version:   1,
				StackName: cfg.Stack.Name,
				Volumes: []snapshotVolumeRecord{{
					Service:    "postgres",
					SourceName: "stackctl-postgres",
					Archive:    "volumes/postgres.tar",
				}},
			}, map[string]string{"volumes/postgres.tar": filepath.Join(t.TempDir(), "payload.tar")})
			if err == nil || !strings.Contains(err.Error(), "podman volume rm stackctl-postgres") {
				t.Fatalf("expected restore remove failure, got %v", err)
			}
		})
	})
}
