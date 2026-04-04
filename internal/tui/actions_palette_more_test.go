package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestActionHelpersCoverIndexesLifecycleAndDeleteMessages(t *testing.T) {
	cases := []struct {
		key  string
		want int
		ok   bool
	}{
		{key: "1", want: 0, ok: true},
		{key: "9", want: 8, ok: true},
		{key: "0", want: 0, ok: false},
		{key: "10", want: 0, ok: false},
		{key: "x", want: 0, ok: false},
	}
	for _, tc := range cases {
		if got, ok := actionIndex(tc.key); got != tc.want || ok != tc.ok {
			t.Fatalf("actionIndex(%q) = (%d, %v), want (%d, %v)", tc.key, got, ok, tc.want, tc.ok)
		}
	}

	running := selectedServiceActions(Service{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"})
	if len(running) != 2 || running[0].Label != "Restart Postgres" || running[1].Label != "Stop Postgres" {
		t.Fatalf("unexpected running service actions: %+v", running)
	}

	stopped := selectedServiceActions(Service{Name: "postgres", DisplayName: "Postgres", Status: "missing", ContainerName: "stack-postgres"})
	if len(stopped) != 1 || stopped[0].Label != "Start Postgres" {
		t.Fatalf("unexpected stopped service actions: %+v", stopped)
	}

	managedDelete := actionDeleteStackSpec(StackProfile{Name: "staging", Mode: "managed"})
	if !strings.Contains(managedDelete.ConfirmMessage, "stackctl-managed local data") {
		t.Fatalf("unexpected managed delete message: %+v", managedDelete)
	}

	currentDelete := actionDeleteStackSpec(StackProfile{Name: "staging", Current: true})
	if !strings.Contains(currentDelete.ConfirmMessage, "fall back to dev-stack") {
		t.Fatalf("unexpected current delete message: %+v", currentDelete)
	}

	configuredDelete := actionDeleteStackSpec(StackProfile{Name: "staging"})
	if !strings.Contains(configuredDelete.ConfirmMessage, "saved stack profile") {
		t.Fatalf("unexpected configured delete message: %+v", configuredDelete)
	}

	if !lifecycleAction(ActionStart) {
		t.Fatal("expected ActionStart to be a lifecycle action")
	}
	if !lifecycleAction(ActionID("restart-service:postgres")) {
		t.Fatal("expected prefixed service lifecycle action to be detected")
	}
	if lifecycleAction(ActionDoctor) {
		t.Fatal("did not expect ActionDoctor to be a lifecycle action")
	}

	confirmation := confirmationState{
		Kind:   confirmationAction,
		Action: ActionSpec{Label: "Restart", ConfirmMessage: "Restart now?"},
	}
	if got := confirmation.confirmationLabel(); got != "Restart" {
		t.Fatalf("unexpected confirmation label %q", got)
	}
	if got := confirmation.confirmationMessage(); got != "Restart now?" {
		t.Fatalf("unexpected fallback confirmation message %q", got)
	}
}

func TestOptimisticLifecycleHelpersCoverServiceAndHistoryBranches(t *testing.T) {
	startingSnapshot := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", PortListening: true, PortConflict: true},
			{Name: "cockpit", DisplayName: "Cockpit", Status: "running", URL: "https://localhost:9090"},
		},
	}
	applyOptimisticServiceLifecycle(&startingSnapshot, "start")
	if startingSnapshot.Services[0].Status != "starting" || startingSnapshot.Services[0].PortConflict {
		t.Fatalf("expected start lifecycle to clear conflicts: %+v", startingSnapshot.Services[0])
	}
	if startingSnapshot.Services[1].Status != "running" {
		t.Fatalf("expected host tools to stay unchanged: %+v", startingSnapshot.Services[1])
	}

	stoppingSnapshot := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", PortListening: true, PortConflict: true},
		},
	}
	applyOptimisticServiceLifecycle(&stoppingSnapshot, "stop")
	if stoppingSnapshot.Services[0].Status != "stopping" || stoppingSnapshot.Services[0].PortListening || stoppingSnapshot.Services[0].PortConflict {
		t.Fatalf("expected stop lifecycle state, got %+v", stoppingSnapshot.Services[0])
	}

	restartingSnapshot := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", PortListening: true, PortConflict: true},
		},
	}
	applyOptimisticServiceLifecycle(&restartingSnapshot, "restart")
	if restartingSnapshot.Services[0].Status != "restarting" || restartingSnapshot.Services[0].PortListening || restartingSnapshot.Services[0].PortConflict {
		t.Fatalf("expected restart lifecycle state, got %+v", restartingSnapshot.Services[0])
	}

	optimisticSnapshot := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", PortListening: true, PortConflict: true},
			{Name: "redis", DisplayName: "Redis", Status: "running", ContainerName: "stack-redis", PortListening: true, PortConflict: true},
		},
	}
	optimisticServiceState(&optimisticSnapshot, "postgres", "restart")
	if optimisticSnapshot.Services[0].Status != "restarting" || optimisticSnapshot.Services[0].PortListening || optimisticSnapshot.Services[0].PortConflict {
		t.Fatalf("expected targeted optimistic restart state, got %+v", optimisticSnapshot.Services[0])
	}
	if optimisticSnapshot.Services[1].Status != "running" {
		t.Fatalf("expected unrelated service to remain unchanged: %+v", optimisticSnapshot.Services[1])
	}

	before := optimisticSnapshot.Services[1]
	optimisticServiceState(&optimisticSnapshot, "missing", "stop")
	if optimisticSnapshot.Services[1] != before {
		t.Fatalf("expected missing service update to be a no-op: before=%+v after=%+v", before, optimisticSnapshot.Services[1])
	}

	startedAt := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	if got := historyTimestamp(historyEntry{StartedAt: startedAt}); got != "2026-04-04 12:00:00" {
		t.Fatalf("unexpected in-progress history timestamp %q", got)
	}
}

func TestPaletteHelpersCoverSelectionPagingAndFiltering(t *testing.T) {
	var nilState *paletteState
	if action, ok := nilState.selectedAction(); ok || action != (paletteAction{}) {
		t.Fatalf("expected nil palette state to have no selection, got %+v %v", action, ok)
	}

	empty := newPaletteState(paletteModeCommand, "Command palette", "Choose an action", nil)
	empty.move(1)
	empty.syncPagination()
	if empty.selected != 0 || empty.offset != 0 {
		t.Fatalf("expected empty palette movement to stay at zero: %+v", empty)
	}
	if got := empty.summary(); got != "0 results" {
		t.Fatalf("unexpected empty palette summary %q", got)
	}

	items := []paletteAction{
		{Title: "Restart stack", Subtitle: "Stack", Search: "restart stack"},
		{Title: "Open pgAdmin", Subtitle: "Open", Search: "open pgadmin"},
		{Title: "Copy Postgres DSN", Subtitle: "Copy", Search: "copy postgres dsn"},
		{Title: "Watch Redis logs", Subtitle: "Logs", Search: "watch redis logs"},
		{Title: "Open Postgres db shell", Subtitle: "DB", Search: "open postgres db shell"},
	}
	state := newPaletteState(paletteModeCommand, "Command palette", "Choose an action", items)
	state.pageSize = 2
	state.selected = 4
	state.syncPagination()
	if state.paginator.Page != 2 || state.offset != 4 {
		t.Fatalf("expected synced pagination to land on the last page: page=%d offset=%d", state.paginator.Page, state.offset)
	}
	if action, ok := state.selectedAction(); !ok || action.Title != "Open Postgres db shell" {
		t.Fatalf("unexpected selected action %+v %v", action, ok)
	}
	if got := state.summary(); !strings.Contains(got, "5-5 of 5") {
		t.Fatalf("unexpected paged summary %q", got)
	}

	state.input.SetValue("postgres")
	state.applyFilter()
	if len(state.filtered) == 0 {
		t.Fatal("expected filtered results for postgres query")
	}

	state.input.SetValue("no-match")
	state.applyFilter()
	if len(state.filtered) != 0 {
		t.Fatalf("expected empty filtered results, got %+v", state.filtered)
	}
	if got := state.summary(); got != "0 results" {
		t.Fatalf("unexpected empty filtered summary %q", got)
	}
}

func TestPaletteKeyHandlingAndLauncherGuardrails(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	model.palette = newPaletteState(
		paletteModeCommand,
		"Command palette",
		"Choose an action",
		[]paletteAction{
			{Kind: paletteActionSection, Title: "Go to Services", Section: servicesSection},
			{Kind: paletteActionSection, Title: "Go to Stacks", Section: stacksSection},
		},
	)

	if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyDown}); !handled || cmd != nil || model.palette.selected != 1 {
		t.Fatalf("expected down key to move selection, got handled=%v selected=%d cmd=%v", handled, model.palette.selected, cmd)
	}
	if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyUp}); !handled || cmd != nil || model.palette.selected != 0 {
		t.Fatalf("expected up key to move selection, got handled=%v selected=%d cmd=%v", handled, model.palette.selected, cmd)
	}
	if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyPgDown}); !handled || cmd != nil {
		t.Fatalf("expected pgdown to be handled, got handled=%v cmd=%v", handled, cmd)
	}
	if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyPgUp}); !handled || cmd != nil {
		t.Fatalf("expected pgup to be handled, got handled=%v cmd=%v", handled, cmd)
	}
	if _, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: 's', Text: "s"}); !handled {
		t.Fatal("expected text input key to be handled")
	}
	if !strings.Contains(model.palette.input.Value(), "s") {
		t.Fatalf("expected palette input to update, got %q", model.palette.input.Value())
	}

	model.palette = newPaletteState(
		paletteModeCommand,
		"Command palette",
		"Choose an action",
		[]paletteAction{{Kind: paletteActionSection, Title: "Go to Stacks", Section: stacksSection}},
	)
	if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyEnter}); !handled || cmd != nil || model.active != stacksSection || model.palette != nil {
		t.Fatalf("expected enter to execute section jump: handled=%v active=%v palette=%v cmd=%v", handled, model.active, model.palette, cmd)
	}

	model.palette = newPaletteState(paletteModeCommand, "Command palette", "Choose", []paletteAction{{Title: "One"}})
	if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyEsc}); !handled || cmd != nil || model.palette != nil {
		t.Fatalf("expected esc to close palette: handled=%v palette=%v cmd=%v", handled, model.palette, cmd)
	}

	postgres := Service{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"}
	cockpit := Service{Name: "cockpit", DisplayName: "Cockpit", Status: "running", URL: "https://localhost:9090"}

	t.Run("log watch warns when the service disappeared", func(t *testing.T) {
		m := Model{snapshot: Snapshot{Services: []Service{postgres}}}
		cmd := m.startServiceLogWatch(paletteAction{ServiceKey: "missing", Title: "Watch missing logs"})
		if cmd == nil || m.banner == nil || m.banner.Message != "selected service is no longer available" {
			t.Fatalf("unexpected missing-service log watch result: banner=%+v cmd=%v", m.banner, cmd)
		}
	})

	t.Run("log watch rejects host tools", func(t *testing.T) {
		m := Model{snapshot: Snapshot{Services: []Service{cockpit}}}
		cmd := m.startServiceLogWatch(paletteAction{ServiceKey: serviceKey(cockpit), Title: "Watch Cockpit logs"})
		if cmd == nil || m.banner == nil || m.banner.Message != "live logs are unavailable for host tools" {
			t.Fatalf("unexpected host-tool log watch result: banner=%+v cmd=%v", m.banner, cmd)
		}
	})

	t.Run("log watch warns when the launcher is unavailable", func(t *testing.T) {
		m := Model{snapshot: Snapshot{Services: []Service{postgres}}}
		cmd := m.startServiceLogWatch(paletteAction{ServiceKey: serviceKey(postgres), Title: "Watch Postgres logs"})
		if cmd == nil || m.banner == nil || m.banner.Message != "live log handoff is unavailable in this model" {
			t.Fatalf("unexpected unavailable-launcher result: banner=%+v cmd=%v", m.banner, cmd)
		}
	})

	t.Run("log watch surfaces launcher errors", func(t *testing.T) {
		m := Model{
			snapshot: Snapshot{Services: []Service{postgres}},
			logWatchLauncher: func(LogWatchRequest) (tea.ExecCommand, error) {
				return nil, errors.New("exec unavailable")
			},
		}
		cmd := m.startServiceLogWatch(paletteAction{ServiceKey: serviceKey(postgres), Title: "Watch Postgres logs"})
		if cmd == nil || m.banner == nil || !strings.Contains(m.banner.Message, "watch logs for postgres failed: exec unavailable") {
			t.Fatalf("unexpected launcher-error result: banner=%+v cmd=%v", m.banner, cmd)
		}
	})

	t.Run("db shell warns when the service disappeared", func(t *testing.T) {
		m := Model{snapshot: Snapshot{Services: []Service{postgres}}}
		cmd := m.startDBShell(paletteAction{ServiceKey: "missing", Title: "Open missing db shell"})
		if cmd == nil || m.banner == nil || m.banner.Message != "selected service is no longer available" {
			t.Fatalf("unexpected missing-service db shell result: banner=%+v cmd=%v", m.banner, cmd)
		}
	})

	t.Run("db shell surfaces launcher errors", func(t *testing.T) {
		m := Model{
			snapshot: Snapshot{Services: []Service{postgres}},
			dbShellLauncher: func(DBShellRequest) (tea.ExecCommand, error) {
				return nil, errors.New("psql unavailable")
			},
		}
		cmd := m.startDBShell(paletteAction{ServiceKey: serviceKey(postgres), Title: "Open Postgres db shell"})
		if cmd == nil || m.banner == nil || !strings.Contains(m.banner.Message, "open postgres db shell failed: psql unavailable") {
			t.Fatalf("unexpected db-shell launcher-error result: banner=%+v cmd=%v", m.banner, cmd)
		}
	})
}
