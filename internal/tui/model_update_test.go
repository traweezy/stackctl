package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestModelInitAndViewBranches(t *testing.T) {
	cfg := configpkg.Default()
	snapshot := tuiTestSnapshot(cfg, nil)

	model := NewModel(func() (Snapshot, error) { return snapshot, nil }).WithAltScreen(true).WithMouse(true)
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected init command batch")
	}
	if _, ok := findMsgOfType[snapshotMsg](cmd()); !ok {
		t.Fatal("expected init batch to include a snapshot load message")
	}

	loadingView := model.View()
	if !strings.Contains(loadingView.Content, "Loading stackctl tui...") {
		t.Fatalf("expected zero-size view to show loading text, got:\n%s", loadingView.Content)
	}
	if !loadingView.AltScreen {
		t.Fatal("expected loading view to preserve alt-screen preference")
	}

	current := loadSnapshotModel(t, model, snapshot)
	rendered := current.View()
	if !rendered.AltScreen {
		t.Fatal("expected loaded view to preserve alt-screen preference")
	}
	if rendered.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected mouse mode to enable cell motion, got %v", rendered.MouseMode)
	}
	if !strings.Contains(rendered.Content, "stackctl tui") {
		t.Fatalf("expected loaded view to render the application title, got:\n%s", rendered.Content)
	}
}

func TestModelUpdateCoversKeyGuardAndToggleBranches(t *testing.T) {
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

	model := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)

	updated, cmd := model.Update(tea.KeyPressMsg{Text: ":"})
	current := updated.(Model)
	if cmd != nil || current.palette == nil {
		t.Fatalf("expected open-palette key to show the command palette, cmd=%v palette=%+v", cmd, current.palette)
	}

	model.runningAction = &runningAction{}
	updated, cmd = model.Update(tea.KeyPressMsg{Text: ":"})
	current = updated.(Model)
	if cmd != nil || current.palette != nil {
		t.Fatalf("expected busy model to ignore palette open, cmd=%v palette=%+v", cmd, current.palette)
	}

	current.runningAction = nil
	current.active = configSection
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	current = updated.(Model)
	if cmd != nil || current.palette != nil {
		t.Fatalf("expected quick jump to stay disabled in config view, cmd=%v palette=%+v", cmd, current.palette)
	}

	current.active = servicesSection
	current.selectedService = serviceKey(postgres)
	updated, _ = current.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	current = updated.(Model)
	if current.palette == nil {
		t.Fatal("expected quick jump to open a palette in services view")
	}
	current.palette = nil

	current.active = overviewSection
	current.palette = nil
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	current = updated.(Model)
	if cmd != nil || current.palette != nil {
		t.Fatalf("expected copy-value key to ignore overview view, cmd=%v palette=%+v", cmd, current.palette)
	}

	current.active = servicesSection
	current.selectedService = serviceKey(postgres)
	updated, _ = current.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	current = updated.(Model)
	if current.palette == nil {
		t.Fatal("expected copy-value key to open the copy palette")
	}

	current.palette = nil
	updated, cmd = current.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	current = updated.(Model)
	if cmd != nil || !current.help.ShowAll {
		t.Fatalf("expected help toggle to expand help, cmd=%v showAll=%v", cmd, current.help.ShowAll)
	}

	updated, cmd = current.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	current = updated.(Model)
	if cmd != nil || !current.showSecrets {
		t.Fatalf("expected secrets toggle to flip on, cmd=%v showSecrets=%v", cmd, current.showSecrets)
	}

	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	current = updated.(Model)
	if cmd != nil || current.layout != compactLayout {
		t.Fatalf("expected layout toggle to switch to compact, cmd=%v layout=%v", cmd, current.layout)
	}

	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	current = updated.(Model)
	if cmd != nil || current.autoRefresh {
		t.Fatalf("expected first auto-refresh toggle to disable refresh, cmd=%v autoRefresh=%v", cmd, current.autoRefresh)
	}

	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	current = updated.(Model)
	if cmd == nil || !current.autoRefresh {
		t.Fatalf("expected second auto-refresh toggle to re-enable refresh, cmd=%v autoRefresh=%v", cmd, current.autoRefresh)
	}

	beforeSelection := current.selectedService
	current.active = overviewSection
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	current = updated.(Model)
	if cmd != nil || current.selectedService != beforeSelection {
		t.Fatalf("expected next-item key to ignore non-list sections, cmd=%v before=%q after=%q", cmd, beforeSelection, current.selectedService)
	}

	current.active = servicesSection
	current.selectedService = serviceKey(postgres)
	updated, cmd = current.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	current = updated.(Model)
	if cmd == nil || current.selectedService == "" {
		t.Fatalf("expected next-item key to cycle services with auto-refresh, cmd=%v selected=%q", cmd, current.selectedService)
	}

	activeBefore := current.active
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	current = updated.(Model)
	if cmd == nil || current.active == activeBefore {
		t.Fatalf("expected previous-section key to change sections and schedule refresh, cmd=%v active=%v", cmd, current.active)
	}
}

func TestModelUpdateCoversProductivityActions(t *testing.T) {
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

	model := NewInspectionModel(
		func() (Snapshot, error) { return snapshot, nil },
		func(request LogWatchRequest) (tea.ExecCommand, error) {
			if request.Service != "postgres" {
				t.Fatalf("unexpected log watch target %q", request.Service)
			}
			return stubExecCommand{}, nil
		},
		nil,
	).WithProductivity(
		func(string) error { return nil },
		func(request ServiceShellRequest) (tea.ExecCommand, error) {
			if request.Service != "postgres" {
				t.Fatalf("unexpected shell target %q", request.Service)
			}
			return stubExecCommand{}, nil
		},
		func(request DBShellRequest) (tea.ExecCommand, error) {
			if request.Service != "postgres" {
				t.Fatalf("unexpected db shell target %q", request.Service)
			}
			return stubExecCommand{}, nil
		},
	)
	current := loadSnapshotModel(t, model, snapshot)
	current.active = servicesSection
	current.selectedService = serviceKey(postgres)

	updated, cmd := current.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	current = updated.(Model)
	if cmd == nil || current.runningHandoff == nil || current.runningHandoff.Action.Kind != paletteActionExecShell {
		t.Fatalf("expected exec-shell shortcut to start a handoff, cmd=%v handoff=%+v", cmd, current.runningHandoff)
	}

	current.runningHandoff = nil
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	current = updated.(Model)
	if cmd == nil || current.runningHandoff == nil || current.runningHandoff.Action.Kind != paletteActionDBShell {
		t.Fatalf("expected db-shell shortcut to start a handoff, cmd=%v handoff=%+v", cmd, current.runningHandoff)
	}

	current.runningHandoff = nil
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	current = updated.(Model)
	if cmd == nil {
		t.Fatal("expected pin-service shortcut to return a banner clear command")
	}
	if _, ok := current.pinnedServices[serviceKey(postgres)]; !ok {
		t.Fatalf("expected selected service to be pinned, got %+v", current.pinnedServices)
	}

	current.active = healthSection
	current.selectedHealth = serviceKey(hostTool)
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	current = updated.(Model)
	if cmd == nil || current.banner == nil || current.banner.Message != "live logs are unavailable for host tools" {
		t.Fatalf("expected host-tool watch logs warning, cmd=%v banner=%+v", cmd, current.banner)
	}

	current.banner = nil
	current.active = servicesSection
	current.selectedService = serviceKey(postgres)
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	current = updated.(Model)
	if cmd == nil || current.runningHandoff == nil || current.runningHandoff.Action.Kind != paletteActionWatchLogs {
		t.Fatalf("expected watch-logs shortcut to start a handoff, cmd=%v handoff=%+v", cmd, current.runningHandoff)
	}
}

func tuiTestSnapshot(cfg configpkg.Config, services []Service) Snapshot {
	cfg.ApplyDerivedFields()
	if services == nil {
		services = []Service{}
	}
	return Snapshot{
		ConfigData:        cfg,
		ConfigPath:        "/tmp/stackctl/config.yaml",
		StackName:         cfg.Stack.Name,
		StackDir:          cfg.Stack.Dir,
		ComposePath:       configpkg.ComposePath(cfg),
		Managed:           cfg.Stack.Managed,
		WaitForServices:   cfg.Behavior.WaitForServicesStart,
		StartupTimeoutSec: cfg.Behavior.StartupTimeoutSec,
		LoadedAt:          time.Now(),
		Services:          services,
		Stacks: []StackProfile{
			{
				Name:       cfg.Stack.Name,
				ConfigPath: "/tmp/stackctl/config.yaml",
				Current:    true,
				Configured: true,
				State:      "running",
				Mode:       overviewModeLabel(cfg.Stack.Managed),
			},
		},
	}
}

func loadSnapshotModel(t *testing.T, model Model, snapshot Snapshot) Model {
	t.Helper()

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	current := updatedModel.(Model)
	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	return updatedModel.(Model)
}
