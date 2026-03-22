package cmd

import (
	"context"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestPortsPrintsConfiguredMappingsWithoutRuntime(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.PgAdmin = 18081
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.commandExists = func(string) bool { return false }
	})

	stdout, _, err := executeRoot(t, "ports")
	if err != nil {
		t.Fatalf("ports returned error: %v", err)
	}

	for _, fragment := range []string{
		"SERVICE",
		"Postgres",
		"devbox",
		"15432 -> 5432",
		"16379 -> 6379",
		"18081 -> 80",
		"19090 -> 9090",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("ports output missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestPortsUsesRuntimeInternalPortsWhenAvailable(t *testing.T) {
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.PgAdmin = 18081
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: "{\"ID\":\"postgres123\",\"Names\":\"local-postgres\",\"Status\":\"Up\",\"State\":\"running\",\"Publishers\":[{\"PublishedPort\":15432,\"TargetPort\":55432,\"Protocol\":\"tcp\"}]}\n" +
					"{\"ID\":\"redis123\",\"Names\":\"local-redis\",\"Status\":\"Up\",\"State\":\"running\",\"Publishers\":[{\"PublishedPort\":16379,\"TargetPort\":6380,\"Protocol\":\"tcp\"}]}\n" +
					"{\"ID\":\"pgadmin123\",\"Names\":\"local-pgadmin\",\"Status\":\"Up\",\"State\":\"running\",\"Publishers\":[{\"PublishedPort\":18081,\"TargetPort\":8080,\"Protocol\":\"tcp\"}]}\n",
			}, nil
		}
	})

	stdout, _, err := executeRoot(t, "ports")
	if err != nil {
		t.Fatalf("ports returned error: %v", err)
	}

	for _, fragment := range []string{
		"15432 -> 55432",
		"16379 -> 6380",
		"18081 -> 8080",
		"19090 -> 9090",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("ports output missing %q:\n%s", fragment, stdout)
		}
	}
}
