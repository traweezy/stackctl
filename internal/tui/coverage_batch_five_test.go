package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/traweezy/stackctl/internal/output"
)

func TestActionCoverageBatchFive(t *testing.T) {
	snapshot := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
			{Name: "redis", DisplayName: "Redis", Status: "missing", ContainerName: "stack-redis"},
			{Name: "cockpit", DisplayName: "Cockpit", Status: "running", URL: "https://localhost:9090"},
			{Name: "pgadmin", DisplayName: "pgAdmin", Status: "running"},
		},
		Stacks: []StackProfile{
			{Name: "dev-stack", Current: true, State: "running", Configured: true},
		},
	}

	actions := availableActions(snapshot, snapshot.Services[0], true)
	labels := make([]string, 0, len(actions))
	for _, action := range actions {
		labels = append(labels, action.Label)
	}
	for _, want := range []string{"Restart Postgres", "Stop Postgres", "Start", "Restart", "Stop", "Doctor", "Open Cockpit"} {
		if !slicesContainsString(labels, want) {
			t.Fatalf("expected available actions to include %q, got %+v", want, labels)
		}
	}
	if slicesContainsString(labels, "Open pgAdmin") {
		t.Fatalf("did not expect pgAdmin action without a URL, got %+v", labels)
	}

	if actions := availableStackActions(StackProfile{Name: "staging", Configured: true}, false); actions != nil {
		t.Fatalf("expected no stack actions without a selection, got %+v", actions)
	}

	startupCases := []struct {
		action ActionID
		wait   bool
		want   bool
	}{
		{action: ActionStart, wait: true, want: true},
		{action: ActionID("restart-service:postgres"), wait: true, want: true},
		{action: ActionID("start-stack:staging"), wait: true, want: true},
		{action: ActionID("stop-service:postgres"), wait: true, want: false},
		{action: ActionRestart, wait: false, want: false},
	}
	for _, tc := range startupCases {
		if got := actionUsesStartupBudget(tc.action, tc.wait); got != tc.want {
			t.Fatalf("actionUsesStartupBudget(%q, %v) = %v, want %v", tc.action, tc.wait, got, tc.want)
		}
	}

	if got := renderActionRail(Model{}); got != "" {
		t.Fatalf("expected empty action rail without a runner, got %q", got)
	}
	if got := renderActionRail(Model{
		runner: func(ActionID) (ActionReport, error) { return ActionReport{}, nil },
		active: stacksSection,
	}); got != "" {
		t.Fatalf("expected empty action rail without available actions, got %q", got)
	}

	model := Model{}
	if cmd := model.cancelConfirmation(); cmd != nil {
		t.Fatalf("expected nil cancel command without confirmation, got %v", cmd)
	}

	model.confirmation = &confirmationState{Kind: confirmationConfigReset, Title: "   "}
	clearMsg := msgOfType[bannerClearMsg](t, model.cancelConfirmation())
	if clearMsg.id == 0 || model.banner == nil || model.banner.Message != "confirmation cancelled" {
		t.Fatalf("unexpected blank-title cancellation result: msg=%+v banner=%+v", clearMsg, model.banner)
	}

	base := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", PortListening: true, PortConflict: true},
		},
		Stacks: []StackProfile{{Name: "dev-stack", Current: true, State: "running"}},
	}
	previous := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", PortListening: true, PortConflict: true},
		},
		Stacks: []StackProfile{{Name: "dev-stack", Current: true, State: "running"}},
	}
	failed := Model{
		snapshot: base,
		history: []historyEntry{{
			ID:        7,
			Action:    "Restart",
			Status:    output.StatusRestart,
			Message:   "restarting stack...",
			StartedAt: time.Now(),
		}},
		runningAction: &runningAction{
			Action:   ActionSpec{ID: ActionRestart, Label: "Restart"},
			History:  7,
			Previous: previous,
		},
	}
	failed.snapshot.Services[0].Status = "restarting"
	if cmd := failed.completeAction(actionMsg{historyID: 99}); cmd != nil {
		t.Fatalf("expected mismatched history ids to be ignored, got %v", cmd)
	}
	cmd := failed.completeAction(actionMsg{
		historyID: 7,
		action:    ActionSpec{ID: ActionRestart, Label: "Restart"},
		report:    ActionReport{Status: output.StatusOK, Message: "restart complete", Details: []string{"detail"}},
		err:       errors.New("compose failed"),
	})
	if cmd == nil {
		t.Fatal("expected banner clear command for failed action completion")
	}
	if failed.runningAction != nil {
		t.Fatalf("expected running action to clear, got %+v", failed.runningAction)
	}
	if failed.history[0].Status != output.StatusFail || !strings.Contains(failed.history[0].Message, "restart failed: compose failed") {
		t.Fatalf("unexpected failed history entry: %+v", failed.history[0])
	}
	if failed.snapshot.Services[0].Status != "running" {
		t.Fatalf("expected failed action to restore the previous snapshot, got %+v", failed.snapshot.Services)
	}

	stoppedStack := applyOptimisticUpdate(base, ActionID("stop-stack:dev-stack"))
	if stoppedStack.Stacks[0].State != "stopping" || stoppedStack.Services[0].Status != "stopping" {
		t.Fatalf("expected stop-stack optimistic update, got stacks=%+v services=%+v", stoppedStack.Stacks, stoppedStack.Services)
	}
	stoppedAction := applyOptimisticUpdate(base, ActionStop)
	if stoppedAction.Stacks[0].State != "stopping" || stoppedAction.Services[0].Status != "stopping" {
		t.Fatalf("expected stop action optimistic update, got stacks=%+v services=%+v", stoppedAction.Stacks, stoppedAction.Services)
	}

	if got := (confirmationState{Kind: confirmationConfigReset}).confirmationMessage(); got != "" {
		t.Fatalf("expected empty confirmation message fallback, got %q", got)
	}
}

func TestInspectCoverageBatchFive(t *testing.T) {
	if got := statusChip("   ", output.StatusOK); got != "" {
		t.Fatalf("expected blank status chip label to return an empty string, got %q", got)
	}

	stackView := renderStacks(Snapshot{Stacks: []StackProfile{{Name: "   "}}}, "", expandedLayout, 120)
	if !strings.Contains(stackView, "No stack detail is available.") {
		t.Fatalf("expected missing stack-detail state, got:\n%s", stackView)
	}

	serviceView := renderServices(Snapshot{Services: []Service{{DisplayName: "!!!"}}}, false, expandedLayout, "", 120, nil)
	if !strings.Contains(serviceView, "No service detail is available.") {
		t.Fatalf("expected missing service-detail state, got:\n%s", serviceView)
	}

	healthView := renderHealth(Snapshot{Services: []Service{{DisplayName: "!!!"}}}, "", 120, nil)
	if !strings.Contains(healthView, "No health detail is available.") {
		t.Fatalf("expected missing health-detail state, got:\n%s", healthView)
	}

	label := stripANSITest(stackListLabel(StackProfile{Name: "   ", Current: true}))
	if !strings.Contains(label, "-") || !strings.Contains(strings.ToLower(label), "current") {
		t.Fatalf("expected blank stack label to fall back to '-' and include current status, got %q", label)
	}
}

func TestPaletteCoverageBatchFive(t *testing.T) {
	state := newPaletteState(paletteModeCommand, "Command palette", "Choose", []paletteAction{{Title: "Restart stack", Search: "restart stack"}})
	state.input.SetValue("zzz")
	state.applyFilter()
	panel := stripANSITest(renderPalettePanel(state, 80, 12))
	if !strings.Contains(panel, "No matching commands.") {
		t.Fatalf("expected empty palette copy, got:\n%s", panel)
	}

	targets := serviceCopyTargets(Service{
		Name:         "nats",
		DisplayName:  "NATS",
		Host:         "localhost",
		ExternalPort: 4222,
		Token:        "secret-token",
		Email:        "ops@example.com",
	}, false)
	wantTargets := map[copyTargetKind]bool{
		copyTargetEndpoint: true,
		copyTargetHostPort: true,
		copyTargetToken:    true,
		copyTargetEmail:    true,
	}
	for _, target := range targets {
		delete(wantTargets, target.Kind)
	}
	if len(wantTargets) != 0 {
		t.Fatalf("expected copy targets to include %+v, got %+v", wantTargets, targets)
	}

	recent := Model{
		history: []historyEntry{
			{
				ID:          1,
				Action:      "Section",
				Status:      output.StatusOK,
				Message:     "ignored",
				CompletedAt: time.Now(),
				Recent:      &paletteAction{Kind: paletteActionSection, Title: "Go to Services"},
			},
			{
				ID:          2,
				Action:      "Copy",
				Status:      output.StatusOK,
				Message:     "copied stackctl connect output to clipboard",
				CompletedAt: time.Now(),
				Recent:      &paletteAction{Kind: paletteActionCopyText, Title: "Copy stackctl connect output"},
			},
		},
	}.recentPaletteActions()
	if len(recent) != 1 || recent[0].Kind != paletteActionCopyText {
		t.Fatalf("expected empty-key recent actions to be skipped, got %+v", recent)
	}

	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	model.palette = newPaletteState(paletteModeCommand, "Command palette", "Choose", []paletteAction{{Title: "Restart stack"}})
	cmd, handled := model.handlePaletteKey(tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 'c'}))
	if !handled || cmd == nil {
		t.Fatalf("expected ctrl+c to be handled by the palette, handled=%v cmd=%v", handled, cmd)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected ctrl+c to emit tea.QuitMsg, got %T", cmd())
	}

	model.layout = expandedLayout
	if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleLayout}); cmd != nil || model.layout != compactLayout {
		t.Fatalf("expected toggle-layout action to switch to compact mode, layout=%v cmd=%v", model.layout, cmd)
	}

	postgres := Service{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", DSN: "postgres://app@localhost/app"}
	model.snapshot = Snapshot{Services: []Service{postgres}}
	model.selectedService = serviceKey(postgres)
	model.clipboardWriter = nil

	copyMsg := msgOfType[copyDoneMsg](t, model.startCopyAction(paletteAction{
		Kind:       paletteActionCopyValue,
		ServiceKey: serviceKey(postgres),
		CopyTarget: copyTargetDSN,
	}))
	if copyMsg.err != nil || !strings.Contains(copyMsg.message, "copied Postgres DSN to clipboard") {
		t.Fatalf("unexpected terminal copy result: %+v", copyMsg)
	}

	textMsg := msgOfType[copyDoneMsg](t, model.startCopyTextAction(paletteAction{
		Kind:      paletteActionCopyText,
		Title:     "Copy stackctl connect output",
		CopyValue: "stackctl connect postgres",
	}))
	if textMsg.err != nil || !strings.Contains(textMsg.message, "copied stackctl connect output to clipboard") {
		t.Fatalf("unexpected terminal copy-text result: %+v", textMsg)
	}
}

func slicesContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
