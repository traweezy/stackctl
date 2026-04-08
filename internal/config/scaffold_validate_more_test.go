package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedScaffoldRenderHelpersAndFileModes(t *testing.T) {
	cfg := Default()
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.Services.Postgres.Image = "docker.io/library/postgres:17"
	cfg.Services.Postgres.DataVolume = "stack_postgres_data"
	cfg.Services.Postgres.MaintenanceDatabase = "template1"
	cfg.Services.Postgres.MaxConnections = 250
	cfg.Services.Postgres.SharedBuffers = "256MB"
	cfg.Services.Postgres.LogMinDurationStatementMS = 500
	cfg.Connection.PostgresDatabase = "stackdb"
	cfg.Connection.PostgresUsername = "stackuser"
	cfg.Connection.PostgresPassword = "stackpass"
	cfg.Services.Redis.Image = "docker.io/library/redis:8.6"
	cfg.Services.Redis.DataVolume = "stack_redis_data"
	cfg.Services.Redis.AppendOnly = true
	cfg.Services.Redis.SavePolicy = "900 1 300 10"
	cfg.Services.Redis.MaxMemoryPolicy = "allkeys-lrm"
	cfg.Connection.RedisPassword = "redispass"
	cfg.Connection.RedisACLUsername = "stack-user"
	cfg.Connection.RedisACLPassword = "aclpass"
	cfg.Services.NATS.Image = "docker.io/library/nats:2.12.5"
	cfg.Connection.NATSToken = "natssecret"
	cfg.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:9"
	cfg.Services.PgAdmin.DataVolume = "stack_pgadmin_data"
	cfg.Services.PgAdmin.BootstrapPostgresServer = true
	cfg.Services.PgAdmin.BootstrapServerName = "Stack Postgres"
	cfg.Services.PgAdmin.BootstrapServerGroup = "Stack"
	cfg.Connection.PgAdminEmail = "pgadmin@example.com"
	cfg.Connection.PgAdminPassword = "pgsecret"

	compose, err := renderManagedCompose(cfg)
	if err != nil {
		t.Fatalf("renderManagedCompose returned error: %v", err)
	}
	composeText := string(compose)
	for _, fragment := range []string{
		`image: "docker.io/library/postgres:17"`,
		`image: "docker.io/library/redis:8.6"`,
		`image: "docker.io/library/nats:2.12.5"`,
		`image: "docker.io/dpage/pgadmin4:9"`,
		`allkeys-lrm`,
		`seaweedfs:`,
		`meilisearch:`,
	} {
		if !strings.Contains(composeText, fragment) {
			t.Fatalf("expected managed compose output to contain %q:\n%s", fragment, composeText)
		}
	}

	natsConfig, err := renderManagedNATSConfig(cfg)
	if err != nil {
		t.Fatalf("renderManagedNATSConfig returned error: %v", err)
	}
	if got := string(natsConfig); !strings.Contains(got, "authorization") || !strings.Contains(got, "natssecret") {
		t.Fatalf("unexpected nats config:\n%s", got)
	}

	redisACL, err := renderManagedRedisACL(cfg)
	if err != nil {
		t.Fatalf("renderManagedRedisACL returned error: %v", err)
	}
	if got := string(redisACL); !strings.Contains(got, "stack-user") || !strings.Contains(got, "aclpass") {
		t.Fatalf("unexpected redis ACL:\n%s", got)
	}

	pgAdminServers, err := renderManagedPgAdminServers(cfg)
	if err != nil {
		t.Fatalf("renderManagedPgAdminServers returned error: %v", err)
	}
	if got := string(pgAdminServers); !strings.Contains(got, "Stack Postgres") || !strings.Contains(got, "template1") {
		t.Fatalf("unexpected pgAdmin bootstrap servers:\n%s", got)
	}

	pgPass, err := renderManagedPGPass(cfg)
	if err != nil {
		t.Fatalf("renderManagedPGPass returned error: %v", err)
	}
	if got := string(pgPass); !strings.Contains(got, "stackuser") || !strings.Contains(got, "stackpass") {
		t.Fatalf("unexpected pgpass content:\n%s", got)
	}

	if mode := scaffoldFileMode(filepath.Join(t.TempDir(), DefaultRedisACLName)); mode != 0o644 {
		t.Fatalf("unexpected redis ACL mode: %v", mode)
	}
	if mode := scaffoldFileMode(filepath.Join(t.TempDir(), "pgpass")); mode != 0o600 {
		t.Fatalf("unexpected non-ACL scaffold mode: %v", mode)
	}
}

func TestScaffoldFileHelpersCoverDirectoryOverwriteAndReadBranches(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "compose.yaml")

	missing, err := scaffoldFileMissing(target)
	if err != nil || !missing {
		t.Fatalf("expected missing scaffold file, got missing=%v err=%v", missing, err)
	}
	needsWrite, err := scaffoldFileNeedsWrite(target, []byte("services: {}\n"))
	if err != nil || !needsWrite {
		t.Fatalf("expected missing scaffold target to need a write, got needsWrite=%v err=%v", needsWrite, err)
	}

	if err := os.WriteFile(target, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write scaffold target: %v", err)
	}
	needsWrite, err = scaffoldFileNeedsWrite(target, []byte("services: {}\n"))
	if err != nil || needsWrite {
		t.Fatalf("expected matching scaffold target to skip writing, got needsWrite=%v err=%v", needsWrite, err)
	}
	needsWrite, err = scaffoldFileNeedsWrite(target, []byte("services:\n  postgres: {}\n"))
	if err != nil || !needsWrite {
		t.Fatalf("expected changed scaffold target to need writing, got needsWrite=%v err=%v", needsWrite, err)
	}

	wrote, err := writeScaffoldFile(target, []byte("unchanged"), false)
	if err != nil || wrote {
		t.Fatalf("expected force=false to skip overwriting, got wrote=%v err=%v", wrote, err)
	}
	wrote, err = writeScaffoldFile(target, []byte("rewritten"), true)
	if err != nil || !wrote {
		t.Fatalf("expected force=true overwrite, got wrote=%v err=%v", wrote, err)
	}

	directoryPath := filepath.Join(dir, "config-dir")
	if err := os.MkdirAll(directoryPath, 0o755); err != nil {
		t.Fatalf("mkdir scaffold dir: %v", err)
	}
	if _, err := scaffoldFileMissing(directoryPath); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory error from scaffoldFileMissing, got %v", err)
	}
	if _, err := writeScaffoldFile(directoryPath, []byte("nope"), true); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory error from writeScaffoldFile, got %v", err)
	}
}

func TestManagedStackNeedsScaffoldAndValidateCoverRemainingBranches(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludeNATS = true
	cfg.Setup.IncludeCockpit = true
	cfg.Stack.ComposeFile = DefaultComposeFileName

	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}
	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil || needsScaffold {
		t.Fatalf("expected freshly scaffolded managed stack to be current, got needsScaffold=%v err=%v", needsScaffold, err)
	}

	cfg.Connection.NATSToken = "rotated-token"
	needsScaffold, err = ManagedStackNeedsScaffold(cfg)
	if err != nil || !needsScaffold {
		t.Fatalf("expected NATS drift to require scaffolding, got needsScaffold=%v err=%v", needsScaffold, err)
	}

	fileBackedDir := filepath.Join(t.TempDir(), "stack-file")
	if err := os.WriteFile(fileBackedDir, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write stack dir file: %v", err)
	}
	cfg = Default()
	cfg.Stack.Dir = fileBackedDir
	cfg.Ports.Cockpit = 70000
	cfg.Ports.NATS = 70000
	cfg.Services.NATSContainer = ""
	cfg.Services.NATS.Image = ""
	cfg.Connection.NATSToken = ""

	issues := Validate(cfg)
	fields := make(map[string]bool, len(issues))
	for _, issue := range issues {
		fields[issue.Field] = true
	}
	for _, field := range []string{
		"stack.dir",
		"services.nats_container",
		"services.nats.image",
		"connection.nats_token",
		"ports.nats",
		"ports.cockpit",
	} {
		if !fields[field] {
			t.Fatalf("expected validation issue for %s, got %v", field, issues)
		}
	}

	cfg = Default()
	cfg.Stack.Dir = filepath.Join(t.TempDir(), "validate-managed")
	if err := os.MkdirAll(cfg.Stack.Dir, 0o755); err != nil {
		t.Fatalf("mkdir stack dir: %v", err)
	}
	if err := os.Mkdir(ComposePath(cfg), 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	fields = make(map[string]bool)
	for _, issue := range Validate(cfg) {
		fields[issue.Field] = true
	}
	if !fields["stack.compose_file"] {
		t.Fatalf("expected compose-directory validation issue, got %v", fields)
	}
}
