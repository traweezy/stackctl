package cmd

import (
	"context"
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
		"Host: devbox",
		"Port: 15432 -> 5432",
		"Database: stackdb",
		"Username: stackuser",
		"Password: stackpass",
		"DSN: postgres://stackuser:stackpass@devbox:15432/stackdb",
		"⚡ Redis",
		"Container: local-redis",
		"Port: 16379 -> 6379",
		"DSN: redis://devbox:16379",
		"🌐 pgAdmin",
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
