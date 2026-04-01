package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestRootHelpGroupsCommands(t *testing.T) {
	stdout, _, err := executeRoot(t, "--help")
	if err != nil {
		t.Fatalf("root help returned error: %v", err)
	}

	for _, fragment := range []string{
		"Lifecycle Commands",
		"Inspect Commands",
		"Operate Commands",
		"Setup & Config Commands",
		"Utility Commands",
		"env",
		"completion",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("root help missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestConfigHelpGroupsCommands(t *testing.T) {
	stdout, _, err := executeRoot(t, "config", "--help")
	if err != nil {
		t.Fatalf("config help returned error: %v", err)
	}

	for _, fragment := range []string{
		"Edit Commands",
		"Inspect Commands",
		"Maintenance Commands",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("config help missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestDBHelpGroupsCommands(t *testing.T) {
	stdout, _, err := executeRoot(t, "db", "--help")
	if err != nil {
		t.Fatalf("db help returned error: %v", err)
	}

	for _, fragment := range []string{
		"Access Commands",
		"Backup & Restore Commands",
		"Maintenance Commands",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("db help missing %q:\n%s", fragment, stdout)
		}
	}
}

func TestCompleteConfiguredStackNamesIncludesKnownStacks(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.knownConfigPaths = func() ([]string, error) {
			return []string{
				"/tmp/stackctl/config.yaml",
				"/tmp/stackctl/stacks/staging.yaml",
				"/tmp/stackctl/stacks/demo.yaml",
			}, nil
		}
	})

	completions, directive := completeConfiguredStackNames(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected directive: %v", directive)
	}

	choices := completionChoices(completions)
	for _, expected := range []string{"demo", configpkg.DefaultStackName, "staging"} {
		if !strings.Contains(strings.Join(choices, ","), expected) {
			t.Fatalf("expected stack completion %q in %v", expected, choices)
		}
	}
}

func TestCompleteStackServiceArgsUsesEnabledServicesFromConfig(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludeMeilisearch = true
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	completions, _ := completeStackServiceArgs(nil, []string{"postgres"}, "")
	choices := completionChoices(completions)

	if containsChoice(choices, "postgres") {
		t.Fatalf("did not expect already-selected service in completions: %v", choices)
	}
	for _, expected := range []string{"redis", "nats", "seaweedfs", "meilisearch"} {
		if !containsChoice(choices, expected) {
			t.Fatalf("expected service completion %q in %v", expected, choices)
		}
	}
	if containsChoice(choices, "pgadmin") {
		t.Fatalf("did not expect disabled pgadmin in completions: %v", choices)
	}
}

func TestCompleteServiceCopyTargetsUsesEnabledServicesFromConfig(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludeMeilisearch = true
		cfg.Setup.IncludePgAdmin = false
		cfg.Setup.IncludeCockpit = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	completions, _ := completeServiceCopyTargets(nil, nil, "")
	choices := completionChoices(completions)

	if containsChoice(choices, "pgadmin") || containsChoice(choices, "cockpit") {
		t.Fatalf("did not expect disabled web UI targets in completions: %v", choices)
	}
	for _, expected := range []string{"postgres", "redis", "nats", "seaweedfs", "meilisearch", "meilisearch-api-key", "seaweedfs-access-key", "seaweedfs-secret-key"} {
		if !containsChoice(choices, expected) {
			t.Fatalf("expected copy target %q in %v", expected, choices)
		}
	}
}

func TestCompleteEnvArgsUsesEnabledServicesFromConfig(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludeMeilisearch = true
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	completions, _ := completeEnvArgs(nil, []string{"postgres"}, "")
	choices := completionChoices(completions)

	if containsChoice(choices, "postgres") {
		t.Fatalf("did not expect already-selected env target in completions: %v", choices)
	}
	for _, expected := range []string{"redis", "nats", "seaweedfs", "meilisearch", "cockpit"} {
		if !containsChoice(choices, expected) {
			t.Fatalf("expected env target %q in %v", expected, choices)
		}
	}
	if containsChoice(choices, "pgadmin") {
		t.Fatalf("did not expect disabled pgadmin in completions: %v", choices)
	}
}

func TestCompleteOpenTargetsUsesEnabledWebUIs(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeCockpit = false
		cfg.Setup.IncludeMeilisearch = true
		cfg.Setup.IncludePgAdmin = true
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	completions, _ := completeOpenTargets(nil, nil, "")
	choices := completionChoices(completions)

	if containsChoice(choices, "cockpit") {
		t.Fatalf("did not expect cockpit target in %v", choices)
	}
	for _, expected := range []string{"meilisearch", "pgadmin", "all"} {
		if !containsChoice(choices, expected) {
			t.Fatalf("expected open target %q in %v", expected, choices)
		}
	}
}

func TestMustRegisterFlagCompletionPanicsForUnknownFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "stackctl"}

	defer func() {
		if recover() == nil {
			t.Fatal("expected mustRegisterFlagCompletion to panic for an unknown flag")
		}
	}()

	mustRegisterFlagCompletion(cmd, "missing", cobra.NoFileCompletions)
}

func TestCompleteExecArgsStopsAfterFirstService(t *testing.T) {
	completions, directive := completeExecArgs(nil, []string{"postgres"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected directive: %v", directive)
	}
	if len(completions) != 0 {
		t.Fatalf("expected no completions after the service arg, got %+v", completions)
	}
}

func TestCompleteRunArgsStopsCompletingAfterDash(t *testing.T) {
	cmd := &cobra.Command{
		Use: "run",
		Run: func(*cobra.Command, []string) {},
	}
	cmd.SetArgs([]string{"postgres", "--", "echo", "hi"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	completions, directive := completeRunArgs(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveDefault {
		t.Fatalf("unexpected directive: %v", directive)
	}
	if len(completions) != 0 {
		t.Fatalf("expected no completions after --, got %+v", completions)
	}
}

func TestCompleteLogsServiceFlagUsesEnabledStackServices(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePgAdmin = false
		cfg.Setup.IncludeSeaweedFS = true
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	completions, directive := completeLogsServiceFlag(nil, nil, "s")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected directive: %v", directive)
	}

	choices := completionChoices(completions)
	if !containsChoice(choices, "seaweedfs") {
		t.Fatalf("expected stack service completion in %v", choices)
	}
	if containsChoice(choices, "pgadmin") {
		t.Fatalf("did not expect disabled pgadmin in %v", choices)
	}
}

func TestFilterCompletionsMatchesCaseInsensitivePrefixes(t *testing.T) {
	filtered := filterCompletions([]cobra.Completion{
		cobra.Completion("postgres"),
		cobra.CompletionWithDesc("Redis", "cache"),
		cobra.Completion("nats"),
	}, "re")

	choices := completionChoices(filtered)
	if len(choices) != 1 || choices[0] != "Redis" {
		t.Fatalf("unexpected filtered completions: %+v", choices)
	}
}

func completionChoices(values []cobra.Completion) []string {
	choices := make([]string, 0, len(values))
	for _, value := range values {
		choice := value
		if tab := strings.IndexRune(choice, '\t'); tab >= 0 {
			choice = choice[:tab]
		}
		choices = append(choices, choice)
	}
	return choices
}

func containsChoice(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
