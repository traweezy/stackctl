package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/traweezy/stackctl/internal/output"
)

func TestPaletteStateMovementAndRenderingHelpers(t *testing.T) {
	items := []paletteAction{
		{Title: "Restart stack", Subtitle: "Stack action"},
		{Title: "Open pgAdmin", Subtitle: "Browser handoff"},
		{Title: "Copy Postgres DSN", Subtitle: "Clipboard"},
	}

	state := newPaletteState(paletteModeCommand, "Command palette", "Choose an action", items)
	state.selected = 99
	state.offset = 99
	state.clampSelection()
	if state.selected != len(items)-1 || state.offset != len(items)-1 {
		t.Fatalf("expected clampSelection to snap to the last item, got selected=%d offset=%d", state.selected, state.offset)
	}

	state.selected = -3
	state.offset = 5
	state.clampSelection()
	if state.selected != 0 || state.offset != 0 {
		t.Fatalf("expected clampSelection to snap to the first item, got selected=%d offset=%d", state.selected, state.offset)
	}

	state.move(-1)
	if state.selected != len(items)-1 {
		t.Fatalf("expected move to wrap upward, got selected=%d", state.selected)
	}
	state.move(1)
	if state.selected != 0 {
		t.Fatalf("expected move to wrap downward, got selected=%d", state.selected)
	}

	plainPanel := stripANSITest(renderPalettePanel(state, 100, 28))
	for _, fragment := range []string{
		"Command palette",
		"Choose an action",
		"Restart stack",
		"Stack action",
		"3 results",
		"type to filter  •  ↑/↓ choose  •  pgup/pgdn page  •  enter run  •  esc close",
	} {
		if !strings.Contains(plainPanel, fragment) {
			t.Fatalf("expected palette panel to contain %q:\n%s", fragment, plainPanel)
		}
	}

	selectedEntry := stripANSITest(renderPaletteEntry(items[0], true, 40))
	if !strings.Contains(selectedEntry, "▸ Restart stack") {
		t.Fatalf("expected selected palette entry marker:\n%s", selectedEntry)
	}

	unselectedEntry := stripANSITest(renderPaletteEntry(items[1], false, 40))
	for _, fragment := range []string{"Open pgAdmin", "Browser handoff"} {
		if !strings.Contains(unselectedEntry, fragment) {
			t.Fatalf("expected unselected entry to contain %q:\n%s", fragment, unselectedEntry)
		}
	}
}

func TestTerminalCopyAndShellPaletteGuardrails(t *testing.T) {
	action := paletteAction{Title: "Copy Postgres DSN", Kind: paletteActionCopyText}
	msg, ok := findMsgOfType[copyDoneMsg](terminalCopyCmd("postgres://app@localhost/app", action, "copied postgres dsn to clipboard")())
	if !ok {
		t.Fatal("expected terminalCopyCmd to emit copyDoneMsg")
	}
	if msg.message != "copied postgres dsn to clipboard" || msg.action.Title != action.Title || msg.err != nil {
		t.Fatalf("unexpected copy result message: %+v", msg)
	}

	postgres := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		ContainerName: "stack-postgres",
		Status:        "running",
	}
	hostTool := Service{
		Name:        "cockpit",
		DisplayName: "Cockpit",
		Status:      "running",
		URL:         "https://localhost:9090",
	}

	t.Run("service shell rejects host tools", func(t *testing.T) {
		model := Model{snapshot: Snapshot{Services: []Service{hostTool}}}
		cmd := model.startServiceShell(paletteAction{ServiceKey: serviceKey(hostTool), Title: "Open Cockpit shell"})
		if cmd == nil {
			t.Fatal("expected warning clear command")
		}
		if model.banner == nil || model.banner.Status != output.StatusWarn || model.banner.Message != "service shells are unavailable for host tools" {
			t.Fatalf("unexpected host-tool banner: %+v", model.banner)
		}
	})

	t.Run("service shell surfaces launcher failures", func(t *testing.T) {
		model := Model{
			snapshot: Snapshot{Services: []Service{postgres}},
			shellLauncher: func(ServiceShellRequest) (tea.ExecCommand, error) {
				return nil, errors.New("exec unavailable")
			},
		}
		cmd := model.startServiceShell(paletteAction{ServiceKey: serviceKey(postgres), Title: "Open Postgres shell"})
		if cmd == nil {
			t.Fatal("expected warning clear command")
		}
		if model.banner == nil || model.banner.Status != output.StatusWarn {
			t.Fatalf("expected warning banner, got %+v", model.banner)
		}
		if !strings.Contains(model.banner.Message, "open postgres shell failed: exec unavailable") {
			t.Fatalf("unexpected shell failure banner: %+v", model.banner)
		}
	})

	t.Run("db shell rejects non-postgres services", func(t *testing.T) {
		model := Model{snapshot: Snapshot{Services: []Service{hostTool}}}
		cmd := model.startDBShell(paletteAction{ServiceKey: serviceKey(hostTool), Title: "Open Cockpit db shell"})
		if cmd == nil {
			t.Fatal("expected warning clear command")
		}
		if model.banner == nil || model.banner.Status != output.StatusWarn || model.banner.Message != "db shell is only available for Postgres" {
			t.Fatalf("unexpected non-postgres db-shell banner: %+v", model.banner)
		}
	})

	t.Run("db shell warns when the launcher is unavailable", func(t *testing.T) {
		model := Model{snapshot: Snapshot{Services: []Service{postgres}}}
		cmd := model.startDBShell(paletteAction{ServiceKey: serviceKey(postgres), Title: "Open Postgres db shell"})
		if cmd == nil {
			t.Fatal("expected warning clear command")
		}
		if model.banner == nil || model.banner.Status != output.StatusWarn || model.banner.Message != "db shell handoff is unavailable in this model" {
			t.Fatalf("unexpected db-shell unavailable banner: %+v", model.banner)
		}
	})
}

func TestPaletteRecentPagingAndPinHelpers(t *testing.T) {
	testCases := []struct {
		action paletteAction
		want   string
	}{
		{action: paletteAction{Kind: paletteActionSidebar, Action: ActionSpec{ID: ActionDoctor}}, want: "sidebar:doctor"},
		{action: paletteAction{Kind: paletteActionJumpStack, StackName: "staging"}, want: "stack:staging"},
		{action: paletteAction{Kind: paletteActionCopyValue, ServiceKey: "postgres", CopyTarget: copyTargetDSN}, want: "copy:postgres:dsn"},
		{action: paletteAction{Kind: paletteActionCopyText, Title: "Copy stackctl connect output"}, want: "copy-text:copy stackctl connect output"},
		{action: paletteAction{Kind: paletteActionWatchLogs, ServiceKey: "postgres"}, want: "logs:postgres"},
		{action: paletteAction{Kind: paletteActionExecShell, ServiceKey: "postgres"}, want: "exec:postgres"},
		{action: paletteAction{Kind: paletteActionDBShell, ServiceKey: "postgres"}, want: "db:postgres"},
		{action: paletteAction{Kind: paletteActionSection}, want: ""},
	}

	for _, tc := range testCases {
		if got := tc.action.recentKey(); got != tc.want {
			t.Fatalf("recentKey(%+v) = %q, want %q", tc.action, got, tc.want)
		}
	}

	state := newPaletteState(paletteModeCommand, "Command palette", "Choose an action", []paletteAction{
		{Title: "One"},
		{Title: "Two"},
		{Title: "Three"},
		{Title: "Four"},
		{Title: "Five"},
	})
	state.setPageSize(2)
	state.page(1)
	if state.selected != 2 || state.offset != 2 {
		t.Fatalf("expected page forward to land on the third item, got selected=%d offset=%d", state.selected, state.offset)
	}
	state.page(10)
	if state.selected != 4 || state.offset != 4 {
		t.Fatalf("expected page clamp to land on the last page, got selected=%d offset=%d", state.selected, state.offset)
	}
	if got := state.summary(); !strings.Contains(got, "5-5 of 5") {
		t.Fatalf("expected paged summary, got %q", got)
	}

	postgres := Service{Name: "postgres", DisplayName: "Postgres", ContainerName: "stack-postgres", Status: "running"}
	redis := Service{Name: "redis", DisplayName: "Redis", ContainerName: "stack-redis", Status: "running"}
	model := Model{
		snapshot: Snapshot{Services: []Service{postgres, redis}},
		pinnedServices: map[string]struct{}{
			serviceKey(postgres): {},
			"missing":            {},
		},
	}
	model.normalizePinnedServices()
	if len(model.pinnedServices) != 1 || !model.servicePinned(serviceKey(postgres)) {
		t.Fatalf("expected normalizePinnedServices to remove stale pins, got %+v", model.pinnedServices)
	}

	now := time.Now()
	model.history = []historyEntry{
		{
			CompletedAt: now.Add(-2 * time.Minute),
			Recent:      &paletteAction{Kind: paletteActionCopyText, Title: "Copy stackctl connect output"},
			Message:     "copied connect output",
		},
		{
			CompletedAt: now.Add(-1 * time.Minute),
			Recent:      &paletteAction{Kind: paletteActionCopyText, Title: "Copy stackctl connect output"},
			Message:     "duplicate ignored",
		},
		{
			CompletedAt: now,
			Recent:      &paletteAction{Kind: paletteActionJumpStack, StackName: "staging", Title: "Go to stack staging"},
		},
		{
			Recent:  &paletteAction{Kind: paletteActionJumpService, ServiceKey: serviceKey(redis), Title: "Go to Redis"},
			Message: "incomplete",
		},
	}

	recent := model.recentPaletteActions()
	if len(recent) != 2 {
		t.Fatalf("expected deduplicated recent actions, got %+v", recent)
	}
	if recent[0].Title != "Go to stack staging" || recent[1].Subtitle != "duplicate ignored" {
		t.Fatalf("unexpected recent palette actions: %+v", recent)
	}
}

func TestPaletteOpenAndExecuteActionHelpers(t *testing.T) {
	postgres := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		ContainerName: "stack-postgres",
		Status:        "running",
		DSN:           "postgres://app:app@localhost:5432/app",
		Database:      "app",
		Username:      "app",
		Password:      "app",
	}
	hostTool := Service{
		Name:        "cockpit",
		DisplayName: "Cockpit",
		Status:      "running",
		URL:         "https://localhost:9090",
	}

	t.Run("jump and copy palettes expose warnings and actions", func(t *testing.T) {
		model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		model.active = stacksSection
		cmd := model.openJumpPalette()
		if cmd == nil || model.banner == nil || model.banner.Message != "no stack profiles are available to jump to" {
			t.Fatalf("expected stack jump warning, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		cmd = model.openCopyPalette()
		if cmd == nil || model.banner == nil || model.banner.Message != "select a service before copying values" {
			t.Fatalf("expected copy warning without a selected service, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		model.snapshot = Snapshot{
			Services: []Service{postgres, hostTool},
			Stacks:   []StackProfile{{Name: "staging", Configured: true}},
		}
		model.selectedService = serviceKey(postgres)

		if cmd = model.openJumpPalette(); cmd != nil {
			t.Fatalf("expected service jump palette to open without a banner clear command, got %v", cmd)
		}
		if model.palette == nil || model.palette.title != "Jump to service" {
			t.Fatalf("expected service jump palette, got %+v", model.palette)
		}

		if cmd = model.openCopyPalette(); cmd != nil {
			t.Fatalf("expected copy palette to open without a banner clear command, got %v", cmd)
		}
		if model.palette == nil || model.palette.title != "Copy value" || len(model.palette.filtered) == 0 {
			t.Fatalf("expected copy palette with actions, got %+v", model.palette)
		}
	})

	t.Run("execute palette actions updates model state", func(t *testing.T) {
		model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		model.snapshot = Snapshot{Services: []Service{postgres, hostTool}}
		model.selectedService = serviceKey(postgres)
		model.clipboardWriter = func(string) error { return nil }

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionSection, Section: configSection}); cmd != nil {
			t.Fatalf("expected section switch to return nil, got %v", cmd)
		}
		if model.active != configSection {
			t.Fatalf("expected active section to switch, got %v", model.active)
		}

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionJumpStack, StackName: "staging"}); cmd != nil {
			t.Fatalf("expected stack jump to return nil, got %v", cmd)
		}
		if model.active != stacksSection || model.selectedStack != "staging" {
			t.Fatalf("expected jump stack selection, got active=%v selectedStack=%q", model.active, model.selectedStack)
		}

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionJumpService, ServiceKey: serviceKey(postgres)}); cmd != nil {
			t.Fatalf("expected service jump to return nil, got %v", cmd)
		}
		if model.active != servicesSection || model.selectedService != serviceKey(postgres) {
			t.Fatalf("expected jump service selection, got active=%v selectedService=%q", model.active, model.selectedService)
		}

		copyMsg, ok := findMsgOfType[copyDoneMsg](model.executePaletteAction(paletteAction{
			Kind:       paletteActionCopyValue,
			Title:      "Copy Postgres DSN",
			ServiceKey: serviceKey(postgres),
			CopyTarget: copyTargetDSN,
		})())
		if !ok || copyMsg.err != nil || !strings.Contains(copyMsg.message, "copied Postgres DSN") {
			t.Fatalf("unexpected copy action result: %+v ok=%v", copyMsg, ok)
		}

		textMsg, ok := findMsgOfType[copyDoneMsg](model.executePaletteAction(paletteAction{
			Kind:      paletteActionCopyText,
			Title:     "Copy stackctl connect output",
			CopyValue: "stackctl connect",
		})())
		if !ok || textMsg.err != nil || !strings.Contains(textMsg.message, "copied stackctl connect output") {
			t.Fatalf("unexpected copy text result: %+v ok=%v", textMsg, ok)
		}

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionPinService, ServiceKey: serviceKey(postgres)}); cmd == nil {
			t.Fatal("expected pin action to return a banner clear command")
		}
		if !model.servicePinned(serviceKey(postgres)) {
			t.Fatalf("expected service to be pinned, got %+v", model.pinnedServices)
		}
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionPinService, ServiceKey: serviceKey(postgres)}); cmd == nil {
			t.Fatal("expected unpin action to return a banner clear command")
		}
		if model.servicePinned(serviceKey(postgres)) {
			t.Fatalf("expected service to be unpinned, got %+v", model.pinnedServices)
		}

		startLayout := model.layout
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleLayout}); cmd != nil {
			t.Fatalf("expected layout toggle to return nil, got %v", cmd)
		}
		if model.layout == startLayout {
			t.Fatalf("expected layout toggle to change layout, got %v", model.layout)
		}

		model.autoRefresh = true
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleAutoRefresh}); cmd != nil {
			t.Fatalf("expected auto-refresh disable to return nil, got %v", cmd)
		}
		if model.autoRefresh {
			t.Fatal("expected auto-refresh to be disabled")
		}
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleAutoRefresh}); cmd == nil {
			t.Fatal("expected auto-refresh enable to schedule a timer")
		}
		if !model.autoRefresh {
			t.Fatal("expected auto-refresh to be re-enabled")
		}

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleSecrets}); cmd != nil {
			t.Fatalf("expected secrets toggle to return nil, got %v", cmd)
		}
		if !model.showSecrets {
			t.Fatal("expected secrets toggle to enable secret display")
		}
	})

	t.Run("copy and sidebar guardrails surface warnings", func(t *testing.T) {
		model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		model.snapshot = Snapshot{Services: []Service{hostTool}}

		cmd := model.startCopyAction(paletteAction{Kind: paletteActionCopyValue, ServiceKey: "missing", CopyTarget: copyTargetDSN})
		if cmd == nil || model.banner == nil || model.banner.Message != "selected service is no longer available" {
			t.Fatalf("expected missing-service warning, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		model.snapshot = Snapshot{Services: []Service{hostTool}}
		cmd = model.startCopyAction(paletteAction{Kind: paletteActionCopyValue, ServiceKey: serviceKey(hostTool), CopyTarget: copyTargetDSN})
		if cmd == nil || model.banner == nil || model.banner.Message != "copy target is unavailable for the selected service" {
			t.Fatalf("expected unavailable-target warning, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		cmd = model.executePaletteAction(paletteAction{Kind: paletteActionCopyText, Title: "Copy empty value"})
		if cmd == nil || model.banner == nil || model.banner.Message != "copy value is unavailable for this action" {
			t.Fatalf("expected empty-copy warning, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		cmd = model.executePaletteAction(paletteAction{
			Kind:   paletteActionSidebar,
			Action: ActionSpec{ID: ActionDoctor, Label: "Doctor", ConfirmMessage: "Confirm doctor"},
		})
		if cmd != nil || model.confirmation == nil || model.confirmation.Action.ID != ActionDoctor {
			t.Fatalf("expected sidebar confirmation to open, got confirmation=%+v cmd=%v", model.confirmation, cmd)
		}
	})
}
