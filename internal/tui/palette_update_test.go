package tui

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	bubblespinner "charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/traweezy/stackctl/internal/output"
)

func TestPaletteAndUpdateAdditionalCoverage(t *testing.T) {
	postgres := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		ContainerName: "stack-postgres",
		Status:        "running",
		DSN:           "postgres://app:app@localhost:5432/app",
		Database:      "app",
		Username:      "app",
		Password:      "secret",
	}
	hostTool := Service{
		Name:        "cockpit",
		DisplayName: "Cockpit",
		Status:      "running",
		URL:         "https://localhost:9090",
	}

	t.Run("selected lifecycle service accepts health stack selections", func(t *testing.T) {
		model := Model{
			snapshot: Snapshot{Services: []Service{postgres, hostTool}},
			active:   healthSection,
		}

		model.selectedHealth = serviceKey(postgres)
		service, ok := model.selectedLifecycleService()
		if !ok || service.Name != "postgres" {
			t.Fatalf("expected health lifecycle selection to resolve postgres, got service=%+v ok=%v", service, ok)
		}

		model.selectedHealth = serviceKey(hostTool)
		if _, ok := model.selectedLifecycleService(); ok {
			t.Fatal("expected host tools to be rejected for lifecycle actions")
		}

		model.selectedHealth = "missing"
		service, ok = model.selectedLifecycleService()
		if !ok || service.Name != "postgres" {
			t.Fatalf("expected missing health selection to fall back to postgres, got service=%+v ok=%v", service, ok)
		}
	})

	t.Run("startSelectedLogWatch covers missing, host-tool, and success flows", func(t *testing.T) {
		model := Model{}
		cmd := model.startSelectedLogWatch()
		if cmd == nil || model.banner == nil || model.banner.Message != "select a service, port, or health target to watch logs" {
			t.Fatalf("expected missing-selection warning, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = Model{
			snapshot:        Snapshot{Services: []Service{hostTool}},
			active:          servicesSection,
			selectedService: serviceKey(hostTool),
		}
		cmd = model.startSelectedLogWatch()
		if cmd == nil || model.banner == nil || model.banner.Message != "live logs are unavailable for host tools" {
			t.Fatalf("expected host-tool warning, got banner=%+v cmd=%v", model.banner, cmd)
		}

		model = Model{
			snapshot:       Snapshot{Services: []Service{postgres}},
			active:         healthSection,
			selectedHealth: serviceKey(postgres),
			logWatchLauncher: func(request LogWatchRequest) (tea.ExecCommand, error) {
				if request.Service != "postgres" {
					t.Fatalf("unexpected log-watch service %q", request.Service)
				}
				return stubExecCommand{}, nil
			},
		}
		cmd = model.startSelectedLogWatch()
		if cmd == nil {
			t.Fatal("expected successful log watch to return a handoff command")
		}
		if model.runningHandoff == nil || model.runningHandoff.Action.Kind != paletteActionWatchLogs {
			t.Fatalf("expected running handoff to be tracked, got %+v", model.runningHandoff)
		}
	})

	t.Run("executePaletteAction covers handoff actions and unknown kinds", func(t *testing.T) {
		model := Model{
			snapshot: Snapshot{Services: []Service{postgres}},
			logWatchLauncher: func(request LogWatchRequest) (tea.ExecCommand, error) {
				if request.Service != "postgres" {
					t.Fatalf("unexpected log-watch request: %+v", request)
				}
				return stubExecCommand{}, nil
			},
			shellLauncher: func(request ServiceShellRequest) (tea.ExecCommand, error) {
				if request.Service != "postgres" {
					t.Fatalf("unexpected shell request: %+v", request)
				}
				return stubExecCommand{}, nil
			},
			dbShellLauncher: func(request DBShellRequest) (tea.ExecCommand, error) {
				if request.Service != "postgres" {
					t.Fatalf("unexpected db-shell request: %+v", request)
				}
				return stubExecCommand{}, nil
			},
		}

		logCmd := model.executePaletteAction(paletteAction{
			Kind:       paletteActionWatchLogs,
			Title:      "Watch Postgres logs",
			ServiceKey: serviceKey(postgres),
		})
		if logCmd == nil || model.runningHandoff == nil || model.runningHandoff.Action.Kind != paletteActionWatchLogs {
			t.Fatalf("expected watch-logs action to start a handoff, got cmd=%v handoff=%+v", logCmd, model.runningHandoff)
		}

		shellCmd := model.executePaletteAction(paletteAction{
			Kind:       paletteActionExecShell,
			Title:      "Open Postgres shell",
			ServiceKey: serviceKey(postgres),
		})
		if shellCmd == nil || model.runningHandoff == nil || model.runningHandoff.Action.Kind != paletteActionExecShell {
			t.Fatalf("expected service-shell action to start a handoff, got cmd=%v handoff=%+v", shellCmd, model.runningHandoff)
		}

		dbCmd := model.executePaletteAction(paletteAction{
			Kind:       paletteActionDBShell,
			Title:      "Open Postgres db shell",
			ServiceKey: serviceKey(postgres),
		})
		if dbCmd == nil || model.runningHandoff == nil || model.runningHandoff.Action.Kind != paletteActionDBShell {
			t.Fatalf("expected db-shell action to start a handoff, got cmd=%v handoff=%+v", dbCmd, model.runningHandoff)
		}

		if cmd := model.executePaletteAction(paletteAction{}); cmd != nil {
			t.Fatalf("expected unknown palette actions to be ignored, got %v", cmd)
		}
	})

	t.Run("completeHandoff handles stale, fallback, and failure branches", func(t *testing.T) {
		model := Model{}
		if cmd := model.completeHandoff(handoffDoneMsg{historyID: 99}); cmd != nil {
			t.Fatalf("expected stale handoff completion to be ignored, got %v", cmd)
		}

		model = Model{
			history: []historyEntry{{
				ID:        7,
				Action:    "Open Postgres shell",
				Status:    output.StatusInfo,
				Message:   "opening Postgres shell",
				StartedAt: time.Now(),
			}},
			runningHandoff: &runningHandoff{History: 7},
		}
		cmd := model.completeHandoff(handoffDoneMsg{
			historyID: 7,
			action:    paletteAction{Title: "Open Postgres shell"},
			refresh:   false,
		})
		if cmd == nil {
			t.Fatal("expected handoff completion to return a banner clear command")
		}
		if model.runningHandoff != nil {
			t.Fatalf("expected running handoff to be cleared, got %+v", model.runningHandoff)
		}
		if got := model.history[0].Message; got != "Open Postgres shell completed" {
			t.Fatalf("unexpected fallback completion message %q", got)
		}
		if model.history[0].Status != output.StatusOK {
			t.Fatalf("expected successful handoff status, got %+v", model.history[0])
		}

		model = Model{
			history: []historyEntry{{
				ID:        9,
				Action:    "Open Postgres shell",
				Status:    output.StatusInfo,
				Message:   "opening Postgres shell",
				StartedAt: time.Now(),
			}},
			runningHandoff: &runningHandoff{History: 9},
		}
		cmd = model.completeHandoff(handoffDoneMsg{
			historyID: 9,
			action:    paletteAction{Title: "Open Postgres shell"},
			err:       errors.New("exec failed"),
		})
		if cmd == nil {
			t.Fatal("expected failing handoff to return a banner clear command")
		}
		if model.history[0].Status != output.StatusFail {
			t.Fatalf("expected failing handoff status, got %+v", model.history[0])
		}
		if !strings.Contains(model.history[0].Message, "open postgres shell failed: exec failed") {
			t.Fatalf("unexpected failing handoff message: %+v", model.history[0])
		}
	})

	t.Run("completeCopy records failure history", func(t *testing.T) {
		model := Model{}
		cmd := model.completeCopy(copyDoneMsg{
			action: paletteAction{
				Kind:  paletteActionCopyValue,
				Title: "Copy Postgres DSN",
			},
			err: errors.New("clipboard unavailable"),
		})
		if cmd == nil {
			t.Fatal("expected copy completion to clear its banner")
		}
		if len(model.history) != 1 {
			t.Fatalf("expected copy completion history entry, got %+v", model.history)
		}
		if model.history[0].Status != output.StatusFail {
			t.Fatalf("expected failing copy status, got %+v", model.history[0])
		}
		if !strings.Contains(model.history[0].Message, "copy postgres dsn failed: clipboard unavailable") {
			t.Fatalf("unexpected failing copy message: %+v", model.history[0])
		}
		if model.history[0].Recent == nil || model.history[0].Recent.Title != "Copy Postgres DSN" {
			t.Fatalf("expected recent copy action to be recorded, got %+v", model.history[0])
		}
	})

	t.Run("palette paging clamps empty and minimum sizes", func(t *testing.T) {
		state := newPaletteState(paletteModeCommand, "Command palette", "Choose an action", nil)
		state.page(1)
		if state.selected != 0 || state.offset != 0 {
			t.Fatalf("expected empty palette paging to stay at zero, got selected=%d offset=%d", state.selected, state.offset)
		}
		state.setPageSize(0)
		if state.pageSize != 1 {
			t.Fatalf("expected page size to clamp to one, got %d", state.pageSize)
		}
	})

	t.Run("update handles keyboard, background, and idle spinner ticks", func(t *testing.T) {
		model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil }).WithAltScreen(true)

		updatedModel, _ := model.Update(tea.KeyboardEnhancementsMsg{Flags: 1})
		current := updatedModel.(Model)
		if !current.keyboardFeatures.SupportsKeyDisambiguation() {
			t.Fatalf("expected keyboard enhancement flags to be stored, got %+v", current.keyboardFeatures)
		}

		updatedModel, _ = current.Update(tea.BackgroundColorMsg{Color: color.Black})
		current = updatedModel.(Model)
		if !current.backgroundDark {
			t.Fatal("expected dark background detection to be stored")
		}

		current.loading = false
		updatedModel, cmd := current.Update(bubblespinner.TickMsg{})
		current = updatedModel.(Model)
		if cmd != nil {
			t.Fatalf("expected idle spinner tick not to schedule a follow-up, got %v", cmd)
		}

		view := current.View().Content
		if !strings.Contains(view, "stackctl tui") {
			t.Fatalf("expected rendered view to include the title, got:\n%s", view)
		}
	})
}
