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

func TestSetupCoverageBatchFive(t *testing.T) {
	t.Run("installed package notices propagate writer errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return []string{"podman"}, nil
			}
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Installed: podman"}, io.Discard, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected installed-package write failure, got %v", err)
		}
	})

	t.Run("podman machine success status propagates writer errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.platform = func() system.Platform { return system.Platform{GOOS: "darwin", PackageManager: "brew"} }
			d.preparePodmanMachine = func(context.Context, system.Runner) error { return nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
					doctorpkg.Check{Status: output.StatusFail, Message: "podman machine initialized"},
				), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "podman machine is initialized and running"}, io.Discard, "setup", "--install", "--yes")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected podman-machine status write failure, got %v", err)
		}
	})

	t.Run("next-steps heading write failures surface", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.isTerminal = func() bool { return false }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(
					doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
					doctorpkg.Check{Status: output.StatusOK, Message: "podman compose available"},
					doctorpkg.Check{Status: output.StatusOK, Message: "skopeo installed"},
				), nil
			}
		})

		err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Next steps:"}, io.Discard, "setup")
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected next-steps heading write failure, got %v", err)
		}
	})

	t.Run("direct next-steps bullet write failures surface", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&substringWriteErrorWriter{target: "- run `stackctl setup --install`"})

		cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return false }
		})

		err := printSetupNextSteps(cmd, cfg, []string{"podman"}, false, false, false)
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected next-steps bullet write failure, got %v", err)
		}
	})

	t.Run("displayRequirementLabels falls back to the platform plan when the configured manager is invalid", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.commandExists = func(string) bool { return false }
		})

		labels := displayRequirementLabels(
			[]system.Requirement{system.RequirementPodman, system.RequirementCockpit},
			"broken",
			system.Platform{GOOS: "linux", PackageManager: "apt", ServiceManager: system.ServiceManagerSystemd},
		)
		if len(labels) == 0 || !containsString(labels, "podman") {
			t.Fatalf("expected fallback package labels, got %+v", labels)
		}
	})

	t.Run("planDisplayLabels returns the fallback labels for an empty plan", func(t *testing.T) {
		fallback := []string{"podman", "skopeo"}
		if got := planDisplayLabels(system.InstallPlan{}, fallback); !sameStrings(got, fallback) {
			t.Fatalf("expected empty plans to preserve fallback labels, got %+v", got)
		}
	})
}

func TestTUIActionCoverageBatchFive(t *testing.T) {
	t.Run("loadTUIStackTargetConfig rejects invalid stack configs", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
			value.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
			value.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("parse failed") }
		})

		_, err := loadTUIStackTargetConfig("staging")
		if err == nil || !strings.Contains(err.Error(), "stack staging has an invalid config: parse failed") {
			t.Fatalf("expected invalid-config error, got %v", err)
		}
	})

	t.Run("loadTUIStackTargetConfig rejects configs with validation issues", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			cfg := configpkg.DefaultForStack("staging")
			cfg.ApplyDerivedFields()
			value.configFilePathForStack = func(string) (string, error) { return "/tmp/stackctl/stacks/staging.yaml", nil }
			value.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{name: "staging.yaml"}, nil }
			value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			value.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
				return []configpkg.ValidationIssue{{Field: "stack.dir", Message: "missing"}}
			}
		})

		_, err := loadTUIStackTargetConfig("staging")
		if err == nil || !strings.Contains(err.Error(), "stack staging config validation failed with 1 issue(s)") {
			t.Fatalf("expected validation error, got %v", err)
		}
	})

	t.Run("runTUIStop reports selected services on success", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.ApplyDerivedFields()
			value.composeStopServices = func(context.Context, system.Runner, configpkg.Config, []string) error { return nil }
		})

		report, err := runTUIStop(configpkg.DefaultForStack(configpkg.DefaultStackName), []string{"postgres"})
		if err != nil {
			t.Fatalf("runTUIStop returned error: %v", err)
		}
		if report.Message != "Postgres stopped" || len(report.Details) < 2 || report.Details[1] != "Service: Postgres" {
			t.Fatalf("unexpected stop report: %+v", report)
		}
	})

	t.Run("runTUIRestart verify-only path reports selected services", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.Behavior.WaitForServicesStart = false
			cfg.ApplyDerivedFields()
			value.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error { return nil }
			value.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			value.portListening = func(port int) bool { return port == cfg.Ports.Postgres }
		})

		cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
		cfg.Behavior.WaitForServicesStart = false
		cfg.ApplyDerivedFields()
		report, err := runTUIRestart(cfg, []string{"postgres"})
		if err != nil {
			t.Fatalf("runTUIRestart returned error: %v", err)
		}
		if report.Message != "Postgres restarted" || len(report.Details) < 2 || report.Details[1] != "Service: Postgres" {
			t.Fatalf("unexpected restart report: %+v", report)
		}
	})
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sameStrings(values []string, want []string) bool {
	if len(values) != len(want) {
		return false
	}
	for idx := range values {
		if values[idx] != want[idx] {
			return false
		}
	}
	return true
}

func TestSnapshotCoverageBatchFive(t *testing.T) {
	t.Run("snapshot save command propagates stop-stack failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("compose down failed")
			}
		})

		_, _, err := executeRoot(t, "snapshot", "save", filepath.Join(t.TempDir(), "snapshot.tar"), "--stop")
		if err == nil || !strings.Contains(err.Error(), "compose down failed") {
			t.Fatalf("expected stop-stack failure, got %v", err)
		}
	})

	t.Run("snapshot restore command propagates manifest validation failures", func(t *testing.T) {
		cfg := configpkg.DefaultForStack(configpkg.DefaultStackName)
		cfg.ApplyDerivedFields()

		dir := t.TempDir()
		payloadPath := filepath.Join(dir, "postgres.tar")
		if err := os.WriteFile(payloadPath, []byte("postgres-volume"), 0o600); err != nil {
			t.Fatalf("write snapshot payload: %v", err)
		}

		archivePath := filepath.Join(dir, "snapshot.tar")
		manifest := snapshotManifest{
			Version:   1,
			StackName: cfg.Stack.Name,
			Volumes: []snapshotVolumeRecord{{
				Service:    "unknown",
				SourceName: "postgres_data",
				Archive:    "volumes/postgres.tar",
			}},
		}
		if err := writeSnapshotArchive(archivePath, manifest, map[string]string{"volumes/postgres.tar": payloadPath}); err != nil {
			t.Fatalf("write snapshot archive: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "snapshot", "restore", archivePath, "--force")
		if err == nil || !strings.Contains(err.Error(), "snapshot services do not match the current stack") {
			t.Fatalf("expected manifest validation failure, got %v", err)
		}
	})
}
