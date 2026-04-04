package tui

import (
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestConfigFieldSpecsSettersAndHelpersCoverRegisteredFields(t *testing.T) {
	t.Run("stack.managed covers both directions", func(t *testing.T) {
		spec := testConfigSpecByKey(t, "stack.managed")

		cfg := configpkg.Default()
		cfg.Stack.Managed = false
		cfg.Setup.ScaffoldDefaultStack = false
		cfg.Stack.Dir = t.TempDir()
		cfg.Stack.ComposeFile = "compose.yaml"

		if err := spec.SetBool(&cfg, true); err != nil {
			t.Fatalf("SetBool(true) returned error: %v", err)
		}
		expectedDir, err := configpkg.ManagedStackDir(cfg.Stack.Name)
		if err != nil {
			t.Fatalf("ManagedStackDir returned error: %v", err)
		}
		if !cfg.Stack.Managed || cfg.Stack.Dir != expectedDir || cfg.Stack.ComposeFile != configpkg.DefaultComposeFileName || !cfg.Setup.ScaffoldDefaultStack {
			t.Fatalf("expected managed toggle to restore derived defaults, got %+v", cfg.Stack)
		}

		if err := spec.SetBool(&cfg, false); err != nil {
			t.Fatalf("SetBool(false) returned error: %v", err)
		}
		if cfg.Stack.Managed || cfg.Setup.ScaffoldDefaultStack {
			t.Fatalf("expected disabling managed mode to disable scaffold defaults, got %+v", cfg.Setup)
		}
	})

	t.Run("per-field setters and helper closures", func(t *testing.T) {
		for _, spec := range configFieldSpecs {
			spec := spec
			t.Run(spec.Key, func(t *testing.T) {
				cfg := configpkg.Default()
				configureFieldSpecPreconditions(t, &cfg, spec)

				if spec.DescriptionFor != nil {
					if description := strings.TrimSpace(spec.DescriptionFor(cfg)); description == "" {
						t.Fatalf("expected DescriptionFor for %s to return text", spec.Key)
					}
				}
				if spec.EditableReason != nil {
					_ = spec.EditableReason(cfg)
				}
				if spec.Suggestions != nil {
					if suggestions := spec.Suggestions(cfg); len(suggestions) == 0 {
						t.Fatalf("expected Suggestions for %s to return values", spec.Key)
					}
				}

				switch spec.Kind {
				case configFieldBool:
					if spec.GetBool == nil || spec.SetBool == nil {
						t.Fatalf("expected bool accessors for %s", spec.Key)
					}
					before := spec.GetBool(cfg)
					if err := spec.SetBool(&cfg, !before); err != nil {
						t.Fatalf("SetBool returned error: %v", err)
					}
					if got := spec.GetBool(cfg); got == before {
						t.Fatalf("expected %s to toggle, still %v", spec.Key, got)
					}
				case configFieldString, configFieldInt:
					if spec.GetString == nil || spec.SetString == nil {
						t.Fatalf("expected string accessors for %s", spec.Key)
					}
					before := spec.GetString(cfg)
					value, expected := configFieldSpecValue(t, spec)
					if err := spec.SetString(&cfg, value); err != nil {
						t.Fatalf("SetString returned error: %v", err)
					}
					if got := spec.GetString(cfg); got != expected {
						t.Fatalf("%s GetString() = %q, want %q", spec.Key, got, expected)
					}
					if expected != before && spec.Key != "stack.name" && spec.Key != "stack.dir" && spec.Key != "system.package_manager" {
						if got := spec.GetString(cfg); got == before {
							t.Fatalf("expected %s to change from %q", spec.Key, before)
						}
					}
				default:
					t.Fatalf("unexpected config field kind %v for %s", spec.Kind, spec.Key)
				}
			})
		}
	})

	t.Run("editable reasons and config-field state styles cover blocked and fallback paths", func(t *testing.T) {
		dirSpec := testConfigSpecByKey(t, "stack.dir")
		composeSpec := testConfigSpecByKey(t, "stack.compose_file")

		managed := configpkg.Default()
		if reason := strings.TrimSpace(dirSpec.EditableReason(managed)); !strings.Contains(reason, "Managed stacks derive the stack directory") {
			t.Fatalf("unexpected managed stack.dir reason: %q", reason)
		}
		if reason := strings.TrimSpace(composeSpec.EditableReason(managed)); !strings.Contains(reason, "Managed stacks always use the embedded compose filename") {
			t.Fatalf("unexpected managed stack.compose_file reason: %q", reason)
		}

		external := managed
		external.Stack.Managed = false
		external.Setup.ScaffoldDefaultStack = false
		if reason := dirSpec.EditableReason(external); reason != "" {
			t.Fatalf("expected external stack.dir reason to clear, got %q", reason)
		}
		if reason := composeSpec.EditableReason(external); reason != "" {
			t.Fatalf("expected external stack.compose_file reason to clear, got %q", reason)
		}

		if got := configFieldStateStyle("invalid"); got.GetForeground() == nil {
			t.Fatal("expected invalid state style to set a foreground color")
		}
		if got := configFieldStateStyle("edited"); got.GetForeground() == nil {
			t.Fatal("expected edited state style to set a foreground color")
		}
		if got := configFieldStateStyle("unknown"); got.GetForeground() == nil {
			t.Fatal("expected fallback state style to set a foreground color")
		}
	})

	t.Run("redis image support helper and secret redaction cover remaining branches", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Services.Redis.Image = "docker.io/library/redis:8.6"
		if !redisImageSupportsLRMPolicies(cfg.Services.Redis.Image) {
			t.Fatal("expected Redis 8.6 image to support LRM policies")
		}
		cfg.Services.Redis.Image = "docker.io/library/redis:5"
		if redisImageSupportsLRMPolicies(cfg.Services.Redis.Image) {
			t.Fatal("expected Redis 5 image not to support LRM policies")
		}

		cfg = configpkg.Default()
		cfg.Connection.PostgresPassword = "postgres"
		cfg.Connection.RedisPassword = "redis"
		cfg.Connection.RedisACLPassword = "redis-acl"
		cfg.Connection.NATSToken = "nats"
		cfg.Connection.SeaweedFSSecretKey = "seaweedfs"
		cfg.Connection.MeilisearchMasterKey = "meili"
		cfg.Connection.PgAdminPassword = "pgadmin"

		redacted := redactConfigSecrets(cfg)
		for key, value := range map[string]string{
			"postgres":  redacted.Connection.PostgresPassword,
			"redis":     redacted.Connection.RedisPassword,
			"redis-acl": redacted.Connection.RedisACLPassword,
			"nats":      redacted.Connection.NATSToken,
			"seaweedfs": redacted.Connection.SeaweedFSSecretKey,
			"meili":     redacted.Connection.MeilisearchMasterKey,
			"pgadmin":   redacted.Connection.PgAdminPassword,
		} {
			if value != maskedSecret {
				t.Fatalf("expected %s secret to be masked, got %q", key, value)
			}
		}
	})
}

func configureFieldSpecPreconditions(t *testing.T, cfg *configpkg.Config, spec configFieldSpec) {
	t.Helper()

	switch spec.Key {
	case "stack.dir", "stack.compose_file":
		cfg.Stack.Managed = false
		cfg.Setup.ScaffoldDefaultStack = false
		cfg.Stack.Dir = t.TempDir()
		cfg.Stack.ComposeFile = "compose.yaml"
	case "stack.managed":
		cfg.Stack.Managed = true
	case "setup.install_cockpit":
		cfg.Setup.IncludeCockpit = true
		cfg.Setup.InstallCockpit = false
	}
}

func configFieldSpecValue(t *testing.T, spec configFieldSpec) (string, string) {
	t.Helper()

	switch {
	case spec.Key == "stack.name":
		return "ops-stack", "ops-stack"
	case spec.Key == "stack.dir":
		path := filepath.Join(t.TempDir(), "external-stack")
		return path, path
	case spec.Key == "stack.compose_file":
		return "compose.custom.yaml", "compose.custom.yaml"
	case spec.Key == "services.postgres.log_min_duration_statement_ms":
		return "250", "250"
	case spec.Key == "system.package_manager":
		return " brew ", "brew"
	case strings.HasPrefix(spec.Key, "ports."):
		return "15432", "15432"
	case spec.Key == "services.postgres.max_connections":
		return "150", "150"
	case spec.Key == "services.seaweedfs.volume_size_limit_mb":
		return "2048", "2048"
	case spec.Key == "behavior.startup_timeout_seconds":
		return "45", "45"
	case spec.Key == "tui.auto_refresh_interval_seconds":
		return "15", "15"
	case strings.Contains(spec.Key, "redis.maxmemory_policy"):
		return "allkeys-lru", "allkeys-lru"
	case strings.Contains(spec.Key, "redis.save_policy"):
		return "900 1 300 10", "900 1 300 10"
	case strings.Contains(spec.Key, "connection.host"):
		return "db.internal", "db.internal"
	default:
		value := "changed-" + strings.NewReplacer(".", "-", "_", "-").Replace(spec.Key)
		return value, value
	}
}
