package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	bubblespinner "charm.land/bubbles/v2/spinner"
	bubblesstopwatch "charm.land/bubbles/v2/stopwatch"
	bubblestimer "charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func TestModelUpdateAdditionalBranchCoverage(t *testing.T) {
	cfg := configpkg.Default()
	postgres := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		Status:        "running",
		ContainerName: "stack-postgres",
		DSN:           "postgres://app:app@localhost:5432/app",
	}
	hostTool := Service{
		Name:        "cockpit",
		DisplayName: "Cockpit",
		Status:      "running",
		URL:         "https://localhost:9090",
	}
	snapshot := tuiTestSnapshot(cfg, []Service{postgres, hostTool})
	snapshot.Services = append(snapshot.Services, Service{
		Name:        "redis",
		DisplayName: "Redis",
		Status:      "stopped",
	})

	t.Run("busy tick, snapshot, and action messages cover non-default branches", func(t *testing.T) {
		model := NewModel(func() (Snapshot, error) { return snapshot, nil }).WithVersion(" v1.2.3 ")
		model = loadSnapshotModel(t, model, snapshot)
		model.autoRefresh = false
		model.loading = true
		_ = model.beginBusy(3 * time.Second)

		updated, cmd := model.Update(snapshotMsg{snapshot: snapshot})
		current := updated.(Model)
		if cmd == nil {
			t.Fatal("expected snapshot update to return a finish-busy command")
		}
		if current.autoRefreshID != model.autoRefreshID {
			t.Fatalf("expected auto refresh id to stay unchanged when auto-refresh is disabled, before=%d after=%d", model.autoRefreshID, current.autoRefreshID)
		}

		current.loading = true
		updated, cmd = current.Update(bubblespinner.TickMsg{})
		current = updated.(Model)
		if cmd == nil {
			t.Fatal("expected busy spinner tick to keep scheduling follow-up work")
		}

		current.loading = false
		_ = current.beginBusy(0)
		current.runningAction = &runningAction{Action: ActionSpec{ID: ActionStart}}
		updated, cmd = current.Update(bubblesstopwatch.TickMsg{})
		current = updated.(Model)
		if !current.isBusy() || current.runningAction == nil {
			t.Fatalf("expected busy stopwatch tick to preserve the running action, cmd=%v running=%+v", cmd, current.runningAction)
		}

		_ = current.finishBusy()
		_ = current.beginBusy(5 * time.Second)
		current.runningAction = nil
		current.runningConfigOp = &configOperation{Message: "Applying config"}
		updated, cmd = current.Update(bubblestimer.TickMsg{})
		current = updated.(Model)
		if !current.isBusy() || current.runningConfigOp == nil || current.busyBudget <= 0 {
			t.Fatalf("expected busy timer tick to preserve the config operation, cmd=%v op=%+v budget=%s", cmd, current.runningConfigOp, current.busyBudget)
		}

		current.runningConfigOp = nil
		current.autoRefresh = false
		updated, cmd = current.Update(logWatchDoneMsg{Service: "Postgres", Err: errors.New("watch failed")})
		current = updated.(Model)
		if cmd == nil || current.banner == nil || !strings.Contains(current.banner.Message, "watch failed") {
			t.Fatalf("expected log watch failure to surface a warning banner, cmd=%v banner=%+v", cmd, current.banner)
		}

		current.banner = nil
		current.loading = false
		current.runningAction = &runningAction{
			Action:   ActionSpec{ID: ActionDoctor, Label: "Doctor"},
			History:  7,
			Previous: snapshot,
		}
		current.history = []historyEntry{{
			ID:        7,
			Action:    "Doctor",
			Status:    output.StatusInfo,
			Message:   "running doctor",
			StartedAt: time.Now(),
		}}
		updated, cmd = current.Update(actionMsg{
			historyID: 7,
			action:    ActionSpec{ID: ActionDoctor, Label: "Doctor"},
			report:    ActionReport{Status: output.StatusOK, Message: "doctor finished"},
		})
		current = updated.(Model)
		if cmd == nil || current.runningAction != nil || current.loading {
			t.Fatalf("expected non-refresh action completion to finish in place, cmd=%v running=%+v loading=%v", cmd, current.runningAction, current.loading)
		}
	})

	t.Run("key handling covers no-op and non-refresh branches", func(t *testing.T) {
		model := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
		model.autoRefresh = false

		model.confirmation = &confirmationState{Kind: confirmationKind(99)}
		updated, cmd := model.Update(tea.KeyPressMsg{Text: "x", Code: 'x'})
		current := updated.(Model)
		if cmd != nil || current.confirmation == nil {
			t.Fatalf("expected unknown confirmation key to be ignored, cmd=%v confirmation=%+v", cmd, current.confirmation)
		}

		current.confirmation = nil
		current.active = servicesSection
		current.snapshot.Services = nil
		current.selectedService = "missing"
		updated, cmd = current.Update(tea.KeyPressMsg{Text: "e", Code: 'e'})
		current = updated.(Model)
		if cmd != nil || current.runningHandoff != nil {
			t.Fatalf("expected exec shortcut without a selection to be ignored, cmd=%v handoff=%+v", cmd, current.runningHandoff)
		}

		updated, cmd = current.Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
		current = updated.(Model)
		if cmd != nil || current.runningHandoff != nil {
			t.Fatalf("expected db-shell shortcut without a selection to be ignored, cmd=%v handoff=%+v", cmd, current.runningHandoff)
		}

		updated, cmd = current.Update(tea.KeyPressMsg{Text: "p", Code: 'p'})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected pin shortcut without a selection to be ignored, cmd=%v", cmd)
		}

		current.runner = func(ActionID) (ActionReport, error) {
			return ActionReport{Status: output.StatusOK, Message: "done"}, nil
		}
		updated, cmd = current.Update(tea.KeyPressMsg{Text: "9", Code: '9'})
		current = updated.(Model)
		if cmd != nil || current.confirmation != nil {
			t.Fatalf("expected out-of-range action key to be ignored, cmd=%v confirmation=%+v", cmd, current.confirmation)
		}

		current.logWatchLauncher = nil
		updated, cmd = current.Update(tea.KeyPressMsg{Text: "w", Code: 'w'})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected watch shortcut to be ignored when no launcher is configured, cmd=%v", cmd)
		}

		current.runningAction = &runningAction{Action: ActionSpec{ID: ActionStart}}
		updated, cmd = current.Update(tea.KeyPressMsg{Text: "r", Code: 'r'})
		current = updated.(Model)
		if cmd != nil || current.runningAction == nil {
			t.Fatalf("expected refresh to be ignored while busy, cmd=%v running=%+v", cmd, current.runningAction)
		}

		current.runningAction = nil
		current.selectedService = serviceKey(postgres)
		updated, cmd = current.Update(tea.KeyPressMsg{Text: "]", Code: ']'})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected next-item on manual refresh mode to avoid scheduling auto refresh, cmd=%v", cmd)
		}

		updated, cmd = current.Update(tea.KeyPressMsg{Text: "[", Code: '['})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected previous-item on manual refresh mode to avoid scheduling auto refresh, cmd=%v", cmd)
		}

		updated, cmd = current.Update(tea.KeyPressMsg{Text: "l", Code: 'l'})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected next-section on manual refresh mode to avoid scheduling auto refresh, cmd=%v", cmd)
		}

		updated, cmd = current.Update(tea.KeyPressMsg{Text: "h", Code: 'h'})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected previous-section on manual refresh mode to avoid scheduling auto refresh, cmd=%v", cmd)
		}
	})
}

func TestTUIHelperRenderingAndBusyBranches(t *testing.T) {
	cfg := configpkg.Default()

	t.Run("helper methods cover compact, missing, and fallback branches", func(t *testing.T) {
		model := Model{}
		model = model.WithProductivity(nil, nil, nil)
		if model.pinnedServices == nil {
			t.Fatal("expected WithProductivity to initialize pinned services when missing")
		}

		model.width = 80
		model.height = 10
		if got := renderSessionRail(model); got != "" {
			t.Fatalf("expected very small layouts to hide the session rail, got %q", got)
		}

		model.width = 120
		model.height = 24
		model.active = stacksSection
		if got := sidebarCompactSelectionLabel(model); got != "" {
			t.Fatalf("expected missing stack selection to render empty compact label, got %q", got)
		}

		model.active = configSection
		if got := sidebarCompactSelectionLabel(model); got != "" {
			t.Fatalf("expected missing config selection to render empty compact label, got %q", got)
		}
		if lines := sidebarSelectionLines(model); lines != nil {
			t.Fatalf("expected missing config selection lines to return nil, got %+v", lines)
		}

		model.active = historySection
		if got := sidebarCompactSelectionLabel(model); got != "" {
			t.Fatalf("expected empty history to render empty compact label, got %q", got)
		}
		lines := sidebarSelectionLines(model)
		if len(lines) != 1 || !strings.Contains(lines[0], "No session history yet") {
			t.Fatalf("expected empty history selection hint, got %+v", lines)
		}

		model.active = section(-1)
		if got := model.currentContent(); got != "" {
			t.Fatalf("expected unknown section content to render empty, got %q", got)
		}

		if got := nextSection(section(-1)); got != overviewSection {
			t.Fatalf("expected unknown section to wrap back to overview, got %v", got)
		}
		if model.activeHasSelectionList() {
			t.Fatal("expected invalid section to report no selection list")
		}
		if model.activeHasServiceSelectionList() {
			t.Fatal("expected invalid section to report no service selection list")
		}
		if cmd := model.cycleActiveSelection(1); cmd != nil {
			t.Fatalf("expected no-op selection cycling for invalid sections, got %v", cmd)
		}
	})

	t.Run("rendering covers masked access and settings groups", func(t *testing.T) {
		appendOnly := true
		service := Service{
			Name:              "pgadmin",
			DisplayName:       "pgAdmin",
			Status:            "running",
			ContainerName:     "stack-pgadmin",
			Image:             "docker.io/dpage/pgadmin4:9",
			DataVolume:        "pgadmin_data",
			Host:              "localhost",
			ExternalPort:      8080,
			InternalPort:      80,
			Endpoint:          "http://localhost:8080",
			URL:               "https://localhost:9090",
			DSN:               "postgres://:secret@localhost:5432/app",
			Database:          "app",
			MaintenanceDB:     "postgres",
			Email:             "ops@example.com",
			MasterKey:         "master-secret",
			Token:             "token-secret",
			AccessKey:         "access-key",
			SecretKey:         "secret-key",
			Username:          "app",
			Password:          "password-secret",
			AppendOnly:        &appendOnly,
			SavePolicy:        "60 1000",
			MaxMemoryPolicy:   "allkeys-lru",
			VolumeSizeLimitMB: 2048,
			ServerMode:        "enabled",
		}

		rendered := strings.Join(renderServiceBlock(service, false, expandedLayout, false), "\n")
		for _, fragment := range []string{
			"Email: ops@example.com",
			"Master key: " + maskedSecret,
			"Token: " + maskedSecret,
			"Access key: access-key",
			"Secret key: " + maskedSecret,
			"Password: " + maskedSecret,
			"Appendonly: enabled",
			"Save policy: 60 1000",
			"Maxmemory policy: allkeys-lru",
			"Volume size limit: 2048 MB",
			"Server mode: enabled",
			"DSN: postgres://:" + maskedSecret + "@localhost:5432/app",
		} {
			if !strings.Contains(rendered, fragment) {
				t.Fatalf("expected service block to contain %q:\n%s", fragment, rendered)
			}
		}

		summary := renderOverviewSummary([]Service{
			service,
			{Name: "cockpit", DisplayName: "Cockpit", Status: "running", URL: "https://localhost:9090"},
			{Name: "redis", DisplayName: "Redis", Status: "missing", ContainerName: "stack-redis"},
			{Name: "nats", DisplayName: "NATS", Status: "warning", ContainerName: "stack-nats"},
		})
		for _, fragment := range []string{"Running: 1", "Stopped: 1", "Attention: 1"} {
			if !collapsedContainsTest(summary, fragment) {
				t.Fatalf("expected overview summary to contain %q:\n%s", fragment, summary)
			}
		}

		hostMissing := Service{Name: "cockpit", DisplayName: "Cockpit", Status: "missing", URL: "https://localhost:9090"}
		if note := healthNote(hostMissing); note != "Service is not installed." {
			t.Fatalf("unexpected host-tool missing health note %q", note)
		}
		if note := healthNote(Service{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", ExternalPort: 5432}); note != "Container is running, but the host port is not reachable yet." {
			t.Fatalf("unexpected running-but-unreachable health note %q", note)
		}
	})

	t.Run("busy helpers cover progress, header, and stop branches", func(t *testing.T) {
		model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil }).WithVersion(" v1.2.3 ")
		model.width = 100
		model.height = 30
		model.snapshot = tuiTestSnapshot(cfg, nil)
		model.loading = true
		model.busyBudget = 5 * time.Second

		if progress := renderBusyProgress(model, 60); progress == "" {
			t.Fatal("expected zero-elapsed busy progress to render")
		}

		model.errMessage = ""
		if header := renderHeader(model); !strings.Contains(header, "version: v1.2.3") {
			t.Fatalf("expected header to show the trimmed version, got:\n%s", header)
		}

		model.errMessage = "config failed"
		if header := renderHeader(model); !strings.Contains(header, "config failed") {
			t.Fatalf("expected header to include the error banner, got:\n%s", header)
		}

		model.errMessage = ""
		model.busyStartedAt = time.Now().Add(-time.Second)
		if progress := renderBusyProgress(model, 60); progress == "" {
			t.Fatal("expected positive elapsed busy progress to render")
		}

		model.loading = false
		startCmd := model.beginBusy(time.Second)
		if startCmd == nil {
			t.Fatal("expected beginBusy to return a batch command")
		}
		stopCmd := model.finishBusy()
		if stopCmd == nil {
			t.Fatal("expected finishBusy to stop the running timer and stopwatch")
		}
	})
}

func TestPaletteAdditionalBranchCoverage(t *testing.T) {
	postgres := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		Status:        "running",
		ContainerName: "stack-postgres",
		DSN:           "postgres://app:app@localhost:5432/app",
	}
	custom := Service{
		Name:        "custom",
		DisplayName: "Custom",
		Status:      "running",
	}

	t.Run("open palettes cover empty and missing-target branches", func(t *testing.T) {
		model := Model{active: stacksSection}
		if cmd := model.openJumpPalette(); cmd == nil || model.banner == nil || model.banner.Message != "no stack profiles are available to jump to" {
			t.Fatalf("expected empty stack jump palette warning, cmd=%v banner=%+v", cmd, model.banner)
		}

		model = Model{active: servicesSection}
		if cmd := model.openJumpPalette(); cmd == nil || model.banner == nil || model.banner.Message != "no services are available to jump to" {
			t.Fatalf("expected empty service jump palette warning, cmd=%v banner=%+v", cmd, model.banner)
		}

		model = Model{}
		if cmd := model.openCopyPalette(); cmd == nil || model.banner == nil || model.banner.Message != "select a service before copying values" {
			t.Fatalf("expected copy palette to require a selected service, cmd=%v banner=%+v", cmd, model.banner)
		}

		model = Model{
			snapshot:        Snapshot{Services: []Service{custom}},
			active:          servicesSection,
			selectedService: serviceKey(custom),
		}
		if cmd := model.openCopyPalette(); cmd == nil || model.banner == nil || model.banner.Message != "no copy targets are available for the selected service" {
			t.Fatalf("expected copy palette to warn when no copy targets exist, cmd=%v banner=%+v", cmd, model.banner)
		}
	})

	t.Run("palette key handling covers nil, enter-without-action, and escape branches", func(t *testing.T) {
		model := Model{}
		if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyEnter}); handled || cmd != nil {
			t.Fatalf("expected nil palette to report not handled, handled=%v cmd=%v", handled, cmd)
		}

		model.palette = newPaletteState(paletteModeCommand, "Command", "Choose", nil)
		if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyEnter}); !handled || cmd != nil || model.palette == nil {
			t.Fatalf("expected enter on an empty palette to be handled without closing it, handled=%v cmd=%v palette=%+v", handled, cmd, model.palette)
		}

		if cmd, handled := model.handlePaletteKey(tea.KeyPressMsg{Code: tea.KeyEsc}); !handled || cmd != nil || model.palette != nil {
			t.Fatalf("expected escape to close the palette, handled=%v cmd=%v palette=%+v", handled, cmd, model.palette)
		}
	})

	t.Run("executePaletteAction covers sidebar confirmation and toggles", func(t *testing.T) {
		model := Model{
			layout:      expandedLayout,
			autoRefresh: false,
		}

		if cmd := model.executePaletteAction(paletteAction{
			Kind:   paletteActionSidebar,
			Action: ActionSpec{ID: ActionRestart, Label: "Restart", ConfirmMessage: "Restart now?"},
		}); cmd != nil || model.confirmation == nil {
			t.Fatalf("expected sidebar confirmation action to stage a confirmation, cmd=%v confirmation=%+v", cmd, model.confirmation)
		}

		model.confirmation = nil
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleLayout}); cmd != nil || model.layout != compactLayout {
			t.Fatalf("expected toggle layout action to flip to compact, cmd=%v layout=%v", cmd, model.layout)
		}

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleAutoRefresh}); cmd == nil || !model.autoRefresh {
			t.Fatalf("expected toggle auto-refresh action to enable auto-refresh, cmd=%v autoRefresh=%v", cmd, model.autoRefresh)
		}

		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleSecrets}); cmd != nil || !model.showSecrets {
			t.Fatalf("expected toggle secrets action to update the model, cmd=%v showSecrets=%v", cmd, model.showSecrets)
		}
	})

	t.Run("pinning and shell handoff helpers cover missing and unavailable branches", func(t *testing.T) {
		model := Model{}
		if cmd := model.togglePinnedService("missing"); cmd == nil || model.banner == nil || model.banner.Message != "service is no longer available to pin" {
			t.Fatalf("expected missing service pin warning, cmd=%v banner=%+v", cmd, model.banner)
		}

		model = Model{}
		if cmd := model.startServiceShell(paletteAction{ServiceKey: "missing", Title: "Open missing shell"}); cmd == nil || model.banner == nil || model.banner.Message != "selected service is no longer available" {
			t.Fatalf("expected missing service shell warning, cmd=%v banner=%+v", cmd, model.banner)
		}

		model = Model{snapshot: Snapshot{Services: []Service{postgres}}}
		if cmd := model.startServiceShell(paletteAction{ServiceKey: serviceKey(postgres), Title: "Open Postgres shell"}); cmd == nil || model.banner == nil || model.banner.Message != "service shell handoff is unavailable in this model" {
			t.Fatalf("expected unavailable service shell warning, cmd=%v banner=%+v", cmd, model.banner)
		}
	})
}
