package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedStackNeedsScaffoldTracksComposePresence(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected managed stack to need scaffolding before compose exists")
	}

	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	needsScaffold, err = ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if needsScaffold {
		t.Fatal("expected managed stack to be fully scaffolded")
	}
}

func TestManagedStackNeedsScaffoldIgnoresExternalStack(t *testing.T) {
	cfg := Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if needsScaffold {
		t.Fatal("external stack should not request managed scaffolding")
	}
}

func TestManagedStackNeedsScaffoldDetectsComposeDrift(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	cfg.Ports.Postgres = 25432

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected managed stack to need scaffolding when compose content drifts from config")
	}
}

func TestManagedStackNeedsScaffoldDetectsNATSConfigDrift(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	cfg.Connection.NATSToken = "updated-token"

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected managed stack to need scaffolding when nats config drifts from config")
	}
}

func TestManagedStackNeedsScaffoldErrorsWhenComposePathIsDirectory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := os.MkdirAll(ComposePath(cfg), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	_, err := ManagedStackNeedsScaffold(cfg)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScaffoldManagedStackCreatesComposeFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Services.Postgres.Image = "docker.io/library/postgres:17"
	cfg.Services.Postgres.DataVolume = "stack_postgres_data"
	cfg.Services.Postgres.MaintenanceDatabase = "template1"
	cfg.Connection.PostgresUsername = "stackuser"
	cfg.Connection.PostgresPassword = "stackpass"
	cfg.Connection.PostgresDatabase = "stackdb"
	cfg.Services.Redis.Image = "docker.io/library/redis:7.4"
	cfg.Services.Redis.DataVolume = "stack_redis_data"
	cfg.Services.Redis.AppendOnly = true
	cfg.Services.Redis.SavePolicy = "900 1 300 10"
	cfg.Services.Redis.MaxMemoryPolicy = "allkeys-lru"
	cfg.Connection.RedisPassword = "redispass"
	cfg.Services.NATS.Image = "docker.io/library/nats:2.12.5"
	cfg.Connection.NATSToken = "natssecret"
	cfg.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:9"
	cfg.Services.PgAdmin.DataVolume = "stack_pgadmin_data"
	cfg.Services.PgAdmin.ServerMode = true
	cfg.Connection.PgAdminEmail = "pgadmin@example.com"
	cfg.Connection.PgAdminPassword = "pgsecret"
	cfg.Ports.Postgres = 15432
	cfg.Ports.NATS = 14222

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}
	if !result.CreatedDir || !result.WroteCompose || !result.WroteNATSConfig {
		t.Fatalf("unexpected scaffold result: %+v", result)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if !strings.Contains(string(data), "local-postgres") {
		t.Fatalf("unexpected scaffolded compose file: %s", string(data))
	}
	if !strings.Contains(string(data), "image: \"docker.io/library/postgres:17\"") {
		t.Fatalf("expected rendered postgres image, got: %s", string(data))
	}
	if !strings.Contains(string(data), "stack_postgres_data:/var/lib/postgresql/data") {
		t.Fatalf("expected rendered postgres data volume, got: %s", string(data))
	}
	if !strings.Contains(string(data), "POSTGRES_USER: \"stackuser\"") {
		t.Fatalf("expected rendered postgres username, got: %s", string(data))
	}
	if !strings.Contains(string(data), "POSTGRES_PASSWORD: \"stackpass\"") {
		t.Fatalf("expected rendered postgres password, got: %s", string(data))
	}
	if !strings.Contains(string(data), "POSTGRES_DB: \"stackdb\"") {
		t.Fatalf("expected rendered postgres database, got: %s", string(data))
	}
	if !strings.Contains(string(data), "\"15432:5432\"") {
		t.Fatalf("expected rendered postgres port mapping, got: %s", string(data))
	}
	if !strings.Contains(string(data), "image: \"docker.io/library/redis:7.4\"") {
		t.Fatalf("expected rendered redis image, got: %s", string(data))
	}
	if !strings.Contains(string(data), "stack_redis_data:/data") {
		t.Fatalf("expected rendered redis data volume, got: %s", string(data))
	}
	if !strings.Contains(string(data), "redis-server") || !strings.Contains(string(data), "--requirepass") || !strings.Contains(string(data), "\"redispass\"") {
		t.Fatalf("expected rendered redis auth command, got: %s", string(data))
	}
	if !strings.Contains(string(data), "--appendonly") || !strings.Contains(string(data), "\"yes\"") {
		t.Fatalf("expected rendered redis appendonly command, got: %s", string(data))
	}
	if !strings.Contains(string(data), "\"900 1 300 10\"") || !strings.Contains(string(data), "\"allkeys-lru\"") {
		t.Fatalf("expected rendered redis tuning, got: %s", string(data))
	}
	if !strings.Contains(string(data), "image: \"docker.io/library/nats:2.12.5\"") {
		t.Fatalf("expected rendered nats image, got: %s", string(data))
	}
	if !strings.Contains(string(data), "\"14222:4222\"") {
		t.Fatalf("expected rendered nats port mapping, got: %s", string(data))
	}
	if !strings.Contains(string(data), "./nats.conf:/etc/nats/nats.conf:ro") {
		t.Fatalf("expected rendered nats config mount, got: %s", string(data))
	}
	if !strings.Contains(string(data), "image: \"docker.io/dpage/pgadmin4:9\"") {
		t.Fatalf("expected rendered pgadmin image, got: %s", string(data))
	}
	if !strings.Contains(string(data), "stack_pgadmin_data:/var/lib/pgadmin") {
		t.Fatalf("expected rendered pgadmin data volume, got: %s", string(data))
	}
	if !strings.Contains(string(data), "PGADMIN_DEFAULT_EMAIL: \"pgadmin@example.com\"") {
		t.Fatalf("expected rendered pgadmin email, got: %s", string(data))
	}
	if !strings.Contains(string(data), "PGADMIN_DEFAULT_PASSWORD: \"pgsecret\"") {
		t.Fatalf("expected rendered pgadmin password, got: %s", string(data))
	}
	if !strings.Contains(string(data), "PGADMIN_CONFIG_SERVER_MODE: \"True\"") {
		t.Fatalf("expected rendered pgadmin server mode, got: %s", string(data))
	}

	natsConfigData, err := os.ReadFile(result.NATSConfigPath)
	if err != nil {
		t.Fatalf("read scaffolded nats config: %v", err)
	}
	if !strings.Contains(string(natsConfigData), "token: \"natssecret\"") {
		t.Fatalf("expected rendered nats token, got: %s", string(natsConfigData))
	}
}

func TestScaffoldManagedStackOmitsPgAdminWhenDisabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludePgAdmin = false

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "pgadmin:") {
		t.Fatalf("expected pgadmin service to be omitted, got: %s", string(data))
	}
	if strings.Contains(string(data), "pgadmin_data") {
		t.Fatalf("expected pgadmin volume to be omitted, got: %s", string(data))
	}
}

func TestScaffoldManagedStackOmitsNATSWhenDisabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludeNATS = false

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "nats:") {
		t.Fatalf("expected nats service to be omitted, got: %s", string(data))
	}
	if _, err := os.Stat(result.NATSConfigPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected nats config file to be absent, got err=%v", err)
	}
}

func TestScaffoldManagedStackAddsSeaweedFSWhenEnabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Services.SeaweedFS.Image = "docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9"
	cfg.Services.SeaweedFS.DataVolume = "stack_seaweedfs_data"
	cfg.Services.SeaweedFS.VolumeSizeLimitMB = 2048
	cfg.Connection.SeaweedFSAccessKey = "seaweed-access"
	cfg.Connection.SeaweedFSSecretKey = "seaweed-secret"
	cfg.Ports.SeaweedFS = 18333

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if !strings.Contains(string(data), "seaweedfs:") {
		t.Fatalf("expected seaweedfs service to be rendered, got: %s", string(data))
	}
	if !strings.Contains(string(data), "image: \"docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9\"") {
		t.Fatalf("expected rendered seaweedfs image, got: %s", string(data))
	}
	if !strings.Contains(string(data), "container_name: \"local-seaweedfs\"") {
		t.Fatalf("expected rendered seaweedfs container, got: %s", string(data))
	}
	if !strings.Contains(string(data), "AWS_ACCESS_KEY_ID: \"seaweed-access\"") {
		t.Fatalf("expected rendered seaweedfs access key, got: %s", string(data))
	}
	if !strings.Contains(string(data), "AWS_SECRET_ACCESS_KEY: \"seaweed-secret\"") {
		t.Fatalf("expected rendered seaweedfs secret key, got: %s", string(data))
	}
	if !strings.Contains(string(data), "- -volume.fileSizeLimitMB=2048") {
		t.Fatalf("expected rendered seaweedfs volume limit, got: %s", string(data))
	}
	if !strings.Contains(string(data), "\"18333:8333\"") {
		t.Fatalf("expected rendered seaweedfs port mapping, got: %s", string(data))
	}
	if !strings.Contains(string(data), "stack_seaweedfs_data:/data") {
		t.Fatalf("expected rendered seaweedfs data volume, got: %s", string(data))
	}
}

func TestScaffoldManagedStackOmitsSeaweedFSWhenDisabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludeSeaweedFS = false

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "seaweedfs:") {
		t.Fatalf("expected seaweedfs service to be omitted, got: %s", string(data))
	}
	if strings.Contains(string(data), "seaweedfs_data") {
		t.Fatalf("expected seaweedfs volume to be omitted, got: %s", string(data))
	}
}

func TestScaffoldManagedStackAddsMeilisearchWhenEnabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludeMeilisearch = true
	cfg.Services.Meilisearch.Image = "docker.io/getmeili/meilisearch:v1.40.0"
	cfg.Services.Meilisearch.DataVolume = "stack_meilisearch_data"
	cfg.Connection.MeilisearchMasterKey = "meili-master-key-123"
	cfg.Ports.Meilisearch = 17700

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "meilisearch:") {
		t.Fatalf("expected meilisearch service to be rendered, got: %s", text)
	}
	if !strings.Contains(text, "image: \"docker.io/getmeili/meilisearch:v1.40.0\"") {
		t.Fatalf("expected rendered meilisearch image, got: %s", text)
	}
	if !strings.Contains(text, "container_name: \"local-meilisearch\"") {
		t.Fatalf("expected rendered meilisearch container, got: %s", text)
	}
	if !strings.Contains(text, "MEILI_HTTP_ADDR: \"0.0.0.0:7700\"") {
		t.Fatalf("expected rendered meilisearch bind address, got: %s", text)
	}
	if !strings.Contains(text, "MEILI_DB_PATH: \"/meili_data\"") {
		t.Fatalf("expected rendered meilisearch db path, got: %s", text)
	}
	if !strings.Contains(text, "MEILI_ENV: \"development\"") {
		t.Fatalf("expected rendered meilisearch env, got: %s", text)
	}
	if !strings.Contains(text, "MEILI_MASTER_KEY: \"meili-master-key-123\"") {
		t.Fatalf("expected rendered meilisearch master key, got: %s", text)
	}
	if strings.Contains(text, "MEILI_NO_ANALYTICS") {
		t.Fatalf("expected meilisearch analytics opt-out to use the CLI flag, got: %s", text)
	}
	if !strings.Contains(text, "- meilisearch") || !strings.Contains(text, "- --no-analytics") {
		t.Fatalf("expected rendered meilisearch command flags, got: %s", text)
	}
	if !strings.Contains(text, "\"17700:7700\"") {
		t.Fatalf("expected rendered meilisearch port mapping, got: %s", text)
	}
	if !strings.Contains(text, "stack_meilisearch_data:/meili_data") {
		t.Fatalf("expected rendered meilisearch data volume, got: %s", text)
	}
}

func TestScaffoldManagedStackOmitsMeilisearchWhenDisabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludeMeilisearch = false

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "meilisearch:") {
		t.Fatalf("expected meilisearch service to be omitted, got: %s", string(data))
	}
	if strings.Contains(string(data), "meilisearch_data") {
		t.Fatalf("expected meilisearch volume to be omitted, got: %s", string(data))
	}
}

func TestScaffoldManagedStackOmitsPostgresWhenDisabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludePostgres = false

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "postgres:") {
		t.Fatalf("expected postgres service to be omitted, got: %s", string(data))
	}
	if strings.Contains(string(data), "postgres_data") {
		t.Fatalf("expected postgres volume to be omitted, got: %s", string(data))
	}
	if strings.Contains(string(data), "depends_on:") {
		t.Fatalf("expected pgadmin to drop postgres dependency when postgres is disabled, got: %s", string(data))
	}
}

func TestScaffoldManagedStackOmitsRedisAuthWhenPasswordIsBlank(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Connection.RedisPassword = ""

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "--requirepass") {
		t.Fatalf("expected redis auth to be omitted, got: %s", string(data))
	}
	if !strings.Contains(string(data), "--appendonly") || !strings.Contains(string(data), "\"no\"") {
		t.Fatalf("expected redis defaults to stay rendered, got: %s", string(data))
	}
}

func TestScaffoldManagedStackTreatsExistingComposeAsAlreadyPresent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("initial scaffold failed: %v", err)
	}

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("repeat scaffold failed: %v", err)
	}
	if !result.AlreadyPresent || result.WroteCompose || result.WroteNATSConfig {
		t.Fatalf("unexpected scaffold result: %+v", result)
	}
}

func TestScaffoldManagedStackOverwritesWhenForced(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("initial scaffold failed: %v", err)
	}
	if err := os.WriteFile(ComposePath(cfg), []byte("custom: true\n"), 0o644); err != nil {
		t.Fatalf("write custom compose file failed: %v", err)
	}

	result, err := ScaffoldManagedStack(cfg, true)
	if err != nil {
		t.Fatalf("forced scaffold failed: %v", err)
	}
	if !result.WroteCompose {
		t.Fatalf("expected forced scaffold to rewrite compose file: %+v", result)
	}

	data, err := os.ReadFile(ComposePath(cfg))
	if err != nil {
		t.Fatalf("read compose file failed: %v", err)
	}
	if strings.Contains(string(data), "custom: true") {
		t.Fatalf("expected embedded template overwrite, got %s", string(data))
	}
}

func TestScaffoldManagedStackRejectsExternalOrInconsistentPaths(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Stack.Managed = false
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "stack.managed") {
		t.Fatalf("unexpected external stack scaffold error: %v", err)
	}

	cfg = Default()
	cfg.Stack.Dir = filepath.Join(t.TempDir(), "other")
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "managed stack dir must be") {
		t.Fatalf("unexpected mismatched managed path error: %v", err)
	}
}

func TestScaffoldManagedStackRejectsFileAtStackPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := os.MkdirAll(filepath.Dir(cfg.Stack.Dir), 0o755); err != nil {
		t.Fatalf("mkdir parent failed: %v", err)
	}
	if err := os.WriteFile(cfg.Stack.Dir, []byte("file"), 0o644); err != nil {
		t.Fatalf("write stack path file failed: %v", err)
	}
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("unexpected stack path error: %v", err)
	}
}

func TestScaffoldManagedStackRejectsDirectoryAtComposePath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := os.MkdirAll(ComposePath(cfg), 0o755); err != nil {
		t.Fatalf("mkdir compose path failed: %v", err)
	}
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("unexpected compose path error: %v", err)
	}
}

func TestValidateRequiresAtLeastOneEnabledStackService(t *testing.T) {
	root := t.TempDir()
	stackDir := filepath.Join(root, "stack")
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	composePath := filepath.Join(stackDir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file failed: %v", err)
	}

	cfg := Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	cfg.Stack.Dir = stackDir
	cfg.Setup.IncludePostgres = false
	cfg.Setup.IncludeRedis = false
	cfg.Setup.IncludeNATS = false
	cfg.Setup.IncludePgAdmin = false

	issues := Validate(cfg)
	found := false
	for _, issue := range issues {
		if issue.Field == "setup" && strings.Contains(issue.Message, "at least one stack service") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected setup validation issue, got %+v", issues)
	}
}

func TestDefaultManagedStackDirReturnsEmptyWhenDataPathFails(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	if got := DefaultManagedStackDir(); got != "" {
		t.Fatalf("expected empty managed stack dir, got %s", got)
	}
}

func TestManagedStacksDirPathAndManagedStackDirUseDefaultName(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataRoot)

	stacksDir, err := ManagedStacksDirPath()
	if err != nil {
		t.Fatalf("ManagedStacksDirPath returned error: %v", err)
	}
	if stacksDir != filepath.Join(dataRoot, "stackctl", "stacks") {
		t.Fatalf("unexpected stacks dir: %s", stacksDir)
	}

	stackDir, err := ManagedStackDir("")
	if err != nil {
		t.Fatalf("ManagedStackDir returned error: %v", err)
	}
	if stackDir != filepath.Join(stacksDir, DefaultStackName) {
		t.Fatalf("unexpected managed stack dir: %s", stackDir)
	}
}

func TestDataDirPathFailsWithoutHomeOrXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	_, err := DataDirPath()
	if err == nil {
		t.Fatal("expected DataDirPath to fail")
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "user home directory") {
		t.Fatalf("unexpected data dir error: %v", err)
	}
}
