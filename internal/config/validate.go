package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ValidationIssue struct {
	Field   string `yaml:"field"`
	Message string `yaml:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "config validation failed"
	}

	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Field, issue.Message))
	}

	return fmt.Sprintf("config validation failed: %s", strings.Join(parts, "; "))
}

func Validate(cfg Config) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	if strings.TrimSpace(cfg.Stack.Name) == "" {
		issues = append(issues, ValidationIssue{Field: "stack.name", Message: "must not be empty"})
	} else if err := ValidateStackName(cfg.Stack.Name); err != nil {
		issues = append(issues, ValidationIssue{Field: "stack.name", Message: err.Error()})
	}
	if strings.TrimSpace(cfg.Stack.Dir) == "" {
		issues = append(issues, ValidationIssue{Field: "stack.dir", Message: "must not be empty"})
	} else {
		if !filepath.IsAbs(cfg.Stack.Dir) {
			issues = append(issues, ValidationIssue{Field: "stack.dir", Message: "must be an absolute path"})
		}
		if info, err := os.Stat(cfg.Stack.Dir); err != nil {
			issues = append(issues, ValidationIssue{Field: "stack.dir", Message: fmt.Sprintf("directory does not exist: %s", cfg.Stack.Dir)})
		} else if !info.IsDir() {
			issues = append(issues, ValidationIssue{Field: "stack.dir", Message: fmt.Sprintf("directory does not exist: %s", cfg.Stack.Dir)})
		}
	}
	if strings.TrimSpace(cfg.Stack.ComposeFile) == "" {
		issues = append(issues, ValidationIssue{Field: "stack.compose_file", Message: "must not be empty"})
	}
	if cfg.Stack.Managed {
		expectedDir, err := ManagedStackDir(cfg.Stack.Name)
		if err != nil {
			issues = append(issues, ValidationIssue{Field: "stack.dir", Message: fmt.Sprintf("resolve managed stack path: %v", err)})
		} else if cfg.Stack.Dir != expectedDir {
			issues = append(issues, ValidationIssue{Field: "stack.dir", Message: fmt.Sprintf("managed stack must use %s", expectedDir)})
		}
		if cfg.Stack.ComposeFile != DefaultComposeFileName {
			issues = append(issues, ValidationIssue{Field: "stack.compose_file", Message: fmt.Sprintf("managed stack must use %s", DefaultComposeFileName)})
		}
	}
	if cfg.Stack.Managed && strings.TrimSpace(cfg.Stack.Dir) != "" && filepath.IsAbs(cfg.Stack.Dir) && strings.TrimSpace(cfg.Stack.ComposeFile) != "" {
		composePath := ComposePath(cfg)
		if info, err := os.Stat(composePath); err != nil {
			issues = append(issues, ValidationIssue{Field: "stack.compose_file", Message: fmt.Sprintf("file does not exist: %s", composePath)})
		} else if info.IsDir() {
			issues = append(issues, ValidationIssue{Field: "stack.compose_file", Message: fmt.Sprintf("path is a directory: %s", composePath)})
		}
	}
	if cfg.EnabledStackServiceCount() == 0 {
		issues = append(issues, ValidationIssue{Field: "setup", Message: "at least one stack service must be enabled"})
	}

	for field, value := range map[string]string{
		"connection.host": cfg.Connection.Host,
	} {
		if strings.TrimSpace(value) == "" {
			issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
		}
	}

	if cfg.PostgresEnabled() {
		for field, value := range map[string]string{
			"services.postgres_container":            cfg.Services.PostgresContainer,
			"services.postgres.image":                cfg.Services.Postgres.Image,
			"services.postgres.data_volume":          cfg.Services.Postgres.DataVolume,
			"services.postgres.maintenance_database": cfg.Services.Postgres.MaintenanceDatabase,
			"connection.postgres_database":           cfg.Connection.PostgresDatabase,
			"connection.postgres_username":           cfg.Connection.PostgresUsername,
			"connection.postgres_password":           cfg.Connection.PostgresPassword,
		} {
			if strings.TrimSpace(value) == "" {
				issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
			}
		}
	}

	if cfg.RedisEnabled() {
		for field, value := range map[string]string{
			"services.redis_container":        cfg.Services.RedisContainer,
			"services.redis.image":            cfg.Services.Redis.Image,
			"services.redis.data_volume":      cfg.Services.Redis.DataVolume,
			"services.redis.save_policy":      cfg.Services.Redis.SavePolicy,
			"services.redis.maxmemory_policy": cfg.Services.Redis.MaxMemoryPolicy,
		} {
			if strings.TrimSpace(value) == "" {
				issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
			}
		}
	}

	if cfg.NATSEnabled() {
		for field, value := range map[string]string{
			"services.nats_container": cfg.Services.NATSContainer,
			"services.nats.image":     cfg.Services.NATS.Image,
			"connection.nats_token":   cfg.Connection.NATSToken,
		} {
			if strings.TrimSpace(value) == "" {
				issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
			}
		}
	}

	if cfg.SeaweedFSEnabled() {
		for field, value := range map[string]string{
			"services.seaweedfs_container":    cfg.Services.SeaweedFSContainer,
			"services.seaweedfs.image":        cfg.Services.SeaweedFS.Image,
			"services.seaweedfs.data_volume":  cfg.Services.SeaweedFS.DataVolume,
			"connection.seaweedfs_access_key": cfg.Connection.SeaweedFSAccessKey,
			"connection.seaweedfs_secret_key": cfg.Connection.SeaweedFSSecretKey,
		} {
			if strings.TrimSpace(value) == "" {
				issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
			}
		}
		if cfg.Services.SeaweedFS.VolumeSizeLimitMB <= 0 {
			issues = append(issues, ValidationIssue{Field: "services.seaweedfs.volume_size_limit_mb", Message: "must be greater than zero"})
		}
	}

	if cfg.PgAdminEnabled() {
		for field, value := range map[string]string{
			"services.pgadmin_container":   cfg.Services.PgAdminContainer,
			"services.pgadmin.image":       cfg.Services.PgAdmin.Image,
			"services.pgadmin.data_volume": cfg.Services.PgAdmin.DataVolume,
			"connection.pgadmin_email":     cfg.Connection.PgAdminEmail,
			"connection.pgadmin_password":  cfg.Connection.PgAdminPassword,
		} {
			if strings.TrimSpace(value) == "" {
				issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
			}
		}
	}

	if cfg.PostgresEnabled() && (cfg.Ports.Postgres < 1 || cfg.Ports.Postgres > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.postgres", Message: "must be between 1 and 65535"})
	}
	if cfg.RedisEnabled() && (cfg.Ports.Redis < 1 || cfg.Ports.Redis > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.redis", Message: "must be between 1 and 65535"})
	}
	if cfg.NATSEnabled() && (cfg.Ports.NATS < 1 || cfg.Ports.NATS > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.nats", Message: "must be between 1 and 65535"})
	}
	if cfg.SeaweedFSEnabled() && (cfg.Ports.SeaweedFS < 1 || cfg.Ports.SeaweedFS > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.seaweedfs", Message: "must be between 1 and 65535"})
	}
	if cfg.PgAdminEnabled() && (cfg.Ports.PgAdmin < 1 || cfg.Ports.PgAdmin > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.pgadmin", Message: "must be between 1 and 65535"})
	}
	if cfg.CockpitEnabled() && (cfg.Ports.Cockpit < 1 || cfg.Ports.Cockpit > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.cockpit", Message: "must be between 1 and 65535"})
	}

	if cfg.Behavior.StartupTimeoutSec <= 0 {
		issues = append(issues, ValidationIssue{Field: "behavior.startup_timeout_seconds", Message: "must be greater than zero"})
	}
	if cfg.TUI.AutoRefreshIntervalSec <= 0 {
		issues = append(issues, ValidationIssue{Field: "tui.auto_refresh_interval_seconds", Message: "must be greater than zero"})
	}

	if strings.TrimSpace(cfg.System.PackageManager) == "" {
		issues = append(issues, ValidationIssue{Field: "system.package_manager", Message: "must not be empty"})
	}

	return issues
}

func ValidateOrError(cfg Config) error {
	issues := Validate(cfg)
	if len(issues) == 0 {
		return nil
	}

	return ValidationError{Issues: issues}
}
