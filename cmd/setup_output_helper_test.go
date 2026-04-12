package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestFormatEnvGroupsAndWriteEnvGroups(t *testing.T) {
	groups := []envGroup{
		{
			Title: "stackctl",
			Entries: []envEntry{
				{Name: "STACKCTL_STACK", Value: "dev-stack"},
			},
		},
		{
			Title: "Postgres",
			Entries: []envEntry{
				{Name: "DATABASE_URL", Value: "postgres://app"},
			},
		},
	}

	rendered := formatEnvGroups(groups, true)
	for _, fragment := range []string{
		"# stackctl",
		"export STACKCTL_STACK='dev-stack'",
		"# Postgres",
		"export DATABASE_URL='postgres://app'",
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("expected formatted env groups to contain %q:\n%s", fragment, rendered)
		}
	}

	for _, failAfter := range []int{1, 2, 3} {
		writer := &failingWriteBuffer{failAfter: failAfter}
		if err := writeEnvGroups(writer, groups, true); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected writeEnvGroups to fail after write %d, got %v", failAfter, err)
		}
	}
}

func TestFormatPortMappingsAndPrintValidationIssues(t *testing.T) {
	mappings := []portMapping{
		{DisplayName: "Postgres", Host: "localhost", ExternalPort: 15432, InternalPort: 5432},
		{DisplayName: "Redis", Host: "localhost", ExternalPort: 16379, InternalPort: 6379},
	}

	rendered := formatPortMappings(mappings)
	for _, fragment := range []string{
		"SERVICE",
		"Postgres",
		"15432 -> 5432",
		"Redis",
		"16379 -> 6379",
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("expected formatted ports to contain %q:\n%s", fragment, rendered)
		}
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&failingWriteBuffer{failAfter: 1})
	issues := []configpkg.ValidationIssue{{Field: "stack.dir", Message: "must exist"}}
	if err := printValidationIssues(cmd, issues); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected printValidationIssues write failure, got %v", err)
	}
}

func TestPrintSetupNextStepsNonTerminalAndWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return false }
	})

	cfg := configpkg.Default()
	cfg.Stack.Managed = false

	t.Run("renders bullets", func(t *testing.T) {
		cmd := &cobra.Command{}
		var out strings.Builder
		cmd.SetOut(&out)

		err := printSetupNextSteps(cmd, cfg, []string{"podman", "skopeo"}, true, true, true)
		if err != nil {
			t.Fatalf("printSetupNextSteps returned error: %v", err)
		}

		text := out.String()
		for _, fragment := range []string{
			"Next steps:",
			"- run `stackctl setup --install` or install podman, skopeo manually first",
			"- run `podman machine init` and `podman machine start` before launching the stack",
			"- install cockpit manually on this platform if you want the Cockpit web UI",
			"- disable `setup.include_cockpit` and `setup.install_cockpit` on this host, or manage Cockpit separately outside stackctl",
			"- run `stackctl start` when the external stack is ready to be launched from this config",
		} {
			if !strings.Contains(text, fragment) {
				t.Fatalf("expected next steps output to contain %q:\n%s", fragment, text)
			}
		}
	})

	t.Run("propagates write failures", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})

		if err := printSetupNextSteps(cmd, cfg, nil, false, false, false); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected printSetupNextSteps write failure, got %v", err)
		}
	})
}
