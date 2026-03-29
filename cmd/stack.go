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

type stackListEntry struct {
	Name       string
	ConfigPath string
	Current    bool
	Configured bool
	State      string
	Mode       string
	Services   string
}

type stackTarget struct {
	Name       string
	ConfigPath string
	Exists     bool
	Config     configpkg.Config
	LoadErr    error
}

type stackDeleteResult struct {
	Name            string
	ConfigPath      string
	PurgedDataDir   string
	ResetToDefault  bool
	ManagedDataKept string
	AdditionalNotes []string
}

func newStackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage named stack profiles",
		Example: "  stackctl stack list\n" +
			"  stackctl stack current\n" +
			"  stackctl stack use staging\n" +
			"  stackctl stack clone dev-stack demo\n" +
			"  stackctl stack rename demo qa\n" +
			"  stackctl stack delete qa --purge-data --force",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}

	cmd.AddGroup(stackCommandGroups()...)
	cmd.SetHelpCommandGroupID(stackGroupInspect)

	listCmd := noArgsCommand(newStackListCmd())
	listCmd.GroupID = stackGroupInspect
	currentCmd := noArgsCommand(newStackCurrentCmd())
	currentCmd.GroupID = stackGroupInspect
	useCmd := newStackUseCmd()
	useCmd.GroupID = stackGroupSelect
	deleteCmd := newStackDeleteCmd()
	deleteCmd.GroupID = stackGroupMaintain
	renameCmd := newStackRenameCmd()
	renameCmd.GroupID = stackGroupMaintain
	cloneCmd := newStackCloneCmd()
	cloneCmd.GroupID = stackGroupMaintain

	cmd.AddCommand(listCmd)
	cmd.AddCommand(currentCmd)
	cmd.AddCommand(useCmd)
	cmd.AddCommand(deleteCmd)
	cmd.AddCommand(renameCmd)
	cmd.AddCommand(cloneCmd)

	return cmd
}

func newStackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List configured stack profiles and the active selection",
		Example: "  stackctl stack list",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := discoverStackEntries(context.Background())
			if err != nil {
				return err
			}

			rows := make([][]string, 0, len(entries))
			for _, entry := range entries {
				current := ""
				if entry.Current {
					current = "*"
				}
				rows = append(rows, []string{
					current,
					entry.Name,
					entry.State,
					entry.Mode,
					entry.Services,
					entry.ConfigPath,
				})
			}

			return output.RenderTable(cmd.OutOrStdout(), []string{
				"CURRENT",
				"NAME",
				"STATE",
				"MODE",
				"SERVICES",
				"CONFIG",
			}, rows)
		},
	}
}

func newStackCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "current",
		Short:   "Print the active stack selection",
		Example: "  stackctl stack current",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), configpkg.SelectedStackName())
			return err
		},
	}
}

func newStackUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "use <name>",
		Short:             "Persist a stack as the default selection",
		Example:           "  stackctl stack use staging",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSingleConfiguredStackArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveStackArg(args[0])
			if err != nil {
				return err
			}

			if err := deps.setCurrentStackName(name); err != nil {
				return err
			}

			path, err := deps.configFilePathForStack(name)
			if err != nil {
				return err
			}
			_, exists, err := loadExistingConfig(path)
			if err != nil {
				return err
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("selected stack %s", name)); err != nil {
				return err
			}
			if exists {
				return output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, fmt.Sprintf("using config %s", path))
			}

			return output.StatusLine(
				cmd.OutOrStdout(),
				output.StatusInfo,
				fmt.Sprintf("no config found at %s; run `stackctl config init` to create it", path),
			)
		},
	}

	return cmd
}

func newStackDeleteCmd() *cobra.Command {
	var force bool
	var purgeData bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a stack profile",
		Long: "Delete a stack profile config. Use --purge-data to also stop and remove " +
			"stackctl-managed local data for that stack.",
		Example: "  stackctl stack delete staging\n" +
			"  stackctl stack delete staging --purge-data --force",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSingleConfiguredStackArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveStackTarget(args[0])
			if err != nil {
				return err
			}
			if !target.Exists {
				return fmt.Errorf("stack %s does not exist", target.Name)
			}

			if purgeData {
				if target.LoadErr != nil {
					return fmt.Errorf("stack %s has an invalid config; delete the config first or fix it before using --purge-data: %w", target.Name, target.LoadErr)
				}
				if !target.Config.Stack.Managed {
					return fmt.Errorf("stack %s uses an external stack; --purge-data only applies to stackctl-managed stacks", target.Name)
				}
				if err := purgeManagedStackPreconditions(context.Background(), target.Config); err != nil {
					return err
				}
			} else if target.LoadErr == nil {
				services, err := runningStackServices(context.Background(), target.Config)
				if err == nil && len(services) > 0 {
					return fmt.Errorf(
						"stack %s is running (%s); rerun with `stackctl stack delete %s --purge-data` to stop it and remove local data safely",
						target.Name,
						strings.Join(services, ", "),
						target.Name,
					)
				}
			}

			if !force {
				ok, err := confirmWithPrompt(cmd, stackDeletePrompt(target, purgeData), false)
				if err != nil {
					return fmt.Errorf("delete confirmation required; rerun with --force")
				}
				if !ok {
					return userCancelled(cmd, "stack delete cancelled")
				}
			}

			result, err := deleteStackTarget(context.Background(), target, purgeData)
			if err != nil {
				return err
			}
			if result.PurgedDataDir != "" {
				if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("deleted managed stack data %s", result.PurgedDataDir)); err != nil {
					return err
				}
			}
			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("deleted stack config %s", result.ConfigPath)); err != nil {
				return err
			}
			if result.ResetToDefault {
				if err := output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, fmt.Sprintf("selected stack reset to %s", configpkg.DefaultStackName)); err != nil {
					return err
				}
			}
			if result.ManagedDataKept != "" {
				return output.StatusLine(
					cmd.OutOrStdout(),
					output.StatusWarn,
					fmt.Sprintf("managed stack data remains at %s; rerun with --purge-data to remove it", result.ManagedDataKept),
				)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&purgeData, "purge-data", false, "Stop the managed stack, remove volumes, and delete stackctl-owned local data")

	return cmd
}

func newStackRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a stack profile",
		Example: "  stackctl stack rename staging qa\n" +
			"  stackctl stack rename demo dev-stack",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeRenameStackArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := resolveStackArg(args[0])
			if err != nil {
				return err
			}
			target, err := resolveStackArg(args[1])
			if err != nil {
				return err
			}
			if source == target {
				return errors.New("source and destination stack names must be different")
			}

			sourceTarget, err := resolveStackTarget(source)
			if err != nil {
				return err
			}
			if !sourceTarget.Exists {
				return fmt.Errorf("stack %s does not exist", source)
			}
			if sourceTarget.LoadErr != nil {
				return fmt.Errorf("load source stack %s: %w", source, sourceTarget.LoadErr)
			}

			targetPath, err := deps.configFilePathForStack(target)
			if err != nil {
				return err
			}
			if _, err := deps.stat(targetPath); err == nil {
				return fmt.Errorf("stack %s already exists at %s", target, targetPath)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("check target stack %s: %w", target, err)
			}

			if services, err := runningStackServices(context.Background(), sourceTarget.Config); err == nil && len(services) > 0 {
				return fmt.Errorf("stack %s is running (%s); stop it before renaming", source, strings.Join(services, ", "))
			}

			renamed := retargetStackConfig(sourceTarget.Config, target)
			if err := deps.saveConfig(targetPath, renamed); err != nil {
				return err
			}

			movedDir := false
			if renamed.Stack.Managed {
				if err := moveManagedStackDir(sourceTarget.Config.Stack.Dir, renamed.Stack.Dir); err != nil {
					_ = deps.removeFile(targetPath)
					return err
				}
				movedDir = sourceTarget.Config.Stack.Dir != renamed.Stack.Dir
				if err := scaffoldManagedStackFiles(cmd, renamed, true); err != nil {
					if movedDir {
						_ = deps.rename(renamed.Stack.Dir, sourceTarget.Config.Stack.Dir)
					}
					_ = deps.removeFile(targetPath)
					return err
				}
			}

			if err := deps.removeFile(sourceTarget.ConfigPath); err != nil {
				return fmt.Errorf("remove old stack config %s: %w", sourceTarget.ConfigPath, err)
			}

			current, err := deps.currentStackName()
			if err != nil {
				return err
			}
			if current == source {
				if err := deps.setCurrentStackName(target); err != nil {
					return err
				}
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("renamed stack %s to %s", source, target)); err != nil {
				return err
			}
			if current == source {
				return output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, fmt.Sprintf("selected stack updated to %s", target))
			}

			return nil
		},
	}

	return cmd
}

func newStackCloneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <source-name> <target-name>",
		Short: "Clone a stack profile into a new stack",
		Example: "  stackctl stack clone dev-stack staging\n" +
			"  stackctl stack clone staging demo",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeCloneStackArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := resolveStackArg(args[0])
			if err != nil {
				return err
			}
			target, err := resolveStackArg(args[1])
			if err != nil {
				return err
			}
			if source == target {
				return errors.New("source and destination stack names must be different")
			}

			sourceTarget, err := resolveStackTarget(source)
			if err != nil {
				return err
			}
			if !sourceTarget.Exists {
				return fmt.Errorf("stack %s does not exist", source)
			}
			if sourceTarget.LoadErr != nil {
				return fmt.Errorf("load source stack %s: %w", source, sourceTarget.LoadErr)
			}

			targetPath, err := deps.configFilePathForStack(target)
			if err != nil {
				return err
			}
			if _, err := deps.stat(targetPath); err == nil {
				return fmt.Errorf("stack %s already exists at %s", target, targetPath)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("check target stack %s: %w", target, err)
			}

			cloned := retargetStackConfig(sourceTarget.Config, target)
			if cloned.Stack.Managed {
				if _, err := deps.stat(cloned.Stack.Dir); err == nil {
					return fmt.Errorf("managed stack directory %s already exists", cloned.Stack.Dir)
				} else if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("check managed stack directory %s: %w", cloned.Stack.Dir, err)
				}
			}

			if err := deps.saveConfig(targetPath, cloned); err != nil {
				return err
			}

			if cloned.Stack.Managed {
				if err := scaffoldManagedStackFiles(cmd, cloned, false); err != nil {
					_ = deps.removeFile(targetPath)
					_ = deps.removeAll(cloned.Stack.Dir)
					return err
				}
			}

			if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("cloned stack %s to %s", source, target)); err != nil {
				return err
			}
			return output.StatusLine(cmd.OutOrStdout(), output.StatusInfo, fmt.Sprintf("new config written to %s", targetPath))
		},
	}

	return cmd
}

func completeSingleConfiguredStackArg(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return completeConfiguredStackNames(cmd, args, toComplete)
}

func completeRenameStackArgs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeConfiguredStackNames(cmd, args, toComplete)
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

func completeCloneStackArgs(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeConfiguredStackNames(cmd, args, toComplete)
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

func discoverStackEntries(ctx context.Context) ([]stackListEntry, error) {
	current := configpkg.SelectedStackName()
	paths, err := deps.knownConfigPaths()
	if err != nil {
		return nil, err
	}

	byName := make(map[string]stackListEntry, len(paths)+1)
	for _, path := range paths {
		name := stackNameFromConfigPath(path)
		if name == "" {
			continue
		}

		entry := stackListEntry{
			Name:       name,
			ConfigPath: path,
			Configured: true,
			State:      "invalid",
			Mode:       "invalid",
			Services:   "-",
		}

		cfg, err := deps.loadConfig(path)
		if err == nil {
			entry.Mode = "external"
			if cfg.Stack.Managed {
				entry.Mode = "managed"
			}
			if summary := configuredStackServiceSummary(cfg); summary != "" {
				entry.Services = summary
			}

			services, runErr := runningStackServices(ctx, cfg)
			switch {
			case runErr != nil:
				entry.State = "unknown"
			case len(services) > 0:
				entry.State = "running"
				entry.Services = strings.Join(services, ", ")
			default:
				entry.State = "stopped"
			}
		}

		if name == current {
			entry.Current = true
		}
		byName[name] = entry
	}

	if _, ok := byName[current]; !ok {
		path, err := deps.configFilePathForStack(current)
		if err != nil {
			return nil, err
		}
		byName[current] = stackListEntry{
			Name:       current,
			ConfigPath: path,
			Current:    true,
			Configured: false,
			State:      "not configured",
			Mode:       "-",
			Services:   "-",
		}
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]stackListEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, byName[name])
	}

	return entries, nil
}

func configuredStackServiceSummary(cfg configpkg.Config) string {
	definitions := enabledStackServiceDefinitions(cfg)
	if len(definitions) == 0 {
		return ""
	}

	services := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		services = append(services, definition.DisplayName)
	}
	return strings.Join(services, ", ")
}

func resolveStackArg(value string) (string, error) {
	name := strings.TrimSpace(value)
	if err := configpkg.ValidateStackName(name); err != nil {
		return "", err
	}
	if name == "" {
		return configpkg.DefaultStackName, nil
	}
	return name, nil
}

func resolveStackTarget(name string) (stackTarget, error) {
	selected, err := resolveStackArg(name)
	if err != nil {
		return stackTarget{}, err
	}

	path, err := deps.configFilePathForStack(selected)
	if err != nil {
		return stackTarget{}, err
	}

	if _, err := deps.stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return stackTarget{Name: selected, ConfigPath: path}, nil
		}
		return stackTarget{}, fmt.Errorf("check stack config %s: %w", path, err)
	}

	cfg, err := deps.loadConfig(path)
	if err != nil {
		return stackTarget{Name: selected, ConfigPath: path, Exists: true, LoadErr: err}, nil
	}

	return stackTarget{
		Name:       selected,
		ConfigPath: path,
		Exists:     true,
		Config:     cfg,
	}, nil
}

func stackDeletePrompt(target stackTarget, purgeData bool) string {
	lines := []string{
		fmt.Sprintf("Delete stack %s?", target.Name),
		"",
		"Config: " + target.ConfigPath,
	}
	if target.LoadErr != nil {
		lines = append(lines, "Config status: invalid")
	}
	if purgeData && target.LoadErr == nil {
		lines = append(lines, "Mode: managed")
		lines = append(lines, "Data dir: "+target.Config.Stack.Dir)
		lines = append(lines, "This also stops the stack and removes local volumes.")
	}
	lines = append(lines, "", "Continue?")
	return strings.Join(lines, "\n")
}

func deleteStackTarget(ctx context.Context, target stackTarget, purgeData bool) (stackDeleteResult, error) {
	result := stackDeleteResult{
		Name:       target.Name,
		ConfigPath: target.ConfigPath,
	}

	if purgeData {
		purgedDataDir, err := purgeManagedStackLocalStateQuiet(ctx, target.Config)
		if err != nil {
			return stackDeleteResult{}, err
		}
		result.PurgedDataDir = purgedDataDir
	}

	if err := deps.removeFile(target.ConfigPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return stackDeleteResult{}, fmt.Errorf("delete stack config %s: %w", target.ConfigPath, err)
	}

	current, err := deps.currentStackName()
	if err != nil {
		return stackDeleteResult{}, err
	}
	if current == target.Name {
		if err := deps.setCurrentStackName(configpkg.DefaultStackName); err != nil {
			return stackDeleteResult{}, err
		}
		result.ResetToDefault = true
	}

	if !purgeData && target.LoadErr == nil && target.Config.Stack.Managed {
		result.ManagedDataKept = target.Config.Stack.Dir
	}

	return result, nil
}

func purgeManagedStackPreconditions(ctx context.Context, cfg configpkg.Config) error {
	if !cfg.Stack.Managed {
		return errors.New("purge requires a stackctl-managed stack")
	}

	dataDir, err := deps.dataDirPath()
	if err != nil {
		return err
	}
	if !withinRoot(dataDir, cfg.Stack.Dir) {
		return fmt.Errorf("managed stack directory %s is outside stackctl data dir %s", cfg.Stack.Dir, dataDir)
	}

	composePath := deps.composePath(cfg)
	services, runErr := runningStackServices(ctx, cfg)
	if len(services) > 0 && !stackComposeFileExists(composePath) {
		return fmt.Errorf("stack %s is running but %s is missing; stop it manually before deleting", cfg.Stack.Name, composePath)
	}
	if runErr != nil && stackComposeFileExists(composePath) {
		return nil
	}

	return nil
}

func purgeManagedStackLocalStateQuiet(ctx context.Context, cfg configpkg.Config) (string, error) {
	if err := purgeManagedStackPreconditions(ctx, cfg); err != nil {
		return "", err
	}

	composePath := deps.composePath(cfg)
	if stackComposeFileExists(composePath) {
		if err := composeDownAndWait(ctx, quietRunner(), cfg, true); err != nil {
			return "", fmt.Errorf("tear down managed stack %s: %w", cfg.Stack.Name, err)
		}
	}

	if err := deps.removeAll(cfg.Stack.Dir); err != nil {
		return "", fmt.Errorf("remove managed stack dir %s: %w", cfg.Stack.Dir, err)
	}

	return cfg.Stack.Dir, nil
}

func moveManagedStackDir(sourceDir, targetDir string) error {
	if sourceDir == targetDir {
		return nil
	}

	if _, err := deps.stat(sourceDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("check managed stack directory %s: %w", sourceDir, err)
	}
	if _, err := deps.stat(targetDir); err == nil {
		return fmt.Errorf("managed stack directory %s already exists", targetDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check target managed stack directory %s: %w", targetDir, err)
	}

	if err := deps.mkdirAll(filepath.Dir(targetDir), 0o750); err != nil {
		return fmt.Errorf("create managed stack directory parent for %s: %w", targetDir, err)
	}
	if err := deps.rename(sourceDir, targetDir); err != nil {
		return fmt.Errorf("rename managed stack directory %s to %s: %w", sourceDir, targetDir, err)
	}

	return nil
}

func scaffoldManagedStackFiles(cmd *cobra.Command, cfg configpkg.Config, force bool) error {
	if !cfg.Stack.Managed {
		return nil
	}

	result, err := deps.scaffoldManagedStack(cfg, force)
	if err != nil {
		return err
	}
	if result.CreatedDir {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("created managed stack directory %s", result.StackDir)); err != nil {
			return err
		}
	}
	if result.WroteCompose {
		if err := output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("wrote managed compose file %s", result.ComposePath)); err != nil {
			return err
		}
	}
	if result.AlreadyPresent {
		return output.StatusLine(cmd.OutOrStdout(), output.StatusOK, fmt.Sprintf("managed stack already exists at %s", result.ComposePath))
	}

	return nil
}

func retargetStackConfig(cfg configpkg.Config, targetName string) configpkg.Config {
	cfg.Stack.Name = targetName
	if cfg.Stack.Managed {
		cfg.Stack.Dir = ""
		cfg.Services.PostgresContainer = ""
		cfg.Services.RedisContainer = ""
		cfg.Services.NATSContainer = ""
		cfg.Services.SeaweedFSContainer = ""
		cfg.Services.MeilisearchContainer = ""
		cfg.Services.PgAdminContainer = ""
		cfg.Services.Postgres.DataVolume = ""
		cfg.Services.Redis.DataVolume = ""
		cfg.Services.SeaweedFS.DataVolume = ""
		cfg.Services.Meilisearch.DataVolume = ""
		cfg.Services.PgAdmin.DataVolume = ""
	}
	cfg.ApplyDerivedFields()
	return cfg
}

func stackComposeFileExists(path string) bool {
	info, err := deps.stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
