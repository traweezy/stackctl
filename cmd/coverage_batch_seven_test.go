package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRootAndFactoryResetCoverageBatchSeven(t *testing.T) {
	t.Run("root command validates logging and stack selection inputs", func(t *testing.T) {
		t.Run("invalid log level is rejected", func(t *testing.T) {
			_, _, err := executeRoot(t, "--log-level", "loud", "version")
			if err == nil || !strings.Contains(err.Error(), "invalid --log-level") {
				t.Fatalf("expected invalid log-level error, got %v", err)
			}
		})

		t.Run("invalid log format is rejected", func(t *testing.T) {
			_, _, err := executeRoot(t, "--log-format", "yaml", "version")
			if err == nil || !strings.Contains(err.Error(), "invalid --log-format") {
				t.Fatalf("expected invalid log-format error, got %v", err)
			}
		})

		t.Run("persistent pre run surfaces env override errors", func(t *testing.T) {
			rootOutput = rootOutputOptions{}
			root := NewRootCmd(NewApp())
			child := &cobra.Command{}
			child.Flags().String("accessible", "", "")
			child.Flags().Bool("plain", false, "")
			child.Flags().String("log-level", "", "")
			child.Flags().String("log-format", "", "")
			child.Flags().String("log-file", "", "")
			if err := child.Flags().Set("accessible", "yes"); err != nil {
				t.Fatalf("set accessible flag: %v", err)
			}

			if err := root.PersistentPreRunE(child, nil); err == nil {
				t.Fatal("expected persistent pre run to surface env override errors")
			}
		})

		t.Run("invalid selected stack from environment is rejected", func(t *testing.T) {
			t.Setenv(configpkg.StackNameEnvVar, "bad stack")

			_, _, err := executeRoot(t, "version")
			if err == nil || !strings.Contains(err.Error(), "validate "+configpkg.StackNameEnvVar) {
				t.Fatalf("expected invalid selected-stack env error, got %v", err)
			}
		})

		t.Run("invalid explicit stack flag is rejected", func(t *testing.T) {
			_, _, err := executeRoot(t, "--stack", "bad stack", "version")
			if err == nil || !strings.Contains(err.Error(), "invalid stack name") {
				t.Fatalf("expected invalid --stack error, got %v", err)
			}
		})
	})

	t.Run("factory reset covers remaining helper branches", func(t *testing.T) {
		t.Run("factory reset surfaces managed-stack discovery errors", func(t *testing.T) {
			rootDir := t.TempDir()
			configDir := filepath.Join(rootDir, "config", "stackctl")
			dataDir := filepath.Join(rootDir, "data", "stackctl")
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				t.Fatalf("mkdir data dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dataDir, "stacks"), []byte("blocking file"), 0o600); err != nil {
				t.Fatalf("write blocking stacks file: %v", err)
			}

			withTestDeps(t, func(d *commandDeps) {
				d.configDirPath = func() (string, error) { return configDir, nil }
				d.dataDirPath = func() (string, error) { return dataDir, nil }
			})

			_, _, err := executeRoot(t, "factory-reset", "--force")
			if err == nil || !strings.Contains(err.Error(), "read managed stacks dir") {
				t.Fatalf("expected managed-stack discovery error, got %v", err)
			}
		})

		t.Run("local compose cleanup targets skip files and sort discovered stacks", func(t *testing.T) {
			rootDir := t.TempDir()
			dataDir := filepath.Join(rootDir, "data", "stackctl")
			stacksDir := filepath.Join(dataDir, "stacks")
			if err := os.MkdirAll(stacksDir, 0o755); err != nil {
				t.Fatalf("mkdir stacks dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(stacksDir, "README.txt"), []byte("ignore"), 0o600); err != nil {
				t.Fatalf("write stacks file: %v", err)
			}

			stackDir := filepath.Join(stacksDir, "beta")
			if err := os.MkdirAll(stackDir, 0o755); err != nil {
				t.Fatalf("mkdir stack dir: %v", err)
			}
			stackCompose := filepath.Join(stackDir, configpkg.DefaultComposeFileName)
			if err := os.WriteFile(stackCompose, []byte("services: {}\n"), 0o600); err != nil {
				t.Fatalf("write stack compose file: %v", err)
			}

			managedConfigDir := filepath.Join(dataDir, "managed-alpha")
			if err := os.MkdirAll(managedConfigDir, 0o755); err != nil {
				t.Fatalf("mkdir managed config dir: %v", err)
			}
			managedCompose := filepath.Join(managedConfigDir, configpkg.DefaultComposeFileName)
			if err := os.WriteFile(managedCompose, []byte("services: {}\n"), 0o600); err != nil {
				t.Fatalf("write managed compose file: %v", err)
			}

			outsideDir := filepath.Join(rootDir, "outside")
			if err := os.MkdirAll(outsideDir, 0o755); err != nil {
				t.Fatalf("mkdir outside dir: %v", err)
			}
			outsideCompose := filepath.Join(outsideDir, configpkg.DefaultComposeFileName)
			if err := os.WriteFile(outsideCompose, []byte("services: {}\n"), 0o600); err != nil {
				t.Fatalf("write outside compose file: %v", err)
			}

			withTestDeps(t, func(d *commandDeps) {
				d.composePath = configpkg.ComposePath
				d.loadConfig = func(path string) (configpkg.Config, error) {
					cfg := configpkg.Default()
					cfg.Stack.Managed = true
					cfg.Stack.ComposeFile = configpkg.DefaultComposeFileName
					switch path {
					case "/tmp/stackctl/stacks/managed-alpha.yaml":
						cfg.Stack.Dir = managedConfigDir
					case "/tmp/stackctl/stacks/outside.yaml":
						cfg.Stack.Dir = outsideDir
					default:
						t.Fatalf("unexpected config path: %s", path)
					}
					return cfg, nil
				}
			})

			targets, err := localComposeCleanupTargets(
				[]string{"/tmp/stackctl/stacks/outside.yaml", "/tmp/stackctl/stacks/managed-alpha.yaml"},
				dataDir,
			)
			if err != nil {
				t.Fatalf("localComposeCleanupTargets returned error: %v", err)
			}

			if len(targets) != 2 {
				t.Fatalf("expected two cleanup targets, got %+v", targets)
			}
			if targets[0].ComposePath != managedCompose || targets[1].ComposePath != stackCompose {
				t.Fatalf("expected sorted cleanup targets, got %+v", targets)
			}
			for _, target := range targets {
				if target.ComposePath == outsideCompose {
					t.Fatalf("did not expect outside compose path in targets: %+v", targets)
				}
			}
		})
	})
}

func TestSetupAndDoctorCoverageBatchSeven(t *testing.T) {
	t.Run("setup command covers remaining error and write branches", func(t *testing.T) {
		t.Run("setup returns config-load errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") }
			})

			_, _, err := executeRoot(t, "setup")
			if err == nil || !strings.Contains(err.Error(), "load failed") {
				t.Fatalf("expected load-config error, got %v", err)
			}
		})

		t.Run("non-interactive config creation surfaces success write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.Stack.Managed = false
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
				d.saveConfig = func(string, configpkg.Config) error { return nil }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "created default config"}, io.Discard, "setup", "--non-interactive")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected created-config write failure, got %v", err)
			}
		})

		t.Run("interactive setup surfaces scaffold refresh errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := managedTestConfigBatchFour("setup-seven")
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
				d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) { return cfg, nil }
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
				d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
					return configpkg.ScaffoldResult{}, errors.New("interactive scaffold failed")
				}
			})

			_, _, err := executeRoot(t, "setup", "--interactive")
			if err == nil || !strings.Contains(err.Error(), "interactive scaffold failed") {
				t.Fatalf("expected interactive scaffold error, got %v", err)
			}
		})

		t.Run("interactive setup surfaces save-status write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.Stack.Managed = false
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
				d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) { return cfg, nil }
				d.saveConfig = func(string, configpkg.Config) error { return nil }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "saved config to"}, io.Discard, "setup", "--interactive")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected saved-config write failure, got %v", err)
			}
		})

		t.Run("setup returns stale-scaffold inspection errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := managedTestConfigBatchFour("setup-seven")
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
					return false, errors.New("stale scaffold check failed")
				}
			})

			_, _, err := executeRoot(t, "setup")
			if err == nil || !strings.Contains(err.Error(), "stale scaffold check failed") {
				t.Fatalf("expected stale-scaffold check error, got %v", err)
			}
		})

		t.Run("setup returns stale-scaffold refresh errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := managedTestConfigBatchFour("setup-seven")
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
				d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
					return configpkg.ScaffoldResult{}, errors.New("stale scaffold refresh failed")
				}
			})

			_, _, err := executeRoot(t, "setup", "--non-interactive")
			if err == nil || !strings.Contains(err.Error(), "stale scaffold refresh failed") {
				t.Fatalf("expected stale-scaffold refresh error, got %v", err)
			}
		})

		t.Run("setup surfaces all-clear status write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.Stack.Managed = false
				cfg.Setup.IncludeCockpit = false
				cfg.Setup.InstallCockpit = false
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					return newReport(
						doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
						doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
						doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
						doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					), nil
				}
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "all required dependencies look available"}, io.Discard, "setup")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected all-clear status write failure, got %v", err)
			}
		})

		t.Run("setup install surfaces package-manager notice write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.System.PackageManager = "broken"
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.commandExists = func(name string) bool { return name == "apt-get" }
				d.platform = func() system.Platform { return system.Platform{GOOS: "linux", PackageManager: "apt"} }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
				}
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "using detected apt for this run"}, io.Discard, "setup", "--install", "--yes")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected package-manager notice write failure, got %v", err)
			}
		})

		t.Run("setup install surfaces cockpit success write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.Setup.IncludeCockpit = true
				cfg.Setup.InstallCockpit = true
				cfg.ApplyDerivedFields()
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.platform = func() system.Platform {
					return system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd}
				}
				d.enableCockpit = func(context.Context, system.Runner) error { return nil }
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
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "enabled cockpit.socket"}, io.Discard, "setup", "--install", "--yes")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected cockpit success write failure, got %v", err)
			}
		})
	})

	t.Run("doctor fixes cover remaining prompts and output branches", func(t *testing.T) {
		t.Run("doctor fix returns scaffold prompt errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := managedTestConfigBatchFour("doctor-seven")
				d.isTerminal = func() bool { return true }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
				d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
					return false, io.ErrUnexpectedEOF
				}
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
			})

			_, _, err := executeRoot(t, "doctor", "--fix")
			if err == nil || !strings.Contains(err.Error(), "automatic fix confirmation required; rerun with --yes") {
				t.Fatalf("expected doctor scaffold prompt error, got %v", err)
			}
		})

		t.Run("doctor fix returns scaffold refresh errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := managedTestConfigBatchFour("doctor-seven")
				d.isTerminal = func() bool { return true }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
				d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
				d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
					return configpkg.ScaffoldResult{}, errors.New("doctor scaffold refresh failed")
				}
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
			})

			_, _, err := executeRoot(t, "doctor", "--fix")
			if err == nil || !strings.Contains(err.Error(), "doctor scaffold refresh failed") {
				t.Fatalf("expected doctor scaffold refresh error, got %v", err)
			}
		})

		t.Run("doctor fix returns podman-machine prompt errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.isTerminal = func() bool { return true }
				d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
				d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
					return false, io.ErrUnexpectedEOF
				}
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					return newReport(
						doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
						doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
						doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
						doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
						doctorpkg.Check{Status: output.StatusFail, Message: "podman machine initialized"},
					), nil
				}
			})

			_, _, err := executeRoot(t, "doctor", "--fix")
			if err == nil || !strings.Contains(err.Error(), "automatic fix confirmation required; rerun with --yes") {
				t.Fatalf("expected podman-machine prompt error, got %v", err)
			}
		})

		t.Run("doctor fix surfaces podman-machine success write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
				d.preparePodmanMachine = func(context.Context, system.Runner) error { return nil }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					return newReport(
						doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
						doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
						doctorpkg.Check{Status: output.StatusOK, Message: "buildah installed"},
						doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
						doctorpkg.Check{Status: output.StatusFail, Message: "podman machine initialized"},
					), nil
				}
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "podman machine is initialized and running"}, io.Discard, "doctor", "--fix", "--yes")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected podman-machine write failure, got %v", err)
			}
		})

		t.Run("doctor fix returns cockpit prompt errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.Setup.IncludeCockpit = true
				cfg.Setup.InstallCockpit = true
				cfg.ApplyDerivedFields()
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.isTerminal = func() bool { return true }
				d.platform = func() system.Platform {
					return system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd}
				}
				d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
					return false, io.ErrUnexpectedEOF
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
			})

			_, _, err := executeRoot(t, "doctor", "--fix")
			if err == nil || !strings.Contains(err.Error(), "automatic fix confirmation required; rerun with --yes") {
				t.Fatalf("expected cockpit prompt error, got %v", err)
			}
		})

		t.Run("doctor fix surfaces cockpit success write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				cfg.Setup.IncludeCockpit = true
				cfg.Setup.InstallCockpit = true
				cfg.ApplyDerivedFields()
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.platform = func() system.Platform {
					return system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd}
				}
				d.enableCockpit = func(context.Context, system.Runner) error { return nil }
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
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "enabled cockpit.socket"}, io.Discard, "doctor", "--fix", "--yes")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected cockpit success write failure, got %v", err)
			}
		})

		t.Run("doctor fix surfaces no-op status write errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.isTerminal = func() bool { return true }
				d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
				}
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "no automatic fixes were applied"}, io.Discard, "doctor", "--fix")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected no-op status write failure, got %v", err)
			}
		})

		t.Run("doctor fix surfaces post-fix report write errors", func(t *testing.T) {
			var runs int

			withTestDeps(t, func(d *commandDeps) {
				cfg := configpkg.Default()
				d.defaultConfig = func() configpkg.Config { return cfg }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
					runs++
					if runs == 1 {
						return doctorpkg.Report{}, nil
					}
					return newReport(doctorpkg.Check{Status: output.StatusWarn, Message: "post-fix output line"}), nil
				}
			})

			err := executeRootWithIO(t, &substringWriteErrorWriter{target: "post-fix output line"}, io.Discard, "doctor", "--fix", "--yes")
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected post-fix report write failure, got %v", err)
			}
		})
	})
}

func TestLifecycleRuntimeAndCommandCoverageBatchSeven(t *testing.T) {
	t.Run("lifecycle and runtime helpers cover remaining reachable branches", func(t *testing.T) {
		t.Run("resolveTargetStackServices rejects invalid names", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			_, err := resolveTargetStackServices(cfg, []string{"bogus"})
			if err == nil || !strings.Contains(err.Error(), "invalid service") {
				t.Fatalf("expected invalid-service error, got %v", err)
			}
		})

		t.Run("ensureSelectedServicePortsAvailable returns runtime errors", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{}, errors.New("runtime failed")
				}
			})

			err := ensureSelectedServicePortsAvailable(context.Background(), cfg, []string{"postgres"})
			if err == nil || !strings.Contains(err.Error(), "runtime failed") {
				t.Fatalf("expected runtime error, got %v", err)
			}
		})

		t.Run("verifySelectedServicesStarted returns nil when nothing is selected", func(t *testing.T) {
			if err := verifySelectedServicesStarted(context.Background(), configpkg.Config{}, []string{"postgres"}); err != nil {
				t.Fatalf("expected nil verification result for empty definitions, got %v", err)
			}
		})

		t.Run("waitForStackService returns terminal container failures", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			definition, ok := serviceDefinitionByKey("postgres")
			if !ok {
				t.Fatal("expected postgres definition")
			}

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{
						Stdout: marshalContainersJSON(system.Container{
							Names:  []string{cfg.Services.PostgresContainer},
							State:  "exited",
							Status: "Exited (1) moments ago",
						}),
					}, nil
				}
			})

			err := waitForStackService(context.Background(), cfg, definition)
			if err == nil || !strings.Contains(err.Error(), "container failed to start") {
				t.Fatalf("expected terminal-container failure, got %v", err)
			}
		})

		t.Run("ensureNoOtherRunningStack surfaces nested discovery errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.configFilePath = func() (string, error) { return "/tmp/stackctl/config.yaml", nil }
				d.knownConfigPaths = func() ([]string, error) { return nil, errors.New("known-config discovery failed") }
			})

			err := ensureNoOtherRunningStack(context.Background())
			if err == nil || !strings.Contains(err.Error(), "known-config discovery failed") {
				t.Fatalf("expected nested discovery error, got %v", err)
			}
		})

		t.Run("waitForStackContainersRemoved returns immediately when no names exist", func(t *testing.T) {
			if err := waitForStackContainersRemoved(context.Background(), configpkg.Config{}); err != nil {
				t.Fatalf("expected nil wait result for empty stack, got %v", err)
			}
		})

		t.Run("ensurePodmanRuntimeReady succeeds on an initialized running machine", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.commandExists = func(string) bool { return true }
				d.podmanVersion = func(context.Context) (string, error) { return system.SupportedPodmanVersion, nil }
				d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
				d.podmanMachineStatus = func(context.Context) system.PodmanMachineState {
					return system.PodmanMachineState{Initialized: true, Running: true}
				}
			})

			if err := ensurePodmanRuntimeReady(); err != nil {
				t.Fatalf("expected podman runtime readiness success, got %v", err)
			}
		})

		t.Run("printServicesJSON returns runtime inspection errors", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{}, errors.New("service inspection failed")
				}
			})

			cmd := &cobra.Command{}
			cmd.SetOut(io.Discard)
			if err := printServicesJSON(cmd, cfg); err == nil || !strings.Contains(err.Error(), "service inspection failed") {
				t.Fatalf("expected services JSON error, got %v", err)
			}
		})

		t.Run("connectionEntries skip enabled services without connection values", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = true
			cfg.ApplyDerivedFields()
			cfg.URLs.Cockpit = ""

			entries := connectionEntries(cfg)
			for _, entry := range entries {
				if entry.Name == "Cockpit" {
					t.Fatalf("did not expect cockpit connection entry when the URL is blank: %+v", entries)
				}
			}
		})
	})

	t.Run("remaining command branches surface their errors and write failures", func(t *testing.T) {
		t.Run("run surfaces invalid services and runtime-state errors", func(t *testing.T) {
			t.Run("invalid run service is rejected", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				_, _, err := executeRoot(t, "run", "bogus", "--", "echo", "hi")
				if err == nil || !strings.Contains(err.Error(), "invalid service") {
					t.Fatalf("expected invalid run-service error, got %v", err)
				}
			})

			t.Run("run readiness surfaces runtime-state errors", func(t *testing.T) {
				cfg := configpkg.Default()
				cfg.ApplyDerivedFields()

				withTestDeps(t, func(d *commandDeps) {
					d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
						return system.CommandResult{}, errors.New("run state failed")
					}
				})

				if err := ensureSelectedRunServicesReady(context.Background(), cfg, []string{"postgres"}); err == nil || !strings.Contains(err.Error(), "run state failed") {
					t.Fatalf("expected run state error, got %v", err)
				}
			})
		})

		t.Run("start covers remaining runtime, write, and success branches", func(t *testing.T) {
			t.Run("start returns compose runtime errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.commandExists = func(string) bool { return false }
				})

				_, _, err := executeRoot(t, "start")
				if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
					t.Fatalf("expected start compose-runtime error, got %v", err)
				}
			})

			t.Run("start returns invalid service errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				_, _, err := executeRoot(t, "start", "bogus")
				if err == nil || !strings.Contains(err.Error(), "invalid service") {
					t.Fatalf("expected start invalid-service error, got %v", err)
				}
			})

			t.Run("start surfaces verbose compose-file write errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Using compose file"}, io.Discard, "--verbose", "start")
				if err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected start compose-file write failure, got %v", err)
				}
			})

			t.Run("start surfaces compose-up errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.composeUp = func(context.Context, system.Runner, configpkg.Config) error { return errors.New("compose up failed") }
				})

				_, _, err := executeRoot(t, "start")
				if err == nil || !strings.Contains(err.Error(), "compose up failed") {
					t.Fatalf("expected start compose-up error, got %v", err)
				}
			})

			t.Run("start surfaces final success write errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.Behavior.WaitForServicesStart = false
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.composeUp = func(context.Context, system.Runner, configpkg.Config) error { return nil }
					d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
						return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
					}
					d.portListening = func(int) bool { return true }
				})

				err := executeRootWithIO(t, &substringWriteErrorWriter{target: "stack started"}, io.Discard, "start")
				if err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected start final-status write failure, got %v", err)
				}
			})
		})

		t.Run("stop covers remaining load, runtime, and verbose branches", func(t *testing.T) {
			t.Run("stop returns config-load errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("stop load failed") }
				})

				_, _, err := executeRoot(t, "stop")
				if err == nil || !strings.Contains(err.Error(), "stop load failed") {
					t.Fatalf("expected stop load error, got %v", err)
				}
			})

			t.Run("stop returns invalid service errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				_, _, err := executeRoot(t, "stop", "bogus")
				if err == nil || !strings.Contains(err.Error(), "invalid service") {
					t.Fatalf("expected stop invalid-service error, got %v", err)
				}
			})

			t.Run("stop returns compose runtime errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.commandExists = func(string) bool { return false }
				})

				_, _, err := executeRoot(t, "stop")
				if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
					t.Fatalf("expected stop compose-runtime error, got %v", err)
				}
			})

			t.Run("stop surfaces verbose compose-file write errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Using compose file"}, io.Discard, "--verbose", "stop")
				if err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected stop compose-file write failure, got %v", err)
				}
			})
		})

		t.Run("restart covers remaining config, preflight, and success branches", func(t *testing.T) {
			t.Run("restart returns config-load errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("restart load failed") }
				})

				_, _, err := executeRoot(t, "restart")
				if err == nil || !strings.Contains(err.Error(), "restart load failed") {
					t.Fatalf("expected restart load error, got %v", err)
				}
			})

			t.Run("restart returns scaffold refresh errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := managedTestConfigBatchFour("restart-seven")
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
					d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
						return configpkg.ScaffoldResult{}, errors.New("restart scaffold failed")
					}
				})

				_, _, err := executeRoot(t, "restart")
				if err == nil || !strings.Contains(err.Error(), "restart scaffold failed") {
					t.Fatalf("expected restart scaffold error, got %v", err)
				}
			})

			t.Run("restart returns invalid service errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.Stack.Managed = false
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				_, _, err := executeRoot(t, "restart", "bogus")
				if err == nil || !strings.Contains(err.Error(), "invalid service") {
					t.Fatalf("expected restart invalid-service error, got %v", err)
				}
			})

			t.Run("restart surfaces verbose compose-file write errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.Stack.Managed = false
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				})

				err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Using compose file"}, io.Discard, "--verbose", "restart")
				if err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected restart compose-file write failure, got %v", err)
				}
			})

			t.Run("restart full stack returns post-down port conflicts", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.Behavior.WaitForServicesStart = false
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error { return nil }
					d.anyContainerExists = func(context.Context, []string) (bool, error) { return false, nil }
					d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
						return system.CommandResult{Stdout: "[]"}, nil
					}
					d.portInUse = func(port int) (bool, error) { return port == cfg.Ports.Postgres, nil }
				})

				_, _, err := executeRoot(t, "restart")
				if err == nil || !strings.Contains(err.Error(), "cannot start stack") {
					t.Fatalf("expected restart full-stack port conflict, got %v", err)
				}
			})

			t.Run("restart surfaces final success write errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.Behavior.WaitForServicesStart = false
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error { return nil }
					d.anyContainerExists = func(context.Context, []string) (bool, error) { return false, nil }
					d.composeUp = func(context.Context, system.Runner, configpkg.Config) error { return nil }
					d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
						return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
					}
					d.portListening = func(int) bool { return true }
				})

				err := executeRootWithIO(t, &substringWriteErrorWriter{target: "stack restarted"}, io.Discard, "restart")
				if err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected restart final-status write failure, got %v", err)
				}
			})
		})

		t.Run("status and ports surface remaining config and runtime errors", func(t *testing.T) {
			t.Run("status returns config-load errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("status load failed") }
				})

				_, _, err := executeRoot(t, "status")
				if err == nil || !strings.Contains(err.Error(), "status load failed") {
					t.Fatalf("expected status load error, got %v", err)
				}
			})

			t.Run("status returns runtime inspection errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					cfg := configpkg.Default()
					cfg.ApplyDerivedFields()
					d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
					d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
						return system.CommandResult{}, errors.New("status runtime failed")
					}
				})

				_, _, err := executeRoot(t, "status")
				if err == nil || !strings.Contains(err.Error(), "status runtime failed") {
					t.Fatalf("expected status runtime error, got %v", err)
				}
			})

			t.Run("ports returns config-load errors", func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("ports load failed") }
				})

				_, _, err := executeRoot(t, "ports")
				if err == nil || !strings.Contains(err.Error(), "ports load failed") {
					t.Fatalf("expected ports load error, got %v", err)
				}
			})
		})
	})
}
