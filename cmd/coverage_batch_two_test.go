package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestConfigCommandAdditionalBranches(t *testing.T) {
	t.Run("init path and load failures surface directly", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("config path unavailable") }
		})

		_, _, err := executeRoot(t, "config", "init", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "config path unavailable") {
			t.Fatalf("unexpected init path error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") }
		})

		_, _, err = executeRoot(t, "config", "init", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "load failed") {
			t.Fatalf("unexpected init load error: %v", err)
		}
	})

	t.Run("init surfaces wizard scaffold and save failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
				return configpkg.Config{}, errors.New("wizard failed")
			}
		})

		_, _, err := executeRoot(t, "config", "init")
		if err == nil || !strings.Contains(err.Error(), "wizard failed") {
			t.Fatalf("unexpected init wizard error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, errors.New("scaffold check failed") }
		})

		_, _, err = executeRoot(t, "config", "init", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "scaffold check failed") {
			t.Fatalf("unexpected init scaffold error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.saveConfig = func(string, configpkg.Config) error { return errors.New("save failed") }
		})

		_, _, err = executeRoot(t, "config", "init", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("unexpected init save error: %v", err)
		}
	})

	t.Run("path and edit subcommands propagate setup failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("path failed") }
		})

		_, _, err := executeRoot(t, "config", "path")
		if err == nil || !strings.Contains(err.Error(), "path failed") {
			t.Fatalf("unexpected config path error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("edit path failed") }
		})

		_, _, err = executeRoot(t, "config", "edit", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "edit path failed") {
			t.Fatalf("unexpected config edit path error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.isTerminal = func() bool { return true }
			d.runWizard = func(io.Reader, io.Writer, configpkg.Config) (configpkg.Config, error) {
				return configpkg.Config{}, errors.New("edit wizard failed")
			}
		})

		_, _, err = executeRoot(t, "config", "edit")
		if err == nil || !strings.Contains(err.Error(), "edit wizard failed") {
			t.Fatalf("unexpected config edit wizard error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, errors.New("edit scaffold failed") }
		})

		_, _, err = executeRoot(t, "config", "edit", "--non-interactive")
		if err == nil || !strings.Contains(err.Error(), "edit scaffold failed") {
			t.Fatalf("unexpected config edit scaffold error: %v", err)
		}
	})

	t.Run("validate and reset cover write prompt and persistence errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
				return []configpkg.ValidationIssue{{Field: "stack.dir", Message: "must exist"}}
			}
		})

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "config", "validate"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected config validate write failure, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.configFilePath = func() (string, error) { return "", errors.New("reset path failed") }
		})

		_, _, err := executeRoot(t, "config", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "reset path failed") {
			t.Fatalf("unexpected config reset path error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, nil }
		})

		stdout, _, err := executeRoot(t, "config", "reset", "--delete")
		if err != nil {
			t.Fatalf("expected delete reset cancellation, got %v", err)
		}
		if !strings.Contains(stdout, "config reset cancelled") {
			t.Fatalf("expected delete cancel message, got %q", stdout)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
			d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) { return false, io.ErrUnexpectedEOF }
		})

		_, _, err = executeRoot(t, "config", "reset", "--delete")
		if err == nil || !strings.Contains(err.Error(), "delete confirmation required") {
			t.Fatalf("unexpected config delete prompt error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, errors.New("reset scaffold failed") }
		})

		_, _, err = executeRoot(t, "config", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "reset scaffold failed") {
			t.Fatalf("unexpected config reset scaffold error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.saveConfig = func(string, configpkg.Config) error { return errors.New("reset save failed") }
		})

		_, _, err = executeRoot(t, "config", "reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "reset save failed") {
			t.Fatalf("unexpected config reset save error: %v", err)
		}
	})

	t.Run("scaffold reports missing config hint", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
		})

		_, _, err := executeRoot(t, "config", "scaffold")
		if err == nil || !strings.Contains(err.Error(), "no stackctl config was found") {
			t.Fatalf("unexpected config scaffold missing-config error: %v", err)
		}
	})
}

func TestDoctorCommandAdditionalBranches(t *testing.T) {
	t.Run("doctor command surfaces run and report failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, errors.New("doctor failed") }
		})

		_, _, err := executeRoot(t, "doctor")
		if err == nil || !strings.Contains(err.Error(), "doctor failed") {
			t.Fatalf("unexpected doctor run error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusOK, Message: "config file found"}), nil
			}
		})

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "doctor"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected doctor report write failure, got %v", err)
		}

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "doctor", "--fix", "--yes"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected doctor --fix initial report write failure, got %v", err)
		}
	})

	t.Run("doctor fix propagates config package and post-fix errors", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) { return doctorpkg.Report{}, nil }
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load config failed") }
		})

		_, _, err := executeRoot(t, "doctor", "--fix", "--yes")
		if err == nil || !strings.Contains(err.Error(), "load config failed") {
			t.Fatalf("unexpected doctor --fix config load error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.System.PackageManager = ""
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
		})

		if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "using detected apt"}, io.Discard, "doctor", "--fix", "--yes"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected package manager notice write failure, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return nil, errors.New("install failed")
			}
		})

		_, _, err = executeRoot(t, "doctor", "--fix", "--yes")
		if err == nil || !strings.Contains(err.Error(), "install failed") {
			t.Fatalf("unexpected doctor --fix install error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"}), nil
			}
			d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
				return []string{"podman"}, nil
			}
		})

		if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "Installed: podman"}, io.Discard, "doctor", "--fix", "--yes"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected installed-line write failure, got %v", err)
		}

		doctorRuns := 0
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				doctorRuns++
				if doctorRuns == 1 {
					return doctorpkg.Report{}, nil
				}
				return doctorpkg.Report{}, errors.New("post-fix rerun failed")
			}
		})

		_, _, err = executeRoot(t, "doctor", "--fix", "--yes")
		if err == nil || !strings.Contains(err.Error(), "post-fix rerun failed") {
			t.Fatalf("unexpected post-fix rerun error: %v", err)
		}

		doctorRuns = 0
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				doctorRuns++
				return doctorpkg.Report{}, nil
			}
		})

		if err := executeRootWithIO(t, &exactWriteErrorWriter{target: "\nPost-fix report:\n"}, io.Discard, "doctor", "--fix", "--yes"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected post-fix heading write failure, got %v", err)
		}
	})
}

func TestFactoryResetAdditionalBranches(t *testing.T) {
	t.Run("early path resolution failures surface directly", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.configDirPath = func() (string, error) { return "", errors.New("config dir failed") }
		})

		_, _, err := executeRoot(t, "factory-reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "config dir failed") {
			t.Fatalf("unexpected factory-reset config dir error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.knownConfigPaths = func() ([]string, error) { return nil, errors.New("config discovery failed") }
		})

		_, _, err = executeRoot(t, "factory-reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "config discovery failed") {
			t.Fatalf("unexpected factory-reset config discovery error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.dataDirPath = func() (string, error) { return "", errors.New("data dir failed") }
		})

		_, _, err = executeRoot(t, "factory-reset", "--force")
		if err == nil || !strings.Contains(err.Error(), "data dir failed") {
			t.Fatalf("unexpected factory-reset data dir error: %v", err)
		}
	})

	t.Run("cleanup target discovery and final status writes are covered", func(t *testing.T) {
		dataDir := t.TempDir()
		stacksPath := filepath.Join(dataDir, "stacks")
		if err := os.WriteFile(stacksPath, []byte("blocking file"), 0o600); err != nil {
			t.Fatalf("write blocking stacks path: %v", err)
		}

		_, err := localComposeCleanupTargets(nil, dataDir)
		if err == nil || !strings.Contains(err.Error(), "read managed stacks dir") {
			t.Fatalf("unexpected cleanup target discovery error: %v", err)
		}

		for _, tc := range []struct {
			name      string
			failAfter int
		}{
			{name: "config status write", failAfter: 2},
			{name: "data status write", failAfter: 3},
			{name: "final status write", failAfter: 4},
		} {
			t.Run(tc.name, func(t *testing.T) {
				withTestDeps(t, nil)
				if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: tc.failAfter}, io.Discard, "factory-reset", "--force"); err == nil || !strings.Contains(err.Error(), "write failed") {
					t.Fatalf("expected factory-reset write failure after %d writes, got %v", tc.failAfter, err)
				}
			})
		}
	})
}

func TestServiceRegistryAdditionalBranches(t *testing.T) {
	disabled := configpkg.DefaultForStack("staging")
	disabled.Setup.IncludePostgres = false
	disabled.Setup.IncludeRedis = false
	disabled.Setup.IncludeNATS = false
	disabled.Setup.IncludeSeaweedFS = false
	disabled.Setup.IncludeMeilisearch = false
	disabled.Setup.IncludePgAdmin = false
	disabled.ApplyDerivedFields()

	testCases := []struct {
		target string
		substr string
	}{
		{target: "postgres", substr: "postgres is not enabled"},
		{target: "postgres-user", substr: "postgres is not enabled"},
		{target: "postgres-password", substr: "postgres is not enabled"},
		{target: "postgres-database", substr: "postgres is not enabled"},
		{target: "redis-password", substr: "redis is not enabled"},
		{target: "nats-token", substr: "nats is not enabled"},
		{target: "seaweedfs-access-key", substr: "seaweedfs is not enabled"},
		{target: "seaweedfs-secret-key", substr: "seaweedfs is not enabled"},
		{target: "meilisearch-api-key", substr: "meilisearch is not enabled"},
		{target: "pgadmin-email", substr: "pgadmin is not enabled"},
		{target: "pgadmin-password", substr: "pgadmin is not enabled"},
	}

	for _, tc := range testCases {
		spec, ok := copyTargetSpec(disabled, tc.target)
		if !ok {
			t.Fatalf("expected copy target %q to exist", tc.target)
		}
		_, err := spec.Resolve(disabled)
		if err == nil || !strings.Contains(err.Error(), tc.substr) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.target, tc.substr, err)
		}
	}

	cfg := configpkg.DefaultForStack("staging")
	cfg.Services.PgAdmin.BootstrapPostgresServer = false
	cfg.ApplyDerivedFields()
	cfg.URLs.Cockpit = ""

	pgAdmin, ok := serviceDefinitionByKey("pgadmin")
	if !ok {
		t.Fatal("expected pgadmin definition")
	}
	runtime := runtimeServiceForDefinition(context.Background(), cfg, pgAdmin, containerMapForDefinitions(cfg, enabledServiceDefinitions(cfg)))
	if runtime.BootstrapServer != "" || runtime.BootstrapGroup != "" {
		t.Fatalf("expected pgadmin bootstrap fields to stay empty when bootstrap is disabled, got %+v", runtime)
	}

	cockpit, ok := serviceDefinitionByKey("cockpit")
	if !ok {
		t.Fatal("expected cockpit definition")
	}
	if entries := cockpit.ConnectionEntries(cfg); len(entries) != 0 {
		t.Fatalf("expected cockpit connection entries to be empty without a URL, got %+v", entries)
	}

	if got := stringsJoin(nil, ", "); got != "" {
		t.Fatalf("expected stringsJoin(nil) to be empty, got %q", got)
	}

	if err := ensureServiceEnabled(cfg, "not-a-service"); err == nil || !strings.Contains(err.Error(), "invalid service") {
		t.Fatalf("unexpected invalid service error: %v", err)
	}
	if err := ensureServiceEnabled(disabled, "postgres"); err == nil || !strings.Contains(err.Error(), "postgres is not enabled") {
		t.Fatalf("unexpected disabled service error: %v", err)
	}
}

func TestRunCommandAdditionalBranches(t *testing.T) {
	t.Run("run command propagates config and readiness failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") }
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "load failed") {
			t.Fatalf("unexpected run config error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
		})

		_, _, err = executeRoot(t, "run", "--no-start", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "postgres is not ready") {
			t.Fatalf("unexpected run --no-start readiness error: %v", err)
		}
	})

	t.Run("run command covers verbose and startup-path failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(configpkg.Default(), "postgres")}, nil
			}
		})

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "--verbose", "run", "--no-start", "postgres", "--", "echo", "hi"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected run verbose write failure, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, errors.New("scaffold failed") }
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "scaffold failed") {
			t.Fatalf("unexpected run scaffold error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			d.podmanComposeAvail = func(context.Context) bool { return false }
		})

		_, _, err = executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "podman compose is not available") {
			t.Fatalf("unexpected run compose-runtime error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		})

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "--verbose", "run", "postgres", "--", "echo", "hi"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected run verbose compose-file write failure, got %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.waitForPort = func(context.Context, int, time.Duration) error { return errors.New("wait failed") }
		})

		_, _, err = executeRoot(t, "run", "postgres", "--", "echo", "hi")
		if err == nil || !strings.Contains(err.Error(), "wait failed") {
			t.Fatalf("unexpected run wait error: %v", err)
		}
	})
}
