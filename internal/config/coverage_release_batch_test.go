package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/traweezy/stackctl/internal/system"
)

func TestValidateCoversRemainingManagedAndOptionalBranches(t *testing.T) {
	t.Run("blank stack basics and disabled services", func(t *testing.T) {
		cfg := Default()
		cfg.Stack.Name = ""
		cfg.Stack.Dir = ""
		cfg.Connection.Host = ""
		cfg.Setup.IncludePostgres = false
		cfg.Setup.IncludeRedis = false
		cfg.Setup.IncludeNATS = false
		cfg.Setup.IncludeSeaweedFS = false
		cfg.Setup.IncludeMeilisearch = false
		cfg.Setup.IncludePgAdmin = false

		fields := validationFields(Validate(cfg))
		for _, field := range []string{"stack.name", "stack.dir", "connection.host", "setup"} {
			if !fields[field] {
				t.Fatalf("expected validation issue for %s, got %v", field, fields)
			}
		}
	})

	t.Run("invalid managed stack name and mismatched compose path", func(t *testing.T) {
		cfg := Default()
		cfg.Stack.Name = "INVALID!"
		cfg.Stack.Dir = t.TempDir()
		cfg.Stack.ComposeFile = "custom-compose.yaml"

		fields := validationFields(Validate(cfg))
		for _, field := range []string{"stack.name", "stack.dir", "stack.compose_file"} {
			if !fields[field] {
				t.Fatalf("expected validation issue for %s, got %v", field, fields)
			}
		}
	})

	t.Run("redis partial acl, empty nats, blank pgadmin, and port bounds", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())

		cfg := Default()
		if err := os.MkdirAll(cfg.Stack.Dir, 0o755); err != nil {
			t.Fatalf("mkdir stack dir: %v", err)
		}
		if err := os.WriteFile(ComposePath(cfg), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("write compose file: %v", err)
		}

		cfg.Connection.RedisACLUsername = "named-user"
		cfg.Connection.RedisACLPassword = ""
		cfg.Services.NATSContainer = ""
		cfg.Services.NATS.Image = ""
		cfg.Connection.NATSToken = ""
		cfg.Services.PgAdminContainer = ""
		cfg.Services.PgAdmin.Image = ""
		cfg.Services.PgAdmin.DataVolume = ""
		cfg.Connection.PgAdminEmail = ""
		cfg.Connection.PgAdminPassword = ""
		cfg.Ports.Redis = 70000
		cfg.Ports.NATS = 70000
		cfg.Ports.PgAdmin = 70000
		cfg.Ports.Cockpit = 70000
		cfg.TUI.AutoRefreshIntervalSec = 0

		fields := validationFields(Validate(cfg))
		for _, field := range []string{
			"connection.redis_acl_username",
			"connection.redis_acl_password",
			"services.nats_container",
			"services.nats.image",
			"connection.nats_token",
			"services.pgadmin_container",
			"services.pgadmin.image",
			"services.pgadmin.data_volume",
			"connection.pgadmin_email",
			"connection.pgadmin_password",
			"ports.redis",
			"ports.nats",
			"ports.pgadmin",
			"ports.cockpit",
			"tui.auto_refresh_interval_seconds",
		} {
			if !fields[field] {
				t.Fatalf("expected validation issue for %s, got %v", field, fields)
			}
		}
	})
}

func TestConfigPathHelpersCoverRemainingErrorBranches(t *testing.T) {
	t.Run("user config resolution errors propagate", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		for _, fn := range []struct {
			name string
			run  func() error
		}{
			{name: "ConfigStacksDirPath", run: func() error { _, err := ConfigStacksDirPath(); return err }},
			{name: "CurrentStackPath", run: func() error { _, err := CurrentStackPath(); return err }},
			{name: "ConfigFilePathForStack", run: func() error { _, err := ConfigFilePathForStack("staging"); return err }},
			{name: "KnownConfigPaths", run: func() error { _, err := KnownConfigPaths(); return err }},
			{name: "SetCurrentStackName", run: func() error { return SetCurrentStackName("staging") }},
		} {
			if err := fn.run(); err == nil {
				t.Fatalf("expected %s to fail when user config dir is unavailable", fn.name)
			}
		}
	})

	t.Run("current stack root errors when stackctl config dir is a file", func(t *testing.T) {
		configRoot := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configRoot)

		if err := os.WriteFile(filepath.Join(configRoot, "stackctl"), []byte("blocking file"), 0o600); err != nil {
			t.Fatalf("write blocking stackctl file: %v", err)
		}

		if _, err := CurrentStackName(); err == nil || !strings.Contains(err.Error(), "open current stack root") {
			t.Fatalf("expected open-root error, got %v", err)
		}
		if _, err := KnownConfigPaths(); err == nil || !strings.Contains(err.Error(), "read stack config directory") {
			t.Fatalf("expected known-config-paths read error, got %v", err)
		}
	})

	t.Run("default stack clear reports remove errors", func(t *testing.T) {
		configRoot := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configRoot)

		currentPath, err := CurrentStackPath()
		if err != nil {
			t.Fatalf("CurrentStackPath returned error: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(currentPath, "nested"), 0o755); err != nil {
			t.Fatalf("mkdir current stack dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(currentPath, "nested", "keep.txt"), []byte("keep"), 0o600); err != nil {
			t.Fatalf("write nested marker: %v", err)
		}

		err = SetCurrentStackName(DefaultStackName)
		if err == nil || !strings.Contains(err.Error(), "clear current stack selection") {
			t.Fatalf("expected clear-current-stack error, got %v", err)
		}
	})
}

func TestConfigLegacyAndDerivedHelpersCoverRemainingBranches(t *testing.T) {
	cfg := Config{}
	cfg.ApplyDerivedFields()

	if cfg.Services.Postgres.MaxConnections != 100 {
		t.Fatalf("expected postgres max connections default, got %d", cfg.Services.Postgres.MaxConnections)
	}
	if cfg.Services.Postgres.SharedBuffers != "128MB" {
		t.Fatalf("expected shared buffers default, got %q", cfg.Services.Postgres.SharedBuffers)
	}
	if cfg.Services.Postgres.LogMinDurationStatementMS != -1 {
		t.Fatalf("expected postgres log duration default, got %d", cfg.Services.Postgres.LogMinDurationStatementMS)
	}
	if cfg.Services.PgAdmin.BootstrapServerName != "Local Postgres" {
		t.Fatalf("expected pgAdmin bootstrap server name default, got %q", cfg.Services.PgAdmin.BootstrapServerName)
	}
	if cfg.Services.PgAdmin.BootstrapServerGroup != "Local" {
		t.Fatalf("expected pgAdmin bootstrap server group default, got %q", cfg.Services.PgAdmin.BootstrapServerGroup)
	}

	legacy := Default()
	legacy.Setup.IncludePostgres = false
	legacy.Setup.IncludeCockpit = false
	legacy.Setup.InstallCockpit = false
	legacy.Setup.ScaffoldDefaultStack = false
	legacy.System.PackageManager = ""
	applyLegacySetupDefaults([]byte("{"), &legacy, system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	})
	if legacy.Setup.IncludePostgres || legacy.Setup.IncludeCockpit || legacy.Setup.InstallCockpit || legacy.Setup.ScaffoldDefaultStack || legacy.System.PackageManager != "" {
		t.Fatalf("expected invalid legacy YAML input to leave config unchanged, got %+v", legacy.Setup)
	}

	if yamlPathPresent(nil, "setup") {
		t.Fatal("expected nil node to report missing yaml path")
	}
	root := &yaml.Node{Kind: yaml.ScalarNode, Value: "scalar"}
	if yamlPathPresent(root, "setup") {
		t.Fatal("expected scalar yaml node to report missing path")
	}
	if yamlPathPresent(&yaml.Node{}, "setup") {
		t.Fatal("expected empty document node to report missing path")
	}
}

func TestManagedScaffoldCoversOptionalFileDriftAndWriteErrors(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	newConfig := func() Config {
		cfg := Default()
		cfg.Connection.RedisACLUsername = "stackctl"
		cfg.Connection.RedisACLPassword = "redis-acl-secret"
		cfg.ApplyDerivedFields()
		return cfg
	}

	t.Run("nats and pgpass drift require scaffold refresh", func(t *testing.T) {
		cfg := newConfig()
		if _, err := ScaffoldManagedStack(cfg, false); err != nil {
			t.Fatalf("ScaffoldManagedStack returned error: %v", err)
		}

		for _, path := range []string{NATSConfigPath(cfg), PGPassPath(cfg)} {
			if err := os.WriteFile(path, []byte("drifted"), 0o600); err != nil {
				t.Fatalf("write drifted file %s: %v", path, err)
			}
			needsScaffold, err := ManagedStackNeedsScaffold(cfg)
			if err != nil {
				t.Fatalf("ManagedStackNeedsScaffold(%s) returned error: %v", path, err)
			}
			if !needsScaffold {
				t.Fatalf("expected drifted %s to require scaffolding", path)
			}
			if _, err := ScaffoldManagedStack(cfg, true); err != nil {
				t.Fatalf("restore scaffold after drift: %v", err)
			}
		}
	})

	t.Run("managed scaffold reports optional file path errors", func(t *testing.T) {
		type target struct {
			name        string
			path        func(Config) string
			needles     []string
			scaffoldErr string
		}

		for _, tc := range []target{
			{
				name:        "nats config",
				path:        NATSConfigPath,
				needles:     []string{"inspect nats config file", "is a directory"},
				scaffoldErr: "write managed nats config file",
			},
			{
				name:        "redis acl",
				path:        RedisACLPath,
				needles:     []string{"inspect redis ACL file", "is a directory"},
				scaffoldErr: "write redis ACL file",
			},
			{
				name:        "pgadmin servers",
				path:        PgAdminServersPath,
				needles:     []string{"inspect pgAdmin server bootstrap file", "is a directory"},
				scaffoldErr: "write pgAdmin server bootstrap file",
			},
			{
				name:        "pgpass",
				path:        PGPassPath,
				needles:     []string{"inspect pgpass bootstrap file", "is a directory"},
				scaffoldErr: "write pgpass bootstrap file",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("XDG_DATA_HOME", t.TempDir())

				cfg := newConfig()
				if _, err := ScaffoldManagedStack(cfg, false); err != nil {
					t.Fatalf("ScaffoldManagedStack returned error: %v", err)
				}

				targetPath := tc.path(cfg)
				if err := os.Remove(targetPath); err != nil {
					t.Fatalf("remove target file %s: %v", targetPath, err)
				}
				if err := os.Mkdir(targetPath, 0o755); err != nil {
					t.Fatalf("mkdir target path %s: %v", targetPath, err)
				}

				_, err := ManagedStackNeedsScaffold(cfg)
				if err == nil {
					t.Fatal("expected ManagedStackNeedsScaffold to fail on directory path")
				}
				for _, needle := range tc.needles {
					if !strings.Contains(err.Error(), needle) {
						t.Fatalf("expected ManagedStackNeedsScaffold error %q to contain %q", err, needle)
					}
				}

				_, err = ScaffoldManagedStack(cfg, true)
				if err == nil || !strings.Contains(err.Error(), tc.scaffoldErr) || !strings.Contains(err.Error(), "is a directory") {
					t.Fatalf("unexpected ScaffoldManagedStack error: %v", err)
				}
			})
		}
	})
}

func validationFields(issues []ValidationIssue) map[string]bool {
	fields := make(map[string]bool, len(issues))
	for _, issue := range issues {
		fields[issue.Field] = true
	}
	return fields
}
