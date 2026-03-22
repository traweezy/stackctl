package tui

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
	if current.active != servicesSection {
		t.Fatalf("expected services section, got %v", current.active)
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
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
	current = navigateToSection(t, current, connectionsSection)

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
	if !strings.Contains(expandedContent, "Copy placeholders: Postgres DSN") {
		t.Fatalf("expected expanded services content to contain copy placeholders:\n%s", expandedContent)
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
	if !strings.Contains(compactContent, "Copy placeholders: Postgres DSN") {
		t.Fatalf("expected compact services content to contain copy placeholders:\n%s", compactContent)
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

func TestWatchLogsKeyUsesSelectedPortService(t *testing.T) {
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
	current.active = portsSection
	current.normalizeSelections()
	current.syncLayout()

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	current = updatedModel.(Model)
	updatedModel, cmd := current.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	current = updatedModel.(Model)
	if cmd == nil {
		t.Fatalf("expected watch logs command from ports section")
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

func TestSelectionKeysSwitchPortDetailPane(t *testing.T) {
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
	current.active = portsSection
	current.normalizeSelections()
	current.syncLayout()

	if !collapsedContainsTest(current.currentContent(), "Host port: 5432") {
		t.Fatalf("expected initial port detail to show postgres:\n%s", current.currentContent())
	}

	updatedModel, _ = current.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	current = updatedModel.(Model)
	if current.selectedPort != "redis" {
		t.Fatalf("expected selected port to switch to redis, got %q", current.selectedPort)
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
	if strings.Contains(servicesContent, "[1] Restart") || strings.Contains(servicesContent, "Actions") {
		t.Fatalf("expected services panel content to stay action-free:\n%s", servicesContent)
	}
	servicesSidebar := renderSidebar(current)
	if !strings.Contains(servicesSidebar, "Actions") || !strings.Contains(servicesSidebar, "[1] Restart") {
		t.Fatalf("expected services sidebar to keep global actions visible:\n%s", servicesSidebar)
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
