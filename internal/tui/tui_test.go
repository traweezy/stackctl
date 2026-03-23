package tui

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
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

func TestSplitPaneWidthsFavorSelectionLists(t *testing.T) {
	leftWidth, rightWidth, stacked := splitPaneWidths(120, defaultListPaneMinW, defaultListPaneMaxW)
	if stacked {
		t.Fatalf("expected wide layouts to stay split")
	}
	if leftWidth != 42 || rightWidth != 75 {
		t.Fatalf("unexpected service pane widths: left=%d right=%d", leftWidth, rightWidth)
	}

	_, _, stacked = splitPaneWidths(splitPaneMinWidth-1, defaultListPaneMinW, defaultListPaneMaxW)
	if !stacked {
		t.Fatalf("expected narrow layouts to stack")
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

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	current = updatedModel.(Model)
	if current.active != configSection {
		t.Fatalf("expected config section, got %v", current.active)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	current = updatedModel.(Model)
	if current.active != servicesSection {
		t.Fatalf("expected services section after config, got %v", current.active)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	current = updatedModel.(Model)
	if current.active != configSection {
		t.Fatalf("expected config section on the way back, got %v", current.active)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	current = updatedModel.(Model)
	if current.active != overviewSection {
		t.Fatalf("expected overview section after config, got %v", current.active)
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

func TestModelRefreshIntervalUsesSnapshotConfig(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	model.snapshot = Snapshot{ConfigData: configpkg.Config{TUI: configpkg.TUIConfig{AutoRefreshIntervalSec: 12}}}

	if got := model.refreshInterval(); got != 12*time.Second {
		t.Fatalf("expected refresh interval from config, got %s", got)
	}
}

func TestRenderHeaderPadsAndColorizesStatus(t *testing.T) {
	model := NewActionModel(func() (Snapshot, error) { return Snapshot{}, nil }, func(ActionID) (ActionReport, error) {
		return ActionReport{}, nil
	})
	model.snapshot = Snapshot{StackName: "dev-stack"}

	raw := renderHeader(model)
	plain := stripANSITest(raw)
	lines := strings.Split(plain, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multiline header, got %q", plain)
	}
	if !strings.HasPrefix(lines[0], "  stackctl tui") {
		t.Fatalf("expected title row to be padded right for alignment:\n%s", plain)
	}
	if !strings.HasPrefix(lines[1], " Refreshing  •") {
		t.Fatalf("expected status row to be padded and aligned:\n%s", plain)
	}
	if raw == plain {
		t.Fatalf("expected header status to include ANSI styling")
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
	current = navigateToSection(t, current, servicesSection)

	view := current.currentContent()
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
	current = navigateToSection(t, current, servicesSection)

	view := current.currentContent()
	for _, fragment := range []string{
		"Host tools",
		"Managed outside stack lifecycle.",
		"●  Cockpit",
		"Status: running",
		"Lifecycle: external to stack lifecycle",
		"Host: devbox",
		"Host port: 9090",
		"URL: https://devbox:9090",
	} {
		if !collapsedContainsTest(view, fragment) {
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
	current = navigateToSection(t, current, servicesSection)

	expandedView := current.View().Content
	expandedContent := current.currentContent()
	for _, fragment := range []string{
		"layout: expanded",
		"Image: docker.io/library/postgres:16",
		"Data volume: postgres_data",
		"Maintenance DB: postgres",
		"Password: ****",
	} {
		if !strings.Contains(expandedView, fragment) {
			t.Fatalf("expected expanded services view to contain %q:\n%s", fragment, expandedView)
		}
	}
	if !strings.Contains(expandedContent, "Actions: c copy") {
		t.Fatalf("expected expanded services content to contain service actions:\n%s", expandedContent)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	current = updatedModel.(Model)
	if current.layout != compactLayout {
		t.Fatalf("expected compact layout after toggle, got %v", current.layout)
	}

	compactView := current.View().Content
	compactContent := current.currentContent()
	for _, fragment := range []string{
		"layout: compact",
		"Status: running",
		"Host port: 5432",
		"DSN: postgres://app:****@localhost:5432/app",
	} {
		if !strings.Contains(compactView, fragment) {
			t.Fatalf("expected compact services view to contain %q:\n%s", fragment, compactView)
		}
	}
	if !strings.Contains(compactContent, "Actions: c copy") {
		t.Fatalf("expected compact services content to contain service actions:\n%s", compactContent)
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

func TestHealthViewShowsServiceCentricSummary(t *testing.T) {
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
				Host:          "localhost",
				ExternalPort:  5432,
				PortListening: true,
			},
			{
				DisplayName:   "Redis",
				Status:        "missing",
				ContainerName: "stack-redis",
				Host:          "localhost",
				ExternalPort:  6379,
				PortListening: false,
			},
			{
				DisplayName:   "pgAdmin",
				Status:        "running",
				ContainerName: "stack-pgadmin",
				Host:          "localhost",
				ExternalPort:  8081,
				PortListening: true,
				URL:           "http://localhost:8081",
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	current = navigateToSection(t, current, healthSection)

	view := current.currentContent()
	for _, fragment := range []string{
		"Healthy: 2",
		"Warnings: 0",
		"Not running: 1",
		"●  Postgres",
		"Status: healthy",
		"Reachability: accepting on localhost:5432",
		"Redis",
		"NOT RUNNING",
		"pgAdmin",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected health view to contain %q:\n%s", fragment, view)
		}
	}
	for _, fragment := range []string{
		"postgres port listening",
		"redis port listening",
	} {
		if strings.Contains(view, fragment) {
			t.Fatalf("expected service-centric health view to avoid raw check fragment %q:\n%s", fragment, view)
		}
	}
}

func TestHealthViewWarnsWhenPortIsBusyOutsideTheStack(t *testing.T) {
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
				Host:          "localhost",
				ExternalPort:  5432,
				PortListening: true,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	current = navigateToSection(t, current, healthSection)

	view := current.View().Content
	for _, fragment := range []string{
		"Healthy: 0",
		"Warnings: 1",
		"Not running: 0",
		"Status: needs attention",
		"Reachability: accepting on localhost:5432",
		"Host port is busy outside this stack.",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected warning health view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestWatchLogsKeyLaunchesSelectedServiceFromServicesPane(t *testing.T) {
	requests := make([]LogWatchRequest, 0, 1)
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(request LogWatchRequest) (tea.ExecCommand, error) {
			requests = append(requests, request)
			return stubExecCommand{}, nil
		},
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
			{Name: "redis", DisplayName: "Redis", Status: "running", ContainerName: "stack-redis"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected watch logs command")
	}
	if len(requests) != 1 {
		t.Fatalf("expected one watch request, got %d", len(requests))
	}
	if requests[0].Service != "postgres" {
		t.Fatalf("expected postgres watch request, got %+v", requests[0])
	}
}

func TestWatchLogsKeyUsesSelectedServicePaneTarget(t *testing.T) {
	requests := make([]LogWatchRequest, 0, 1)
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(request LogWatchRequest) (tea.ExecCommand, error) {
			requests = append(requests, request)
			return stubExecCommand{}, nil
		},
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", Host: "localhost", ExternalPort: 5432, InternalPort: 5432},
			{Name: "redis", DisplayName: "Redis", Status: "running", ContainerName: "stack-redis", Host: "localhost", ExternalPort: 6379, InternalPort: 6379},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	current = updatedModel.(Model)
	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected watch logs command from services section")
	}
	if len(requests) != 1 {
		t.Fatalf("expected one watch request, got %d", len(requests))
	}
	if requests[0].Service != "redis" {
		t.Fatalf("expected redis watch request, got %+v", requests[0])
	}
}

func TestWatchLogsKeyWarnsForHostTools(t *testing.T) {
	launcherCalls := 0
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) {
			launcherCalls++
			return stubExecCommand{}, nil
		},
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{DisplayName: "Cockpit", Status: "running", Host: "localhost", URL: "https://localhost:9090"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected transient banner clear command for host tools")
	}
	if launcherCalls != 0 {
		t.Fatalf("expected host tools to skip log watch launcher, got %d calls", launcherCalls)
	}
	if current.banner == nil || !strings.Contains(current.banner.Message, "host tools") {
		t.Fatalf("expected host tool warning banner, got %+v", current.banner)
	}
}

func TestServiceDetailShowsConciseLiveLogHint(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	content := current.currentContent()
	if !collapsedContainsTest(content, "Actions: w logs") {
		t.Fatalf("expected concise service actions hint:\n%s", content)
	}
	for _, fragment := range []string{
		"w watch selected service",
		"Returns here when the stream exits.",
		"Copy placeholders:",
	} {
		if strings.Contains(content, fragment) {
			t.Fatalf("expected services content to omit %q:\n%s", fragment, content)
		}
	}
}

func TestFooterHelpShowsWatchLogsOnlyWhenAvailable(t *testing.T) {
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	)
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	if !strings.Contains(stripANSITest(current.footerView()), "watch logs") {
		t.Fatalf("expected footer help to show live-log shortcut for stack services")
	}

	current.active = overviewSection
	current.syncLayout()
	if strings.Contains(stripANSITest(current.footerView()), "watch logs") {
		t.Fatalf("expected overview footer help to hide live-log shortcut")
	}
}

func TestFooterViewWrapsShortHelpAcrossLines(t *testing.T) {
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	).WithProductivity(
		func(string) error { return nil },
		func(ServiceShellRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		func(DBShellRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 78, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				Name:          "postgres",
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				DSN:           "postgres://app:secret@localhost:5432/app",
			},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	footer := stripANSITest(current.footerView())
	if lipgloss.Height(footer) < 2 {
		t.Fatalf("expected wrapped footer help at narrow widths:\n%s", footer)
	}
	if strings.Contains(footer, "…") {
		t.Fatalf("expected wrapped footer help instead of truncation:\n%s", footer)
	}
	for _, fragment := range []string{"palette", "jump to service", "copy value", "watch logs", "service shell", "db shell", "pin service"} {
		if !strings.Contains(footer, fragment) {
			t.Fatalf("expected wrapped footer to contain %q:\n%s", fragment, footer)
		}
	}
}

func TestPaletteChromeUsesOverlaySpecificHeaderAndFooter(t *testing.T) {
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	).WithProductivity(
		func(string) error { return nil },
		func(ServiceShellRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		func(DBShellRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Managed:   true,
		LoadedAt:  time.Now(),
		Services: []Service{
			{
				Name:          "postgres",
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				DSN:           "postgres://app:secret@localhost:5432/app",
			},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	current.openJumpPalette()
	header := stripANSITest(renderHeader(current))
	if !strings.Contains(header, "Jump to service") {
		t.Fatalf("expected jump palette header label:\n%s", header)
	}
	if strings.Contains(header, "Command palette") {
		t.Fatalf("expected jump palette header to omit generic command palette label:\n%s", header)
	}

	footer := stripANSITest(current.footerView())
	if !strings.Contains(footer, "type to filter") || !strings.Contains(footer, "enter run") {
		t.Fatalf("expected palette footer instructions:\n%s", footer)
	}
	if strings.Contains(footer, "watch logs") {
		t.Fatalf("expected palette footer to omit global service shortcuts:\n%s", footer)
	}

	current.openCopyPalette()
	header = stripANSITest(renderHeader(current))
	if !strings.Contains(header, "Copy value") {
		t.Fatalf("expected copy palette header label:\n%s", header)
	}
}

func TestCopyShortcutOpensPaletteAndCopiesSelectedValue(t *testing.T) {
	copied := ""
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	).WithProductivity(
		func(value string) error {
			copied = value
			return nil
		},
		nil,
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				Name:          "postgres",
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
				Username:      "app",
				Password:      "secret-password",
				Database:      "app",
				DSN:           "postgres://app:secret-password@localhost:5432/app",
			},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	current = updatedModel.(Model)
	if cmd != nil {
		t.Fatalf("expected copy shortcut to open the palette without a command")
	}
	if current.palette == nil {
		t.Fatalf("expected copy palette to open")
	}
	if got := current.palette.filtered[0].Title; got != "Copy Postgres DSN" {
		t.Fatalf("expected first copy target to be the DSN, got %q", got)
	}

	updatedModel, cmd = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected copy command after selecting a palette item")
	}

	msg := cmd()
	copyMsg, ok := msg.(copyDoneMsg)
	if !ok {
		t.Fatalf("expected copyDoneMsg, got %T", msg)
	}
	updatedModel, clearCmd := current.Update(copyMsg)
	current = updatedModel.(Model)
	if clearCmd == nil {
		t.Fatalf("expected banner clear command after copy")
	}
	if copied != "postgres://app:secret-password@localhost:5432/app" {
		t.Fatalf("unexpected copied value: %q", copied)
	}
	if len(current.history) == 0 || current.history[len(current.history)-1].Action != "Copy Postgres DSN" {
		t.Fatalf("expected copy action in history, got %+v", current.history)
	}
}

func TestQuickJumpPaletteFuzzySelectsService(t *testing.T) {
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
			{Name: "redis", DisplayName: "Redis", Status: "running", ContainerName: "stack-redis"},
		},
	}
	current.active = overviewSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	current = updatedModel.(Model)
	if current.palette == nil {
		t.Fatalf("expected jump palette to open")
	}

	for _, ch := range []string{"r", "e", "d", "i", "s"} {
		updatedModel, _ = current.Update(tea.KeyPressMsg{Code: rune(ch[0]), Text: ch})
		current = updatedModel.(Model)
	}
	if len(current.palette.filtered) == 0 || current.palette.filtered[0].ServiceKey != "redis" {
		t.Fatalf("expected fuzzy search to rank redis first, got %+v", current.palette.filtered)
	}

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if cmd != nil {
		t.Fatalf("expected jump selection to navigate without a command")
	}
	if current.active != servicesSection || current.selectedService != "redis" {
		t.Fatalf("expected jump palette to focus redis in services, got active=%v selected=%q", current.active, current.selectedService)
	}
}

func TestPinShortcutMovesServiceIntoPinnedGroup(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
			{Name: "redis", DisplayName: "Redis", Status: "running", ContainerName: "stack-redis"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected pin toggle to return a banner clear command")
	}

	content := current.currentContent()
	if !collapsedContainsTest(content, "Pinned") {
		t.Fatalf("expected pinned group in services pane:\n%s", content)
	}
	if strings.Index(content, "Pinned") > strings.Index(content, "Stack services") {
		t.Fatalf("expected pinned group to render before stack services:\n%s", content)
	}
}

func TestExecShellShortcutLaunchesSelectedServiceAndRefreshes(t *testing.T) {
	loadCount := 0
	requests := make([]ServiceShellRequest, 0, 1)
	model := NewInspectionModel(
		func() (Snapshot, error) {
			loadCount++
			return Snapshot{StackName: fmt.Sprintf("stack-%d", loadCount)}, nil
		},
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	).WithProductivity(
		nil,
		func(request ServiceShellRequest) (tea.ExecCommand, error) {
			requests = append(requests, request)
			return stubExecCommand{}, nil
		},
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected service shell shortcut to launch a handoff command")
	}
	if len(requests) != 1 || requests[0].Service != "postgres" {
		t.Fatalf("unexpected shell request capture: %+v", requests)
	}
	if current.runningHandoff == nil {
		t.Fatalf("expected shell handoff to be tracked as running")
	}

	updatedModel, reloadCmd := current.Update(handoffDoneMsg{
		historyID: current.runningHandoff.History,
		action:    current.runningHandoff.Action,
		message:   "closed Postgres shell",
		refresh:   true,
	})
	current = updatedModel.(Model)
	loaded := snapshotFromCmd(t, reloadCmd)
	if loaded.snapshot.StackName != "stack-1" {
		t.Fatalf("unexpected refreshed snapshot after shell handoff: %+v", loaded.snapshot)
	}
	if len(current.history) == 0 || current.history[len(current.history)-1].Action != "Open Postgres shell" {
		t.Fatalf("expected shell action to be recorded in history, got %+v", current.history)
	}
}

func TestCommandPaletteShowsRecentActionsAheadOfStaticEntries(t *testing.T) {
	model := NewInspectionModel(
		func() (Snapshot, error) { return Snapshot{}, nil },
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	).WithProductivity(
		func(string) error { return nil },
		nil,
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", DSN: "postgres://app:secret@localhost:5432/app"},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	current = updatedModel.(Model)
	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	msg := cmd()
	copyMsg, ok := msg.(copyDoneMsg)
	if !ok {
		t.Fatalf("expected copyDoneMsg, got %T", msg)
	}
	updatedModel, _ = current.Update(copyMsg)
	current = updatedModel.(Model)

	updatedModel, _ = current.Update(tea.KeyPressMsg{Text: ":"})
	current = updatedModel.(Model)
	if current.palette == nil {
		t.Fatalf("expected command palette to open")
	}
	if len(current.palette.filtered) == 0 || current.palette.filtered[0].Subtitle != "copied Postgres DSN to clipboard" {
		t.Fatalf("expected recent actions to lead the palette, got %+v", current.palette.filtered)
	}
}

func TestLogWatchDoneReloadsSnapshot(t *testing.T) {
	loadCount := 0
	model := NewInspectionModel(
		func() (Snapshot, error) {
			loadCount++
			return Snapshot{StackName: fmt.Sprintf("stack-%d", loadCount)}, nil
		},
		func(LogWatchRequest) (tea.ExecCommand, error) { return stubExecCommand{}, nil },
		nil,
	)

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{StackName: "dev-stack"}
	current.loading = false

	updatedModel, cmd := current.Update(logWatchDoneMsg{Service: "Postgres"})
	current = updatedModel.(Model)
	if !current.loading {
		t.Fatalf("expected watch completion to trigger a refresh")
	}
	loaded := snapshotFromCmd(t, cmd)
	if loaded.snapshot.StackName != "stack-1" {
		t.Fatalf("unexpected refreshed snapshot after log watch: %+v", loaded.snapshot)
	}
}

func TestSelectionKeysSwitchServiceDetailPane(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)
	current.snapshot = Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "stack-postgres", Host: "localhost", ExternalPort: 5432, InternalPort: 5432},
			{Name: "redis", DisplayName: "Redis", Status: "running", ContainerName: "stack-redis", Host: "localhost", ExternalPort: 6379, InternalPort: 6379},
		},
	}
	current.active = servicesSection
	current.normalizeSelections()
	current.syncLayout()

	if !collapsedContainsTest(current.currentContent(), "Host port: 5432") {
		t.Fatalf("expected initial port detail to show postgres:\n%s", current.currentContent())
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	current = updatedModel.(Model)
	if current.selectedService != "redis" {
		t.Fatalf("expected selected service to switch to redis, got %q", current.selectedService)
	}
	if !collapsedContainsTest(current.currentContent(), "Host port: 6379") {
		t.Fatalf("expected port detail to switch to redis:\n%s", current.currentContent())
	}
}

func TestHealthViewFallsBackToRawChecksWhenServicesAreUnavailable(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Health: []HealthLine{
			{Status: output.StatusWarn, Message: "postgres port listening"},
			{Status: output.StatusFail, Message: "redis port listening"},
		},
		DoctorChecks: []DoctorCheck{
			{Status: output.StatusWarn, Message: "podman compose missing alias"},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	current = navigateToSection(t, current, healthSection)

	view := current.currentContent()
	for _, fragment := range []string{
		"Live service health is unavailable; showing raw checks instead.",
		"postgres port listening",
		"redis port listening",
		"Doctor findings",
		"podman compose missing alias",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected raw health fallback to contain %q:\n%s", fragment, view)
		}
	}
}

func TestOverviewExcludesCockpitFromStackServiceCount(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName:  "dev-stack",
		ConfigPath: "/tmp/stackctl/config.yaml",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "missing",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
			},
			{
				DisplayName:   "Redis",
				Status:        "missing",
				ContainerName: "stack-redis",
				Host:          "localhost",
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
		"Running: 0",
		"Stopped: 2",
		"Attention: 0",
		"Stack",
		"Runtime",
		"Stack services: 0 / 2 running",
		"Host: localhost",
		"Ports: Postgres 5432  •  Redis 6379",
		"Host tools",
		"External to stack start, stop, and restart.",
		"Cockpit: running",
		"URL: https://devbox:9090",
		"Helpful commands",
		"stackctl start  •  stackctl services  •  stackctl health",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected overview to contain %q:\n%s", fragment, view)
		}
	}
}

func TestOverviewExpandedLayoutShowsPathsAndManagedMode(t *testing.T) {
	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName:         "dev-stack",
		ConfigPath:        "/tmp/stackctl/config.yaml",
		StackDir:          "/tmp/stackctl/stacks/dev-stack",
		ComposePath:       "/tmp/stackctl/stacks/dev-stack/compose.yaml",
		Managed:           true,
		WaitForServices:   true,
		StartupTimeoutSec: 45,
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
			},
			{
				DisplayName:   "Redis",
				Status:        "running",
				ContainerName: "stack-redis",
				Host:          "localhost",
				ExternalPort:  6379,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)

	view := current.View().Content
	for _, fragment := range []string{
		"Mode: managed",
		"Config: /tmp/stackctl/config.yaml",
		"Paths",
		"Stack dir: /tmp/stackctl/stacks/dev-stack",
		"Compose: /tmp/stackctl/stacks/dev-stack/compose.yaml",
		"Startup timeout: 45s",
		"Wait on start: on",
		"stackctl services  •  stackctl health  •  stackctl connect",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected expanded overview to contain %q:\n%s", fragment, view)
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
	current = navigateToSection(t, current, servicesSection)

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

func TestModelRunsStartActionOptimisticallyAndRecordsHistory(t *testing.T) {
	loadCount := 0
	loader := func() (Snapshot, error) {
		loadCount++
		return Snapshot{
			StackName: "dev-stack",
			Services: []Service{
				{
					DisplayName:   "Postgres",
					Status:        "running",
					ContainerName: "stack-postgres",
					Host:          "localhost",
					ExternalPort:  5432,
					PortListening: true,
				},
				{
					DisplayName:   "Redis",
					Status:        "running",
					ContainerName: "stack-redis",
					Host:          "localhost",
					ExternalPort:  6379,
					PortListening: true,
				},
			},
		}, nil
	}

	model := NewActionModel(loader, func(ActionID) (ActionReport, error) {
		return ActionReport{
			Status:  output.StatusOK,
			Message: "stack started",
			Details: []string{"Wait for services: on"},
			Refresh: true,
		}, nil
	})
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	initial := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "missing",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
			},
			{
				DisplayName:   "Redis",
				Status:        "missing",
				ContainerName: "stack-redis",
				Host:          "localhost",
				ExternalPort:  6379,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: initial})
	current = updatedModel.(Model)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	current = updatedModel.(Model)
	if current.runningAction == nil {
		t.Fatalf("expected running action state")
	}
	if current.snapshot.Services[0].Status != "starting" || current.snapshot.Services[1].Status != "starting" {
		t.Fatalf("expected optimistic starting state, got %+v", current.snapshot.Services)
	}
	if !strings.Contains(current.View().Content, "starting stack...") {
		t.Fatalf("expected start banner in view:\n%s", current.View().Content)
	}
	if cmd == nil {
		t.Fatalf("expected async action command")
	}

	actionMsgValue, ok := cmd().(actionMsg)
	if !ok {
		t.Fatalf("expected actionMsg, got %T", cmd())
	}

	updatedModel, cmd = current.Update(actionMsgValue)
	current = updatedModel.(Model)
	if !current.loading {
		t.Fatalf("expected snapshot refresh after successful lifecycle action")
	}
	if cmd == nil {
		t.Fatalf("expected snapshot reload after successful action")
	}

	loaded := snapshotFromCmd(t, cmd)
	updatedModel, _ = current.Update(loaded)
	current = updatedModel.(Model)
	if loadCount != 1 {
		t.Fatalf("expected loader to run once after action, got %d", loadCount)
	}

	for _, service := range current.snapshot.Services {
		if service.Status != "running" {
			t.Fatalf("expected refreshed running state, got %+v", current.snapshot.Services)
		}
	}

	current = navigateToSection(t, current, historySection)

	view := current.View().Content
	for _, fragment := range []string{
		"History",
		"Start",
		"Status: completed",
		"Message: stack started",
		"Wait for services: on",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected history view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestModelCancelsConfirmedActionWithoutRunningIt(t *testing.T) {
	called := false
	model := NewActionModel(func() (Snapshot, error) { return Snapshot{}, nil }, func(ActionID) (ActionReport, error) {
		called = true
		return ActionReport{}, nil
	})
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	current = updatedModel.(Model)
	if current.confirmation == nil {
		t.Fatalf("expected stop confirmation to be shown")
	}
	if strings.Contains(current.currentContent(), "Stop the local stack now?") {
		t.Fatalf("expected confirmation modal to stay out of panel content:\n%s", current.currentContent())
	}
	if strings.Contains(current.View().Content, "Stack services: 1 / 1 running") {
		t.Fatalf("expected confirmation to replace the main panel content:\n%s", current.View().Content)
	}
	if !strings.Contains(current.View().Content, "Stop the local stack now?") {
		t.Fatalf("expected confirmation prompt in view:\n%s", current.View().Content)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	current = updatedModel.(Model)
	if called {
		t.Fatalf("expected cancelled action not to run")
	}
	if current.confirmation != nil {
		t.Fatalf("expected confirmation to be cleared after cancellation")
	}
	if current.banner == nil {
		t.Fatalf("expected cancelled action to show a transient banner")
	}

	current = navigateToSection(t, current, historySection)

	view := current.View().Content
	for _, fragment := range []string{
		"History",
		"Stop",
		"Status: completed with warnings",
		"Message: stop cancelled",
	} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("expected cancelled action history to contain %q:\n%s", fragment, view)
		}
	}
}

func TestModelRestoresSnapshotWhenActionFails(t *testing.T) {
	model := NewActionModel(func() (Snapshot, error) { return Snapshot{}, nil }, func(ActionID) (ActionReport, error) {
		return ActionReport{}, fmt.Errorf("compose unavailable")
	})
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
				PortListening: true,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	current = updatedModel.(Model)
	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.runningAction == nil {
		t.Fatalf("expected restart action to begin after confirmation")
	}
	if cmd == nil {
		t.Fatalf("expected async restart command")
	}

	actionMsgValue, ok := cmd().(actionMsg)
	if !ok {
		t.Fatalf("expected actionMsg, got %T", cmd())
	}
	updatedModel, _ = current.Update(actionMsgValue)
	current = updatedModel.(Model)
	if current.snapshot.Services[0].Status != "running" {
		t.Fatalf("expected snapshot to be restored after failure, got %+v", current.snapshot.Services)
	}
	if strings.Contains(current.currentContent(), "restart failed: compose unavailable") {
		t.Fatalf("expected failure status to stay out of panel content:\n%s", current.currentContent())
	}
	if !strings.Contains(current.View().Content, "restart failed: compose unavailable") {
		t.Fatalf("expected failure banner in view:\n%s", current.View().Content)
	}
}

func TestModelClearsTransientBannerAfterDelay(t *testing.T) {
	model := NewActionModel(func() (Snapshot, error) { return Snapshot{}, nil }, func(ActionID) (ActionReport, error) {
		return ActionReport{}, nil
	})
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	current = updatedModel.(Model)

	updatedModel, clearCmd := current.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	current = updatedModel.(Model)
	if clearCmd == nil {
		t.Fatalf("expected banner clear command after cancellation")
	}
	if current.banner == nil {
		t.Fatalf("expected visible banner before clear")
	}

	clearMsg, ok := clearCmd().(bannerClearMsg)
	if !ok {
		t.Fatalf("expected bannerClearMsg, got %T", clearCmd())
	}

	updatedModel, _ = current.Update(clearMsg)
	current = updatedModel.(Model)
	if current.banner != nil {
		t.Fatalf("expected banner to clear after timeout")
	}
	if strings.Contains(current.View().Content, "stop cancelled") {
		t.Fatalf("expected cleared banner to disappear from view:\n%s", current.View().Content)
	}
}

func TestModelIgnoresStaleBannerClearMessages(t *testing.T) {
	model := NewActionModel(func() (Snapshot, error) { return Snapshot{}, nil }, func(ActionID) (ActionReport, error) {
		return ActionReport{Status: output.StatusOK, Message: "stack started"}, nil
	})
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	runningSnapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: runningSnapshot})
	current = updatedModel.(Model)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	current = updatedModel.(Model)
	updatedModel, cancelClearCmd := current.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	current = updatedModel.(Model)
	if cancelClearCmd == nil {
		t.Fatalf("expected clear command from cancelled banner")
	}

	updatedModel, actionCmd := current.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	current = updatedModel.(Model)
	if actionCmd == nil {
		t.Fatalf("expected action command after starting stack")
	}
	if current.runningAction == nil {
		t.Fatalf("expected running action after triggering a new action")
	}

	clearMsg, ok := cancelClearCmd().(bannerClearMsg)
	if !ok {
		t.Fatalf("expected bannerClearMsg, got %T", cancelClearCmd())
	}
	updatedModel, _ = current.Update(clearMsg)
	current = updatedModel.(Model)
	if current.banner == nil {
		t.Fatalf("expected stale clear message not to remove current pending banner")
	}
	if !strings.Contains(current.View().Content, "running doctor diagnostics...") {
		t.Fatalf("expected pending banner to remain after stale clear:\n%s", current.View().Content)
	}
}

func TestSidebarKeepsGlobalActionsOutOfPanelContent(t *testing.T) {
	model := NewActionModel(func() (Snapshot, error) { return Snapshot{}, nil }, func(ActionID) (ActionReport, error) {
		return ActionReport{}, nil
	})
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	current := updatedModel.(Model)

	snapshot := Snapshot{
		StackName: "dev-stack",
		Services: []Service{
			{
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "stack-postgres",
				Host:          "localhost",
				ExternalPort:  5432,
				PortListening: true,
			},
			{
				DisplayName:  "Cockpit",
				Status:       "running",
				Host:         "localhost",
				ExternalPort: 9090,
				URL:          "https://localhost:9090",
			},
			{
				DisplayName:  "pgAdmin",
				Status:       "running",
				Host:         "localhost",
				ExternalPort: 8081,
				URL:          "http://localhost:8081",
			},
		},
	}

	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	current = updatedModel.(Model)

	overviewContent := current.currentContent()
	if strings.Contains(overviewContent, "[1] Restart") || strings.Contains(overviewContent, "Actions") {
		t.Fatalf("expected overview panel content to stay action-free:\n%s", overviewContent)
	}
	overviewSidebar := renderSidebar(current)
	for _, fragment := range []string{
		"Actions",
		"[1] Restart",
		"[2] Stop",
		"[3] Doctor",
		"[4] Open Cockpit",
	} {
		if !strings.Contains(overviewSidebar, fragment) {
			t.Fatalf("expected sidebar to show %q:\n%s", fragment, overviewSidebar)
		}
	}
	if !strings.Contains(overviewSidebar, "[5] Open pgAdmin") {
		t.Fatalf("expected sidebar to show pgAdmin open action:\n%s", overviewSidebar)
	}
	for _, fragment := range []string{
		"r refresh",
		"m compact",
		"s secrets",
	} {
		if strings.Contains(overviewSidebar, fragment) {
			t.Fatalf("expected sidebar to avoid footer keybind duplication %q:\n%s", fragment, overviewSidebar)
		}
	}
	if strings.Contains(overviewSidebar, "y/enter") || strings.Contains(overviewSidebar, "n/esc") {
		t.Fatalf("expected sidebar to avoid confirmation key hints:\n%s", overviewSidebar)
	}
	if !strings.Contains(overviewSidebar, "Stack") || !strings.Contains(overviewSidebar, "Open") {
		t.Fatalf("expected sidebar to show global actions:\n%s", overviewSidebar)
	}

	current = navigateToSection(t, current, servicesSection)
	servicesContent := current.currentContent()
	if strings.Contains(servicesContent, "[1] Restart") {
		t.Fatalf("expected services panel content to keep sidebar actions out of the detail pane:\n%s", servicesContent)
	}
	if !strings.Contains(servicesContent, "Actions:") {
		t.Fatalf("expected services panel content to show contextual service actions:\n%s", servicesContent)
	}
	servicesSidebar := renderSidebar(current)
	for _, fragment := range []string{
		"Actions",
		"[1] Restart Postgres",
		"[2] Stop Postgres",
		"[3] Restart",
		"[4] Stop",
	} {
		if !strings.Contains(servicesSidebar, fragment) {
			t.Fatalf("expected services sidebar to show %q:\n%s", fragment, servicesSidebar)
		}
	}
}

func TestConfigSectionRendersEditorSummary(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	view := current.currentContent()
	for _, fragment := range []string{
		"Config fields",
		"Draft values • saved",
		"Stack",
		"Postgres",
		"Redis",
		"Stack / Name",
		"Managed mode",
		"Config detail",
		"Values",
		"Field",
		"Draft dev-stack",
		"Saved dev-stack",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected config section to contain %q:\n%s", fragment, view)
		}
	}
}

func TestConfigSectionFitsViewportAndKeepsFooterAtCompactHeight(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModelSized(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""), 120, 24)
	current = navigateToSection(t, current, configSection)

	renderedEditor := current.configEditor.View(false)
	if got, max := lipgloss.Height(renderedEditor), current.viewport.Height(); got > max {
		t.Fatalf("expected config editor to fit viewport height, got %d > %d:\n%s", got, max, renderedEditor)
	}

	view := current.View().Content
	if got := lipgloss.Height(view); got > 24 {
		t.Fatalf("expected full view to fit terminal height, got %d > 24:\n%s", got, view)
	}
	for _, fragment := range []string{
		"Config fields",
		"Config detail",
		"ctrl+s save/apply",
		"q quit",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected compact config view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestConfigSectionFitsViewportWhenStacked(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModelSized(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""), 92, 24)
	current = navigateToSection(t, current, configSection)

	renderedEditor := current.configEditor.View(false)
	if got, max := lipgloss.Height(renderedEditor), current.viewport.Height(); got > max {
		t.Fatalf("expected stacked config editor to fit viewport height, got %d > %d:\n%s", got, max, renderedEditor)
	}

	view := current.View().Content
	for _, fragment := range []string{
		"Config fields",
		"Config detail",
		"ctrl+s save/apply",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected stacked compact config view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestConfigSectionKeepsFooterAndEditorVisibleAcrossCompactSizes(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())

	type sizeCase struct {
		width   int
		height  int
		stacked bool
	}

	cases := []sizeCase{
		{width: 80, height: 20, stacked: true},
		{width: 88, height: 20, stacked: true},
		{width: 92, height: 22, stacked: true},
		{width: 100, height: 20, stacked: false},
		{width: 110, height: 20, stacked: false},
		{width: 120, height: 22, stacked: false},
	}

	for _, tc := range cases {
		current := loadConfigSnapshotModelSized(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""), tc.width, tc.height)
		current = navigateToSection(t, current, configSection)

		view := current.View().Content
		if got := lipgloss.Height(view); got > tc.height {
			t.Fatalf("size %dx%d overflowed full view: %d > %d\n%s", tc.width, tc.height, got, tc.height, view)
		}
		for _, fragment := range []string{
			"Config fields",
			"Config detail",
			"ctrl+s save/apply",
			"q quit",
		} {
			if !collapsedContainsTest(view, fragment) {
				t.Fatalf("size %dx%d missing %q:\n%s", tc.width, tc.height, fragment, view)
			}
		}
		if tc.stacked && !collapsedContainsTest(view, "ctrl+s save/apply") {
			t.Fatalf("size %dx%d lost the primary config save hint:\n%s", tc.width, tc.height, view)
		}
	}
}

func TestConfigSectionScrollsFieldListWithoutLosingFooter(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModelSized(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""), 120, 24)
	current = navigateToSection(t, current, configSection)

	for range 20 {
		updatedModel, _ := current.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		current = updatedModel.(Model)
	}

	view := current.View().Content
	for _, fragment := range []string{
		"Redis / Save policy",
		"ctrl+s save/apply",
		"q quit",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected scrolled config view to contain %q:\n%s", fragment, view)
		}
	}
	if got := lipgloss.Height(view); got > 24 {
		t.Fatalf("expected scrolled config view to fit terminal height, got %d > 24:\n%s", got, view)
	}
}

func TestConfigSectionFieldHeaderUsesSavedDraftEditingAndInvalidStates(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	if view := current.currentContent(); !collapsedContainsTest(view, "Stack / Name") || collapsedContainsTest(view, "saved Stack / Name") {
		t.Fatalf("expected clean field header to omit extra state copy:\n%s", view)
	}

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if view := current.currentContent(); !collapsedContainsTest(view, "editing Stack / Name") {
		t.Fatalf("expected active edit header to show editing state:\n%s", view)
	}

	current.configEditor.input.SetValue("dev-stack-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if view := current.currentContent(); !collapsedContainsTest(view, "edited Stack / Name") {
		t.Fatalf("expected unsaved field header to show edited state:\n%s", view)
	}

	current.configEditor.selectedKey = "ports.postgres"
	current.configEditor.refreshList(false)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current.configEditor.input.SetValue("bad-port")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if view := current.currentContent(); !collapsedContainsTest(view, "invalid Postgres / Port") {
		t.Fatalf("expected invalid field header to show invalid state:\n%s", view)
	}
}

func TestConfigFieldGroupPlacesHostUnderStack(t *testing.T) {
	stackSpecs := groupedConfigFieldSpecs("Stack")
	foundHost := false
	for _, spec := range stackSpecs {
		if spec.Key == "connection.host" {
			foundHost = true
			break
		}
	}
	if !foundHost {
		t.Fatalf("expected connection.host to be grouped under Stack, got %+v", stackSpecs)
	}
}

func TestConfigSectionEditsStackNameAndUpdatesManagedDir(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.configEditor.editing {
		t.Fatalf("expected stack-name edit to commit")
	}
	if current.configEditor.draft.Stack.Name != "dev-stack-ops" {
		t.Fatalf("unexpected edited stack name: %+v", current.configEditor.draft.Stack)
	}
	if !strings.Contains(current.configEditor.draft.Stack.Dir, "dev-stack-ops") {
		t.Fatalf("expected managed stack dir to follow the edited name: %+v", current.configEditor.draft.Stack)
	}
}

func TestConfigSectionAllowsTypingNWhileEditing(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "n")
	if !current.configEditor.editing {
		t.Fatalf("expected editor to stay active after typing n")
	}
	if got := current.configEditor.input.Value(); got != "dev-stackn" {
		t.Fatalf("expected n to be inserted into the field, got %q", got)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.configEditor.draft.Stack.Name != "dev-stackn" {
		t.Fatalf("expected edited stack name to include n, got %+v", current.configEditor.draft.Stack)
	}
}

func TestConfigSectionClearsPortInputErrorAfterCorrection(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "ports.postgres"
	current.configEditor.refreshList(false)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current.configEditor.input.SetValue("not-a-port")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.configEditor.input.Err == nil {
		t.Fatalf("expected invalid port entry to leave an inline error")
	}
	if !current.configEditor.editing {
		t.Fatalf("expected invalid port entry to keep the field in edit mode")
	}

	current.configEditor.input.SetValue("5432")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.configEditor.editing {
		t.Fatalf("expected corrected port entry to commit")
	}
	if current.configEditor.input.Err != nil {
		t.Fatalf("expected corrected port entry to clear the inline error, got %v", current.configEditor.input.Err)
	}

	current.configEditor.selectedKey = "services.redis.maxmemory_policy"
	current.configEditor.refreshList(false)
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.configEditor.input.Err != nil {
		t.Fatalf("expected next field to use its own validator, got %v", current.configEditor.input.Err)
	}
	view := current.currentContent()
	if collapsedContainsTest(view, "enter a valid port") {
		t.Fatalf("expected stale port validation error to disappear after moving to another field:\n%s", view)
	}
}

func TestConfigSectionEscapeCancelsEdit(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	current = updatedModel.(Model)
	if current.configEditor.editing {
		t.Fatalf("expected escape to cancel the edit")
	}
	if current.configEditor.draft.Stack.Name != cfg.Stack.Name {
		t.Fatalf("expected escape to discard in-progress edits, got %+v", current.configEditor.draft.Stack)
	}
}

func TestConfigSectionPreviewShowsDiffForDraftChanges(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	current = updatedModel.(Model)
	view := current.currentContent()
	for _, fragment := range []string{
		"Config diff",
		"--- saved",
		"+++ draft",
		"- name: dev-stack",
		"+ name: dev-stack-ops",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected diff preview to contain %q:\n%s", fragment, view)
		}
	}
}

func TestConfigSectionResetConfirmationRestoresDraft(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	current = updatedModel.(Model)
	if current.confirmation == nil || current.confirmation.Kind != confirmationConfigReset {
		t.Fatalf("expected reset confirmation, got %+v", current.confirmation)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if current.configEditor.draft.Stack.Name != cfg.Stack.Name {
		t.Fatalf("expected reset draft to restore stack name, got %+v", current.configEditor.draft.Stack)
	}
	if current.confirmation != nil {
		t.Fatalf("expected reset confirmation to clear")
	}
}

func TestConfigSectionApplyDefaultsFillsEmptyValues(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Connection.PostgresPassword = ""
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "connection.postgres_password"
	current.configEditor.refreshList(false)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	current = updatedModel.(Model)
	if current.configEditor.draft.Connection.PostgresPassword != "app" {
		t.Fatalf("expected derived defaults to restore postgres password, got %+v", current.configEditor.draft.Connection)
	}
}

func TestConfigSectionScaffoldWarnsForExternalStacks(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected warning banner clear command")
	}
	if current.banner == nil || !strings.Contains(current.banner.Message, "managed stack") {
		t.Fatalf("expected scaffold warning banner, got %+v", current.banner)
	}
}

func TestConfigSectionHidesUnavailableScaffoldActionForExternalStacks(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	view := current.currentContent()
	if collapsedContainsTest(view, "g / G") {
		t.Fatalf("expected external-stack config detail to hide unavailable scaffold keys:\n%s", view)
	}
	if !collapsedContainsTest(view, "ctrl+s would only update stackctl metadata here") {
		t.Fatalf("expected external-stack workflow guidance in config view:\n%s", view)
	}
}

func TestConfigFieldDescriptionCompactsLongValues(t *testing.T) {
	cfg := configpkg.Default()
	var stackDirSpec configFieldSpec
	for _, spec := range configFieldSpecs {
		if spec.Key == "stack.dir" {
			stackDirSpec = spec
			break
		}
	}
	if stackDirSpec.Key == "" {
		t.Fatal("expected stack.dir field spec")
	}

	description := fieldItemDescription(stackDirSpec, cfg, false, nil)
	if !strings.Contains(description, "…") {
		t.Fatalf("expected compact description to use an ellipsis, got %q", description)
	}
	if strings.Contains(description, cfg.Stack.Dir) {
		t.Fatalf("expected compact description to hide the full path, got %q", description)
	}
}

func TestConfigSectionShowsRedisMaxmemoryPolicyHints(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.redis.maxmemory_policy"
	current.configEditor.refreshList(false)

	view := current.currentContent()
	for _, fragment := range []string{
		"Redis / Maxmemory policy",
		"Redis policies",
		"Values noeviction, allkeys-lru, allkeys-lfu",
		"volatile-random, volatile-ttl",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected redis maxmemory detail to contain %q:\n%s", fragment, view)
		}
	}

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	if !current.configEditor.input.ShowSuggestions {
		t.Fatalf("expected redis maxmemory edit mode to enable suggestions")
	}

	editView := current.currentContent()
	for _, fragment := range []string{
		"Redis policies",
		"noeviction, allkeys-lru, allkeys-lfu",
	} {
		if !collapsedContainsTest(editView, fragment) {
			t.Fatalf("expected edit view to contain %q:\n%s", fragment, editView)
		}
	}
}

func TestConfigSectionShowsTUIAutoRefreshIntervalHints(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "tui.auto_refresh_interval_seconds"
	current.configEditor.refreshList(false)

	view := current.currentContent()
	for _, fragment := range []string{
		"TUI / Auto refresh interval",
		"future TUI sessions",
		"Common values",
		"Values 5, 10, 30, 60",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected TUI auto-refresh detail to contain %q:\n%s", fragment, view)
		}
	}
}

func TestSelectedFieldEffectDifferentiatesRepresentativeFields(t *testing.T) {
	cfg := configpkg.Default()

	cases := []struct {
		key      string
		contains string
	}{
		{key: "services.redis.maxmemory_policy", contains: "Redis eviction policy"},
		{key: "behavior.startup_timeout_seconds", contains: "how long stackctl waits"},
		{key: "tui.auto_refresh_interval_seconds", contains: "future TUI sessions"},
		{key: "system.package_manager", contains: "package manager"},
	}

	for _, tc := range cases {
		var spec configFieldSpec
		for _, candidate := range configFieldSpecs {
			if candidate.Key == tc.key {
				spec = candidate
				break
			}
		}
		if spec.Key == "" {
			t.Fatalf("expected config field spec for %s", tc.key)
		}
		if got := selectedFieldEffect(spec, cfg); !strings.Contains(got, tc.contains) {
			t.Fatalf("expected effect for %s to contain %q, got %q", tc.key, tc.contains, got)
		}
	}
}

func TestRedisMaxmemoryPolicySuggestionsIncludeLRMForRedis86(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Services.Redis.Image = "docker.io/library/redis:8.6"

	values := redisMaxMemoryPolicySuggestions(cfg)
	if !slices.Contains(values, "allkeys-lrm") || !slices.Contains(values, "volatile-lrm") {
		t.Fatalf("expected redis 8.6 suggestions to include LRM policies, got %+v", values)
	}
}

func TestConfigSectionSaveUpdatesHeaderAutoRefreshInterval(t *testing.T) {
	cfg := configpkg.Default()
	savedCfg := cfg
	manager := configTestManager()
	manager.SaveConfig = func(_ string, next configpkg.Config) error {
		savedCfg = next
		return nil
	}

	model := NewFullModel(func() (Snapshot, error) {
		return configSnapshot(savedCfg, ConfigSourceLoaded, ""), nil
	}, nil, nil, &manager)
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "tui.auto_refresh_interval_seconds"
	current.configEditor.refreshList(false)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current.configEditor.input.SetValue("10")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 's'})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected save command for TUI interval")
	}

	msg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	updatedModel, reloadCmd := current.Update(msg)
	current = updatedModel.(Model)
	loaded := snapshotFromCmd(t, reloadCmd)
	updatedModel, _ = current.Update(loaded)
	current = updatedModel.(Model)

	if got := current.refreshInterval(); got != 10*time.Second {
		t.Fatalf("expected saved TUI interval to apply to the model, got %s", got)
	}
	if !strings.Contains(current.View().Content, "auto-refresh: 10s") {
		t.Fatalf("expected header to show updated auto-refresh interval:\n%s", current.View().Content)
	}
}

func TestConfigSectionHighlightsUnsavedDraftState(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)

	view := current.currentContent()
	for _, fragment := range []string{
		"Draft values • unsaved",
		"Unsaved draft",
		"ctrl+s saves only; stack target changes do not restart the current stack",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected unsaved draft view to contain %q:\n%s", fragment, view)
		}
	}
	if collapsedContainsTest(view, "Unsaved field:") {
		t.Fatalf("expected duplicate field-state notice to be removed:\n%s", view)
	}
}

func TestConfigSectionTreatsMaintenanceDatabaseAsConfigOnly(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.postgres.maintenance_database"
	current.configEditor.draft.Services.Postgres.MaintenanceDatabase = "template1"
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	spec, ok := current.configEditor.selectedSpec()
	if !ok {
		t.Fatal("expected selected config field")
	}
	if got := selectedFieldEffect(spec, current.configEditor.draft); !strings.Contains(got, "future database commands only") {
		t.Fatalf("expected helper-only effect text, got %q", got)
	}

	plan := current.configEditor.applyPlan()
	if plan.Allowed || plan.Reason != "use ctrl+s to save config-only changes" {
		t.Fatalf("expected maintenance database change to stay config-only, got %+v", plan)
	}

	view := current.currentContent()
	for _, fragment := range []string{
		"ctrl+s writes config only",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected config-only view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestConfigSectionExplainsManagedRuntimeFollowUp(t *testing.T) {
	cfg := configpkg.Default()
	snapshot := configSnapshot(cfg, ConfigSourceLoaded, "")
	snapshot.Services = []Service{{Name: "postgres", DisplayName: "Postgres", ContainerName: "local-postgres", Status: "running"}}

	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, snapshot)
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.postgres.image"
	current.configEditor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	spec, ok := current.configEditor.selectedSpec()
	if !ok {
		t.Fatal("expected selected config field")
	}
	if got := selectedFieldEffect(spec, current.configEditor.draft); !strings.Contains(got, "refreshes compose automatically") {
		t.Fatalf("expected managed field effect to mention automatic apply behavior, got %q", got)
	}
	lines := current.configEditor.runtimeImpactLines()
	if !slices.Contains(lines, "Saving refreshes the managed compose file and restarts the running stack automatically.") {
		t.Fatalf("expected managed runtime impact to mention automatic restart, got %+v", lines)
	}

	view := current.currentContent()
	for _, fragment := range []string{
		"ctrl+s saves, refreshes compose, and restarts running services",
	} {
		if !collapsedContainsTest(view, fragment) {
			t.Fatalf("expected managed runtime view to contain %q:\n%s", fragment, view)
		}
	}
}

func TestConfigSectionExplainsExternalMetadataOnlyChanges(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	cfg.Stack.Dir = t.TempDir()

	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.postgres.image"
	current.configEditor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	spec, ok := current.configEditor.selectedSpec()
	if !ok {
		t.Fatal("expected selected config field")
	}
	if got := selectedFieldEffect(spec, current.configEditor.draft); !strings.Contains(got, "does not rewrite your compose file") {
		t.Fatalf("expected external field effect to mention metadata-only behavior, got %q", got)
	}
	lines := current.configEditor.runtimeImpactLines()
	if !slices.Contains(lines, "External compose services keep running until you change them yourself.") {
		t.Fatalf("expected external runtime impact to mention untouched compose services, got %+v", lines)
	}
}

func TestConfigSectionSaveUsesDraftAndReloadsSnapshot(t *testing.T) {
	cfg := configpkg.Default()
	savedCfg := cfg
	saveCalls := 0
	manager := configTestManager()
	manager.SaveConfig = func(path string, next configpkg.Config) error {
		saveCalls++
		if path != "/tmp/stackctl/config.yaml" {
			t.Fatalf("unexpected save path: %s", path)
		}
		savedCfg = next
		return nil
	}

	loader := func() (Snapshot, error) {
		return configSnapshot(savedCfg, ConfigSourceLoaded, ""), nil
	}
	model := NewFullModel(loader, nil, nil, &manager)
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 's'})
	current = updatedModel.(Model)
	if current.runningConfigOp == nil {
		t.Fatalf("expected save operation to start")
	}
	if cmd == nil {
		t.Fatalf("expected save command")
	}

	resultMsg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	if saveCalls != 1 {
		t.Fatalf("expected one save call, got %d", saveCalls)
	}
	if savedCfg.Stack.Name != "dev-stack-ops" {
		t.Fatalf("expected saved config to use edited stack name, got %+v", savedCfg.Stack)
	}
	if !strings.Contains(resultMsg.Message, "future stackctl commands use the new stack target") {
		t.Fatalf("expected save result to include follow-up guidance, got %q", resultMsg.Message)
	}

	updatedModel, reloadCmd := current.Update(resultMsg)
	current = updatedModel.(Model)
	if !current.loading {
		t.Fatalf("expected save to trigger a snapshot reload")
	}
	loaded := snapshotFromCmd(t, reloadCmd)
	updatedModel, _ = current.Update(loaded)
	current = updatedModel.(Model)
	if current.configEditor.dirty() {
		t.Fatalf("expected editor draft to be clean after save reload")
	}
	if current.configEditor.draft.Stack.Name != "dev-stack-ops" {
		t.Fatalf("expected reloaded draft to preserve saved stack name, got %+v", current.configEditor.draft.Stack)
	}
}

func TestConfigSectionSaveMentionsScaffoldForRunningManagedServices(t *testing.T) {
	cfg := configpkg.Default()
	manager := configTestManager()
	loader := func() (Snapshot, error) {
		snapshot := configSnapshot(cfg, ConfigSourceLoaded, "")
		snapshot.Services = []Service{{Name: "postgres", DisplayName: "Postgres", ContainerName: "local-postgres", Status: "running"}}
		return snapshot, nil
	}
	model := NewFullModel(loader, nil, nil, &manager)
	snapshot := configSnapshot(cfg, ConfigSourceLoaded, "")
	snapshot.Services = []Service{{Name: "postgres", DisplayName: "Postgres", ContainerName: "local-postgres", Status: "running"}}
	current := loadConfigSnapshotModel(t, model, snapshot)
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.postgres.image"
	current.configEditor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 's'})
	current = updatedModel.(Model)
	if current.runningConfigOp == nil {
		t.Fatalf("expected save operation to start")
	}
	if cmd == nil {
		t.Fatalf("expected save command")
	}

	resultMsg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	if !strings.Contains(resultMsg.Message, "restart the stack to load the refreshed compose file") {
		t.Fatalf("expected save message to mention restart follow-up, got %q", resultMsg.Message)
	}
}

func TestConfigSectionApplyFromFreshManagedConfigSavesAndScaffolds(t *testing.T) {
	cfg := configpkg.Default()
	savedCfg := cfg
	saveCalls := 0
	scaffoldCalls := 0
	forcedScaffold := false
	manager := configTestManager()
	manager.SaveConfig = func(path string, next configpkg.Config) error {
		saveCalls++
		savedCfg = next
		return nil
	}
	manager.ManagedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
		return true, nil
	}
	manager.ScaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
		scaffoldCalls++
		forcedScaffold = force
		return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
	}

	model := NewFullModel(func() (Snapshot, error) {
		return configSnapshot(savedCfg, ConfigSourceLoaded, ""), nil
	}, nil, nil, &manager)

	snapshot := configSnapshot(cfg, ConfigSourceMissing, "No stackctl config was found.")
	snapshot.ConfigNeedsScaffold = true
	current := loadConfigSnapshotModel(t, model, snapshot)
	current = navigateToSection(t, current, configSection)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	current = updatedModel.(Model)
	if current.runningConfigOp == nil {
		t.Fatalf("expected apply operation to start")
	}
	if cmd == nil {
		t.Fatalf("expected apply command")
	}

	resultMsg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	if saveCalls != 1 || scaffoldCalls != 1 {
		t.Fatalf("expected one save and one scaffold call, got save=%d scaffold=%d", saveCalls, scaffoldCalls)
	}
	if forcedScaffold {
		t.Fatalf("expected fresh scaffold apply to avoid force overwrite")
	}
	if !strings.Contains(resultMsg.Message, "ready for the next stack start") {
		t.Fatalf("expected apply result to explain next start behavior, got %q", resultMsg.Message)
	}
}

func TestConfigSectionApplyConfirmsRestartForRunningManagedChanges(t *testing.T) {
	cfg := configpkg.Default()
	saveCalls := 0
	scaffoldCalls := 0
	forcedScaffold := false
	restarted := 0
	manager := configTestManager()
	manager.SaveConfig = func(string, configpkg.Config) error {
		saveCalls++
		return nil
	}
	manager.ScaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
		scaffoldCalls++
		forcedScaffold = force
		return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
	}

	snapshot := configSnapshot(cfg, ConfigSourceLoaded, "")
	snapshot.Services = []Service{{Name: "postgres", DisplayName: "Postgres", ContainerName: "local-postgres", Status: "running"}}
	model := NewFullModel(func() (Snapshot, error) {
		return snapshot, nil
	}, nil, func(action ActionID) (ActionReport, error) {
		if action != ActionRestart {
			t.Fatalf("expected apply to use restart, got %q", action)
		}
		restarted++
		return ActionReport{Status: output.StatusOK, Message: "stack restarted", Refresh: true}, nil
	}, &manager)
	current := loadConfigSnapshotModel(t, model, snapshot)
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.postgres.image"
	current.configEditor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	current = updatedModel.(Model)
	if current.runningConfigOp == nil {
		t.Fatalf("expected save/apply operation to start")
	}
	if cmd == nil {
		t.Fatalf("expected save/apply command")
	}

	resultMsg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	if saveCalls != 1 || scaffoldCalls != 1 || restarted != 1 {
		t.Fatalf("expected save, scaffold, and restart once, got save=%d scaffold=%d restart=%d", saveCalls, scaffoldCalls, restarted)
	}
	if !forcedScaffold {
		t.Fatalf("expected managed apply to force scaffold refresh when compose content changed")
	}
	if !strings.Contains(resultMsg.Message, "stack restarted") {
		t.Fatalf("expected apply result to include restart message, got %q", resultMsg.Message)
	}
}

func TestConfigSectionApplyWarnsForManualFollowUpChanges(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	managedDir, err := configpkg.ManagedStackDir("custom-stack")
	if err != nil {
		t.Fatalf("managed stack dir: %v", err)
	}
	current.configEditor.selectedKey = "stack.name"
	current.configEditor.draft.Stack.Name = "custom-stack"
	current.configEditor.draft.Stack.Dir = managedDir
	current.configEditor.draft.Stack.ComposeFile = configpkg.DefaultComposeFileName
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	current = updatedModel.(Model)
	if current.runningConfigOp == nil {
		t.Fatalf("expected save command for manual-follow-up change")
	}
	if cmd == nil {
		t.Fatalf("expected save command for manual-follow-up change")
	}
	resultMsg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	if !strings.Contains(resultMsg.Message, "future stackctl commands use the new stack target") {
		t.Fatalf("expected manual-follow-up save guidance, got %q", resultMsg.Message)
	}
}

func TestConfigSectionApplyDirectsExternalStacksToPlainSave(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	cfg.Stack.Dir = t.TempDir()
	saveCalls := 0
	scaffoldCalls := 0
	manager := configTestManager()
	manager.SaveConfig = func(string, configpkg.Config) error {
		saveCalls++
		return nil
	}
	manager.ScaffoldManagedStack = func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
		scaffoldCalls++
		return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
	}

	model := NewFullModel(func() (Snapshot, error) {
		return configSnapshot(cfg, ConfigSourceLoaded, ""), nil
	}, nil, nil, &manager)
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "services.postgres.image"
	current.configEditor.draft.Services.Postgres.Image = "docker.io/library/postgres:18"
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)

	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected save command for external metadata-only changes")
	}
	resultMsg, ok := cmd().(configOperationMsg)
	if !ok {
		t.Fatalf("expected configOperationMsg, got %T", cmd())
	}
	if !strings.Contains(resultMsg.Message, "external compose files were not changed") {
		t.Fatalf("expected external save guidance, got %q", resultMsg.Message)
	}
	if saveCalls != 1 || scaffoldCalls != 0 {
		t.Fatalf("expected external apply alias to save only, got save=%d scaffold=%d", saveCalls, scaffoldCalls)
	}
}

func TestConfigSectionPreservesDirtyDraftAcrossSnapshotRefresh(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	updatedModel, _ := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)
	current = sendTextToModel(t, current, "-ops")
	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	current = updatedModel.(Model)

	refreshed := cfg
	refreshed.Stack.Name = "external-change"
	updatedModel, _ = current.Update(snapshotMsg{snapshot: configSnapshot(refreshed, ConfigSourceLoaded, "")})
	current = updatedModel.(Model)
	if current.configEditor.draft.Stack.Name != "dev-stack-ops" {
		t.Fatalf("expected dirty draft to survive refresh, got %+v", current.configEditor.draft.Stack)
	}
}

var ansiStripPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var whitespaceCollapsePattern = regexp.MustCompile(`\s+`)

func stripANSITest(value string) string {
	return ansiStripPattern.ReplaceAllString(value, "")
}

func collapsedContainsTest(value, fragment string) bool {
	return strings.Contains(
		whitespaceCollapsePattern.ReplaceAllString(stripANSITest(value), " "),
		whitespaceCollapsePattern.ReplaceAllString(fragment, " "),
	)
}

type stubExecCommand struct{}

func (stubExecCommand) Run() error { return nil }

func (stubExecCommand) SetStdin(io.Reader) {}

func (stubExecCommand) SetStdout(io.Writer) {}

func (stubExecCommand) SetStderr(io.Writer) {}

func snapshotFromCmd(t *testing.T, cmd tea.Cmd) snapshotMsg {
	t.Helper()

	msg := cmd()
	if loaded, ok := msg.(snapshotMsg); ok {
		return loaded
	}

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected snapshotMsg or tea.BatchMsg, got %T", msg)
	}

	for _, child := range batch {
		if child == nil {
			continue
		}
		childMsg := child()
		loaded, ok := childMsg.(snapshotMsg)
		if ok {
			return loaded
		}
	}

	t.Fatalf("expected batched snapshotMsg, got %T", msg)
	return snapshotMsg{}
}

func navigateToSection(t *testing.T, current Model, target section) Model {
	t.Helper()

	for step := 0; step < len(sections)+1; step++ {
		if current.active == target {
			return current
		}
		updatedModel, _ := current.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		current = updatedModel.(Model)
	}

	t.Fatalf("failed to navigate to section %v", target)
	return current
}

func newConfigTestModel(cfg configpkg.Config, manager ConfigManager) Model {
	loader := func() (Snapshot, error) {
		return configSnapshot(cfg, ConfigSourceLoaded, ""), nil
	}
	return NewFullModel(loader, nil, nil, &manager)
}

func loadConfigSnapshotModel(t *testing.T, model Model, snapshot Snapshot) Model {
	t.Helper()

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	current := updatedModel.(Model)
	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	return updatedModel.(Model)
}

func loadConfigSnapshotModelSized(t *testing.T, model Model, snapshot Snapshot, width int, height int) Model {
	t.Helper()

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	current := updatedModel.(Model)
	updatedModel, _ = current.Update(snapshotMsg{snapshot: snapshot})
	return updatedModel.(Model)
}

func configSnapshot(cfg configpkg.Config, source ConfigSourceState, problem string) Snapshot {
	cfg.ApplyDerivedFields()
	return Snapshot{
		ConfigPath:        "/tmp/stackctl/config.yaml",
		ConfigData:        cfg,
		ConfigSource:      source,
		ConfigProblem:     problem,
		StackName:         cfg.Stack.Name,
		StackDir:          cfg.Stack.Dir,
		ComposePath:       configpkg.ComposePath(cfg),
		Managed:           cfg.Stack.Managed,
		WaitForServices:   cfg.Behavior.WaitForServicesStart,
		StartupTimeoutSec: cfg.Behavior.StartupTimeoutSec,
		LoadedAt:          time.Now(),
	}
}

func configTestManager() ConfigManager {
	return ConfigManager{
		DefaultConfig:  configpkg.Default,
		SaveConfig:     func(string, configpkg.Config) error { return nil },
		ValidateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		MarshalConfig:  configpkg.Marshal,
		ManagedStackNeedsScaffold: func(configpkg.Config) (bool, error) {
			return false, nil
		},
		ScaffoldManagedStack: func(cfg configpkg.Config, _ bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg), WroteCompose: true}, nil
		},
	}
}

func sendTextToModel(t *testing.T, current Model, value string) Model {
	t.Helper()

	for _, r := range value {
		updatedModel, _ := current.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		current = updatedModel.(Model)
	}
	return current
}
