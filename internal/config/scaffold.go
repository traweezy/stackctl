package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"text/template"

	embedded "github.com/traweezy/stackctl/templates"
)

type ScaffoldResult struct {
	StackDir        string
	ComposePath     string
	NATSConfigPath  string
	CreatedDir      bool
	WroteCompose    bool
	WroteNATSConfig bool
	AlreadyPresent  bool
}

func ManagedStackNeedsScaffold(cfg Config) (bool, error) {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return false, nil
	}

	if missing, err := scaffoldFileMissing(ComposePath(cfg)); err != nil {
		return false, fmt.Errorf("inspect compose file %s: %w", ComposePath(cfg), err)
	} else if missing {
		return true, nil
	}

	if cfg.Setup.IncludeNATS {
		if missing, err := scaffoldFileMissing(NATSConfigPath(cfg)); err != nil {
			return false, fmt.Errorf("inspect nats config file %s: %w", NATSConfigPath(cfg), err)
		} else if missing {
			return true, nil
		}
	}

	return false, nil
}

func ScaffoldManagedStack(cfg Config, force bool) (ScaffoldResult, error) {
	result := ScaffoldResult{
		StackDir:       cfg.Stack.Dir,
		ComposePath:    ComposePath(cfg),
		NATSConfigPath: NATSConfigPath(cfg),
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

	result.AlreadyPresent = !result.WroteCompose && !result.WroteNATSConfig

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

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return false, err
	}

	return true, nil
}
