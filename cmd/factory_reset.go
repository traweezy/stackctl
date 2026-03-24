package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

type composeCleanupTarget struct {
	StackDir    string
	ComposePath string
}

func newFactoryResetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "factory-reset",
		Short: "DANGEROUS: delete all stackctl local config and data",
		Long: "DANGEROUS: stop managed stacks discovered under stackctl's local data directory, " +
			"remove their volumes, and then delete all stackctl-owned config and data directories.",
		Example: "  stackctl factory-reset\n" +
			"  stackctl factory-reset --force",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := deps.configDirPath()
			if err != nil {
				return err
			}
			configPaths, err := deps.knownConfigPaths()
			if err != nil {
				return err
			}
			dataDir, err := deps.dataDirPath()
			if err != nil {
				return err
			}

			targets, err := localComposeCleanupTargets(configPaths, dataDir)
			if err != nil {
				return err
			}

			if !force {
				ok, err := confirmWithPrompt(cmd, factoryResetPrompt(configDir, dataDir, targets), false)
				if err != nil {
					return fmt.Errorf("factory reset confirmation required; rerun with --force")
				}
				if !ok {
					return userCancelled(cmd, "factory reset cancelled")
				}
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusReset, "factory-resetting stackctl local state..."); err != nil {
				return err
			}

			for _, target := range targets {
				if err := output.StatusLine(cmd.OutOrStdout(), output.StatusReset, fmt.Sprintf("removing managed stack %s...", target.ComposePath)); err != nil {
					return err
				}
				if err := deps.composeDownPath(context.Background(), runnerFor(cmd), target.StackDir, target.ComposePath, true); err != nil {
					return fmt.Errorf("tear down managed stack %s: %w", target.ComposePath, err)
				}
			}

			if err := deps.removeAll(configDir); err != nil {
				return fmt.Errorf("remove config dir %s: %w", configDir, err)
			}
			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("deleted config dir %s", configDir)); err != nil {
				return err
			}

			if err := deps.removeAll(dataDir); err != nil {
				return fmt.Errorf("remove data dir %s: %w", dataDir, err)
			}
			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("deleted data dir %s", dataDir)); err != nil {
				return err
			}

			return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, "stackctl local state removed")
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip the DANGEROUS confirmation prompt")

	return cmd
}

func localComposeCleanupTargets(configPaths []string, dataDir string) ([]composeCleanupTarget, error) {
	targets := map[string]composeCleanupTarget{}

	stacksDir := filepath.Join(dataDir, "stacks")
	entries, err := os.ReadDir(stacksDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read managed stacks dir %s: %w", stacksDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stackDir := filepath.Join(stacksDir, entry.Name())
		composePath := filepath.Join(stackDir, configpkg.DefaultComposeFileName)
		if composeFileExists(composePath) {
			targets[composePath] = composeCleanupTarget{StackDir: stackDir, ComposePath: composePath}
		}
	}

	for _, configPath := range configPaths {
		cfg, err := deps.loadConfig(configPath)
		if err != nil || !cfg.Stack.Managed {
			continue
		}
		composePath := deps.composePath(cfg)
		if withinRoot(dataDir, cfg.Stack.Dir) && composeFileExists(composePath) {
			targets[composePath] = composeCleanupTarget{StackDir: cfg.Stack.Dir, ComposePath: composePath}
		}
	}

	values := make([]composeCleanupTarget, 0, len(targets))
	for _, target := range targets {
		values = append(values, target)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].ComposePath < values[j].ComposePath
	})

	return values, nil
}

func factoryResetPrompt(configDir, dataDir string, targets []composeCleanupTarget) string {
	lines := []string{
		"DANGEROUS: This permanently deletes all stackctl local state.",
		"",
		"Config dir: " + configDir,
		"Data dir:   " + dataDir,
	}
	if len(targets) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Managed stacks to stop and wipe: %d", len(targets)))
		for _, target := range targets {
			lines = append(lines, "  - "+target.ComposePath)
		}
	}
	lines = append(lines, "")
	lines = append(lines, "Continue?")
	return strings.Join(lines, "\n")
}

func composeFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func withinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".."
}
