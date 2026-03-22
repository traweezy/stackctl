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

func TestModelAutoRefreshSchedulesAndCanBeDisabled(t *testing.T) {
	previousInterval := autoRefreshInterval
	autoRefreshInterval = 0
	defer func() {
		autoRefreshInterval = previousInterval
	}()

	loadCount := 0
	model := NewModel(func() (Snapshot, error) {
		loadCount++
		return Snapshot{StackName: fmt.Sprintf("stack-%d", loadCount)}, nil
	})

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	updatedModel, cmd := current.Update(snapshotMsg{snapshot: Snapshot{StackName: "stack-0"}})
	current = updatedModel.(Model)
	if !current.autoRefresh {
		t.Fatalf("expected auto-refresh to be enabled by default")
	}
	if cmd == nil {
		t.Fatalf("expected auto-refresh command after snapshot load")
	}

	autoTick := cmd()
	autoMsg, ok := autoTick.(autoRefreshMsg)
	if !ok {
		t.Fatalf("expected autoRefreshMsg, got %T", autoTick)
	}

	updatedModel, cmd = current.Update(autoMsg)
	current = updatedModel.(Model)
	if !current.loading {
		t.Fatalf("expected loading state during auto-refresh")
	}
	if cmd == nil {
		t.Fatalf("expected loader command during auto-refresh")
	}

	loadedMsg := cmd()
	loaded, ok := loadedMsg.(snapshotMsg)
	if !ok {
		t.Fatalf("expected snapshotMsg from auto-refresh loader, got %T", loadedMsg)
	}
	if loaded.snapshot.StackName != "stack-1" {
		t.Fatalf("unexpected auto-refreshed snapshot: %+v", loaded.snapshot)
	}
	if loadCount != 1 {
		t.Fatalf("expected loader to run once during auto-refresh, got %d", loadCount)
	}

	updatedModel, cmd = current.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	current = updatedModel.(Model)
	if current.autoRefresh {
		t.Fatalf("expected auto-refresh to be disabled after toggle")
	}
	if cmd != nil {
		t.Fatalf("expected no timer command when auto-refresh is disabled")
	}
	if !strings.Contains(current.View().Content, "auto-refresh: off") {
		t.Fatalf("expected header to show auto-refresh disabled:\n%s", current.View().Content)
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
		"Host port: 9090",
		"URL: https://devbox:9090",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected cockpit services view to contain %q:\n%s", fragment, view)
		}
	}
	if strings.Contains(view, "unknown") {
		t.Fatalf("expected cockpit services view to avoid unknown placeholders:\n%s", view)
	}
}

func TestModelTogglesCompactLayout(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Image:         "docker.io/library/postgres:16",
				DataVolume:    "postgres_data",
				Host:          "localhost",
				ExternalPort:  5432,
				InternalPort:  5432,
				Database:      "app",
				MaintenanceDB: "postgres",
				Username:      "app",
				Password:      "secret-password",
				DSN:           "postgres://app:secret-password@localhost:5432/app",
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

	expandedView := current.View().Content
	for _, fragment := range []string{
		"layout: expanded",
		"Image: docker.io/library/postgres:16",
		"Data volume: postgres_data",
		"Maintenance DB: postgres",
		"Password: ****",
		"Copy placeholders: Postgres DSN",
	} {
		if !strings.Contains(expandedView, fragment) {
			t.Fatalf("expected expanded services view to contain %q:\n%s", fragment, expandedView)
		}
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	current = updatedModel.(Model)
	if current.layout != compactLayout {
		t.Fatalf("expected compact layout after toggle, got %v", current.layout)
	}

	compactView := current.View().Content
	for _, fragment := range []string{
		"layout: compact",
		"Status: running",
		"Host port: 5432",
		"DSN: postgres://app:****@localhost:5432/app",
		"Copy placeholders: Postgres DSN",
	} {
		if !strings.Contains(compactView, fragment) {
			t.Fatalf("expected compact services view to contain %q:\n%s", fragment, compactView)
		}
	}
	for _, fragment := range []string{
		"Image: docker.io/library/postgres:16",
		"Data volume: postgres_data",
		"Maintenance DB: postgres",
		"Password: ****",
	} {
		if strings.Contains(compactView, fragment) {
			t.Fatalf("expected compact services view to omit %q:\n%s", fragment, compactView)
		}
	}
}

func TestOverviewExcludesCockpitFromStackServiceCount(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "missing",
				ContainerName: "stack-postgres",
				ExternalPort:  5432,
			},
			{
				DisplayName:   "Redis",
				Status:        "missing",
				ContainerName: "stack-redis",
				ExternalPort:  6379,
			},
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

	view := current.View().Content
	for _, fragment := range []string{
		"Stack services running: 0 / 2",
		"Cockpit: running",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected overview to contain %q:\n%s", fragment, view)
		}
	}
}

func TestServicesViewUsesFriendlyStoppedLabels(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "missing",
				ContainerName: "stack-postgres",
				ExternalPort:  5432,
				Database:      "app",
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updatedModel.(Model)

	view := current.View().Content
	for _, fragment := range []string{
		"○  Postgres",
		"Status: not running",
		"Host port: 5432",
		"Database: app",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected services view to contain %q:\n%s", fragment, view)
		}
	}
	for _, fragment := range []string{
		"Status: missing",
		"unknown",
	} {
		if strings.Contains(view, fragment) {
			t.Fatalf("expected services view to avoid %q:\n%s", fragment, view)
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
