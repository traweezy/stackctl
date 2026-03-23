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

	for field, value := range map[string]string{
		"services.postgres_container":            cfg.Services.PostgresContainer,
		"services.redis_container":               cfg.Services.RedisContainer,
		"services.postgres.image":                cfg.Services.Postgres.Image,
		"services.postgres.data_volume":          cfg.Services.Postgres.DataVolume,
		"services.postgres.maintenance_database": cfg.Services.Postgres.MaintenanceDatabase,
		"services.redis.image":                   cfg.Services.Redis.Image,
		"services.redis.data_volume":             cfg.Services.Redis.DataVolume,
		"services.redis.save_policy":             cfg.Services.Redis.SavePolicy,
		"services.redis.maxmemory_policy":        cfg.Services.Redis.MaxMemoryPolicy,
		"connection.host":                        cfg.Connection.Host,
		"connection.postgres_database":           cfg.Connection.PostgresDatabase,
		"connection.postgres_username":           cfg.Connection.PostgresUsername,
		"connection.postgres_password":           cfg.Connection.PostgresPassword,
	} {
		if strings.TrimSpace(value) == "" {
			issues = append(issues, ValidationIssue{Field: field, Message: "must not be empty"})
		}
	}

	if cfg.Setup.IncludePgAdmin {
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

	for field, value := range map[string]int{
		"ports.postgres": cfg.Ports.Postgres,
		"ports.redis":    cfg.Ports.Redis,
		"ports.cockpit":  cfg.Ports.Cockpit,
	} {
		if value < 1 || value > 65535 {
			issues = append(issues, ValidationIssue{Field: field, Message: "must be between 1 and 65535"})
		}
	}
	if cfg.Setup.IncludePgAdmin && (cfg.Ports.PgAdmin < 1 || cfg.Ports.PgAdmin > 65535) {
		issues = append(issues, ValidationIssue{Field: "ports.pgadmin", Message: "must be between 1 and 65535"})
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
