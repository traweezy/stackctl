package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func richRuntimeCoverageConfig() configpkg.Config {
	cfg := configpkg.Default()
	cfg.Setup.IncludeNATS = true
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.Setup.IncludePgAdmin = true
	cfg.Setup.IncludeCockpit = true
	cfg.Connection.RedisPassword = "redis-default"
	cfg.Connection.RedisACLUsername = "cache"
	cfg.Connection.RedisACLPassword = "cache-secret"
	cfg.Connection.NATSToken = "dev-nats-token"
	cfg.Connection.SeaweedFSAccessKey = "weed-access"
	cfg.Connection.SeaweedFSSecretKey = "weed-secret"
	cfg.Connection.MeilisearchMasterKey = "meili-master"
	cfg.Connection.PgAdminEmail = "ops@example.com"
	cfg.Connection.PgAdminPassword = "pgadmin-secret"
	cfg.Services.Postgres.MaxConnections = 250
	cfg.Services.Postgres.SharedBuffers = "256MB"
	cfg.Services.Postgres.LogMinDurationStatementMS = -1
	cfg.Services.Redis.AppendOnly = true
	cfg.Services.Redis.SavePolicy = "60 1000"
	cfg.Services.Redis.MaxMemoryPolicy = "allkeys-lru"
	cfg.Services.SeaweedFS.VolumeSizeLimitMB = 2048
	cfg.Services.PgAdmin.ServerMode = true
	cfg.Services.PgAdmin.BootstrapPostgresServer = true
	cfg.Services.PgAdmin.BootstrapServerName = "Stack Postgres"
	cfg.Services.PgAdmin.BootstrapServerGroup = "Stack"
	cfg.ApplyDerivedFields()
	return cfg
}

func TestPrintServicesInfoWriterFailuresCoverLaterBranches(t *testing.T) {
	cfg := richRuntimeCoverageConfig()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.portListening = func(int) bool { return true }
		d.portInUse = func(int) (bool, error) { return false, nil }
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
	})

	failures := 0
	for failAfter := 1; failAfter <= 96; failAfter++ {
		cmd := &cobra.Command{Use: "services"}
		writer := &failingWriteBuffer{failAfter: failAfter}
		cmd.SetOut(writer)

		err := printServicesInfo(cmd, cfg)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected printServicesInfo to fail with a write error at write %d, got %v", failAfter, err)
		}
		failures++
	}

	if failures < 20 {
		t.Fatalf("expected to trigger many late writer failures, got %d", failures)
	}
}

func TestRuntimeAdditionalHelperAndFirstRunBranches(t *testing.T) {
	t.Run("loadRuntimeConfig surfaces first-run scaffold, save, status, and validation failures", func(t *testing.T) {
		t.Run("config path failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.configFilePath = func() (string, error) { return "", errors.New("config path boom") }
			})

			_, err := loadRuntimeConfig(NewRootCmd(NewApp()), false)
			if err == nil || !strings.Contains(err.Error(), "config path boom") {
				t.Fatalf("unexpected config path error: %v", err)
			}
		})

		t.Run("first-run scaffold failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.isTerminal = func() bool { return true }
				d.promptYesNo = func(_ io.Reader, _ io.Writer, _ string, _ bool) (bool, error) { return true, nil }
				d.runWizard = func(_ io.Reader, _ io.Writer, _ configpkg.Config) (configpkg.Config, error) {
					return configpkg.Default(), nil
				}
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) { return false, errors.New("scaffold boom") }
			})

			root := NewRootCmd(NewApp())
			var stdout strings.Builder
			root.SetOut(&stdout)

			_, err := loadRuntimeConfig(root, true)
			if err == nil || !strings.Contains(err.Error(), "scaffold boom") {
				t.Fatalf("unexpected scaffold error: %v", err)
			}
		})

		t.Run("first-run save failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.isTerminal = func() bool { return true }
				d.promptYesNo = func(_ io.Reader, _ io.Writer, _ string, _ bool) (bool, error) { return true, nil }
				d.runWizard = func(_ io.Reader, _ io.Writer, _ configpkg.Config) (configpkg.Config, error) {
					cfg := configpkg.Default()
					cfg.Stack.Managed = false
					cfg.Setup.ScaffoldDefaultStack = false
					return cfg, nil
				}
				d.saveConfig = func(string, configpkg.Config) error { return errors.New("save boom") }
			})

			root := NewRootCmd(NewApp())
			var stdout strings.Builder
			root.SetOut(&stdout)

			_, err := loadRuntimeConfig(root, true)
			if err == nil || !strings.Contains(err.Error(), "save boom") {
				t.Fatalf("unexpected save error: %v", err)
			}
		})

		t.Run("first-run status write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.isTerminal = func() bool { return true }
				d.promptYesNo = func(_ io.Reader, _ io.Writer, _ string, _ bool) (bool, error) { return true, nil }
				d.runWizard = func(_ io.Reader, _ io.Writer, _ configpkg.Config) (configpkg.Config, error) {
					cfg := configpkg.Default()
					cfg.Stack.Managed = false
					cfg.Setup.ScaffoldDefaultStack = false
					return cfg, nil
				}
			})

			root := NewRootCmd(NewApp())
			root.SetOut(&failingWriteBuffer{failAfter: 2})

			_, err := loadRuntimeConfig(root, true)
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected status write failure, got %v", err)
			}
		})

		t.Run("validation issue print failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.validateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
					return []configpkg.ValidationIssue{{Field: "stack.dir", Message: "invalid"}}
				}
			})

			root := NewRootCmd(NewApp())
			root.SetOut(&failingWriteBuffer{failAfter: 1})

			_, err := loadRuntimeConfig(root, false)
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected validation print failure, got %v", err)
			}
		})
	})

	t.Run("runtime helpers cover empty, error, and fallback branches", func(t *testing.T) {
		if got := formatPorts(nil); got != "-" {
			t.Fatalf("expected empty port list to render '-', got %q", got)
		}

		state := inspectStackServicePort(0, false)
		if state.Listening || state.Conflict || state.CheckErr != nil {
			t.Fatalf("expected zero-port inspection to stay empty, got %+v", state)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.portInUse = func(int) (bool, error) { return false, errors.New("probe failed") }
		})
		state = inspectStackServicePort(8080, false)
		if state.CheckErr == nil || !strings.Contains(state.CheckErr.Error(), "probe failed") {
			t.Fatalf("expected inspectStackServicePort to report probe failures, got %+v", state)
		}

		line := checkStackServicePort(
			configpkg.Default(),
			serviceDefinition{
				Key:              "postgres",
				DisplayName:      "Postgres",
				PrimaryPortLabel: "postgres port listening",
				PrimaryPort:      func(configpkg.Config) int { return 5432 },
				ContainerName:    func(configpkg.Config) string { return "stack-postgres" },
			},
			nil,
		)
		if line.Status != output.StatusFail || !strings.Contains(line.Message, "probe failed") {
			t.Fatalf("expected failing port check output, got %+v", line)
		}

		if got := cockpitRuntimeStateLabel(system.CockpitState{Installed: true, State: "inactive"}, stackServicePortState{}); got != "inactive" {
			t.Fatalf("expected inactive cockpit runtime label, got %q", got)
		}

		cfg := configpkg.Default()
		cfg.Setup.IncludeCockpit = true
		cfg.ApplyDerivedFields()
		entries := connectionEntries(cfg)
		if len(entries) == 0 {
			t.Fatal("expected connection entries for enabled services")
		}

		groups, err := envGroups(cfg, []string{"postgres", "postgres"})
		if err != nil {
			t.Fatalf("expected duplicate env selections to collapse, got %v", err)
		}
		if len(groups) < 2 {
			t.Fatalf("expected duplicate env target selection to keep a single postgres group, got %+v", groups)
		}

		cmd := &cobra.Command{Use: "env"}
		err = printEnvJSON(cmd, cfg, []string{"invalid"})
		if err == nil || !strings.Contains(err.Error(), "invalid env target") {
			t.Fatalf("expected invalid env target error, got %v", err)
		}
	})
}
