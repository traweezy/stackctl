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
		cfg.Connection.NATSToken = "natssecret"
		cfg.Services.NATS.Image = "docker.io/library/nats:2.12.5"
		cfg.Connection.PgAdminEmail = "pgadmin@example.com"
		cfg.Connection.PgAdminPassword = "pgsecret"
		cfg.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:9"
		cfg.Services.PgAdmin.DataVolume = "stack_pgadmin_data"
		cfg.Services.PgAdmin.ServerMode = true
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.NATS = 14222
		cfg.Ports.PgAdmin = 18081
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"postgres123456","Names":["local-postgres"],"Image":"postgres:16","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":15432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"redis123456","Names":["local-redis"],"Image":"redis:7","Status":"Exited (0)","State":"exited","Ports":[{"host_port":16379,"container_port":6379,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"nats123456","Names":["local-nats"],"Image":"nats:2.12.5","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":14222,"container_port":4222,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"pgadmin123456","Names":["local-pgadmin"],"Image":"dpage/pgadmin4:latest","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":18081,"container_port":80,"protocol":"tcp"}],"CreatedAt":"now"}]`,
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
		"Max connections: 100",
		"Shared buffers: 128MB",
		"Log min duration: disabled",
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
		"📡 NATS",
		"Container: local-nats",
		"Image: docker.io/library/nats:2.12.5",
		"Port: 14222 -> 4222",
		"Token: natssecret",
		"DSN: nats://natssecret@devbox:14222",
		"🌐 pgAdmin",
		"Image: docker.io/dpage/pgadmin4:9",
		"Data volume: stack_pgadmin_data",
		"Email: pgadmin@example.com",
		"Password: pgsecret",
		"Server mode: enabled",
		"Bootstrap server: Local Postgres",
		"Bootstrap group: Local",
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

	if strings.Count(stdout, "Status: missing") < 4 {
		t.Fatalf("expected missing services to be reported clearly: %s", stdout)
	}
	for _, fragment := range []string{
		"Port: 5432 -> 5432",
		"Port: 6379 -> 6379",
		"Port: 4222 -> 4222",
		"Port: 8081 -> 80",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("expected missing services to keep configured ports %q:\n%s", fragment, stdout)
		}
	}
}

func TestServicesMarksCockpitNeedsAttentionWhenSocketPortIsNotListening(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
		d.portListening = func(port int) bool {
			return port != cfg.Ports.Cockpit
		}
		d.portInUse = func(int) (bool, error) { return false, nil }
	})

	stdout, _, err := executeRoot(t, "services", "--json")
	if err != nil {
		t.Fatalf("services --json returned error: %v", err)
	}

	var services []runtimeService
	if err := json.Unmarshal([]byte(stdout), &services); err != nil {
		t.Fatalf("parse services json: %v\n%s", err, stdout)
	}
	if services[4].Name != "cockpit" || services[4].Status != "needs attention" || services[4].PortListening {
		t.Fatalf("unexpected cockpit runtime service: %+v", services[4])
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
		cfg.Connection.NATSToken = "natssecret"
		cfg.Services.NATS.Image = "docker.io/library/nats:2.12.5"
		cfg.Connection.PgAdminEmail = "pgadmin@example.com"
		cfg.Connection.PgAdminPassword = "pgsecret"
		cfg.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:9"
		cfg.Services.PgAdmin.DataVolume = "stack_pgadmin_data"
		cfg.Services.PgAdmin.ServerMode = true
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.NATS = 14222
		cfg.Ports.PgAdmin = 18081
		cfg.Ports.Cockpit = 19090
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"postgres123456","Names":["local-postgres"],"Image":"postgres:16","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":15432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"redis123456","Names":["local-redis"],"Image":"redis:7","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":16379,"container_port":6379,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"nats123456","Names":["local-nats"],"Image":"nats:2.12.5","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":14222,"container_port":4222,"protocol":"tcp"}],"CreatedAt":"now"},{"Id":"pgadmin123456","Names":["local-pgadmin"],"Image":"dpage/pgadmin4:latest","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":18081,"container_port":80,"protocol":"tcp"}],"CreatedAt":"now"}]`,
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
	if len(services) != 5 {
		t.Fatalf("expected 5 runtime services, got %d", len(services))
	}

	if services[0].Name != "postgres" || services[0].DSN != "postgres://stackuser:stackpass@devbox:15432/stackdb" {
		t.Fatalf("unexpected postgres service: %+v", services[0])
	}
	if services[0].Image != "docker.io/library/postgres:17" || services[0].DataVolume != "stack_postgres_data" || services[0].MaintenanceDB != "template1" || services[0].MaxConnections != 100 || services[0].SharedBuffers != "128MB" || services[0].LogMinDurationMS != -1 {
		t.Fatalf("unexpected postgres config: %+v", services[0])
	}
	if services[1].Name != "redis" || services[1].DSN != "redis://:redispass@devbox:16379" {
		t.Fatalf("unexpected redis service: %+v", services[1])
	}
	if services[1].Image != "docker.io/library/redis:7.4" || services[1].DataVolume != "stack_redis_data" || services[1].AppendOnly == nil || !*services[1].AppendOnly || services[1].SavePolicy != "900 1 300 10" || services[1].MaxMemoryPolicy != "allkeys-lru" {
		t.Fatalf("unexpected redis config: %+v", services[1])
	}
	if services[2].Name != "nats" || services[2].DSN != "nats://natssecret@devbox:14222" {
		t.Fatalf("unexpected nats service: %+v", services[2])
	}
	if services[2].Image != "docker.io/library/nats:2.12.5" || services[2].ExternalPort != 14222 || services[2].InternalPort != 4222 {
		t.Fatalf("unexpected nats config: %+v", services[2])
	}
	if services[3].Name != "pgadmin" || services[3].Email != "pgadmin@example.com" || services[3].URL != "http://devbox:18081" {
		t.Fatalf("unexpected pgadmin service: %+v", services[3])
	}
	if services[3].Image != "docker.io/dpage/pgadmin4:9" || services[3].DataVolume != "stack_pgadmin_data" || services[3].ServerMode != "enabled" || services[3].BootstrapServer != "Local Postgres" || services[3].BootstrapGroup != "Local" {
		t.Fatalf("unexpected pgadmin config: %+v", services[3])
	}
	if services[4].Name != "cockpit" || services[4].URL != "https://devbox:19090" || services[4].Status != "running" {
		t.Fatalf("unexpected cockpit service: %+v", services[4])
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

func TestServicesCopyUsesClipboardForNATSTarget(t *testing.T) {
	var copied string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.NATSToken = "natssecret"
		cfg.Ports.NATS = 14222
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.copyToClipboard = func(_ context.Context, _ system.Runner, value string) error {
			copied = value
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "services", "--copy", "nats")
	if err != nil {
		t.Fatalf("services --copy nats returned error: %v", err)
	}
	if copied != "nats://natssecret@devbox:14222" {
		t.Fatalf("unexpected copied value: %q", copied)
	}
	if !strings.Contains(stdout, "copied NATS DSN to clipboard") {
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

func TestServicesIncludeSeaweedFSDetailsWhenEnabled(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Services.SeaweedFS.Image = "docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9"
		cfg.Services.SeaweedFS.DataVolume = "stack_seaweedfs_data"
		cfg.Services.SeaweedFS.VolumeSizeLimitMB = 2048
		cfg.Connection.SeaweedFSAccessKey = "seaweed-access"
		cfg.Connection.SeaweedFSSecretKey = "seaweed-secret"
		cfg.Ports.SeaweedFS = 18333
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"seaweed123456","Names":["local-seaweedfs"],"Image":"chrislusf/seaweedfs:4.17","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":18333,"container_port":8333,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
		}
	})

	stdout, _, err := executeRoot(t, "services")
	if err != nil {
		t.Fatalf("services returned error: %v", err)
	}

	for _, fragment := range []string{
		"🪣 SeaweedFS",
		"Container: local-seaweedfs",
		"Image: docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9",
		"Data volume: stack_seaweedfs_data",
		"Port: 18333 -> 8333",
		"Endpoint: http://devbox:18333",
		"Access key: seaweed-access",
		"Secret key: seaweed-secret",
		"Volume size limit: 2048 MB",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("services output missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestServicesJSONIncludesSeaweedFSWhenEnabled(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Services.SeaweedFS.DataVolume = "stack_seaweedfs_data"
		cfg.Services.SeaweedFS.VolumeSizeLimitMB = 2048
		cfg.Connection.SeaweedFSAccessKey = "seaweed-access"
		cfg.Connection.SeaweedFSSecretKey = "seaweed-secret"
		cfg.Ports.SeaweedFS = 18333
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"seaweed123456","Names":["local-seaweedfs"],"Image":"chrislusf/seaweedfs:4.17","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":18333,"container_port":8333,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
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
	if len(services) != 6 {
		t.Fatalf("expected 6 runtime services, got %d", len(services))
	}

	seaweed := services[3]
	if seaweed.Name != "seaweedfs" || seaweed.Endpoint != "http://devbox:18333" {
		t.Fatalf("unexpected seaweedfs runtime service: %+v", seaweed)
	}
	if seaweed.AccessKey != "seaweed-access" || seaweed.VolumeSizeLimitMB != 2048 {
		t.Fatalf("unexpected seaweedfs access config: %+v", seaweed)
	}
	if seaweed.DataVolume != "stack_seaweedfs_data" || seaweed.ExternalPort != 18333 || seaweed.InternalPort != 8333 {
		t.Fatalf("unexpected seaweedfs port config: %+v", seaweed)
	}
}

func TestServicesIncludeMeilisearchDetailsWhenEnabled(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeMeilisearch = true
		cfg.Services.Meilisearch.Image = "docker.io/getmeili/meilisearch:v1.40.0"
		cfg.Services.Meilisearch.DataVolume = "stack_meilisearch_data"
		cfg.Connection.MeilisearchMasterKey = "meili-master-key-123"
		cfg.Ports.Meilisearch = 17700
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"meili123456","Names":["local-meilisearch"],"Image":"getmeili/meilisearch:v1.40.0","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":17700,"container_port":7700,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
		}
	})

	stdout, _, err := executeRoot(t, "services")
	if err != nil {
		t.Fatalf("services returned error: %v", err)
	}

	for _, fragment := range []string{
		"🔎 Meilisearch",
		"Container: local-meilisearch",
		"Image: docker.io/getmeili/meilisearch:v1.40.0",
		"Data volume: stack_meilisearch_data",
		"Port: 17700 -> 7700",
		"URL: http://devbox:17700",
		"Master key: meili-master-key-123",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("services output missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestServicesJSONIncludesMeilisearchWhenEnabled(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeMeilisearch = true
		cfg.Services.Meilisearch.DataVolume = "stack_meilisearch_data"
		cfg.Connection.MeilisearchMasterKey = "meili-master-key-123"
		cfg.Ports.Meilisearch = 17700
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: `[{"Id":"meili123456","Names":["local-meilisearch"],"Image":"getmeili/meilisearch:v1.40.0","Status":"Up 5 minutes","State":"running","Ports":[{"host_port":17700,"container_port":7700,"protocol":"tcp"}],"CreatedAt":"now"}]`,
			}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
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

	var meili runtimeService
	found := false
	for _, service := range services {
		if service.Name == "meilisearch" {
			meili = service
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected meilisearch runtime service in %+v", services)
	}
	if meili.URL != "http://devbox:17700" || meili.DataVolume != "stack_meilisearch_data" {
		t.Fatalf("unexpected meilisearch runtime service: %+v", meili)
	}
	if meili.ExternalPort != 17700 || meili.InternalPort != 7700 {
		t.Fatalf("unexpected meilisearch port config: %+v", meili)
	}
	if meili.MasterKey != "" {
		t.Fatalf("expected meilisearch master key to stay out of json output, got %+v", meili)
	}
}

func TestServicesJSONKeepsDefaultInternalPortsWhenContainersAreMissing(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Ports.Postgres = 15432
		cfg.Ports.Redis = 16379
		cfg.Ports.NATS = 14222
		cfg.Ports.SeaweedFS = 18333
		cfg.Ports.PgAdmin = 18081
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: `[]`}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{State: "not installed"}
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

	portsByService := map[string]int{}
	for _, service := range services {
		portsByService[service.Name] = service.InternalPort
	}

	for serviceName, want := range map[string]int{
		"postgres":  5432,
		"redis":     6379,
		"nats":      4222,
		"seaweedfs": 8333,
		"pgadmin":   80,
	} {
		if got := portsByService[serviceName]; got != want {
			t.Fatalf("expected %s internal port %d, got %d", serviceName, want, got)
		}
	}
}

func TestServicesCopyUsesClipboardForSeaweedFSTargets(t *testing.T) {
	var copied []string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Connection.SeaweedFSAccessKey = "seaweed-access"
		cfg.Connection.SeaweedFSSecretKey = "seaweed-secret"
		cfg.Ports.SeaweedFS = 18333
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.copyToClipboard = func(_ context.Context, _ system.Runner, value string) error {
			copied = append(copied, value)
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "services", "--copy", "seaweedfs")
	if err != nil {
		t.Fatalf("services --copy seaweedfs returned error: %v", err)
	}
	if len(copied) != 1 || copied[0] != "http://devbox:18333" {
		t.Fatalf("unexpected copied endpoint: %v", copied)
	}
	if !strings.Contains(stdout, "copied SeaweedFS endpoint to clipboard") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}

	stdout, _, err = executeRoot(t, "services", "--copy", "seaweedfs-secret-key")
	if err != nil {
		t.Fatalf("services --copy seaweedfs-secret-key returned error: %v", err)
	}
	if len(copied) != 2 || copied[1] != "seaweed-secret" {
		t.Fatalf("unexpected copied secret key: %v", copied)
	}
	if !strings.Contains(stdout, "copied SeaweedFS secret key to clipboard") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestServicesCopyUsesClipboardForMeilisearchAPIKey(t *testing.T) {
	var copied string

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeMeilisearch = true
		cfg.Connection.MeilisearchMasterKey = "meili-master-key-123"
		cfg.ApplyDerivedFields()

		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.copyToClipboard = func(_ context.Context, _ system.Runner, value string) error {
			copied = value
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "services", "--copy", "meilisearch-api-key")
	if err != nil {
		t.Fatalf("services --copy meilisearch-api-key returned error: %v", err)
	}
	if copied != "meili-master-key-123" {
		t.Fatalf("unexpected copied value: %q", copied)
	}
	if !strings.Contains(stdout, "copied Meilisearch API key to clipboard") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}
