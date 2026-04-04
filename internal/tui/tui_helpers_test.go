package tui

import (
	"errors"
	"testing"
)

func TestTUISelectionAndSidebarHelpers(t *testing.T) {
	snapshot := Snapshot{
		Services: []Service{
			{
				Name:          "postgres",
				DisplayName:   "Postgres Primary",
				Status:        "running",
				ContainerName: "stack-postgres",
			},
			{
				Name:        "cockpit",
				DisplayName: "Cockpit Dashboard",
				Status:      "running",
				URL:         "https://localhost:9090",
			},
		},
		Stacks: []StackProfile{
			{Name: "dev-stack", Current: true, State: "running"},
			{Name: "observability-stack", Current: false, State: "stopped"},
		},
	}

	t.Run("cycleActiveSelection updates the active list only", func(t *testing.T) {
		model := Model{
			snapshot:        snapshot,
			active:          stacksSection,
			selectedStack:   "dev-stack",
			selectedService: "postgres",
			selectedHealth:  "postgres",
		}
		if cmd := model.cycleActiveSelection(1); cmd != nil {
			t.Fatalf("expected no command from cycleActiveSelection, got %T", cmd)
		}
		if model.selectedStack != "observability-stack" {
			t.Fatalf("expected stack selection to advance, got %q", model.selectedStack)
		}

		model.active = servicesSection
		model.selectedService = "postgres"
		model.cycleActiveSelection(1)
		if model.selectedService != serviceKey(snapshot.Services[1]) {
			t.Fatalf("expected service selection to advance, got %q", model.selectedService)
		}

		model.active = healthSection
		model.selectedHealth = serviceKey(snapshot.Services[1])
		model.cycleActiveSelection(1)
		if model.selectedHealth != serviceKey(snapshot.Services[0]) {
			t.Fatalf("expected health selection to wrap, got %q", model.selectedHealth)
		}
	})

	t.Run("selectedLifecycleService filters out host tools", func(t *testing.T) {
		model := Model{snapshot: snapshot, active: servicesSection, selectedService: "postgres"}
		service, ok := model.selectedLifecycleService()
		if !ok || service.Name != "postgres" {
			t.Fatalf("expected stack service lifecycle selection, got service=%+v ok=%v", service, ok)
		}

		model.selectedService = serviceKey(snapshot.Services[1])
		if rejectedService, ok := model.selectedLifecycleService(); ok {
			t.Fatalf("expected host tool lifecycle selection to be rejected, got service=%+v ok=%v", rejectedService, ok)
		}

		model.active = historySection
		if rejectedService, ok := model.selectedLifecycleService(); ok {
			t.Fatalf("expected non-service section to return no lifecycle selection, got service=%+v ok=%v", rejectedService, ok)
		}
	})

	t.Run("watchLogsErrorMessage and previousSection cover fallback labels", func(t *testing.T) {
		if got := watchLogsErrorMessage("", errors.New("boom")); got != "watch logs for selected service failed: boom" {
			t.Fatalf("unexpected fallback watch logs message: %q", got)
		}
		if got := watchLogsErrorMessage("Postgres", errors.New("boom")); got != "watch logs for postgres failed: boom" {
			t.Fatalf("unexpected named watch logs message: %q", got)
		}
		if got := previousSection(overviewSection); got != historySection {
			t.Fatalf("expected previousSection to wrap from overview to history, got %v", got)
		}
		if got := previousSection(servicesSection); got != configSection {
			t.Fatalf("expected previousSection to move backward, got %v", got)
		}
	})

	t.Run("sidebarCompactSelectionLabel reflects the active selection", func(t *testing.T) {
		model := Model{
			snapshot:        snapshot,
			active:          stacksSection,
			selectedStack:   "observability-stack",
			selectedService: "postgres",
			selectedHealth:  serviceKey(snapshot.Services[1]),
			history: []historyEntry{
				{Action: "Open observability dashboard"},
			},
		}
		if got := sidebarCompactSelectionLabel(model); got != "observabili…" {
			t.Fatalf("unexpected compact stack label: %q", got)
		}

		model.active = servicesSection
		if got := sidebarCompactSelectionLabel(model); got != "Postgres Pr…" {
			t.Fatalf("unexpected compact service label: %q", got)
		}

		model.active = healthSection
		if got := sidebarCompactSelectionLabel(model); got != "Cockpit Das…" {
			t.Fatalf("unexpected compact health label: %q", got)
		}

		model.active = historySection
		if got := sidebarCompactSelectionLabel(model); got != "Open observ…" {
			t.Fatalf("unexpected compact history label: %q", got)
		}
	})
}
