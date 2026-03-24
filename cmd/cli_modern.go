package cmd

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

const (
	commandGroupLifecycle = "lifecycle"
	commandGroupInspect   = "inspect"
	commandGroupOperate   = "operate"
	commandGroupConfig    = "config"
	commandGroupUtility   = "utility"

	configGroupEdit      = "config-edit"
	configGroupInspect   = "config-inspect"
	configGroupMaintain  = "config-maintain"
	dbGroupAccess        = "db-access"
	dbGroupBackupRestore = "db-backup-restore"
	dbGroupMaintain      = "db-maintain"
)

func rootCommandGroups() []*cobra.Group {
	return []*cobra.Group{
		{ID: commandGroupLifecycle, Title: "Lifecycle Commands"},
		{ID: commandGroupInspect, Title: "Inspect Commands"},
		{ID: commandGroupOperate, Title: "Operate Commands"},
		{ID: commandGroupConfig, Title: "Setup & Config Commands"},
		{ID: commandGroupUtility, Title: "Utility Commands"},
	}
}

func configCommandGroups() []*cobra.Group {
	return []*cobra.Group{
		{ID: configGroupEdit, Title: "Edit Commands"},
		{ID: configGroupInspect, Title: "Inspect Commands"},
		{ID: configGroupMaintain, Title: "Maintenance Commands"},
	}
}

func dbCommandGroups() []*cobra.Group {
	return []*cobra.Group{
		{ID: dbGroupAccess, Title: "Access Commands"},
		{ID: dbGroupBackupRestore, Title: "Backup & Restore Commands"},
		{ID: dbGroupMaintain, Title: "Maintenance Commands"},
	}
}

func noArgsCommand(cmd *cobra.Command) *cobra.Command {
	cmd.Args = cobra.NoArgs
	cmd.ValidArgsFunction = cobra.NoFileCompletions
	return cmd
}

func noFileCompletion(cmd *cobra.Command) *cobra.Command {
	if cmd.ValidArgsFunction == nil {
		cmd.ValidArgsFunction = cobra.NoFileCompletions
	}
	return cmd
}

func mustRegisterFlagCompletion(cmd *cobra.Command, flagName string, fn cobra.CompletionFunc) {
	if err := cmd.RegisterFlagCompletionFunc(flagName, fn); err != nil {
		panic(err)
	}
}

func completeConfiguredStackNames(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	descriptions := map[string]string{
		configpkg.DefaultStackName: "default stack",
	}

	names := []string{configpkg.DefaultStackName}
	paths, err := deps.knownConfigPaths()
	if err == nil {
		for _, path := range paths {
			name := stackNameFromConfigPath(path)
			if name == "" || slices.Contains(names, name) {
				continue
			}
			names = append(names, name)
			descriptions[name] = "configured stack"
		}
	}

	sort.Strings(names)
	return filterCompletions(namesWithDescriptions(names, descriptions), toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeStackServiceArgs(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		if definition, ok := serviceDefinitionByAlias(arg); ok && definition.Kind == serviceKindStack {
			seen[definition.Key] = struct{}{}
		}
	}

	completions := make([]cobra.Completion, 0, len(serviceDefinitions()))
	for _, definition := range completionServiceDefinitions() {
		if definition.Kind != serviceKindStack {
			continue
		}
		if _, ok := seen[definition.Key]; ok {
			continue
		}

		description := definition.DisplayName
		if len(definition.Aliases) > 1 {
			description += " (aliases: " + strings.Join(definition.Aliases[1:], ", ") + ")"
		}
		completions = append(completions, cobra.CompletionWithDesc(definition.Key, description))
	}

	return filterCompletions(completions, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeExecArgs(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return completeStackServiceArgs(nil, args, toComplete)
}

func completeLogsServiceFlag(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions := make([]cobra.Completion, 0, len(serviceDefinitions()))
	for _, definition := range completionServiceDefinitions() {
		if definition.Kind != serviceKindStack {
			continue
		}

		description := definition.DisplayName
		if len(definition.Aliases) > 1 {
			description += " (aliases: " + strings.Join(definition.Aliases[1:], ", ") + ")"
		}
		completions = append(completions, cobra.CompletionWithDesc(definition.Key, description))
	}

	return filterCompletions(completions, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeServiceCopyTargets(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	completions := make([]cobra.Completion, 0, 12)
	for _, definition := range completionServiceDefinitions() {
		for _, target := range definition.CopyTargets() {
			completions = append(completions, cobra.CompletionWithDesc(target.PrimaryAlias, target.Label))
		}
	}

	sort.Slice(completions, func(i, j int) bool {
		return strings.Compare(string(completions[i]), string(completions[j])) < 0
	})
	return filterCompletions(completions, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeOpenTargets(_ *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	cfg, ok := loadCompletionConfig()
	if !ok {
		return filterCompletions([]cobra.Completion{
			cobra.CompletionWithDesc("cockpit", "open the Cockpit web UI"),
			cobra.CompletionWithDesc("pgadmin", "open the pgAdmin web UI"),
			cobra.CompletionWithDesc("all", "open every enabled web UI"),
		}, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	completions := make([]cobra.Completion, 0, 3)
	if cfg.CockpitEnabled() {
		completions = append(completions, cobra.CompletionWithDesc("cockpit", "open the Cockpit web UI"))
	}
	if cfg.PgAdminEnabled() {
		completions = append(completions, cobra.CompletionWithDesc("pgadmin", "open the pgAdmin web UI"))
	}
	completions = append(completions, cobra.CompletionWithDesc("all", "open every enabled web UI"))
	return filterCompletions(completions, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completionServiceDefinitions() []serviceDefinition {
	cfg, ok := loadCompletionConfig()
	if ok {
		return enabledServiceDefinitions(cfg)
	}

	return serviceDefinitions()
}

func loadCompletionConfig() (configpkg.Config, bool) {
	cfg, err := deps.loadConfig("")
	if err != nil {
		return configpkg.Config{}, false
	}
	return cfg, true
}

func stackNameFromConfigPath(path string) string {
	base := filepath.Base(path)
	if base == "config.yaml" {
		return configpkg.DefaultStackName
	}

	if filepath.Ext(base) != ".yaml" {
		return ""
	}

	return strings.TrimSuffix(base, ".yaml")
}

func namesWithDescriptions(names []string, descriptions map[string]string) []cobra.Completion {
	completions := make([]cobra.Completion, 0, len(names))
	for _, name := range names {
		description := descriptions[name]
		if description == "" {
			completions = append(completions, cobra.Completion(name))
			continue
		}
		completions = append(completions, cobra.CompletionWithDesc(name, description))
	}
	return completions
}

func filterCompletions(completions []cobra.Completion, toComplete string) []cobra.Completion {
	if toComplete == "" {
		return completions
	}

	filtered := make([]cobra.Completion, 0, len(completions))
	needle := strings.ToLower(toComplete)
	for _, completion := range completions {
		choice := string(completion)
		if tab := strings.IndexRune(choice, '\t'); tab >= 0 {
			choice = choice[:tab]
		}
		if strings.HasPrefix(strings.ToLower(choice), needle) {
			filtered = append(filtered, completion)
		}
	}
	return filtered
}
