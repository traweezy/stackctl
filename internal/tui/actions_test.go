package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/traweezy/stackctl/internal/output"
)

func TestActionSpecsAndTargetsCoverServiceAndStackVariants(t *testing.T) {
	service := Service{Name: "postgres", DisplayName: "Postgres"}
	spec := actionStartServiceSpec(service)
	if spec.ID != ActionID("start-service:postgres") {
		t.Fatalf("unexpected service action id: %q", spec.ID)
	}
	if spec.Label != "Start Postgres" || spec.Group != "Service" {
		t.Fatalf("unexpected service action spec: %+v", spec)
	}
	if spec.PendingMessage != "starting postgres..." || spec.PendingStatus != output.StatusStart || spec.DefaultStatus != output.StatusOK {
		t.Fatalf("unexpected service action spec details: %+v", spec)
	}

	serviceCases := []struct {
		action ActionID
		verb   string
		target string
		ok     bool
	}{
		{ActionID("start-service:postgres"), "start", "postgres", true},
		{ActionID("stop-service:redis"), "stop", "redis", true},
		{ActionID("restart-service:nats"), "restart", "nats", true},
		{ActionID("doctor"), "", "", false},
	}
	for _, tc := range serviceCases {
		verb, target, ok := ServiceActionTarget(tc.action)
		if verb != tc.verb || target != tc.target || ok != tc.ok {
			t.Fatalf("ServiceActionTarget(%q) = (%q, %q, %v)", tc.action, verb, target, ok)
		}
	}

	stackCases := []struct {
		action ActionID
		verb   string
		target string
		ok     bool
	}{
		{ActionID("use-stack:staging"), "use", "staging", true},
		{ActionID("delete-stack:staging"), "delete", "staging", true},
		{ActionID("start-stack:staging"), "start", "staging", true},
		{ActionID("stop-stack:staging"), "stop", "staging", true},
		{ActionID("restart-stack:staging"), "restart", "staging", true},
		{ActionID("restart-service:postgres"), "", "", false},
	}
	for _, tc := range stackCases {
		verb, target, ok := StackActionTarget(tc.action)
		if verb != tc.verb || target != tc.target || ok != tc.ok {
			t.Fatalf("StackActionTarget(%q) = (%q, %q, %v)", tc.action, verb, target, ok)
		}
	}
}

func TestConfirmationHelpersAndHistoryRendering(t *testing.T) {
	action := ActionSpec{Label: "Restart", ConfirmMessage: "Restart the stack now?"}
	actionConfirmation := newActionConfirmation(action)
	if actionConfirmation.confirmationLabel() != "Restart" {
		t.Fatalf("unexpected action confirmation label: %+v", actionConfirmation)
	}
	if actionConfirmation.confirmationMessage() != "Restart the stack now?" {
		t.Fatalf("unexpected action confirmation message: %+v", actionConfirmation)
	}
	if got := (Model{confirmation: actionConfirmation}).confirmationSidebarLabel(); got != "Confirm Restart" {
		t.Fatalf("unexpected action confirmation sidebar label: %q", got)
	}

	resetConfirmation := newConfigResetConfirmation()
	if resetConfirmation.confirmationLabel() != "Reset draft" {
		t.Fatalf("unexpected reset confirmation label: %+v", resetConfirmation)
	}
	if !strings.Contains(resetConfirmation.confirmationMessage(), "Discard the unsaved config changes") {
		t.Fatalf("unexpected reset confirmation message: %q", resetConfirmation.confirmationMessage())
	}
	if got := (Model{confirmation: resetConfirmation}).confirmationSidebarLabel(); got != "Confirm Reset draft" {
		t.Fatalf("unexpected reset confirmation sidebar label: %q", got)
	}
	if got := (Model{}).confirmationSidebarLabel(); got != "" {
		t.Fatalf("expected empty sidebar label without confirmation, got %q", got)
	}

	plainConfirmation := stripANSITest(renderConfirmation(actionConfirmation))
	for _, fragment := range []string{
		"Confirm action",
		"Restart",
		"Restart the stack now?",
		"y / enter confirm  •  n / esc cancel",
	} {
		if !strings.Contains(plainConfirmation, fragment) {
			t.Fatalf("expected confirmation view to contain %q:\n%s", fragment, plainConfirmation)
		}
	}
	if renderConfirmation(nil) != "" {
		t.Fatal("expected nil confirmation to render as an empty string")
	}

	startedAt := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(42 * time.Second)
	history := []historyEntry{
		{
			ID:          1,
			Action:      "Restart",
			Status:      output.StatusWarn,
			Message:     "restart finished",
			Details:     []string{"Wait for services: off"},
			StartedAt:   startedAt,
			CompletedAt: completedAt,
		},
		{
			ID:          2,
			Action:      "Doctor",
			Status:      output.StatusFail,
			Message:     "doctor still found issues",
			StartedAt:   startedAt.Add(-time.Minute),
			CompletedAt: completedAt.Add(time.Minute),
		},
	}

	plainHistory := stripANSITest(renderHistory(history))
	for _, fragment := range []string{
		"History",
		"Status: completed with warnings",
		"Status: failed",
		"When: 2026-04-04 12:00:42",
		"Duration: 42s",
		"Wait for services: off",
	} {
		if !strings.Contains(plainHistory, fragment) {
			t.Fatalf("expected history view to contain %q:\n%s", fragment, plainHistory)
		}
	}
	if got := stripANSITest(renderHistory(nil)); !strings.Contains(got, "No actions have run in this session yet.") {
		t.Fatalf("unexpected empty history view:\n%s", got)
	}

	if got := historyStatusLabel(historyEntry{}); got != "in progress" {
		t.Fatalf("unexpected in-progress history label: %q", got)
	}
	if got := historyStatusLabel(historyEntry{Status: "custom", CompletedAt: completedAt}); got != "custom" {
		t.Fatalf("unexpected custom history label: %q", got)
	}
	if got := historyTimestamp(history[0]); got != "2026-04-04 12:00:42" {
		t.Fatalf("unexpected history timestamp: %q", got)
	}
	if got := historyDuration(history[0]); got != "42s" {
		t.Fatalf("unexpected history duration: %q", got)
	}
	if got := historyDuration(historyEntry{}); got != "0s" {
		t.Fatalf("unexpected zero history duration: %q", got)
	}
}

func TestCancelConfirmationHandlesConfigReset(t *testing.T) {
	model := Model{confirmation: newConfigResetConfirmation()}

	cmd := model.cancelConfirmation()
	if model.confirmation != nil {
		t.Fatalf("expected confirmation to clear after cancellation")
	}
	if model.banner == nil {
		t.Fatal("expected cancellation banner")
	}
	if model.banner.Status != output.StatusWarn || model.banner.Message != "reset draft cancelled" {
		t.Fatalf("unexpected cancellation banner: %+v", model.banner)
	}
	if cmd == nil {
		t.Fatal("expected transient banner clear command")
	}
	if len(model.history) != 0 {
		t.Fatalf("expected config reset cancellation not to add history entries: %+v", model.history)
	}
}

func TestApplyOptimisticUpdateCoversStackAndServiceTransitions(t *testing.T) {
	base := Snapshot{
		Services: []Service{
			{
				Name:          "postgres",
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				PortListening: true,
				PortConflict:  true,
			},
			{
				Name:          "redis",
				DisplayName:   "Redis",
				Status:        "running",
				ContainerName: "stack-redis",
				PortListening: true,
				PortConflict:  true,
			},
		},
		Stacks: []StackProfile{
			{Name: "dev-stack", Current: true, State: "running"},
			{Name: "staging", Current: false, State: "stopped"},
		},
	}

	restartedService := applyOptimisticUpdate(base, ActionID("restart-service:postgres"))
	if restartedService.Services[0].Status != "restarting" || restartedService.Services[0].PortListening || restartedService.Services[0].PortConflict {
		t.Fatalf("unexpected restarted service state: %+v", restartedService.Services[0])
	}
	if restartedService.Services[1].Status != "running" || !restartedService.Services[1].PortListening || !restartedService.Services[1].PortConflict {
		t.Fatalf("expected unrelated service to stay unchanged: %+v", restartedService.Services[1])
	}

	restartedStack := applyOptimisticUpdate(base, ActionRestart)
	if restartedStack.Stacks[0].State != "restarting" {
		t.Fatalf("expected current stack to restart optimistically: %+v", restartedStack.Stacks)
	}
	for _, service := range restartedStack.Services {
		if service.Status != "restarting" || service.PortListening || service.PortConflict {
			t.Fatalf("expected stack services to restart optimistically: %+v", restartedStack.Services)
		}
	}

	startedOtherStack := applyOptimisticUpdate(base, ActionID("start-stack:staging"))
	if startedOtherStack.Stacks[1].State != "starting" {
		t.Fatalf("expected target stack to enter starting state: %+v", startedOtherStack.Stacks)
	}
	if startedOtherStack.Services[0].Status != "running" || !startedOtherStack.Services[0].PortListening || !startedOtherStack.Services[0].PortConflict {
		t.Fatalf("expected non-current stack start to leave current services untouched: %+v", startedOtherStack.Services)
	}
}
