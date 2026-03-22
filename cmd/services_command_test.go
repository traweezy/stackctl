package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestServicesPrintsDetailedRuntimeInfo(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		cfg.Services.Postgres.Image = "docker.io/library/postgres:17"
		cfg.Services.Postgres.DataVolume = "stack_postgres_data"
		cfg.Services.Postgres.MaintenanceDatabase = "template1"
		cfg.Connection.RedisPassword = "redispass"
		cfg.Services.Redis.Image = "docker.io/library/redis:7.4"
		cfg.Services.Redis.DataVolume = "stack_redis_data"
		cfg.Services.Redis.AppendOnly = true
		cfg.Services.Redis.SavePolicy = "900 1 300 10"
		cfg.Services.Redis.MaxMemoryPolicy = "allkeys-lru"
		cfg.Connection.PgAdminEmail = "pgadmin@example.com"
		cfg.Connection.PgAdminPassword = "pgsecret"
		cfg.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:9"
		cfg.Services.PgAdmin.DataVolume = "stack_pgadmin_data"
		cfg.Services.PgAdmin.ServerMode = true
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.PgAdmin = 18081
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"postgres123456","Names":["local-postgres"],"Image":"postgres:16","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":15432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"redis123456","Names":["local-redis"],"Image":"redis:7","Status":"Exited (0)","State":"exited","Ports":[{"host_port":16379,"container_port":6379,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"pgadmin123456","Names":["local-pgadmin"],"Image":"dpage/pgadmin4:latest","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":18081,"container_port":80,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
	})

	stdout, _, err := executeRoot(t, "services")
	if err != nil {
		t.Fatalf("services returned error: %v", err)
	}

	for _, fragment := range []string{
		"🗄️ Postgres",
		"Status: running",
		"Container: local-postgres",
		"Image: docker.io/library/postgres:17",
		"Data volume: stack_postgres_data",
		"Host: devbox",
		"Port: 15432 -> 5432",
		"Database: stackdb",
		"Maintenance DB: template1",
		"Username: stackuser",
		"Password: stackpass",
		"DSN: postgres://stackuser:stackpass@devbox:15432/stackdb",
		"⚡ Redis",
		"Container: local-redis",
		"Image: docker.io/library/redis:7.4",
		"Data volume: stack_redis_data",
		"Port: 16379 -> 6379",
		"Password: redispass",
		"Appendonly: enabled",
		"Save policy: 900 1 300 10",
		"Maxmemory policy: allkeys-lru",
		"DSN: redis://:redispass@devbox:16379",
		"🌐 pgAdmin",
		"Image: docker.io/dpage/pgadmin4:9",
		"Data volume: stack_pgadmin_data",
		"Email: pgadmin@example.com",
		"Password: pgsecret",
		"Server mode: enabled",
		"URL: http://devbox:18081",
		"🖥️ Cockpit",
		"URL: https://devbox:19090",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("services output missing %q:\n%s", fragment, stdout)
		}
	}

	if !strings.Contains(stdout, "⚡ Redis\n  Status: stopped") {
		t.Fatalf("services should report stopped containers clearly: %s", stdout)
	}
}

func TestServicesHandlesMissingContainersCleanly(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: `[]`}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
		}
	})

	stdout, _, err := executeRoot(t, "services")
	if err != nil {
		t.Fatalf("services returned error: %v", err)
	}

	if strings.Count(stdout, "Status: missing") < 3 {
		t.Fatalf("expected missing services to be reported clearly: %s", stdout)
	}
}

func TestServicesJSONPrintsStructuredRuntimeInfo(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		cfg.Services.Postgres.Image = "docker.io/library/postgres:17"
		cfg.Services.Postgres.DataVolume = "stack_postgres_data"
		cfg.Services.Postgres.MaintenanceDatabase = "template1"
		cfg.Connection.RedisPassword = "redispass"
		cfg.Services.Redis.Image = "docker.io/library/redis:7.4"
		cfg.Services.Redis.DataVolume = "stack_redis_data"
		cfg.Services.Redis.AppendOnly = true
		cfg.Services.Redis.SavePolicy = "900 1 300 10"
		cfg.Services.Redis.MaxMemoryPolicy = "allkeys-lru"
		cfg.Connection.PgAdminEmail = "pgadmin@example.com"
		cfg.Connection.PgAdminPassword = "pgsecret"
		cfg.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:9"
		cfg.Services.PgAdmin.DataVolume = "stack_pgadmin_data"
		cfg.Services.PgAdmin.ServerMode = true
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.PgAdmin = 18081
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"postgres123456","Names":["local-postgres"],"Image":"postgres:16","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":15432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"redis123456","Names":["local-redis"],"Image":"redis:7","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":16379,"container_port":6379,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"pgadmin123456","Names":["local-pgadmin"],"Image":"dpage/pgadmin4:latest","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":18081,"container_port":80,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
	})

	stdout, _, err := executeRoot(t, "services", "--json")
	if err != nil {
		t.Fatalf("services --json returned error: %v", err)
	}

	var services []runtimeService
	if err := json.Unmarshal([]byte(stdout), &services); err != nil {
		t.Fatalf("parse services json: %v\n%s", err, stdout)
	}
	if len(services) != 4 {
		t.Fatalf("expected 4 runtime services, got %d", len(services))
	}

	if services[0].Name != "postgres" || services[0].DSN != "postgres://stackuser:stackpass@devbox:15432/stackdb" {
		t.Fatalf("unexpected postgres service: %+v", services[0])
	}
	if services[0].Image != "docker.io/library/postgres:17" || services[0].DataVolume != "stack_postgres_data" || services[0].MaintenanceDB != "template1" {
		t.Fatalf("unexpected postgres config: %+v", services[0])
	}
	if services[1].Name != "redis" || services[1].DSN != "redis://:redispass@devbox:16379" {
		t.Fatalf("unexpected redis service: %+v", services[1])
	}
	if services[1].Image != "docker.io/library/redis:7.4" || services[1].DataVolume != "stack_redis_data" || services[1].AppendOnly == nil || !*services[1].AppendOnly || services[1].SavePolicy != "900 1 300 10" || services[1].MaxMemoryPolicy != "allkeys-lru" {
		t.Fatalf("unexpected redis config: %+v", services[1])
	}
	if services[2].Name != "pgadmin" || services[2].Email != "pgadmin@example.com" || services[2].URL != "http://devbox:18081" {
		t.Fatalf("unexpected pgadmin service: %+v", services[2])
	}
	if services[2].Image != "docker.io/dpage/pgadmin4:9" || services[2].DataVolume != "stack_pgadmin_data" || services[2].ServerMode != "enabled" {
		t.Fatalf("unexpected pgadmin config: %+v", services[2])
	}
	if services[3].Name != "cockpit" || services[3].URL != "https://devbox:19090" || services[3].Status != "running" {
		t.Fatalf("unexpected cockpit service: %+v", services[3])
	}
}

func TestServicesCopyUsesClipboardForKnownTarget(t *testing.T) {
	var copied string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.PostgresDatabase = "stackdb"
		cfg.Connection.PostgresUsername = "stackuser"
		cfg.Connection.PostgresPassword = "stackpass"
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.copyToClipboard = func(_ context.Context, _ system.Runner, value string) error {
			copied = value
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "services", "--copy", "postgres")
	if err != nil {
		t.Fatalf("services --copy returned error: %v", err)
	}
	if copied != "postgres://stackuser:stackpass@devbox:5432/stackdb" {
		t.Fatalf("unexpected copied value: %q", copied)
	}
	if !strings.Contains(stdout, "copied postgres DSN to clipboard") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestServicesCopyRejectsInvalidTarget(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	_, _, err := executeRoot(t, "services", "--copy", "not-a-target")
	if err == nil || !strings.Contains(err.Error(), "invalid copy target") {
		t.Fatalf("expected invalid copy target error, got %v", err)
	}
}
