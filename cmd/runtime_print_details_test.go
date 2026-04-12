package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestPrintServicesInfoRendersExpandedRuntimeFields(t *testing.T) {
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

	cmd := &cobra.Command{Use: "services"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := printServicesInfo(cmd, cfg); err != nil {
		t.Fatalf("printServicesInfo returned error: %v", err)
	}

	text := out.String()
	for _, fragment := range []string{
		"Max connections: 250",
		"Shared buffers: 256MB",
		"Log min duration: disabled",
		"Username: cache",
		"Password: cache-secret",
		"Appendonly: enabled",
		"Save policy: 60 1000",
		"Maxmemory policy: allkeys-lru",
		"Token: dev-nats-token",
		"Access key: weed-access",
		"Secret key: weed-secret",
		"Volume size limit: 2048 MB",
		"Endpoint: http://localhost:8333",
		"Master key: meili-master",
		"Email: ops@example.com",
		"Server mode: enabled",
		"Bootstrap server: Stack Postgres",
		"Bootstrap group: Stack",
		"URL: https://localhost:9090",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected services output to contain %q:\n%s", fragment, text)
		}
	}
}
