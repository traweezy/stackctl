package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func TestModelUpdateFallsBackToViewportForUnhandledMessages(t *testing.T) {
	cfg := configpkg.Default()
	snapshot := tuiTestSnapshot(cfg, nil)

	current := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
	updated, cmd := current.Update(struct{ Name string }{Name: "unhandled"})
	next := updated.(Model)

	if next.active != current.active {
		t.Fatalf("expected unhandled message to preserve active section, before=%v after=%v", current.active, next.active)
	}
	if cmd != nil {
		t.Fatalf("expected viewport fallback for an unhandled message not to schedule a command, got %v", cmd)
	}
}

func TestHandleConfigKeySaveCommitsEditsAndStartsSaveFlow(t *testing.T) {
	cfg := configpkg.Default()
	saveCalls := 0
	manager := configTestManager()
	manager.SaveConfig = func(string, configpkg.Config) error {
		saveCalls++
		return nil
	}

	model := newConfigTestModel(cfg, manager)
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "ports.postgres"
	current.configEditor.refreshList(false)

	if cmd := current.configEditor.beginEdit(false); cmd == nil {
		t.Fatal("expected postgres port edit to begin")
	}
	current.configEditor.input.SetValue("15432")

	cmd, handled := current.handleConfigKey(tea.KeyPressMsg(tea.Key{Mod: tea.ModCtrl, Code: 's'}))
	if !handled || cmd == nil || current.runningConfigOp == nil {
		t.Fatalf("expected ctrl+s to commit the edit and start saving, handled=%v cmd=%v op=%+v", handled, cmd, current.runningConfigOp)
	}
	if current.configEditor.editing {
		t.Fatal("expected ctrl+s save path to leave edit mode after commit")
	}

	result := msgOfType[configOperationMsg](t, cmd)
	if result.Status != output.StatusOK || !result.Reload {
		t.Fatalf("unexpected ctrl+s save result: %+v", result)
	}
	if saveCalls != 1 {
		t.Fatalf("expected one save call after ctrl+s, got %d", saveCalls)
	}
}

func TestExecutePaletteActionSidebarUpdatesRecentHistory(t *testing.T) {
	cfg := configpkg.Default()
	snapshot := tuiTestSnapshot(cfg, nil)

	current := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
	current.runner = func(ActionID) (ActionReport, error) { return ActionReport{}, nil }

	cmd := current.executePaletteAction(paletteAction{
		Kind: paletteActionSidebar,
		Action: ActionSpec{
			ID:             ActionDoctor,
			Label:          "Doctor",
			Group:          "Sidebar",
			PendingStatus:  output.StatusInfo,
			PendingMessage: "Running doctor",
			DefaultStatus:  output.StatusOK,
		},
	})
	if cmd == nil || current.runningAction == nil {
		t.Fatalf("expected sidebar palette action to start running, cmd=%v action=%+v", cmd, current.runningAction)
	}
	if len(current.history) == 0 {
		t.Fatal("expected sidebar action to append a history entry")
	}

	recent := current.history[len(current.history)-1].Recent
	if recent == nil || recent.Search != "doctor sidebar" {
		t.Fatalf("expected recent action metadata to be recorded, got %+v", recent)
	}
}
