package tui

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestConfigApplyPlanPendingMessage(t *testing.T) {
	testCases := []struct {
		name string
		plan configApplyPlan
		want string
	}{
		{name: "restart", plan: configApplyPlan{Restart: true}, want: "Applying config changes"},
		{name: "scaffold", plan: configApplyPlan{Scaffold: true}, want: "Saving and scaffolding config"},
		{name: "save", plan: configApplyPlan{Save: true}, want: "Applying config changes"},
		{name: "check only", plan: configApplyPlan{}, want: "Checking config changes"},
	}

	for _, tc := range testCases {
		if got := tc.plan.pendingMessage(); got != tc.want {
			t.Fatalf("%s: pendingMessage() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestConfigListHelpersRenderAndFilter(t *testing.T) {
	group := configListItem{kind: configListGroupRow, group: "Connections"}
	if got := group.FilterValue(); got != "Connections" {
		t.Fatalf("unexpected group filter value: %q", got)
	}

	field := configListItem{
		kind:    configListFieldRow,
		group:   "Services",
		label:   "Redis policy",
		value:   "noeviction",
		warning: true,
		spec:    configFieldSpec{Key: "services.redis.maxmemory_policy"},
	}
	if got := field.FilterValue(); !strings.Contains(got, "services.redis.maxmemory_policy") || !strings.Contains(got, "Redis policy") {
		t.Fatalf("unexpected field filter value: %q", got)
	}

	delegate := configListDelegate{}
	model := list.New([]list.Item{group, field}, delegate, 40, 5)
	model.Select(1)

	var rendered bytes.Buffer
	delegate.Render(&rendered, model, 0, group)
	if plain := stripANSITest(rendered.String()); !strings.Contains(plain, "Connections") {
		t.Fatalf("expected group render to include group name, got %q", plain)
	}

	rendered.Reset()
	delegate.Render(&rendered, model, 1, field)
	plain := stripANSITest(rendered.String())
	if !strings.Contains(plain, "▸ ") || !strings.Contains(plain, "!Redis policy") || !strings.Contains(plain, "noeviction") {
		t.Fatalf("unexpected field render output: %q", plain)
	}

	if cmd := delegate.Update(tea.KeyPressMsg{}, &model); cmd != nil {
		t.Fatalf("expected list delegate update to be nil, got %v", cmd)
	}
}

func TestConfigEditorSelectionValidationAndEditingHelpers(t *testing.T) {
	editor := newConfigEditor()
	editor.width = 120
	editor.height = 36
	editor.draft = configpkg.DefaultForStack("dev-stack")
	editor.fieldList.SetItems([]list.Item{
		configListItem{kind: configListGroupRow, group: "Stack"},
		configListItem{kind: configListFieldRow, spec: configFieldSpec{Key: "stack.name"}},
	})

	editor.fieldList.Select(1)
	editor.syncSelectedKey()
	if editor.selectedKey != "stack.name" {
		t.Fatalf("expected selected key to track field rows, got %q", editor.selectedKey)
	}

	editor.fieldList.Select(0)
	editor.syncSelectedKey()
	if editor.selectedKey != "" {
		t.Fatalf("expected selected key to clear on group rows, got %q", editor.selectedKey)
	}

	manager := &ConfigManager{
		DefaultConfig: func() configpkg.Config { return configpkg.DefaultForStack("dev-stack") },
		ValidateConfig: func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.name", Message: "invalid"}}
		},
		ManagedStackNeedsScaffold: func(configpkg.Config) (bool, error) {
			return true, errors.New("scaffold unavailable")
		},
	}

	editor.draft = configpkg.DefaultForStack("dev-stack")
	editor.syncValidation(manager)
	if len(editor.issues) != 1 || len(editor.issueIndex["stack.name"]) != 1 {
		t.Fatalf("expected validation issues to be indexed, got %+v %+v", editor.issues, editor.issueIndex)
	}
	if !editor.needsScaffold || editor.scaffoldProblem != "scaffold unavailable" {
		t.Fatalf("expected scaffold state to be captured, got needsScaffold=%v scaffoldProblem=%q", editor.needsScaffold, editor.scaffoldProblem)
	}

	editor.syncValidation(nil)
	if len(editor.issues) != 0 || len(editor.issueIndex) != 0 || editor.needsScaffold || editor.scaffoldProblem != "" {
		t.Fatalf("expected nil manager to reset validation state, got %+v %+v needsScaffold=%v scaffoldProblem=%q", editor.issues, editor.issueIndex, editor.needsScaffold, editor.scaffoldProblem)
	}

	editor = newConfigEditor()
	editor.width = 120
	editor.draft = configpkg.DefaultForStack("dev-stack")
	editor.selectedKey = "setup.include_nats"
	if cmd := editor.beginEdit(false); cmd != nil {
		t.Fatalf("expected bool edits to complete immediately, got cmd %v", cmd)
	}
	if editor.draft.Setup.IncludeNATS {
		t.Fatalf("expected bool edit to toggle the selected value, got %+v", editor.draft.Setup)
	}

	editor.selectedKey = "stack.compose_file"
	if cmd := editor.beginEdit(false); cmd != nil || editor.editing {
		t.Fatalf("expected blocked edit to stay inactive, editing=%v cmd=%v", editor.editing, cmd)
	}

	editor.selectedKey = "system.package_manager"
	cmd := editor.beginEdit(false)
	if cmd == nil {
		t.Fatal("expected package manager edit to return a focus command")
	}
	if !editor.editing {
		t.Fatal("expected package manager field to enter editing mode")
	}
	if editor.input.Value() != editor.draft.System.PackageManager {
		t.Fatalf("expected input value %q, got %q", editor.draft.System.PackageManager, editor.input.Value())
	}
	if !editor.input.ShowSuggestions || len(editor.input.AvailableSuggestions()) == 0 {
		t.Fatal("expected package manager edit to expose suggestions")
	}
}

func TestConfigEditorSummaryFollowUpAndRuntimeImpactHelpers(t *testing.T) {
	editor := newConfigEditor()
	if got := editor.summaryStatus(); got != "No config file exists yet. Review the defaults and save when ready." {
		t.Fatalf("unexpected missing summary status: %q", got)
	}

	editor.source = ConfigSourceLoaded
	if got := editor.summaryStatus(); got != "Loaded from disk." {
		t.Fatalf("unexpected loaded summary status: %q", got)
	}

	editor.source = ConfigSourceUnavailable
	editor.sourceMessage = "custom message"
	if got := editor.summaryStatus(); got != "custom message" {
		t.Fatalf("unexpected custom summary status: %q", got)
	}

	previous := configpkg.DefaultForStack("dev-stack")
	next := previous
	next.Connection.Host = "db.internal"
	next.ApplyDerivedFields()

	if got := applyFollowUpMessage(previous, next, configApplyPlan{Restart: true, Save: true, RunningStack: 1}); got != "" {
		t.Fatalf("expected restart follow-up to stay empty, got %q", got)
	}
	if got := applyFollowUpMessage(previous, next, configApplyPlan{Scaffold: true, RunningStack: 1}); !strings.Contains(got, "restart the stack") {
		t.Fatalf("expected scaffold follow-up for running stack, got %q", got)
	}
	if got := applyFollowUpMessage(previous, next, configApplyPlan{Scaffold: true}); !strings.Contains(got, "ready for the next stack start") {
		t.Fatalf("expected scaffold follow-up for stopped stack, got %q", got)
	}
	if got := applyFollowUpMessage(previous, next, configApplyPlan{Save: true}); !strings.Contains(got, "running services were not changed") {
		t.Fatalf("expected local-only save follow-up, got %q", got)
	}

	renamed := previous
	renamed.Stack.Name = "dev-stack-ops"
	renamed.ApplyDerivedFields()
	if got := saveFollowUpMessage(previous, renamed, 0); !strings.Contains(got, "future stackctl commands use the new stack target") {
		t.Fatalf("expected stack-target follow-up, got %q", got)
	}

	managedCompose := previous
	managedCompose.Services.Postgres.Image = "docker.io/library/postgres:18"
	managedCompose.ApplyDerivedFields()
	if got := saveFollowUpMessage(previous, managedCompose, 1); !strings.Contains(got, "compose refresh and restart") {
		t.Fatalf("expected managed compose follow-up, got %q", got)
	}

	editor = newConfigEditor()
	editor.source = ConfigSourceLoaded
	editor.baseline = previous
	editor.draft = previous
	lines := editor.runtimeImpactLines()
	if !slices.Contains(lines, "No runtime changes are pending right now.") {
		t.Fatalf("expected unchanged runtime impact, got %+v", lines)
	}

	editor.draft = managedCompose
	editor.runningStack = 2
	lines = editor.runtimeImpactLines()
	if !slices.Contains(lines, "Saving refreshes the managed compose file and restarts the running stack automatically.") {
		t.Fatalf("expected managed runtime impact, got %+v", lines)
	}

	external := configpkg.DefaultForStack("dev-stack")
	external.Stack.Managed = false
	external.Setup.ScaffoldDefaultStack = false
	external.Stack.Dir = "/tmp/dev-stack"
	external.Stack.ComposeFile = "compose.yml"
	external.ApplyDerivedFields()

	editor.baseline = external
	editor.draft = external
	editor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
	editor.runningStack = 0
	lines = editor.runtimeImpactLines()
	if !slices.Contains(lines, "This only updates stackctl metadata and helper commands. Your compose file stays untouched.") {
		t.Fatalf("expected external metadata-only impact, got %+v", lines)
	}
	if !slices.Contains(lines, "External compose services keep running until you change them yourself.") {
		t.Fatalf("expected external compose follow-up, got %+v", lines)
	}
}

func TestConfigEditorValidatorsAndSuggestions(t *testing.T) {
	testCases := []struct {
		value string
		want  int
		ok    bool
	}{
		{value: "-1", want: -1, ok: true},
		{value: "25", want: 25, ok: true},
		{value: " 90 ", want: 90, ok: true},
		{value: "", ok: false},
		{value: "0", ok: false},
		{value: "abc", ok: false},
	}

	for _, tc := range testCases {
		got, err := parsePostgresLogDurationSetting(tc.value)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("parsePostgresLogDurationSetting(%q) = (%d, %v), want (%d, nil)", tc.value, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("expected parsePostgresLogDurationSetting(%q) to fail", tc.value)
		}
	}

	if err := validPostgresLogDurationSettingText(configpkg.Config{}, "-1"); err != nil {
		t.Fatalf("expected postgres validator to accept -1, got %v", err)
	}
	if err := validPostgresLogDurationSettingText(configpkg.Config{}, "0"); err == nil {
		t.Fatal("expected postgres validator to reject zero")
	}

	validateLength := minLengthText(4)
	if err := validateLength(configpkg.Config{}, "abc"); err == nil {
		t.Fatal("expected minLengthText to reject short values")
	}
	if err := validateLength(configpkg.Config{}, "  abcd  "); err != nil {
		t.Fatalf("expected minLengthText to accept trimmed values, got %v", err)
	}

	cfg := configpkg.Config{}
	if err := stringSetter(func(cfg *configpkg.Config) *string { return &cfg.System.PackageManager })(&cfg, "  brew  "); err != nil {
		t.Fatalf("stringSetter returned error: %v", err)
	}
	if cfg.System.PackageManager != "brew" {
		t.Fatalf("expected stringSetter to trim values, got %q", cfg.System.PackageManager)
	}

	suggestions := packageManagerConfigSuggestions("DNF")
	if !slices.Contains(suggestions, "dnf") || !slices.Contains(suggestions, "apt") || !slices.Contains(suggestions, "brew") {
		t.Fatalf("expected package manager suggestions to include common values, got %+v", suggestions)
	}
	if count := strings.Count(strings.Join(suggestions, ","), "dnf"); count != 1 {
		t.Fatalf("expected package manager suggestions to de-duplicate values, got %+v", suggestions)
	}
}
