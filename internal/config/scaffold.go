package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	embedded "github.com/traweezy/stackctl/templates"
)

type ScaffoldResult struct {
	StackDir            string
	ComposePath         string
	NATSConfigPath      string
	RedisACLPath        string
	PgAdminServersPath  string
	PGPassPath          string
	CreatedDir          bool
	WroteCompose        bool
	WroteNATSConfig     bool
	WroteRedisACL       bool
	WrotePgAdminServers bool
	WrotePGPass         bool
	AlreadyPresent      bool
}

func ManagedStackNeedsScaffold(cfg Config) (bool, error) {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return false, nil
	}

	composeData, err := renderManagedCompose(cfg)
	if err != nil {
		return false, err
	}
	if needsWrite, err := scaffoldFileNeedsWrite(ComposePath(cfg), composeData); err != nil {
		return false, fmt.Errorf("inspect compose file %s: %w", ComposePath(cfg), err)
	} else if needsWrite {
		return true, nil
	}

	if cfg.Setup.IncludeNATS {
		natsConfigData, err := renderManagedNATSConfig(cfg)
		if err != nil {
			return false, err
		}
		if needsWrite, err := scaffoldFileNeedsWrite(NATSConfigPath(cfg), natsConfigData); err != nil {
			return false, fmt.Errorf("inspect nats config file %s: %w", NATSConfigPath(cfg), err)
		} else if needsWrite {
			return true, nil
		}
	}
	if cfg.RedisACLEnabled() {
		redisACLData, err := renderManagedRedisACL(cfg)
		if err != nil {
			return false, err
		}
		if needsWrite, err := scaffoldFileNeedsWrite(RedisACLPath(cfg), redisACLData); err != nil {
			return false, fmt.Errorf("inspect redis ACL file %s: %w", RedisACLPath(cfg), err)
		} else if needsWrite {
			return true, nil
		}
	}
	if cfg.PgAdminBootstrapEnabled() {
		serversData, err := renderManagedPgAdminServers(cfg)
		if err != nil {
			return false, err
		}
		if needsWrite, err := scaffoldFileNeedsWrite(PgAdminServersPath(cfg), serversData); err != nil {
			return false, fmt.Errorf("inspect pgAdmin server bootstrap file %s: %w", PgAdminServersPath(cfg), err)
		} else if needsWrite {
			return true, nil
		}

		pgPassData, err := renderManagedPGPass(cfg)
		if err != nil {
			return false, err
		}
		if needsWrite, err := scaffoldFileNeedsWrite(PGPassPath(cfg), pgPassData); err != nil {
			return false, fmt.Errorf("inspect pgpass bootstrap file %s: %w", PGPassPath(cfg), err)
		} else if needsWrite {
			return true, nil
		}
	}

	return false, nil
}

func ScaffoldManagedStack(cfg Config, force bool) (ScaffoldResult, error) {
	result := ScaffoldResult{
		StackDir:           cfg.Stack.Dir,
		ComposePath:        ComposePath(cfg),
		NATSConfigPath:     NATSConfigPath(cfg),
		RedisACLPath:       RedisACLPath(cfg),
		PgAdminServersPath: PgAdminServersPath(cfg),
		PGPassPath:         PGPassPath(cfg),
	}

	if !cfg.Stack.Managed {
		return result, errors.New("managed stack scaffolding requires stack.managed = true")
	}

	expectedDir, err := ManagedStackDir(cfg.Stack.Name)
	if err != nil {
		return result, err
	}
	if cfg.Stack.Dir != expectedDir {
		return result, fmt.Errorf("managed stack dir must be %s", expectedDir)
	}
	if cfg.Stack.ComposeFile != DefaultComposeFileName {
		return result, fmt.Errorf("managed stack compose file must be %s", DefaultComposeFileName)
	}

	if info, err := os.Stat(cfg.Stack.Dir); err == nil {
		if !info.IsDir() {
			return result, fmt.Errorf("managed stack path %s is not a directory", cfg.Stack.Dir)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(cfg.Stack.Dir, 0o750); err != nil {
			return result, fmt.Errorf("create managed stack directory %s: %w", cfg.Stack.Dir, err)
		}
		result.CreatedDir = true
	} else {
		return result, fmt.Errorf("inspect managed stack directory %s: %w", cfg.Stack.Dir, err)
	}

	composeData, err := renderManagedCompose(cfg)
	if err != nil {
		return result, err
	}

	wroteCompose, err := writeScaffoldFile(result.ComposePath, composeData, force)
	if err != nil {
		return result, fmt.Errorf("write managed compose file %s: %w", result.ComposePath, err)
	}
	result.WroteCompose = wroteCompose

	if cfg.Setup.IncludeNATS {
		natsConfigData, err := renderManagedNATSConfig(cfg)
		if err != nil {
			return result, err
		}
		wroteNATSConfig, err := writeScaffoldFile(result.NATSConfigPath, natsConfigData, force)
		if err != nil {
			return result, fmt.Errorf("write managed nats config file %s: %w", result.NATSConfigPath, err)
		}
		result.WroteNATSConfig = wroteNATSConfig
	}
	if cfg.RedisACLEnabled() {
		redisACLData, err := renderManagedRedisACL(cfg)
		if err != nil {
			return result, err
		}
		wroteRedisACL, err := writeScaffoldFile(result.RedisACLPath, redisACLData, force)
		if err != nil {
			return result, fmt.Errorf("write redis ACL file %s: %w", result.RedisACLPath, err)
		}
		result.WroteRedisACL = wroteRedisACL
	}
	if cfg.PgAdminBootstrapEnabled() {
		serversData, err := renderManagedPgAdminServers(cfg)
		if err != nil {
			return result, err
		}
		wroteServers, err := writeScaffoldFile(result.PgAdminServersPath, serversData, force)
		if err != nil {
			return result, fmt.Errorf("write pgAdmin server bootstrap file %s: %w", result.PgAdminServersPath, err)
		}
		result.WrotePgAdminServers = wroteServers

		pgPassData, err := renderManagedPGPass(cfg)
		if err != nil {
			return result, err
		}
		wrotePGPass, err := writeScaffoldFile(result.PGPassPath, pgPassData, force)
		if err != nil {
			return result, fmt.Errorf("write pgpass bootstrap file %s: %w", result.PGPassPath, err)
		}
		result.WrotePGPass = wrotePGPass
	}

	result.AlreadyPresent = !result.WroteCompose &&
		!result.WroteNATSConfig &&
		!result.WroteRedisACL &&
		!result.WrotePgAdminServers &&
		!result.WrotePGPass

	return result, nil
}

func renderManagedCompose(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	tmpl, err := template.New("dev-stack-compose").Option("missingkey=error").Parse(string(embedded.DevStackComposeYAML()))
	if err != nil {
		return nil, fmt.Errorf("parse embedded compose template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("render managed compose template: %w", err)
	}

	return buf.Bytes(), nil
}

func renderManagedNATSConfig(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	tmpl, err := template.New("dev-stack-nats").Option("missingkey=error").Parse(string(embedded.DevStackNATSConfig()))
	if err != nil {
		return nil, fmt.Errorf("parse embedded nats template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("render managed nats template: %w", err)
	}

	return buf.Bytes(), nil
}

func renderManagedRedisACL(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	tmpl, err := template.New("dev-stack-redis-acl").Option("missingkey=error").Parse(string(embedded.DevStackRedisACL()))
	if err != nil {
		return nil, fmt.Errorf("parse embedded redis ACL template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("render managed redis ACL template: %w", err)
	}

	return buf.Bytes(), nil
}

func renderManagedPgAdminServers(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	tmpl, err := template.New("dev-stack-pgadmin-servers").Option("missingkey=error").Parse(string(embedded.DevStackPgAdminServers()))
	if err != nil {
		return nil, fmt.Errorf("parse embedded pgAdmin server template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("render managed pgAdmin server template: %w", err)
	}

	return buf.Bytes(), nil
}

func renderManagedPGPass(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	tmpl, err := template.New("dev-stack-pgpass").Option("missingkey=error").Parse(string(embedded.DevStackPGPass()))
	if err != nil {
		return nil, fmt.Errorf("parse embedded pgpass template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("render managed pgpass template: %w", err)
	}

	return buf.Bytes(), nil
}

func scaffoldFileMissing(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("%s is a directory", path)
		}
		return false, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}

	return false, err
}

func scaffoldFileNeedsWrite(path string, expected []byte) (bool, error) {
	missing, err := scaffoldFileMissing(path)
	if err != nil || missing {
		return missing, err
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	defer root.Close()

	current, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		return false, err
	}

	return !bytes.Equal(current, expected), nil
}

func writeScaffoldFile(path string, data []byte, force bool) (bool, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if info.IsDir() {
			return false, fmt.Errorf("%s is a directory", path)
		}
		if !force {
			return false, nil
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return false, err
	}

	if err := os.WriteFile(path, data, scaffoldFileMode(path)); err != nil {
		return false, err
	}

	return true, nil
}

func scaffoldFileMode(path string) os.FileMode {
	if filepath.Base(path) == DefaultRedisACLName {
		return 0o644
	}

	return 0o600
}
