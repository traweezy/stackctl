package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func managedTestConfigBatchFour(name string) configpkg.Config {
	cfg := configpkg.DefaultForStack(name)
	cfg.Stack.Managed = true
	cfg.Setup.ScaffoldDefaultStack = true
	cfg.ApplyDerivedFields()
	return cfg
}

func TestSetupCoverageBatchFour(t *testing.T) {
	t.Run("missing config non-interactive propagates scaffold errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := managedTestConfigBatchFour("batch-four")
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
			d.defaultConfig = func() configpkg.Config { return cfg }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{}, errors.New("scaffold failed")
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "scaffold failed") {
			t.Fatalf("expected scaffold failure, got %v", err)
		}
	})

	t.Run("missing config non-interactive propagates save errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := managedTestConfigBatchFour("batch-four")
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
			d.defaultConfig = func() configpkg.Config { return cfg }
			d.saveConfig = func(string, configpkg.Config) error { return errors.New("save failed") }
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("expected save failure, got %v", err)
		}
	})

	t.Run("missing config prompt errors are returned", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
				return false, errors.New("prompt failed")
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "prompt failed") {
			t.Fatalf("expected prompt failure, got %v", err)
		}
	})

	t.Run("interactive wizard errors are returned", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return true, nil }
			d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
				return configpkg.Config{}, errors.New("wizard failed")
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "wizard failed") {
			t.Fatalf("expected wizard failure, got %v", err)
		}
	})

	t.Run("missing scaffold warning surfaces write errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := managedTestConfigBatchFour("batch-four")
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.isTerminal = func() bool { return false }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "managed stack files are missing"}, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected warning write failure, got %v", err)
		}
	})

	t.Run("doctor output write errors are returned", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusWarn, Message: "doctor output line"}), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "doctor output line"}, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected doctor output write failure, got %v", err)
		}
	})

	t.Run("missing requirements write errors are returned", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Missing requirements:"}, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected missing-requirements write failure, got %v", err)
		}
	})

	t.Run("podman machine guidance write errors are returned", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusFail, Message: "podman machine initialized"},
				), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "podman machine still needs initialization or startup"}, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected podman-machine write failure, got %v", err)
		}
	})

	t.Run("manual cockpit guidance write errors are returned", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.Setup.IncludeCockpit = true
			cfg.Setup.InstallCockpit = true
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform {
				return system.Platform{GOOS: "linux", PackageManager: "apt", ServiceManager: system.ServiceManagerSystemd}
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "cockpit.socket installed"},
				), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "cockpit helpers are enabled but cockpit must be installed manually on this platform"}, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected manual cockpit write failure, got %v", err)
		}
	})

	t.Run("install mode returns package manager resolution errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.commandExists = func(string) bool { return false }
			d.platform = func() system.Platform { return system.Platform{GOOS: "linux"} }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "resolve package manager") {
			t.Fatalf("expected package manager resolution error, got %v", err)
		}
	})

	t.Run("install mode propagates package installation errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return nil, errors.New("install failed")
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "install failed") {
			t.Fatalf("expected install failure, got %v", err)
		}
	})

	t.Run("install mode propagates podman machine preparation errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
			d.preparePodmanMachine = func(context.Context, system.Runner) error { return errors.New("machine failed") }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusFail, Message: "podman machine initialized"},
				), nil
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "machine failed") {
			t.Fatalf("expected podman machine failure, got %v", err)
		}
	})

	t.Run("install mode propagates cockpit enablement errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.Setup.IncludeCockpit = true
			cfg.Setup.InstallCockpit = true
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform {
				return system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd}
			}
			d.enableCockpit = func(context.Context, system.Runner) error { return errors.New("enable failed") }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "cockpit.socket active"},
				), nil
			}
		})

		err := executeRootWithIO(t, io.Discard, io.Discard, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "enable failed") {
			t.Fatalf("expected cockpit enable failure, got %v", err)
		}
	})

	t.Run("printSetupNextSteps surfaces plain output write errors", func(t *testing.T) {
		cmd := NewRootCmd(NewApp())
		rootOutput = rootOutputOptions{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})

		if err := printSetupNextSteps(cmd, configpkg.DefaultForStack(configpkg.DefaultStackName), nil, false, false, false); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected next-steps write failure, got %v", err)
		}
	})
}

func TestDoctorCoverageBatchFour(t *testing.T) {
	t.Run("runDoctorFixes propagates diagnostic errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return doctorpkg.Report{}, errors.New("doctor failed")
			}
		})

		cmd := NewRootCmd(NewApp())
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "doctor failed") {
			t.Fatalf("expected doctor failure, got %v", err)
		}
	})

	t.Run("runDoctorFixes propagates managed scaffold checks", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := managedTestConfigBatchFour("doctor-four")
			d.defaultConfig = func() configpkg.Config { return cfg }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
				return false, errors.New("scaffold check failed")
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
		})

		cmd := NewRootCmd(NewApp())
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "scaffold check failed") {
			t.Fatalf("expected scaffold check failure, got %v", err)
		}
	})

	t.Run("runDoctorFixes propagates install errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.defaultConfig = func() configpkg.Config { return cfg }
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return nil, errors.New("install failed")
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		cmd := NewRootCmd(NewApp())
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "install failed") {
			t.Fatalf("expected install failure, got %v", err)
		}
	})

	t.Run("runDoctorFixes propagates podman machine preparation errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.defaultConfig = func() configpkg.Config { return cfg }
			d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
			d.preparePodmanMachine = func(context.Context, system.Runner) error { return errors.New("machine failed") }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusFail, Message: "podman machine initialized"},
				), nil
			}
		})

		cmd := NewRootCmd(NewApp())
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "machine failed") {
			t.Fatalf("expected machine failure, got %v", err)
		}
	})

	t.Run("runDoctorFixes propagates cockpit enablement errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.Setup.IncludeCockpit = true
			cfg.Setup.InstallCockpit = true
			cfg.ApplyDerivedFields()
			d.defaultConfig = func() configpkg.Config { return cfg }
			d.platform = func() system.Platform {
				return system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd}
			}
			d.enableCockpit = func(context.Context, system.Runner) error { return errors.New("enable failed") }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "cockpit.socket installed"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "cockpit.socket active"},
				), nil
			}
		})

		cmd := NewRootCmd(NewApp())
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "enable failed") {
			t.Fatalf("expected enable failure, got %v", err)
		}
	})

	t.Run("runDoctorFixes returns second diagnostic run errors", func(t *testing.T) {
		runCount := 0
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				runCount++
				if runCount == 2 {
					return doctorpkg.Report{}, errors.New("post-fix doctor failed")
				}
				return doctorpkg.Report{}, nil
			}
		})

		cmd := NewRootCmd(NewApp())
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "post-fix doctor failed") {
			t.Fatalf("expected post-fix failure, got %v", err)
		}
	})

	t.Run("runDoctorFixes surfaces post-fix heading write errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
		})

		cmd := NewRootCmd(NewApp())
		cmd.SetOut(&exactWriteErrorWriter{target: "\nPost-fix report:\n"})
		if err := runDoctorFixes(cmd, true); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected post-fix heading write failure, got %v", err)
		}
	})
}

func TestTUIActionCoverageBatchFour(t *testing.T) {
	t.Run("runTUIAction propagates config load errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("config path failed") }
		})

		_, err := runTUIAction(stacktui.ActionStart)
		if err == nil || !strings.Contains(err.Error(), "config path failed") {
			t.Fatalf("expected config path failure, got %v", err)
		}
	})

	t.Run("runTUIUseStack returns setenv errors for invalid values", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.setCurrentStackName = func(string) error { return nil }
		})

		_, err := runTUIUseStack("broken\x00stack")
		if err == nil {
			t.Fatal("expected setenv failure")
		}
	})

	t.Run("runTUIDeleteStack propagates target resolution errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePathForStack = func(string) (string, error) { return "", errors.New("target failed") }
		})

		_, err := runTUIDeleteStack("staging")
		if err == nil || !strings.Contains(err.Error(), "target failed") {
			t.Fatalf("expected target resolution failure, got %v", err)
		}
	})

	t.Run("runTUIStart propagates compose up errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.composeUp = func(context.Context, system.Runner, configpkg.Config) error { return errors.New("compose up failed") }
		})

		_, err := runTUIStart(configpkg.DefaultForStack(configpkg.DefaultStackName), nil)
		if err == nil || !strings.Contains(err.Error(), "compose up failed") {
			t.Fatalf("expected compose-up failure, got %v", err)
		}
	})

	t.Run("runTUIStart propagates compose up service errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				return errors.New("compose up services failed")
			}
		})

		_, err := runTUIStart(configpkg.DefaultForStack(configpkg.DefaultStackName), []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "compose up services failed") {
			t.Fatalf("expected compose-up-services failure, got %v", err)
		}
	})

	t.Run("runTUIStart propagates wait failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.waitForPort = func(context.Context, int, time.Duration) error { return errors.New("wait failed") }
		})

		cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
		cfg.Behavior.WaitForServicesStart = true
		_, err := runTUIStart(cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "wait failed") {
			t.Fatalf("expected wait failure, got %v", err)
		}
	})

	t.Run("runTUIStop propagates compose down errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("compose down failed")
			}
		})

		_, err := runTUIStop(configpkg.DefaultForStack(configpkg.DefaultStackName), nil)
		if err == nil || !strings.Contains(err.Error(), "compose down failed") {
			t.Fatalf("expected compose-down failure, got %v", err)
		}
	})

	t.Run("runTUIStop propagates compose stop service errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.composeStopServices = func(context.Context, system.Runner, configpkg.Config, []string) error {
				return errors.New("compose stop failed")
			}
		})

		_, err := runTUIStop(configpkg.DefaultForStack(configpkg.DefaultStackName), []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "compose stop failed") {
			t.Fatalf("expected compose-stop failure, got %v", err)
		}
	})

	t.Run("runTUIRestart propagates compose down errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("compose down failed")
			}
		})

		_, err := runTUIRestart(configpkg.DefaultForStack(configpkg.DefaultStackName), nil)
		if err == nil || !strings.Contains(err.Error(), "compose down failed") {
			t.Fatalf("expected compose-down failure, got %v", err)
		}
	})

	t.Run("runTUIRestart propagates compose up service errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				return errors.New("compose restart services failed")
			}
		})

		_, err := runTUIRestart(configpkg.DefaultForStack(configpkg.DefaultStackName), []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "compose restart services failed") {
			t.Fatalf("expected restart compose-up-services failure, got %v", err)
		}
	})
}

func TestTUIAndRuntimeCoverageBatchFour(t *testing.T) {
	t.Run("loadTUIConfig propagates config path errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("config path failed") }
		})

		_, _, err := loadTUIConfig()
		if err == nil || !strings.Contains(err.Error(), "config path failed") {
			t.Fatalf("expected config path failure, got %v", err)
		}
	})

	t.Run("loadTUIEditableConfig propagates config path errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("config path failed") }
		})

		_, _, _, _, err := loadTUIEditableConfig()
		if err == nil || !strings.Contains(err.Error(), "config path failed") {
			t.Fatalf("expected config path failure, got %v", err)
		}
	})

	t.Run("buildTUIStackProfiles drops discovery errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.knownConfigPaths = func() ([]string, error) { return nil, errors.New("discover failed") }
		})

		profiles := buildTUIStackProfiles(context.Background())
		if profiles != nil {
			t.Fatalf("expected nil profiles on discovery failure, got %v", profiles)
		}
	})

	t.Run("buildTUIServiceShellCommand propagates runtime readiness errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.commandExists = func(string) bool { return false }
		})

		_, err := buildTUIServiceShellCommand(stacktui.ServiceShellRequest{Service: "postgres"})
		if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
			t.Fatalf("expected runtime readiness error, got %v", err)
		}
	})

	t.Run("buildTUIServiceShellCommand rejects disabled services", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.Setup.IncludeMeilisearch = false
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, err := buildTUIServiceShellCommand(stacktui.ServiceShellRequest{Service: "meilisearch"})
		if err == nil || !strings.Contains(err.Error(), "is not enabled") {
			t.Fatalf("expected disabled-service error, got %v", err)
		}
	})

	t.Run("buildTUIDBShellCommand propagates runtime readiness errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.commandExists = func(string) bool { return false }
		})

		_, err := buildTUIDBShellCommand(stacktui.DBShellRequest{Service: "postgres"})
		if err == nil || !strings.Contains(err.Error(), "podman is not installed") {
			t.Fatalf("expected runtime readiness error, got %v", err)
		}
	})

	t.Run("buildTUIDBShellCommand rejects invalid service names", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, err := buildTUIDBShellCommand(stacktui.DBShellRequest{Service: "not-a-service"})
		if err == nil || !strings.Contains(err.Error(), "invalid service") {
			t.Fatalf("expected invalid-service error, got %v", err)
		}
	})

	t.Run("suppressInteractiveExitError keeps non-exit errors", func(t *testing.T) {
		err := suppressInteractiveExitError(errors.New("boom"))
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected regular error to be preserved, got %v", err)
		}
		if err := suppressInteractiveExitError(&exec.ExitError{}); err != nil {
			t.Fatalf("expected exit errors to be suppressed, got %v", err)
		}
	})

	t.Run("quietRequested and verboseRequested fall back when flags are absent", func(t *testing.T) {
		rootOutput = rootOutputOptions{Quiet: true, Verbose: true}
		cmd := &cobra.Command{}
		if !quietRequested(cmd) {
			t.Fatal("expected quiet fallback to root output")
		}
		if !verboseRequested(cmd) {
			t.Fatal("expected verbose fallback to root output")
		}
	})

	t.Run("commandSpinnerTheme evaluates terminal files", func(t *testing.T) {
		theme := commandSpinnerTheme(os.Stdout)
		if styles := theme.Theme(false); styles == nil {
			t.Fatal("expected spinner theme styles")
		}
	})

	t.Run("envGroupForDefinition returns empty groups without env handlers", func(t *testing.T) {
		if group := envGroupForDefinition(configpkg.DefaultForStack(configpkg.DefaultStackName), serviceDefinition{DisplayName: "Noop"}); len(group.Entries) != 0 {
			t.Fatalf("expected no env entries, got %v", group)
		}
		group := envGroupForDefinition(configpkg.DefaultForStack(configpkg.DefaultStackName), serviceDefinition{
			DisplayName: "Empty",
			EnvEntries:  func(configpkg.Config) []envEntry { return nil },
		})
		if len(group.Entries) != 0 {
			t.Fatalf("expected empty env group, got %v", group)
		}
	})

	t.Run("ensureSelectedServicePortsAvailable returns nil when nothing matches", func(t *testing.T) {
		cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
		cfg.Setup.IncludePostgres = false
		cfg.ApplyDerivedFields()
		if err := ensureSelectedServicePortsAvailable(context.Background(), cfg, []string{"postgres"}); err != nil {
			t.Fatalf("expected nil when no selected services are enabled, got %v", err)
		}
	})

	t.Run("otherRunningLocalStack skips broken configs and runtime errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack("staging")
			d.knownConfigPaths = func() ([]string, error) {
				return []string{"/tmp/current.yaml", "/tmp/broken.yaml", "/tmp/staging.yaml"}, nil
			}
			d.loadConfig = func(path string) (configpkg.Config, error) {
				if strings.Contains(path, "broken") {
					return configpkg.Config{}, errors.New("load failed")
				}
				return cfg, nil
			}
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{}, errors.New("runtime failed")
			}
		})

		active, err := otherRunningLocalStack(context.Background(), "/tmp/current.yaml")
		if err != nil {
			t.Fatalf("otherRunningLocalStack returned error: %v", err)
		}
		if active != nil {
			t.Fatalf("expected no active stack, got %#v", active)
		}
	})

	t.Run("run command rejects a missing post-dash command", func(t *testing.T) {
		withTestDeps(t, nil)
		_, _, err := executeRoot(t, "run", "postgres", "--")
		if err == nil || !strings.Contains(err.Error(), "usage: stackctl run") {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
}
