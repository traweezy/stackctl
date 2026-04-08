package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestCompletionAndStackHelperBranches(t *testing.T) {
	t.Run("open target completion includes cockpit when it is the only enabled web UI", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeCockpit = true
			cfg.Setup.IncludeMeilisearch = false
			cfg.Setup.IncludePgAdmin = false
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		completions, directive := completeOpenTargets(nil, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("unexpected directive: %v", directive)
		}

		choices := completionChoices(completions)
		if !containsChoice(choices, "cockpit") {
			t.Fatalf("expected cockpit target in %v", choices)
		}
		if containsChoice(choices, "meilisearch") || containsChoice(choices, "pgadmin") {
			t.Fatalf("did not expect disabled web UI targets in %v", choices)
		}
	})

	t.Run("stack name parsing ignores non-yaml paths", func(t *testing.T) {
		if got := stackNameFromConfigPath("/tmp/stackctl/notes.txt"); got != "" {
			t.Fatalf("expected non-yaml config path to produce an empty stack name, got %q", got)
		}
	})

	t.Run("configured service summary returns empty when no stack services are enabled", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePostgres = false
		cfg.Setup.IncludeRedis = false
		cfg.Setup.IncludeNATS = false
		cfg.Setup.IncludeSeaweedFS = false
		cfg.Setup.IncludeMeilisearch = false
		cfg.Setup.IncludePgAdmin = false
		cfg.Setup.IncludeCockpit = false
		cfg.ApplyDerivedFields()

		if got := configuredStackServiceSummary(cfg); got != "" {
			t.Fatalf("expected empty stack service summary, got %q", got)
		}
	})

	t.Run("resolve stack target covers invalid args config path failures and successful loads", func(t *testing.T) {
		if _, err := resolveStackTarget("INVALID!"); err == nil {
			t.Fatal("expected invalid stack name to fail before path lookup")
		}

		withTestDeps(t, func(d *commandDeps) {
			d.configFilePathForStack = func(string) (string, error) {
				return "", errors.New("config path unavailable")
			}
		})
		if _, err := resolveStackTarget("staging"); err == nil || err.Error() != "config path unavailable" {
			t.Fatalf("expected config path failure, got %v", err)
		}

		cfg := configpkg.DefaultForStack("staging")
		withTestDeps(t, func(d *commandDeps) {
			d.configFilePathForStack = func(string) (string, error) {
				return "/tmp/stackctl/stacks/staging.yaml", nil
			}
			d.stat = func(string) (os.FileInfo, error) {
				return fakeFileInfo{name: "staging.yaml"}, nil
			}
			d.loadConfig = func(string) (configpkg.Config, error) {
				return cfg, nil
			}
		})

		target, err := resolveStackTarget("staging")
		if err != nil {
			t.Fatalf("expected successful stack target load, got %v", err)
		}
		if !target.Exists || target.Config.Stack.Name != "staging" || target.LoadErr != nil {
			t.Fatalf("unexpected resolved target: %+v", target)
		}
	})

	t.Run("service copy target surfaces resolver failures", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Connection.RedisACLUsername = ""
		cfg.Connection.RedisACLPassword = ""
		cfg.ApplyDerivedFields()

		_, _, err := serviceCopyTarget(cfg, "redis-username")
		if err == nil || err.Error() != "redis ACL auth is not enabled in this stack" {
			t.Fatalf("expected redis ACL resolver failure, got %v", err)
		}
	})
}
