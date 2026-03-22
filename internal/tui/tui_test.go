package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModelInitLoadsSnapshot(t *testing.T) {
	model := NewModel(func() (Snapshot, error) {
		return Snapshot{StackName: "dev-stack"}, nil
	})

	msg := model.Init()()
	loaded, ok := msg.(snapshotMsg)
	if !ok {
		t.Fatalf("expected snapshotMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("unexpected load error: %v", loaded.err)
	}
	if loaded.snapshot.StackName != "dev-stack" {
		t.Fatalf("unexpected snapshot: %+v", loaded.snapshot)
	}
}

func TestModelNavigatesSectionsAndRefreshes(t *testing.T) {
	loadCount := 0
	model := NewModel(func() (Snapshot, error) {
		loadCount++
		return Snapshot{StackName: fmt.Sprintf("stack-%d", loadCount)}, nil
	})

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	updatedModel, _ = current.Update(snapshotMsg{snapshot: Snapshot{StackName: "stack-1"}})
	current = updatedModel.(Model)
	if current.snapshot.StackName != "stack-1" {
		t.Fatalf("unexpected initial snapshot: %+v", current.snapshot)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updatedModel.(Model)
	if current.active != servicesSection {
		t.Fatalf("expected services section, got %v", current.active)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	current = updatedModel.(Model)
	if current.active != overviewSection {
		t.Fatalf("expected overview section, got %v", current.active)
	}

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	current = updatedModel.(Model)
	if !current.loading {
		t.Fatalf("expected loading state during refresh")
	}
	if cmd == nil {
		t.Fatalf("expected refresh command")
	}

	refreshMsg := cmd()
	loaded, ok := refreshMsg.(snapshotMsg)
	if !ok {
		t.Fatalf("expected refresh snapshotMsg, got %T", refreshMsg)
	}
	if loaded.snapshot.StackName != "stack-1" {
		t.Fatalf("unexpected refreshed snapshot: %+v", loaded.snapshot)
	}
	if loadCount != 1 {
		t.Fatalf("expected loader to run once during refresh, got %d", loadCount)
	}
}

func TestViewMasksSecretsUntilToggled(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName: "Postgres",
				Status:      "running",
				Username:    "app",
				Password:    "secret-password",
				DSN:         "postgres://app:secret-password@localhost:5432/app",
			},
		},
		Connections: []Connection{
			{Name: "Postgres", Value: "postgres://app:secret-password@localhost:5432/app"},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updatedModel.(Model)

	view := current.View().Content
	if strings.Contains(view, "secret-password") {
		t.Fatalf("expected secrets to be masked by default:\n%s", view)
	}
	if !strings.Contains(view, maskedSecret) {
		t.Fatalf("expected masked secrets in view:\n%s", view)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	current = updatedModel.(Model)
	view = current.View().Content
	if !strings.Contains(view, "secret-password") {
		t.Fatalf("expected secrets to be visible after toggle:\n%s", view)
	}
}

func TestServicesViewShowsCockpitRuntimeDetails(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:  "Cockpit",
				Status:       "running",
				Host:         "devbox",
				ExternalPort: 9090,
				URL:          "https://devbox:9090",
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updatedModel.(Model)

	view := current.View().Content
	for _, fragment := range []string{
		"●  Cockpit",
		"Status: running",
		"Host: devbox",
		"Port: 9090 -> unknown",
		"URL: https://devbox:9090",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected cockpit services view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestModelRendersInitialErrorState(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	updatedModel, _ = current.Update(snapshotMsg{err: fmt.Errorf("no stackctl config was found")})
	current = updatedModel.(Model)

	view := current.View().Content
	if !strings.Contains(view, "Dashboard unavailable") {
		t.Fatalf("expected error state in view:\n%s", view)
	}
	if !strings.Contains(view, "no stackctl config was found") {
		t.Fatalf("expected error message in view:\n%s", view)
	}
}
