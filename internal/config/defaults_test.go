package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigHasDerivedURLs(t *testing.T) {
	cfg := Default()

	if cfg.URLs.SeaweedFS != "http://localhost:8333" {
		t.Fatalf("unexpected seaweedfs URL: %s", cfg.URLs.SeaweedFS)
	}
	if cfg.URLs.Cockpit != "https://localhost:9090" {
		t.Fatalf("unexpected cockpit URL: %s", cfg.URLs.Cockpit)
	}

	if cfg.URLs.PgAdmin != "http://localhost:8081" {
		t.Fatalf("unexpected pgadmin URL: %s", cfg.URLs.PgAdmin)
	}
	if cfg.Connection.RedisPassword != "" {
		t.Fatalf("expected redis password to default to empty, got %q", cfg.Connection.RedisPassword)
	}
	if !cfg.Setup.IncludePostgres {
		t.Fatal("expected postgres to be enabled by default")
	}
	if !cfg.Setup.IncludeRedis {
		t.Fatal("expected redis to be enabled by default")
	}
	if !cfg.Setup.IncludeCockpit {
		t.Fatal("expected cockpit to be enabled by default")
	}
	if cfg.Connection.NATSToken != "stackctl" {
		t.Fatalf("unexpected nats token: %s", cfg.Connection.NATSToken)
	}
	if cfg.Connection.SeaweedFSAccessKey != "stackctl" {
		t.Fatalf("unexpected seaweedfs access key: %s", cfg.Connection.SeaweedFSAccessKey)
	}
	if cfg.Connection.SeaweedFSSecretKey != "stackctlsecret" {
		t.Fatalf("unexpected seaweedfs secret key: %s", cfg.Connection.SeaweedFSSecretKey)
	}
	if cfg.Ports.NATS != 4222 {
		t.Fatalf("unexpected nats port: %d", cfg.Ports.NATS)
	}
	if cfg.Ports.SeaweedFS != 8333 {
		t.Fatalf("unexpected seaweedfs port: %d", cfg.Ports.SeaweedFS)
	}
	if !cfg.Setup.IncludeNATS {
		t.Fatal("expected nats to be enabled by default")
	}
	if cfg.Setup.IncludeSeaweedFS {
		t.Fatal("expected seaweedfs to be disabled by default")
	}
	if cfg.Connection.PgAdminEmail != "admin@example.com" {
		t.Fatalf("unexpected pgadmin email: %s", cfg.Connection.PgAdminEmail)
	}
	if cfg.Connection.PgAdminPassword != "admin" {
		t.Fatalf("unexpected pgadmin password: %s", cfg.Connection.PgAdminPassword)
	}
	if cfg.Services.Postgres.MaxConnections != 100 {
		t.Fatalf("unexpected postgres max connections: %d", cfg.Services.Postgres.MaxConnections)
	}
	if cfg.Services.Postgres.SharedBuffers != "128MB" {
		t.Fatalf("unexpected postgres shared buffers: %s", cfg.Services.Postgres.SharedBuffers)
	}
	if cfg.Services.Postgres.LogMinDurationStatementMS != -1 {
		t.Fatalf("unexpected postgres log duration default: %d", cfg.Services.Postgres.LogMinDurationStatementMS)
	}
	if !cfg.Services.PgAdmin.BootstrapPostgresServer {
		t.Fatal("expected pgadmin postgres bootstrap to default on")
	}
	if cfg.Services.PgAdmin.BootstrapServerName != "Local Postgres" {
		t.Fatalf("unexpected pgadmin bootstrap server name: %s", cfg.Services.PgAdmin.BootstrapServerName)
	}
	if cfg.Services.PgAdmin.BootstrapServerGroup != "Local" {
		t.Fatalf("unexpected pgadmin bootstrap server group: %s", cfg.Services.PgAdmin.BootstrapServerGroup)
	}
}

func TestDefaultForNamedStackUsesStackSpecificManagedDefaults(t *testing.T) {
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	cfg := DefaultForStack("staging")

	if cfg.Stack.Name != "staging" {
		t.Fatalf("unexpected stack name: %s", cfg.Stack.Name)
	}
	if cfg.Stack.Dir != filepath.Join(dataRoot, "stackctl", "stacks", "staging") {
		t.Fatalf("unexpected stack dir: %s", cfg.Stack.Dir)
	}
	if cfg.Services.PostgresContainer != "stackctl-staging-postgres" {
		t.Fatalf("unexpected postgres container: %s", cfg.Services.PostgresContainer)
	}
	if cfg.Services.RedisContainer != "stackctl-staging-redis" {
		t.Fatalf("unexpected redis container: %s", cfg.Services.RedisContainer)
	}
	if cfg.Services.NATSContainer != "stackctl-staging-nats" {
		t.Fatalf("unexpected nats container: %s", cfg.Services.NATSContainer)
	}
	if cfg.Services.SeaweedFSContainer != "stackctl-staging-seaweedfs" {
		t.Fatalf("unexpected seaweedfs container: %s", cfg.Services.SeaweedFSContainer)
	}
	if cfg.Services.PgAdminContainer != "stackctl-staging-pgadmin" {
		t.Fatalf("unexpected pgadmin container: %s", cfg.Services.PgAdminContainer)
	}
	if cfg.Services.SeaweedFS.DataVolume != "stackctl-staging-seaweedfs-data" {
		t.Fatalf("unexpected seaweedfs data volume: %s", cfg.Services.SeaweedFS.DataVolume)
	}
}
