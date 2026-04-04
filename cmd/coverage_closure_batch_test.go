package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSetupAdditionalControlFlowBranches(t *testing.T) {
	t.Run("rejects conflicting interactive flags", func(t *testing.T) {
		_, _, err := executeRoot(t, "setup", "--interactive", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
			t.Fatalf("unexpected setup flag error: %v", err)
		}
	})

	t.Run("requires non-interactive mode without a terminal", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return false }
		})

		_, _, err := executeRoot(t, "setup")
		if err == nil || !strings.Contains(err.Error(), "rerun with --non-interactive") {
			t.Fatalf("unexpected setup no-terminal error: %v", err)
		}
	})

	t.Run("install noops when nothing is missing", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
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

		stdout, _, err := executeRoot(t, "setup", "--install", "--yes")
		if err != nil {
			t.Fatalf("setup --install returned error: %v", err)
		}
		if !strings.Contains(stdout, "nothing to install") {
			t.Fatalf("expected setup no-op output, got:\n%s", stdout)
		}
	})

	t.Run("warns about stale scaffold when it cannot prompt", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return false }
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"}), nil
			}
		})

		stdout, _, err := executeRoot(t, "setup")
		if err != nil {
			t.Fatalf("setup returned error: %v", err)
		}
		if !strings.Contains(stdout, "managed stack files are missing") {
			t.Fatalf("expected stale-scaffold warning, got:\n%s", stdout)
		}
	})

	t.Run("returns install package errors", func(t *testing.T) {
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
			t.Fatalf("unexpected setup install error: %v", err)
		}
	})
}

func TestDoctorAdditionalFixBranches(t *testing.T) {
	t.Run("unsupported package manager returns guidance", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.System.PackageManager = "pkgx"
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.commandExists = func(string) bool { return false }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		_, _, err := executeRoot(t, "doctor", "--fix", "--yes")
		if err == nil || !strings.Contains(err.Error(), "doctor cannot install missing packages automatically") {
			t.Fatalf("unexpected doctor package-manager error: %v", err)
		}
	})

	t.Run("declined scaffold refresh reports no automatic fixes", func(t *testing.T) {
		var doctorRuns int

		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			d.isTerminal = func() bool { return true }
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return true, nil }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				doctorRuns++
				return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"}), nil
			}
		})

		stdout, _, err := executeRoot(t, "doctor", "--fix")
		if err != nil {
			t.Fatalf("doctor --fix returned error: %v", err)
		}
		if doctorRuns != 2 {
			t.Fatalf("expected doctor to run twice, got %d", doctorRuns)
		}
		if !strings.Contains(stdout, "no automatic fixes were applied") {
			t.Fatalf("expected no-fixes message, got:\n%s", stdout)
		}
	})
}

func TestDBAdditionalGuardrails(t *testing.T) {
	t.Run("dump rejects positional and flag output together", func(t *testing.T) {
		_, _, err := executeRoot(t, "db", "dump", "dump.sql", "--output", "other.sql")
		if err == nil || !strings.Contains(err.Error(), "either a positional path or --output") {
			t.Fatalf("unexpected db dump flag error: %v", err)
		}
	})

	t.Run("restore requires force when confirmation is unavailable", func(t *testing.T) {
		path := writeTempFile(t, "dump.sql", "select 1;\n")
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		})

		_, _, err := executeRoot(t, "db", "restore", path)
		if err == nil || !strings.Contains(err.Error(), "database restore confirmation required") {
			t.Fatalf("unexpected db restore confirmation error: %v", err)
		}
	})

	t.Run("reset refuses the maintenance database", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Connection.PostgresDatabase = "postgres"
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "db", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "does not support resetting the postgres maintenance database") {
			t.Fatalf("unexpected db reset maintenance-db error: %v", err)
		}
	})
}

func TestConfigAdditionalCommandBranches(t *testing.T) {
	t.Run("validate returns missing config guidance", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
		})

		_, _, err := executeRoot(t, "config", "validate")
		if err == nil || !strings.Contains(err.Error(), "no stackctl config was found") {
			t.Fatalf("unexpected config validate error: %v", err)
		}
	})

	t.Run("view returns marshal errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.marshalConfig = func(configpkg.Config) ([]byte, error) { return nil, errors.New("marshal failed") }
		})

		_, _, err := executeRoot(t, "config", "view")
		if err == nil || !strings.Contains(err.Error(), "marshal failed") {
			t.Fatalf("unexpected config view error: %v", err)
		}
	})
}

func TestRunAndRestartAdditionalBranches(t *testing.T) {
	t.Run("run requires command after dash", func(t *testing.T) {
		_, _, err := executeRoot(t, "run", "postgres")
		if err == nil || !strings.Contains(err.Error(), "usage: stackctl run") {
			t.Fatalf("unexpected run usage error: %v", err)
		}
	})

	t.Run("restart returns compose down errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("compose down failed")
			}
		})

		_, _, err := executeRoot(t, "restart")
		if err == nil || !strings.Contains(err.Error(), "compose down failed") {
			t.Fatalf("unexpected restart compose-down error: %v", err)
		}
	})

	t.Run("restart returns compose up service errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
			d.portInUse = func(int) (bool, error) { return false, nil }
			d.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				return errors.New("compose up services failed")
			}
		})

		_, _, err := executeRoot(t, "restart", "postgres")
		if err == nil || !strings.Contains(err.Error(), "compose up services failed") {
			t.Fatalf("unexpected restart service error: %v", err)
		}
	})
}

func TestRuntimeHelperAdditionalBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Setup.IncludePgAdmin = false
	cfg.ApplyDerivedFields()

	t.Run("envGroups rejects invalid and disabled services", func(t *testing.T) {
		if _, err := envGroups(cfg, []string{"invalid"}); err == nil || !strings.Contains(err.Error(), "invalid env target") {
			t.Fatalf("unexpected invalid env target error: %v", err)
		}
		if _, err := envGroups(cfg, []string{"pgadmin"}); err == nil || !strings.Contains(err.Error(), "pgadmin is not enabled") {
			t.Fatalf("unexpected disabled env target error: %v", err)
		}
	})

	t.Run("envGroups dedupes repeated aliases", func(t *testing.T) {
		groups, err := envGroups(cfg, []string{"postgres", "postgres"})
		if err != nil {
			t.Fatalf("envGroups returned error: %v", err)
		}
		if len(groups) < 2 {
			t.Fatalf("expected stackctl + postgres groups, got %+v", groups)
		}
		if got := groups[1].Title; got != "Postgres" {
			t.Fatalf("unexpected deduped group order/title: %+v", groups)
		}
	})

	t.Run("service copy target rejects invalid keys", func(t *testing.T) {
		if _, _, err := serviceCopyTarget(cfg, "not-a-target"); err == nil || !strings.Contains(err.Error(), "invalid copy target") {
			t.Fatalf("unexpected copy-target error: %v", err)
		}
	})

	t.Run("status helpers cover fallback states", func(t *testing.T) {
		containers := map[string]system.Container{
			"postgres": {
				State: "paused",
				Ports: []system.ContainerPort{{HostPort: 5432, ContainerPort: 15432}},
			},
			"redis": {
				State: "",
			},
		}

		if got := containerStatus(containers, "missing"); got != "missing" {
			t.Fatalf("unexpected missing container status %q", got)
		}
		if got := containerStatus(containers, "postgres"); got != "paused" {
			t.Fatalf("unexpected paused container status %q", got)
		}
		if got := containerStatus(containers, "redis"); got != "stopped" {
			t.Fatalf("unexpected stopped container status %q", got)
		}
		if got := cockpitRuntimeStateLabel(system.CockpitState{Installed: true, Active: true}, stackServicePortState{}); got != "needs attention" {
			t.Fatalf("unexpected cockpit attention label %q", got)
		}
		if got := cockpitRuntimeStateLabel(system.CockpitState{Installed: false}, stackServicePortState{}); got != "missing" {
			t.Fatalf("unexpected cockpit missing label %q", got)
		}
		if got := containerInternalPort(containers, "postgres", 9999); got != 15432 {
			t.Fatalf("unexpected fallback internal port %d", got)
		}
	})
}

func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}
