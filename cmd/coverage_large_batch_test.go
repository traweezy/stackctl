package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSetupLargeCoverageBatch(t *testing.T) {
	t.Run("returns config path errors before setup begins", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("config path failed") }
		})

		_, _, err := executeRoot(t, "setup")
		if err == nil || !strings.Contains(err.Error(), "config path failed") {
			t.Fatalf("unexpected setup config-path error: %v", err)
		}
	})

	t.Run("propagates interactive prompt errors for first-run setup", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
				return false, errors.New("prompt failed")
			}
		})

		_, _, err := executeRoot(t, "setup")
		if err == nil || !strings.Contains(err.Error(), "prompt failed") {
			t.Fatalf("unexpected setup prompt error: %v", err)
		}
	})

	t.Run("propagates interactive wizard errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
				return configpkg.Config{}, errors.New("wizard failed")
			}
		})

		_, _, err := executeRoot(t, "setup", "--interactive")
		if err == nil || !strings.Contains(err.Error(), "wizard failed") {
			t.Fatalf("unexpected setup wizard error: %v", err)
		}
	})

	t.Run("propagates stale-scaffold prompt errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
				return false, errors.New("refresh prompt failed")
			}
		})

		_, _, err := executeRoot(t, "setup")
		if err == nil || !strings.Contains(err.Error(), "refresh prompt failed") {
			t.Fatalf("unexpected scaffold prompt error: %v", err)
		}
	})

	t.Run("returns doctor errors after setup preparation", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return doctorpkg.Report{}, errors.New("doctor failed")
			}
		})

		_, _, err := executeRoot(t, "setup")
		if err == nil || !strings.Contains(err.Error(), "doctor failed") {
			t.Fatalf("unexpected setup doctor error: %v", err)
		}
	})

	t.Run("cancelled installs return a user-facing cancellation message", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		})

		stdout, _, err := executeRoot(t, "setup", "--install")
		if err != nil {
			t.Fatalf("setup --install returned error: %v", err)
		}
		if !strings.Contains(stdout, "setup install cancelled") {
			t.Fatalf("expected install cancellation message, got:\n%s", stdout)
		}
	})

	t.Run("propagates podman machine preparation errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = false
			cfg.Setup.InstallCockpit = false
			cfg.System.PackageManager = "brew"
			d.platform = func() system.Platform {
				return system.Platform{GOOS: "darwin", PackageManager: "brew", ServiceManager: system.ServiceManagerNone}
			}
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
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
			t.Fatalf("unexpected podman machine error: %v", err)
		}
	})

	t.Run("propagates cockpit enable errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = true
			cfg.Setup.InstallCockpit = true
			cfg.System.PackageManager = "dnf"
			d.platform = func() system.Platform {
				return system.Platform{
					GOOS:           "linux",
					PackageManager: "dnf",
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
					doctorpkg.Check{Status: output.StatusMiss, Message: "cockpit.socket installed"},
				), nil
			}
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return []string{"cockpit", "cockpit-podman"}, nil
			}
			d.enableCockpit = func(context.Context, system.Runner) error { return errors.New("enable failed") }
		})

		_, _, err := executeRoot(t, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "enable failed") {
			t.Fatalf("unexpected cockpit enable error: %v", err)
		}
	})
}

func TestRunLargeCoverageBatch(t *testing.T) {
	t.Run("propagates scaffold refresh errors before runtime startup", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{}, errors.New("scaffold failed")
			}
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "scaffold failed") {
			t.Fatalf("unexpected run scaffold error: %v", err)
		}
	})

	t.Run("returns compose runtime errors before starting services", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.commandExists = func(string) bool { return false }
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
			t.Fatalf("unexpected run compose-runtime error: %v", err)
		}
	})

	t.Run("blocks when another local stack is already running", func(t *testing.T) {
		current := configpkg.Default()
		current.ApplyDerivedFields()
		other := retargetStackConfig(configpkg.Default(), "staging")

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(path string) (configpkg.Config, error) {
				if strings.Contains(path, "staging.yaml") {
					return other, nil
				}
				return current, nil
			}
			d.knownConfigPaths = func() ([]string, error) {
				return []string{"/tmp/stackctl/config.yaml", "/tmp/stackctl/stacks/staging.yaml"}, nil
			}
			d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "compose.yaml"}, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(other, "postgres")}, nil
			}
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "another local stack is already running: staging") {
			t.Fatalf("unexpected other-stack run error: %v", err)
		}
	})

	t.Run("returns selected port conflict errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: `[]`}, nil
			}
			d.portInUse = func(port int) (bool, error) { return port == cfg.Ports.Postgres, nil }
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "cannot start postgres") {
			t.Fatalf("unexpected run port-conflict error: %v", err)
		}
	})

	t.Run("propagates compose up service errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: `[]`}, nil
			}
			d.portInUse = func(int) (bool, error) { return false, nil }
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				return errors.New("compose up failed")
			}
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "compose up failed") {
			t.Fatalf("unexpected run compose-up error: %v", err)
		}
	})

	t.Run("returns host command errors in no-start mode", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Connection.Host = "devbox"
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.runExternalCommand = func(context.Context, system.Runner, string, []string) error {
				return errors.New("command failed")
			}
		})

		_, _, err := executeRoot(t, "run", "--no-start", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "command failed") {
			t.Fatalf("unexpected run external-command error: %v", err)
		}
	})
}

func TestRestartLargeCoverageBatch(t *testing.T) {
	t.Run("returns compose runtime errors before restart actions", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.commandExists = func(string) bool { return false }
		})

		_, _, err := executeRoot(t, "restart", "postgres")
		if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
			t.Fatalf("unexpected restart compose-runtime error: %v", err)
		}
	})

	t.Run("blocks when another local stack is already running", func(t *testing.T) {
		current := configpkg.Default()
		current.ApplyDerivedFields()
		other := retargetStackConfig(configpkg.Default(), "staging")

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(path string) (configpkg.Config, error) {
				if strings.Contains(path, "staging.yaml") {
					return other, nil
				}
				return current, nil
			}
			d.knownConfigPaths = func() ([]string, error) {
				return []string{"/tmp/stackctl/config.yaml", "/tmp/stackctl/stacks/staging.yaml"}, nil
			}
			d.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "compose.yaml"}, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(other, "postgres")}, nil
			}
		})

		_, _, err := executeRoot(t, "restart", "postgres")
		if err == nil || !strings.Contains(err.Error(), "another local stack is already running: staging") {
			t.Fatalf("unexpected other-stack restart error: %v", err)
		}
	})

	t.Run("returns wait-for-service errors when startup verification fails", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Behavior.WaitForServicesStart = true
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.waitForPort = func(context.Context, int, time.Duration) error {
				return errors.New("timed out")
			}
		})

		_, _, err := executeRoot(t, "restart", "postgres")
		if err == nil || !strings.Contains(err.Error(), "postgres port 5432 did not become ready: timed out") {
			t.Fatalf("unexpected restart wait error: %v", err)
		}
	})

	t.Run("returns verification failures in non-wait mode", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Behavior.WaitForServicesStart = false
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: exitedRuntimeContainerJSON(cfg, "postgres")}, nil
			}
		})

		_, _, err := executeRoot(t, "restart", "postgres")
		if err == nil || !strings.Contains(err.Error(), "postgres container failed to start") {
			t.Fatalf("unexpected restart verification error: %v", err)
		}
	})
}

func TestDBResetLargeCoverageBatch(t *testing.T) {
	t.Run("cancelled confirmation returns a user-facing cancellation message", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "stackdb"
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		})

		stdout, _, err := executeRoot(t, "db", "reset")
		if err != nil {
			t.Fatalf("db reset returned error: %v", err)
		}
		if !strings.Contains(stdout, "database reset cancelled") {
			t.Fatalf("expected db reset cancellation output, got:\n%s", stdout)
		}
	})

	t.Run("returns termination step errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "stackdb"
			cfg.Services.Postgres.MaintenanceDatabase = "template1"
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, commandArgs []string, _ bool) error {
				if strings.Contains(strings.Join(commandArgs, " "), "pg_terminate_backend") {
					return errors.New("terminate failed")
				}
				return nil
			}
		})

		_, _, err := executeRoot(t, "db", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "terminate failed") {
			t.Fatalf("unexpected db reset termination error: %v", err)
		}
	})

	t.Run("returns drop step errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "stackdb"
			cfg.Services.Postgres.MaintenanceDatabase = "template1"
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.composeExec = func(_ context.Context, _ system.Runner, _ configpkg.Config, _ string, _ []string, commandArgs []string, _ bool) error {
				if containsSequence(commandArgs, []string{"-c", `DROP DATABASE IF EXISTS "stackdb"`}) {
					return errors.New("drop failed")
				}
				return nil
			}
		})

		_, _, err := executeRoot(t, "db", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "drop failed") {
			t.Fatalf("unexpected db reset drop error: %v", err)
		}
	})
}

func TestPortMappingHelperBatch(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Connection.Host = "devbox"
	cfg.Ports.Postgres = 15432
	cfg.Ports.Redis = 16379
	cfg.ApplyDerivedFields()

	t.Run("loadPortMappings merges runtime details when available", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres", "redis")}, nil
			}
			d.portListening = func(int) bool { return true }
			d.portInUse = func(int) (bool, error) { return false, nil }
		})

		mappings := loadPortMappings(context.Background(), cfg)
		if len(mappings) < 2 {
			t.Fatalf("expected merged port mappings, got %+v", mappings)
		}
		if mappings[0].Host != "devbox" || mappings[0].ExternalPort != 15432 {
			t.Fatalf("expected runtime host and port data to be retained, got %+v", mappings[0])
		}
	})

	t.Run("loadPortMappings falls back to configured values on runtime errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{}, errors.New("ps failed")
			}
		})

		mappings := loadPortMappings(context.Background(), cfg)
		if len(mappings) == 0 {
			t.Fatalf("expected configured mappings fallback, got %+v", mappings)
		}
		if mappings[0].Host != "devbox" || mappings[0].ExternalPort != 15432 {
			t.Fatalf("expected configured mapping fallback, got %+v", mappings[0])
		}
		if text := formatPortMappings(mappings); !strings.Contains(text, "Postgres") || !strings.Contains(text, "15432 -> 5432") {
			t.Fatalf("unexpected formatted mappings:\n%s", text)
		}
	})
}

func exitedRuntimeContainerJSON(cfg configpkg.Config, services ...string) string {
	definitions := selectedStackServiceDefinitions(cfg, services)
	containers := make([]system.Container, 0, len(definitions))
	for _, definition := range definitions {
		if definition.ContainerName == nil || definition.PrimaryPort == nil {
			continue
		}
		containers = append(containers, system.Container{
			ID:     definition.Key + "-exited",
			Image:  definition.Key + ":latest",
			Names:  []string{definition.ContainerName(cfg)},
			Status: "Exited (1)",
			State:  "exited",
			Ports: []system.ContainerPort{
				{
					HostPort:      definition.PrimaryPort(cfg),
					ContainerPort: definition.DefaultInternalPort,
					Protocol:      "tcp",
				},
			},
			CreatedAt: "now",
		})
	}

	return marshalContainersJSON(containers...)
}
