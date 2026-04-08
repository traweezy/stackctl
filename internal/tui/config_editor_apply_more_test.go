package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func TestConfigEditorCommandHelpersCoverFailureAndRestartBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("saveConfigCmd reports failures and validation warnings", func(t *testing.T) {
		boom := errors.New("save boom")
		manager := configTestManager()
		manager.SaveConfig = func(string, configpkg.Config) error { return boom }

		msg := msgOfType[configOperationMsg](t, saveConfigCmd(&manager, "/tmp/stackctl/config.yaml", cfg, cfg, 0))
		if msg.Status != output.StatusFail || !errors.Is(msg.Err, boom) || !strings.Contains(msg.Message, "save config failed") {
			t.Fatalf("unexpected save failure message: %+v", msg)
		}

		manager = configTestManager()
		manager.ValidateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.name", Message: "warning"}}
		}

		next := cfg
		next.Connection.Host = "db.internal"
		msg = msgOfType[configOperationMsg](t, saveConfigCmd(&manager, "/tmp/stackctl/config.yaml", cfg, next, 0))
		if msg.Status != output.StatusWarn || !msg.Reload {
			t.Fatalf("expected validation warning save result, got %+v", msg)
		}
		if !strings.Contains(msg.Message, "with 1 validation issue") || !strings.Contains(msg.Message, "running services were not changed") {
			t.Fatalf("unexpected save warning message: %q", msg.Message)
		}
	})

	t.Run("scaffoldConfigCmd reports save failures scaffold failures and restart follow-up", func(t *testing.T) {
		boom := errors.New("save boom")
		manager := configTestManager()
		manager.SaveConfig = func(string, configpkg.Config) error { return boom }

		msg := msgOfType[configOperationMsg](t, scaffoldConfigCmd(&manager, "/tmp/stackctl/config.yaml", cfg, false, 0))
		if msg.Status != output.StatusFail || !errors.Is(msg.Err, boom) {
			t.Fatalf("unexpected scaffold save failure: %+v", msg)
		}

		manager = configTestManager()
		manager.ScaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{}, errors.New("scaffold boom")
		}

		msg = msgOfType[configOperationMsg](t, scaffoldConfigCmd(&manager, "/tmp/stackctl/config.yaml", cfg, false, 0))
		if msg.Status != output.StatusFail || !strings.Contains(msg.Message, "scaffold failed: scaffold boom") {
			t.Fatalf("unexpected scaffold failure message: %+v", msg)
		}

		manager = configTestManager()
		manager.ScaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{ComposePath: "/tmp/stackctl/compose.yaml"}, nil
		}

		msg = msgOfType[configOperationMsg](t, scaffoldConfigCmd(&manager, "/tmp/stackctl/config.yaml", cfg, true, 1))
		if msg.Status != output.StatusOK || !msg.Reload {
			t.Fatalf("unexpected scaffold success message: %+v", msg)
		}
		if !strings.Contains(msg.Message, "managed stack scaffold is up to date") || !strings.Contains(msg.Message, "restart the stack to apply updated compose changes") {
			t.Fatalf("unexpected scaffold success text: %q", msg.Message)
		}
	})

	t.Run("applyConfigCmd covers validation save scaffold and restart branches", func(t *testing.T) {
		manager := configTestManager()
		manager.ValidateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
			return []configpkg.ValidationIssue{{Field: "stack.name", Message: "warning"}}
		}

		msg := msgOfType[configOperationMsg](t, applyConfigCmd(&manager, nil, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{}))
		if msg.Status != output.StatusWarn || !strings.Contains(msg.Message, "apply blocked by 1 validation issue") {
			t.Fatalf("unexpected validation-blocked apply result: %+v", msg)
		}

		saveBoom := errors.New("save boom")
		manager = configTestManager()
		manager.SaveConfig = func(string, configpkg.Config) error { return saveBoom }
		msg = msgOfType[configOperationMsg](t, applyConfigCmd(&manager, nil, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{Allowed: true, Save: true}))
		if msg.Status != output.StatusFail || !errors.Is(msg.Err, saveBoom) {
			t.Fatalf("unexpected save failure apply result: %+v", msg)
		}

		manager = configTestManager()
		manager.ScaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{}, errors.New("scaffold boom")
		}
		msg = msgOfType[configOperationMsg](t, applyConfigCmd(&manager, nil, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{Allowed: true, Save: true, Scaffold: true}))
		if msg.Status != output.StatusFail || !msg.Reload || !strings.Contains(msg.Message, "scaffold failed: scaffold boom") {
			t.Fatalf("unexpected scaffold failure apply result: %+v", msg)
		}

		manager = configTestManager()
		msg = msgOfType[configOperationMsg](t, applyConfigCmd(&manager, nil, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{Allowed: true, Restart: true}))
		if msg.Status != output.StatusFail || !strings.Contains(msg.Message, "restart is unavailable in this model") {
			t.Fatalf("unexpected restart-unavailable apply result: %+v", msg)
		}

		restartBoom := errors.New("restart boom")
		msg = msgOfType[configOperationMsg](t, applyConfigCmd(&manager, func(ActionID) (ActionReport, error) {
			return ActionReport{}, restartBoom
		}, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{Allowed: true, Save: true, Restart: true}))
		if msg.Status != output.StatusFail || !msg.Reload || !strings.Contains(msg.Message, "restart failed: restart boom") {
			t.Fatalf("unexpected restart failure apply result: %+v", msg)
		}

		msg = msgOfType[configOperationMsg](t, applyConfigCmd(&manager, func(ActionID) (ActionReport, error) {
			return ActionReport{}, nil
		}, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{Allowed: true, Restart: true}))
		if msg.Status != output.StatusOK || !msg.Reload || !strings.Contains(msg.Message, "stack restarted") {
			t.Fatalf("unexpected restart success apply result: %+v", msg)
		}

		msg = msgOfType[configOperationMsg](t, applyConfigCmd(&manager, nil, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{Allowed: true}))
		if msg.Status != output.StatusInfo || msg.Message != "nothing new needed to be applied" {
			t.Fatalf("unexpected no-op apply result: %+v", msg)
		}
	})
}

func TestConfigEditorApplyPlanRuntimeImpactAndSummaryBranches(t *testing.T) {
	base := configpkg.Default()
	base.ApplyDerivedFields()

	t.Run("issues and scaffold problems block apply", func(t *testing.T) {
		editor := newConfigEditor()
		editor.baseline = base
		editor.draft = base
		editor.issues = []configpkg.ValidationIssue{{Field: "stack.name", Message: "bad"}}
		if got := editor.applyPlan().Reason; got != "fix validation issues before applying" {
			t.Fatalf("unexpected validation-block reason: %q", got)
		}

		editor.issues = nil
		editor.scaffoldProblem = "compose missing"
		if got := editor.applyPlan().Reason; got != "resolve the managed scaffold problem before applying" {
			t.Fatalf("unexpected scaffold-block reason: %q", got)
		}
	})

	t.Run("stack-target and compose-template changes explain follow-up", func(t *testing.T) {
		editor := newConfigEditor()
		editor.baseline = base
		editor.draft = base
		editor.draft.Stack.Dir = "/tmp/stackctl/other"
		editor.runningStack = 1

		plan := editor.applyPlan()
		if plan.Allowed || plan.Reason != "stack target changes are save-only" {
			t.Fatalf("unexpected stack-target plan: %+v", plan)
		}
		lines := strings.Join(editor.runtimeImpactLines(), "\n")
		if !strings.Contains(lines, "changes which stack future stackctl commands target") || !strings.Contains(lines, "manual follow-up") {
			t.Fatalf("unexpected stack-target runtime impact: %q", lines)
		}

		editor = newConfigEditor()
		editor.baseline = base
		editor.draft = base
		editor.draft.Ports.Postgres = 15432
		editor.runningStack = 1
		plan = editor.applyPlan()
		if !plan.Allowed || !plan.Save || !plan.Scaffold || !plan.ForceScaffold || !plan.Restart {
			t.Fatalf("unexpected managed compose apply plan: %+v", plan)
		}
		lines = strings.Join(editor.runtimeImpactLines(), "\n")
		if !strings.Contains(lines, "restarts the running stack automatically") {
			t.Fatalf("unexpected running managed impact text: %q", lines)
		}

		editor = newConfigEditor()
		manual := base
		manual.Setup.ScaffoldDefaultStack = false
		manual.ApplyDerivedFields()
		editor.baseline = manual
		editor.draft = manual
		editor.draft.Ports.Postgres = 15432
		plan = editor.applyPlan()
		if plan.Allowed || plan.Reason != "save first, then handle this stack change manually" {
			t.Fatalf("unexpected manual-compose plan: %+v", plan)
		}
		lines = strings.Join(editor.runtimeImpactLines(), "\n")
		if !strings.Contains(lines, "update the managed compose file yourself") || !strings.Contains(lines, "manual follow-up") {
			t.Fatalf("unexpected manual managed impact text: %q", lines)
		}
	})

	t.Run("runtime impact handles empty loaded missing and external drafts", func(t *testing.T) {
		editor := newConfigEditor()
		editor.baseline = base
		editor.draft = base
		editor.source = ConfigSourceLoaded
		if got := strings.Join(editor.runtimeImpactLines(), "\n"); !strings.Contains(got, "No runtime changes are pending right now.") {
			t.Fatalf("unexpected loaded no-change impact: %q", got)
		}

		editor.source = ConfigSourceMissing
		if got := strings.Join(editor.runtimeImpactLines(), "\n"); !strings.Contains(got, "Nothing is running yet.") {
			t.Fatalf("unexpected missing no-change impact: %q", got)
		}

		external := configpkg.DefaultForStack("external")
		external.Stack.Managed = false
		external.Setup.ScaffoldDefaultStack = false
		external.ApplyDerivedFields()

		editor = newConfigEditor()
		editor.baseline = external
		editor.draft = external
		editor.draft.Ports.Postgres = 15432
		got := strings.Join(editor.runtimeImpactLines(), "\n")
		if !strings.Contains(got, "compose file stays untouched") || !strings.Contains(got, "External compose services keep running") {
			t.Fatalf("unexpected external runtime impact: %q", got)
		}
	})

	t.Run("summaryStatus uses loaded missing and fallback copy", func(t *testing.T) {
		editor := newConfigEditor()
		editor.source = ConfigSourceLoaded
		if got := editor.summaryStatus(); got != "Loaded from disk." {
			t.Fatalf("unexpected loaded summary: %q", got)
		}

		editor.source = ConfigSourceMissing
		if got := editor.summaryStatus(); !strings.Contains(got, "No config file exists yet") {
			t.Fatalf("unexpected missing summary: %q", got)
		}

		editor.source = ConfigSourceState("")
		if got := editor.summaryStatus(); got != "The current config could not be loaded. Saving will replace it." {
			t.Fatalf("unexpected fallback summary: %q", got)
		}
	})
}

func TestPaletteAndActionHelperBranches(t *testing.T) {
	t.Run("palette helpers cover empty fuzzy and pagination branches", func(t *testing.T) {
		if score, ok := paletteMatchScore("", "postgres"); !ok || score != 0 {
			t.Fatalf("expected empty query match, got score=%d ok=%v", score, ok)
		}
		if score, ok := paletteMatchScore("pg", ""); ok || score != 0 {
			t.Fatalf("expected empty candidate miss, got score=%d ok=%v", score, ok)
		}
		if score, ok := paletteMatchScore("post", "postgres"); !ok || score <= 0 {
			t.Fatalf("expected substring match, got score=%d ok=%v", score, ok)
		}
		if score, ok := paletteMatchScore("pg", "postgres"); !ok || score <= 0 {
			t.Fatalf("expected fuzzy match, got score=%d ok=%v", score, ok)
		}
		if _, ok := paletteMatchScore("zz", "postgres"); ok {
			t.Fatal("expected fuzzy miss")
		}

		if panel := renderPalettePanel(nil, 80, 20); panel != "" {
			t.Fatalf("expected nil palette panel to be empty, got %q", panel)
		}

		state := newPaletteState(paletteModeCommand, "Command palette", "Choose an action", []paletteAction{
			{Title: "One"},
			{Title: "Two"},
			{Title: "Three"},
			{Title: "Four"},
			{Title: "Five"},
		})
		state.filtered = nil
		state.pageSize = 0
		state.syncPagination()
		if state.paginator.TotalPages != 0 || state.offset != 0 {
			t.Fatalf("expected empty pagination reset, got %+v", state.paginator)
		}

		state.filtered = append([]paletteAction(nil), state.items...)
		state.pageSize = 3
		state.syncPagination()
		state.selected = 3
		rendered := stripANSITest(renderPalettePanel(state, 72, 12))
		if !strings.Contains(rendered, "+ 1 more") || !strings.Contains(rendered, "pgup/pgdn page") {
			t.Fatalf("unexpected palette panel output: %q", rendered)
		}

		if _, ok := selectedServiceByKey(Snapshot{}, "   "); ok {
			t.Fatal("expected blank service key miss")
		}
		snapshot := Snapshot{Services: []Service{{Name: "postgres", DisplayName: "Postgres"}}}
		if service, ok := selectedServiceByKey(snapshot, serviceKey(snapshot.Services[0])); !ok || service.Name != "postgres" {
			t.Fatalf("expected keyed service selection, got service=%+v ok=%v", service, ok)
		}
		if _, ok := selectedServiceByKey(snapshot, "missing"); ok {
			t.Fatal("expected missing keyed service selection to fail")
		}

		if action := recentPaletteActionForActionSpec(ActionSpec{}); action != nil {
			t.Fatalf("expected empty action id to skip recents, got %+v", action)
		}
		if action := recentPaletteActionForActionSpec(ActionSpec{ID: ActionDoctor, Label: "Doctor", Group: "Sidebar"}); action == nil || action.Search != "doctor sidebar" {
			t.Fatalf("unexpected recent palette action: %+v", action)
		}
	})

	t.Run("action helpers cover lifecycle stop budget duration optimistic updates and confirmation text", func(t *testing.T) {
		if got := stackLifecycleActions(StackProfile{State: "running"}); len(got) != 2 {
			t.Fatalf("expected running lifecycle actions, got %+v", got)
		}
		if got := stackLifecycleActions(StackProfile{State: "stopped"}); len(got) != 1 {
			t.Fatalf("expected stopped lifecycle action, got %+v", got)
		}
		if got := stackLifecycleActions(StackProfile{State: "unknown"}); got != nil {
			t.Fatalf("expected unknown lifecycle state to return nil, got %+v", got)
		}

		if !actionUsesStopBudget(ActionStop) || !actionUsesStopBudget(ActionID(actionStopServicePrefix+"postgres")) || !actionUsesStopBudget(ActionID(actionStopStackPrefix+"dev")) {
			t.Fatal("expected stop actions to use the stop budget")
		}
		if actionUsesStopBudget(ActionRestart) {
			t.Fatal("did not expect restart to use the stop budget")
		}

		if got := historyDuration(historyEntry{}); got != "0s" {
			t.Fatalf("unexpected zero history duration: %q", got)
		}
		active := historyEntry{StartedAt: timeNow().Add(-2 * time.Second)}
		if got := historyDuration(active); got == "" || got == "0s" {
			t.Fatalf("expected active history duration, got %q", got)
		}

		snapshot := Snapshot{Services: []Service{{Name: "postgres", Status: "running", PortListening: true, PortConflict: true}}}
		optimisticServiceState(&snapshot, "postgres", "start")
		if snapshot.Services[0].Status != "starting" || snapshot.Services[0].PortConflict {
			t.Fatalf("unexpected optimistic start state: %+v", snapshot.Services[0])
		}
		optimisticServiceState(&snapshot, "postgres", "stop")
		if snapshot.Services[0].Status != "stopping" || snapshot.Services[0].PortListening || snapshot.Services[0].PortConflict {
			t.Fatalf("unexpected optimistic stop state: %+v", snapshot.Services[0])
		}
		optimisticServiceState(&snapshot, "postgres", "restart")
		if snapshot.Services[0].Status != "restarting" || snapshot.Services[0].PortListening || snapshot.Services[0].PortConflict {
			t.Fatalf("unexpected optimistic restart state: %+v", snapshot.Services[0])
		}
		before := snapshot.Services[0]
		optimisticServiceState(&snapshot, "postgres", "noop")
		if snapshot.Services[0] != before {
			t.Fatalf("expected unknown optimistic verb to leave service unchanged: before=%+v after=%+v", before, snapshot.Services[0])
		}

		if !currentStackProfile([]StackProfile{{Name: "dev", Current: true}}, "DEV") {
			t.Fatal("expected case-insensitive current stack match")
		}
		if currentStackProfile([]StackProfile{{Name: "dev", Current: false}}, "dev") {
			t.Fatal("expected non-current stack not to match")
		}

		if got := (confirmationState{Message: "custom"}).confirmationMessage(); got != "custom" {
			t.Fatalf("unexpected explicit confirmation message: %q", got)
		}
		if got := (confirmationState{Kind: confirmationAction, Action: ActionSpec{ConfirmMessage: "confirm restart"}}).confirmationMessage(); got != "confirm restart" {
			t.Fatalf("unexpected action confirmation message: %q", got)
		}
		if got := (confirmationState{}).confirmationMessage(); got != "" {
			t.Fatalf("unexpected empty confirmation message: %q", got)
		}
	})
}

func TestModelConfirmationAndConfigSelectionBranches(t *testing.T) {
	cfg := configpkg.Default()
	manager := configTestManager()
	model := newConfigTestModel(cfg, manager)
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	nextModel, cmd := current.handleConfirmation()
	if cmd != nil || nextModel.(Model).confirmation != nil {
		t.Fatalf("expected nil confirmation to no-op, model=%+v cmd=%v", nextModel, cmd)
	}

	current.confirmation = &confirmationState{Kind: confirmationConfigReset}
	current.configEditor.draft.Connection.Host = "db.internal"
	nextModel, cmd = current.handleConfirmation()
	current = nextModel.(Model)
	if current.configEditor.draft.Connection.Host != current.configEditor.baseline.Connection.Host {
		t.Fatalf("expected reset confirmation to restore the baseline draft, got %+v", current.configEditor.draft.Connection)
	}
	if cmd == nil || current.banner == nil || current.banner.Message != "config draft reset" {
		t.Fatalf("expected reset confirmation banner, cmd=%v banner=%+v", cmd, current.banner)
	}

	current.confirmation = &confirmationState{Kind: confirmationKind(99)}
	nextModel, cmd = current.handleConfirmation()
	if cmd != nil || nextModel.(Model).confirmation == nil {
		t.Fatalf("expected unknown confirmation to no-op without clearing state, model=%+v cmd=%v", nextModel, cmd)
	}

	current.confirmation = &confirmationState{Kind: confirmationAction, Action: ActionSpec{ID: ActionDoctor, Label: "Doctor"}}
	nextModel, cmd = current.handleConfirmation()
	current = nextModel.(Model)
	if cmd != nil || current.confirmation == nil || current.confirmation.Action.ID != ActionDoctor {
		t.Fatalf("expected action confirmation without runner to remain pending, model=%+v cmd=%v", current, cmd)
	}

	current.configEditor.selectedKey = "ports.postgres"
	current.configEditor.refreshList(false)
	if got := sidebarCompactSelectionLabel(current); got != "Postgres po…" {
		t.Fatalf("unexpected compact config selection label: %q", got)
	}
	lines := sidebarSelectionLines(current)
	if len(lines) != 2 || !strings.Contains(lines[0], "Postgres port") || !strings.Contains(lines[1], "Ports") {
		t.Fatalf("unexpected config sidebar selection lines: %+v", lines)
	}

	current.active = stacksSection
	current.snapshot.Stacks = []StackProfile{{Name: "dev-stack", Current: true, State: "running", Mode: "Managed"}}
	current.selectedStack = "dev-stack"
	lines = sidebarSelectionLines(current)
	if len(lines) != 4 || !strings.Contains(lines[3], "Managed") {
		t.Fatalf("unexpected stack sidebar selection lines: %+v", lines)
	}

	current.active = historySection
	current.history = nil
	lines = sidebarSelectionLines(current)
	if len(lines) != 1 || !strings.Contains(stripANSITest(lines[0]), "No session history yet") {
		t.Fatalf("unexpected empty-history selection lines: %+v", lines)
	}
}
