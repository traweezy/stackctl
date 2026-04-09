package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModelAndPaletteEdgeCases(t *testing.T) {
	t.Run("applyOptimisticUpdate handles restart-stack actions", func(t *testing.T) {
		base := Snapshot{
			Services: []Service{{Name: "postgres", ContainerName: "stack-postgres", Status: "running", PortListening: true, PortConflict: true}},
			Stacks:   []StackProfile{{Name: "dev-stack", Current: true, State: "running"}},
		}

		updated := applyOptimisticUpdate(base, ActionID("restart-stack:dev-stack"))
		if updated.Stacks[0].State != "restarting" || updated.Services[0].Status != "restarting" {
			t.Fatalf("unexpected optimistic restart state: %+v %+v", updated.Stacks, updated.Services)
		}
	})

	t.Run("selected helpers return false when the injected selection is missing", func(t *testing.T) {
		originalPickSelectedName := inspectPickSelectedName
		t.Cleanup(func() { inspectPickSelectedName = originalPickSelectedName })
		inspectPickSelectedName = func(string, []string) string { return "missing" }

		snapshot := Snapshot{
			Services: []Service{{Name: "postgres"}},
			Stacks:   []StackProfile{{Name: "dev-stack"}},
		}
		if _, ok := selectedService(snapshot, "postgres"); ok {
			t.Fatal("expected selectedService to fail when the injected selection is missing")
		}
		if _, ok := selectedStackProfile(snapshot, "dev-stack"); ok {
			t.Fatal("expected selectedStackProfile to fail when the injected selection is missing")
		}
	})

	t.Run("renderStackDetailPane and stackWorkflowLines cover empty and unconfigured stacks", func(t *testing.T) {
		rendered := stripANSITest(renderStackDetailPane(StackProfile{Name: "dev-stack"}, expandedLayout))
		if !strings.Contains(strings.ToLower(rendered), "unknown") {
			t.Fatalf("expected blank stack states to render as unknown, got %q", rendered)
		}

		lines := stackWorkflowLines(StackProfile{Name: "dev-stack", Configured: false}, expandedLayout)
		if len(lines) == 0 || !strings.Contains(lines[0], "Save defaults from Config") {
			t.Fatalf("unexpected unconfigured workflow lines: %+v", lines)
		}
	})

	t.Run("palette filtering pagination and actions cover remaining branches", func(t *testing.T) {
		state := newPaletteState(paletteModeCommand, "Command", "Prompt", []paletteAction{
			{Title: "Alpha", Search: "alpha", ServiceKey: "postgres"},
			{Title: "Alpha", Search: "alpha", ServiceKey: "redis"},
		})
		state.input.SetValue("alpha")
		state.applyFilter()
		if len(state.filtered) != 2 || state.filtered[0].ServiceKey != "postgres" || state.filtered[1].ServiceKey != "redis" {
			t.Fatalf("expected equal-score matches to preserve input order, got %+v", state.filtered)
		}

		state.filtered = nil
		state.syncPagination()
		if state.paginator.Page != 0 || state.paginator.TotalPages != 0 || state.offset != 0 {
			t.Fatalf("unexpected empty pagination state: page=%d total=%d offset=%d", state.paginator.Page, state.paginator.TotalPages, state.offset)
		}

		state.filtered = []paletteAction{{Title: "One"}}
		state.pageSize = 1
		state.selected = 10
		state.syncPagination()
		if state.paginator.Page != 0 {
			t.Fatalf("expected oversized selections to clamp to page 0, got %d", state.paginator.Page)
		}
		state.selected = -1
		state.syncPagination()
		if state.paginator.Page != 0 {
			t.Fatalf("expected negative selections to clamp to page 0, got %d", state.paginator.Page)
		}

		if targets := serviceCopyTargets(Service{}, false); len(targets) != 0 {
			t.Fatalf("expected empty service copy targets to stay empty, got %+v", targets)
		}
		if targets := serviceCopyTargets(Service{DisplayName: "Postgres", DSN: "   "}, false); len(targets) != 0 {
			t.Fatalf("expected blank copy target values to be skipped, got %+v", targets)
		}

		model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
		model.layout = compactLayout
		model.configManager = &ConfigManager{}
		model.configEditor = newConfigEditor()
		updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		model = updatedModel.(Model)
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleLayout}); cmd != nil || model.layout != expandedLayout {
			t.Fatalf("expected toggle layout to switch back to expanded, cmd=%v layout=%v", cmd, model.layout)
		}
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionToggleSecrets}); cmd != nil || !model.showSecrets {
			t.Fatalf("expected toggle secrets to enable secrets, cmd=%v showSecrets=%v", cmd, model.showSecrets)
		}
		if cmd := model.executePaletteAction(paletteAction{Kind: paletteActionKind(999)}); cmd != nil {
			t.Fatalf("expected unknown palette actions to be ignored, got %v", cmd)
		}

		handoffCmd := model.startHandoffAction(
			paletteAction{Kind: paletteActionExecShell, Title: "Open shell"},
			stubExecCommand{},
			"info",
			"opening shell",
			"closed shell",
			true,
		)
		if handoffCmd == nil || model.runningHandoff == nil || len(model.history) == 0 {
			t.Fatalf("expected startHandoffAction to queue history and handoff state, cmd=%v history=%+v handoff=%+v", handoffCmd, model.history, model.runningHandoff)
		}
		handoffDone, ok := handoffDoneCallback(model.runningHandoff.History, paletteAction{Kind: paletteActionExecShell, Title: "Open shell"}, "closed shell", true)(nil).(handoffDoneMsg)
		if !ok || handoffDone.message != "closed shell" || !handoffDone.refresh {
			t.Fatalf("expected handoff callback to produce a completion message, handoff=%+v", handoffDone)
		}
	})

	t.Run("model action keys and healthNote cover remaining branches", func(t *testing.T) {
		snapshot := Snapshot{
			Stacks:   []StackProfile{{Name: "dev-stack", Current: true, Configured: true, State: "running"}},
			Services: []Service{{Name: "postgres", Status: "running"}},
		}
		model := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
		model.runner = func(ActionID) (ActionReport, error) { return ActionReport{}, nil }

		updated, cmd := model.Update(tea.KeyPressMsg{Code: '1', Text: ""})
		if cmd != nil {
			t.Fatalf("expected invalid action index to be ignored, got %v", cmd)
		}
		if _, ok := updated.(Model); !ok {
			t.Fatalf("expected invalid action key to preserve the model type, got %T", updated)
		}

		note := healthNote(Service{Name: "postgres", ContainerName: "stack-postgres", Status: "missing"})
		if note != "Managed container is not present yet." {
			t.Fatalf("unexpected missing managed-service note %q", note)
		}
	})
}
