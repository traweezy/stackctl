package config

import (
	"errors"
	"fmt"
	"os"

	embedded "github.com/traweezy/stackctl/templates"
)

type ScaffoldResult struct {
	StackDir       string
	ComposePath    string
	CreatedDir     bool
	WroteCompose   bool
	AlreadyPresent bool
}

func ManagedStackNeedsScaffold(cfg Config) (bool, error) {
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		return false, nil
	}

	info, err := os.Stat(ComposePath(cfg))
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("compose path %s is a directory", ComposePath(cfg))
		}
		return false, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}

	return false, fmt.Errorf("inspect compose file %s: %w", ComposePath(cfg), err)
}

func ScaffoldManagedStack(cfg Config, force bool) (ScaffoldResult, error) {
	result := ScaffoldResult{
		StackDir:    cfg.Stack.Dir,
		ComposePath: ComposePath(cfg),
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
		if err := os.MkdirAll(cfg.Stack.Dir, 0o755); err != nil {
			return result, fmt.Errorf("create managed stack directory %s: %w", cfg.Stack.Dir, err)
		}
		result.CreatedDir = true
	} else {
		return result, fmt.Errorf("inspect managed stack directory %s: %w", cfg.Stack.Dir, err)
	}

	if info, err := os.Stat(result.ComposePath); err == nil {
		if info.IsDir() {
			return result, fmt.Errorf("managed compose path %s is a directory", result.ComposePath)
		}
		if !force {
			result.AlreadyPresent = true
			return result, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("inspect managed compose file %s: %w", result.ComposePath, err)
	}

	if err := os.WriteFile(result.ComposePath, embedded.DevStackComposeYAML(), 0o644); err != nil {
		return result, fmt.Errorf("write managed compose file %s: %w", result.ComposePath, err)
	}
	result.WroteCompose = true

	return result, nil
}
