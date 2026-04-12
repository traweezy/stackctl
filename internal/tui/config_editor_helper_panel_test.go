package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestConfigEditorAdditionalHelperCoverage(t *testing.T) {
	t.Run("delegate renders unselected fields and save plan covers manual compose changes", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("dev-stack")
		delegate := configListDelegate{}
		model := list.New([]list.Item{
			configListItem{kind: configListGroupRow, group: "Stack"},
			configListItem{kind: configListFieldRow, label: "Stack name", value: "dev-stack", spec: testConfigSpecByKey(t, "stack.name")},
		}, delegate, 40, 4)
		model.Select(0)

		var rendered bytes.Buffer
		delegate.Render(&rendered, model, 1, model.Items()[1])
		if plain := stripANSITest(rendered.String()); !strings.Contains(plain, "Stack name") || strings.Contains(plain, "▸ ") {
			t.Fatalf("unexpected unselected field render output: %q", plain)
		}

		editor := newConfigEditor()
		editor.draft = cfg
		editor.baseline = cfg
		editor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
		editor.draft.Setup.ScaffoldDefaultStack = false
		editor.draft.ApplyDerivedFields()

		plan := editor.savePlan()
		if plan.Allowed || plan.Reason != "save first, then handle this stack change manually" {
			t.Fatalf("unexpected save plan for manual compose changes: %+v", plan)
		}
		if got := editor.compactNextStepSummary(plan); got != "ctrl+s saves now, then you finish the stack change manually" {
			t.Fatalf("unexpected compact summary: %q", got)
		}
	})

	t.Run("diff and preview helpers cover unreadable sources and blank filtering", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("dev-stack")
		cfg.Connection.Host = "db.internal"
		cfg.ApplyDerivedFields()

		editor := newConfigEditor()
		editor.source = ConfigSourceUnavailable
		editor.sourceMessage = "load failed"
		editor.draft = cfg

		diffText, err := editor.diffText(false)
		if err != nil {
			t.Fatalf("diffText returned error: %v", err)
		}
		if !strings.Contains(diffText, "db.internal") {
			t.Fatalf("expected unreadable-source diff to include new config content, got %q", diffText)
		}
		if got := editor.workflowStatusLine(); !strings.Contains(got, "Draft replaces unreadable config") {
			t.Fatalf("unexpected workflow status line: %q", got)
		}

		message := scaffoldResultMessage(configpkg.ScaffoldResult{
			CreatedDir:          true,
			StackDir:            "/tmp/dev-stack",
			WroteNATSConfig:     true,
			NATSConfigPath:      "/tmp/dev-stack/nats.conf",
			WroteRedisACL:       true,
			RedisACLPath:        "/tmp/dev-stack/users.acl",
			WrotePgAdminServers: true,
			PgAdminServersPath:  "/tmp/dev-stack/pgadmin-servers.json",
			WrotePGPass:         true,
			PGPassPath:          "/tmp/dev-stack/pgpass",
		})
		for _, fragment := range []string{
			"created managed stack directory /tmp/dev-stack",
			"wrote managed nats config file /tmp/dev-stack/nats.conf",
			"wrote managed redis ACL file /tmp/dev-stack/users.acl",
			"wrote managed pgAdmin server bootstrap file /tmp/dev-stack/pgadmin-servers.json",
			"wrote managed pgpass file /tmp/dev-stack/pgpass",
		} {
			if !strings.Contains(message, fragment) {
				t.Fatalf("expected scaffold message to contain %q, got %q", fragment, message)
			}
		}

		if got := joinOperationSteps("saved config", "  ", "", "restart required"); got != "saved config  •  restart required" {
			t.Fatalf("unexpected joined operation steps: %q", got)
		}
	})

	t.Run("effect and runtime helpers cover remaining follow-up branches", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("dev-stack")
		cfg.Setup.IncludeCockpit = false
		cfg.ApplyDerivedFields()

		spec := testConfigSpecByKey(t, "setup.install_cockpit")
		if got := selectedFieldEditBlock(spec, cfg); got != "Turn on Cockpit helpers first to change install behavior." {
			t.Fatalf("unexpected install_cockpit edit block: %q", got)
		}

		cfg.Setup.IncludeCockpit = true
		cfg.ApplyDerivedFields()
		if got := specificFieldEffect(spec, cfg); !strings.Contains(got, "Controls whether setup and doctor fix install and enable Cockpit automatically.") {
			t.Fatalf("unexpected install_cockpit field effect: %q", got)
		}

		external := cfg
		external.Stack.Managed = false
		external.Setup.ScaffoldDefaultStack = false
		external.Stack.Dir = t.TempDir()
		external.Stack.ComposeFile = "compose.yaml"
		external.ApplyDerivedFields()

		scaffoldSpec := testConfigSpecByKey(t, "setup.scaffold_default_stack")
		if got := selectedFieldEditBlock(scaffoldSpec, external); got != "External stacks do not use the managed scaffold flow." {
			t.Fatalf("unexpected scaffold edit block: %q", got)
		}

		editor := newConfigEditor()
		editor.source = ConfigSourceLoaded
		editor.baseline = cfg
		editor.draft = cfg
		editor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
		editor.draft.ApplyDerivedFields()
		lines := editor.runtimeImpactLines()
		if !strings.Contains(strings.Join(lines, "\n"), "Saving refreshes the managed compose file for the next start.") {
			t.Fatalf("expected stopped-stack compose refresh message, got %+v", lines)
		}
	})

	t.Run("validators parsers and field setters cover blank and error cases", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("dev-stack")

		if got := wrapText("   ", 30); got != "" {
			t.Fatalf("expected blank text to stay blank after wrapping, got %q", got)
		}
		if err := validStackNameText(cfg, "   "); err == nil || !strings.Contains(err.Error(), "value must not be empty") {
			t.Fatalf("expected blank stack name to fail, got %v", err)
		}
		if err := validPortText(cfg, " "); err == nil || !strings.Contains(err.Error(), "port must not be empty") {
			t.Fatalf("expected blank port to fail, got %v", err)
		}
		if err := positiveIntText(cfg, " "); err == nil || !strings.Contains(err.Error(), "value must not be empty") {
			t.Fatalf("expected blank positive int to fail, got %v", err)
		}

		if major, minor, ok := parseImageVersionTag("redis:8.bad"); ok || major != 0 || minor != 0 {
			t.Fatalf("expected invalid minor image tag to fail, got (%d, %d, %v)", major, minor, ok)
		}
		if redisImageSupportsLRMPolicies("redis@sha256:deadbeef") {
			t.Fatal("expected digest-only image to skip LRM policy support")
		}

		nameSpec := testConfigSpecByKey(t, "stack.name")
		cfg.Stack.Managed = true
		if err := nameSpec.SetString(&cfg, "invalid name"); err == nil {
			t.Fatal("expected managed stack.name setter to reject invalid names")
		}

		managedSpec := testConfigSpecByKey(t, "stack.managed")
		cfg.Stack.Name = "invalid name"
		if err := managedSpec.SetBool(&cfg, true); err == nil {
			t.Fatal("expected managed stack toggle to reject invalid names")
		}

		dirSpec := testConfigSpecByKey(t, "stack.dir")
		originalWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		badWD := filepath.Join(t.TempDir(), "missing")
		if err := os.MkdirAll(badWD, 0o755); err != nil {
			t.Fatalf("mkdir bad cwd: %v", err)
		}
		if err := os.Chdir(badWD); err != nil {
			t.Fatalf("chdir bad cwd: %v", err)
		}
		if err := os.RemoveAll(badWD); err != nil {
			t.Fatalf("remove bad cwd: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(originalWD)
		})
		if err := dirSpec.SetString(&cfg, "relative/path"); err == nil || !strings.Contains(err.Error(), "resolve stack directory") {
			t.Fatalf("expected stack.dir setter to fail with a missing cwd, got %v", err)
		}

		durationSpec := testConfigSpecByKey(t, "services.postgres.log_min_duration_statement_ms")
		if err := durationSpec.SetString(&cfg, "zero"); err == nil {
			t.Fatal("expected postgres log duration setter to reject invalid values")
		}

		suggestions := packageManagerConfigSuggestions("  ")
		for _, value := range suggestions {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("did not expect blank package manager suggestion in %+v", suggestions)
			}
		}
	})
}

func TestConfigEditorWorkflowPanelAndPaletteHelpers(t *testing.T) {
	editor := newConfigEditor()
	editor.draft = configpkg.DefaultForStack("dev-stack")
	editor.width = 120
	editor.height = 28

	if got := stripANSITest(editor.workflowPanel(48)); !strings.Contains(got, "Keys") {
		t.Fatalf("expected workflow panel to include key summary, got %q", got)
	}
	if got := stripANSITest(editor.statusPanel(48)); !strings.Contains(got, "Status") {
		t.Fatalf("expected status panel to include status summary, got %q", got)
	}

	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	model.snapshot.Services = []Service{{Name: "custom", DisplayName: "Custom"}}
	model.selectedService = "custom"
	if cmd := model.openCopyPalette(); cmd == nil || model.banner == nil || model.banner.Message != "no copy targets are available for the selected service" {
		t.Fatalf("expected copy palette warning for services without copy targets, cmd=%v banner=%+v", cmd, model.banner)
	}

	if _, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: 'x', Text: "x"}); handled {
		t.Fatal("expected bare model without palette to ignore palette key handling")
	}
}
