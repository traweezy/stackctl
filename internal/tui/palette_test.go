package tui

import (
	"errors"
	"strings"
	"testing"

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
