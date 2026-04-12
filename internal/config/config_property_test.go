package config

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func TestApplyDerivedFieldsAndMarshalStayStable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cfg := drawRapidConfig(t)

		once := cfg
		once.ApplyDerivedFields()

		twice := once
		twice.ApplyDerivedFields()

		if !reflect.DeepEqual(once, twice) {
			t.Fatalf("ApplyDerivedFields should be idempotent:\nfirst:  %+v\nsecond: %+v", once, twice)
		}

		data, err := Marshal(once)
		if err != nil {
			t.Fatalf("Marshal returned error: %v", err)
		}

		var roundTripped Config
		if err := yaml.Unmarshal(data, &roundTripped); err != nil {
			t.Fatalf("yaml.Unmarshal returned error: %v", err)
		}
		roundTripped.ApplyDerivedFields()

		if !reflect.DeepEqual(once, roundTripped) {
			t.Fatalf("round-trip mismatch:\nwant: %+v\ngot:  %+v", once, roundTripped)
		}
	})
}

func drawRapidConfig(t *rapid.T) Config {
	t.Helper()

	cfg := Default()
	cfg.Stack.Name = rapid.StringMatching(`[a-z0-9][a-z0-9_-]{0,11}`).Draw(t, "stack-name")
	cfg.Stack.Managed = rapid.Bool().Draw(t, "stack-managed")
	cfg.Setup.ScaffoldDefaultStack = rapid.Bool().Draw(t, "setup-scaffold-default-stack")
	cfg.Setup.IncludePostgres = rapid.Bool().Draw(t, "setup-include-postgres")
	cfg.Setup.IncludeRedis = rapid.Bool().Draw(t, "setup-include-redis")
	cfg.Setup.IncludeNATS = rapid.Bool().Draw(t, "setup-include-nats")
	cfg.Setup.IncludeSeaweedFS = rapid.Bool().Draw(t, "setup-include-seaweedfs")
	cfg.Setup.IncludeMeilisearch = rapid.Bool().Draw(t, "setup-include-meilisearch")
	cfg.Setup.IncludePgAdmin = rapid.Bool().Draw(t, "setup-include-pgadmin")
	cfg.Setup.IncludeCockpit = rapid.Bool().Draw(t, "setup-include-cockpit")
	cfg.Setup.InstallCockpit = rapid.Bool().Draw(t, "setup-install-cockpit")
	cfg.Connection.Host = rapid.SampledFrom([]string{"localhost", "127.0.0.1", "stack.local"}).Draw(t, "connection-host")
	cfg.Connection.PostgresDatabase = rapid.StringMatching(`[a-z0-9_]{1,12}`).Draw(t, "connection-postgres-database")
	cfg.Connection.PostgresUsername = rapid.StringMatching(`[a-z0-9_]{1,12}`).Draw(t, "connection-postgres-username")
	cfg.Connection.PostgresPassword = rapid.StringMatching(`[a-z0-9_-]{1,16}`).Draw(t, "connection-postgres-password")
	cfg.Connection.RedisPassword = rapid.StringMatching(`[a-z0-9_-]{0,16}`).Draw(t, "connection-redis-password")
	cfg.Connection.NATSToken = rapid.StringMatching(`[a-z0-9_-]{1,16}`).Draw(t, "connection-nats-token")
	cfg.Connection.SeaweedFSAccessKey = rapid.StringMatching(`[A-Za-z0-9]{1,12}`).Draw(t, "connection-seaweedfs-access-key")
	cfg.Connection.SeaweedFSSecretKey = rapid.StringMatching(`[A-Za-z0-9]{8,16}`).Draw(t, "connection-seaweedfs-secret-key")
	cfg.Connection.MeilisearchMasterKey = rapid.StringMatching(`[A-Za-z0-9_-]{16,24}`).Draw(t, "connection-meilisearch-master-key")
	cfg.Connection.PgAdminEmail = rapid.SampledFrom([]string{"admin@example.com", "ops@example.com"}).Draw(t, "connection-pgadmin-email")
	cfg.Connection.PgAdminPassword = rapid.StringMatching(`[A-Za-z0-9_-]{6,16}`).Draw(t, "connection-pgadmin-password")
	cfg.Services.Postgres.Image = rapid.SampledFrom([]string{"docker.io/library/postgres:16", "docker.io/library/postgres:17"}).Draw(t, "services-postgres-image")
	cfg.Services.Redis.Image = rapid.SampledFrom([]string{"docker.io/library/redis:7", "docker.io/library/redis:7.4"}).Draw(t, "services-redis-image")
	cfg.Services.NATS.Image = rapid.SampledFrom([]string{"docker.io/library/nats:2.12.5"}).Draw(t, "services-nats-image")
	cfg.Services.SeaweedFS.Image = rapid.SampledFrom([]string{"docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9"}).Draw(t, "services-seaweedfs-image")
	cfg.Services.Meilisearch.Image = rapid.SampledFrom([]string{"docker.io/getmeili/meilisearch:v1.40.0"}).Draw(t, "services-meilisearch-image")
	cfg.Services.PgAdmin.Image = rapid.SampledFrom([]string{"docker.io/dpage/pgadmin4:latest", "docker.io/dpage/pgadmin4:9"}).Draw(t, "services-pgadmin-image")
	cfg.Ports.Postgres = rapid.IntRange(1024, 65535).Draw(t, "ports-postgres")
	cfg.Ports.Redis = rapid.IntRange(1024, 65535).Draw(t, "ports-redis")
	cfg.Ports.NATS = rapid.IntRange(1024, 65535).Draw(t, "ports-nats")
	cfg.Ports.SeaweedFS = rapid.IntRange(1024, 65535).Draw(t, "ports-seaweedfs")
	cfg.Ports.Meilisearch = rapid.IntRange(1024, 65535).Draw(t, "ports-meilisearch")
	cfg.Ports.PgAdmin = rapid.IntRange(1024, 65535).Draw(t, "ports-pgadmin")
	cfg.Ports.Cockpit = rapid.IntRange(1024, 65535).Draw(t, "ports-cockpit")
	cfg.Behavior.WaitForServicesStart = rapid.Bool().Draw(t, "behavior-wait-for-services-start")
	cfg.Behavior.StartupTimeoutSec = rapid.IntRange(1, 600).Draw(t, "behavior-startup-timeout-seconds")
	cfg.TUI.AutoRefreshIntervalSec = rapid.IntRange(1, 120).Draw(t, "tui-auto-refresh-interval-seconds")

	return cfg
}
