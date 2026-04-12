package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	bubblesstopwatch "charm.land/bubbles/v2/stopwatch"
	bubblestimer "charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func TestTUIAdditionalKeyMapAndInitCoverage(t *testing.T) {
	keys := defaultKeyMap()
	if got := keys.PrevItem.Help().Desc; got != "previous item" {
		t.Fatalf("unexpected PrevItem help: %q", got)
	}
	if got := keys.NextItem.Help().Desc; got != "next item" {
		t.Fatalf("unexpected NextItem help: %q", got)
	}
	if got := keys.WatchLogs.Help().Desc; got != "watch logs" {
		t.Fatalf("unexpected WatchLogs help: %q", got)
	}
	if got := keys.ToggleAutoRefresh.Help().Desc; got != "toggle auto-refresh" {
		t.Fatalf("unexpected ToggleAutoRefresh help: %q", got)
	}
	if got := keys.ToggleLayout.Help().Desc; got != "toggle compact view" {
		t.Fatalf("unexpected ToggleLayout help: %q", got)
	}
	if got := keys.ToggleSecrets.Help().Desc; got != "show or hide secrets" {
		t.Fatalf("unexpected ToggleSecrets help: %q", got)
	}

	helpMap := helpBindings{
		short: []key.Binding{keys.Refresh},
		full:  [][]key.Binding{{keys.PrevItem, keys.NextItem}, {keys.WatchLogs}},
	}
	if got := helpMap.FullHelp(); len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 1 {
		t.Fatalf("unexpected full help layout: %+v", got)
	}

	model := NewFullModel(func() (Snapshot, error) { return Snapshot{}, nil }, nil, nil, nil).WithVersion(" v1.2.3 ")
	if model.loading != true || model.autoRefresh != true || model.mouseEnabled != MouseEnabledFromEnv() || model.altScreen != AltScreenEnabledFromEnv() {
		t.Fatalf("unexpected new model defaults: %+v", model)
	}
	if strings.TrimSpace(model.appVersion) != "v1.2.3" {
		t.Fatalf("expected version to be trimmed, got %q", model.appVersion)
	}

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a batch command")
	}
	if _, ok := findMsgOfType[snapshotMsg](cmd()); !ok {
		t.Fatal("expected init batch to include a snapshot load message")
	}
}

func TestTUIAdditionalUpdateCoverage(t *testing.T) {
	snapshot := Snapshot{}
	model := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)

	updated, cmd := model.Update(tea.BackgroundColorMsg{})
	current := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected background color update to return nil cmd, got %v", cmd)
	}

	current.loading = true
	current.busyStopwatch = bubblesstopwatch.New(bubblesstopwatch.WithInterval(time.Second))
	updated, cmd = current.Update(bubblesstopwatch.TickMsg{})
	current = updated.(Model)
	if !current.isBusy() {
		t.Fatalf("expected stopwatch tick to preserve busy state, cmd=%v", cmd)
	}

	current.loading = true
	current.busyBudget = time.Second
	current.busyTimer = bubblestimer.New(time.Second, bubblestimer.WithInterval(time.Second))
	updated, cmd = current.Update(bubblestimer.TickMsg{})
	current = updated.(Model)
	if cmd == nil {
		t.Fatal("expected timer tick while busy to return a command")
	}

	current.configEditor.baseline = current.configEditor.draft
	updated, cmd = current.Update(configOperationMsg{Status: output.StatusOK, Message: "saved", Reload: true})
	current = updated.(Model)
	if cmd == nil || !current.loading || current.configEditor.source != ConfigSourceLoaded {
		t.Fatalf("expected reload config operation to trigger a refresh, model=%+v cmd=%v", current, cmd)
	}

	current.loading = false
	current.autoRefresh = true
	current.autoRefreshID = 7
	updated, cmd = current.Update(logWatchDoneMsg{Service: "postgres"})
	current = updated.(Model)
	if cmd == nil || !current.loading {
		t.Fatalf("expected log watch completion to trigger a refresh, model=%+v cmd=%v", current, cmd)
	}

	current.confirmation = &confirmationState{Title: "Stop the stack?"}
	updated, cmd = current.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if cmd != nil {
		t.Fatalf("expected unrelated confirmation key to be ignored, got %v", cmd)
	}
	if updated.(Model).confirmation == nil {
		t.Fatal("expected confirmation to remain active after ignored key")
	}
}

func TestTUIAdditionalHelperCoverageBatchTwo(t *testing.T) {
	if got := section(99).Title(); got != "Unknown" {
		t.Fatalf("unexpected unknown section title: %q", got)
	}

	model := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	initMsg := model.Init()()
	batch, ok := initMsg.(tea.BatchMsg)
	if !ok || len(batch) < 2 {
		t.Fatalf("expected init to return a batch of startup commands, got %T", initMsg)
	}
	if msg, ok := findMsgOfType[snapshotMsg](initMsg); !ok || msg.err != nil {
		t.Fatalf("expected init batch to include a snapshot load message, msg=%+v ok=%v", msg, ok)
	}
	if colorMsg := batch[1](); colorMsg == nil {
		t.Fatal("expected init to execute a background-color request command")
	}
	if previousSection(section(99)) != overviewSection {
		t.Fatal("expected previousSection to fall back to overview for unknown sections")
	}

	cfg := configpkg.Default()
	manager := configTestManager()
	current := loadConfigSnapshotModel(t, newConfigTestModel(cfg, manager), configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.selectedKey = "ports.postgres"
	current.configEditor.refreshList(false)
	if cmd := current.configEditor.beginEdit(false); cmd == nil {
		t.Fatal("expected config edit to begin")
	}
	current.configEditor.input.SetValue("invalid-port")
	cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Mod: tea.ModCtrl, Code: 's'})
	if !handled || cmd != nil || current.configEditor.input.Err == nil {
		t.Fatalf("expected invalid edit save to be handled without a command, handled=%v cmd=%v err=%v", handled, cmd, current.configEditor.input.Err)
	}
	current.configEditor.cancelEdit()

	current.runningConfigOp = &configOperation{Message: "busy"}
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if !handled || cmd != nil {
		t.Fatalf("expected force scaffold to short-circuit while busy, handled=%v cmd=%v", handled, cmd)
	}

	current.runningConfigOp = nil
	current.configEditor.draft.Stack.Managed = false
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if !handled || cmd == nil || current.banner == nil || !strings.Contains(current.banner.Message, "enable a managed stack") {
		t.Fatalf("expected force scaffold to warn when scaffolding is unavailable, handled=%v cmd=%v banner=%+v", handled, cmd, current.banner)
	}

	guardModel := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	cmd, handled = guardModel.startConfigSaveFlow()
	if !handled || cmd != nil {
		t.Fatalf("expected save flow to stop when config editing is unavailable, handled=%v cmd=%v", handled, cmd)
	}

	current = loadConfigSnapshotModel(t, newConfigTestModel(cfg, manager), configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)
	current.configEditor.baseline = current.configEditor.draft
	current.configEditor.source = ConfigSourceLoaded
	current.configEditor.scaffoldProblem = "boom"
	cmd, handled = current.startConfigSaveFlow()
	if !handled || cmd == nil || current.banner == nil || current.banner.Message != "resolve the managed scaffold problem before applying" {
		t.Fatalf("expected custom save-plan reason banner, handled=%v cmd=%v banner=%+v", handled, cmd, current.banner)
	}

	snapshot := Snapshot{StackName: "dev-stack"}
	current = loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
	current.palette = newPaletteState(
		paletteModeJump,
		"Jump",
		"Choose a destination",
		[]paletteAction{{Title: "Go to services", Section: servicesSection}},
	)
	if body := renderBody(current); !strings.Contains(body, "Jump") {
		t.Fatalf("expected renderBody to show the palette panel, got %q", body)
	}

	current.active = servicesSection
	current.selectedService = "missing"
	if got := sidebarCompactSelectionLabel(current); got != "" {
		t.Fatalf("expected compact services selection label to be empty, got %q", got)
	}

	current.active = healthSection
	current.selectedHealth = "missing"
	if got := sidebarCompactSelectionLabel(current); got != "" {
		t.Fatalf("expected compact health selection label to be empty, got %q", got)
	}

	current.active = section(99)
	if got := sidebarCompactSelectionLabel(current); got != "" {
		t.Fatalf("expected compact selection label to be empty for unknown sections, got %q", got)
	}

	current.active = stacksSection
	current.selectedStack = "missing"
	if got := sidebarSelectionLines(current); got != nil {
		t.Fatalf("expected no stack selection lines for a missing profile, got %+v", got)
	}

	current.active = servicesSection
	current.selectedService = "missing"
	if got := sidebarSelectionLines(current); got != nil {
		t.Fatalf("expected no service selection lines for a missing service, got %+v", got)
	}

	current.active = healthSection
	current.selectedHealth = "missing"
	if got := sidebarSelectionLines(current); got != nil {
		t.Fatalf("expected no health selection lines for a missing service, got %+v", got)
	}

	service := Service{DisplayName: "Cockpit", Status: "missing"}
	if got := healthNote(service); got != "Service is not installed." {
		t.Fatalf("unexpected host-tool missing health note: %q", got)
	}

	dsn := "postgres://stackuser@localhost:5432/app"
	if got := maskConnectionValue(dsn, false); got != dsn {
		t.Fatalf("expected DSN without a password to stay unchanged, got %q", got)
	}

	busyModel := NewModel(func() (Snapshot, error) { return Snapshot{}, nil })
	if startCmd := busyModel.beginBusy(time.Second); startCmd == nil {
		t.Fatal("expected beginBusy to return a batch command")
	}
	startMsg := busyModel.busyStopwatch.Start()()
	sequence := reflect.ValueOf(startMsg)
	if sequence.Kind() != reflect.Slice || sequence.Len() == 0 {
		t.Fatalf("expected stopwatch start command to yield a command sequence, got %T", startMsg)
	}
	firstCmd, ok := sequence.Index(0).Interface().(tea.Cmd)
	if !ok || firstCmd == nil {
		t.Fatalf("expected stopwatch start sequence to begin with a command, got %T", sequence.Index(0).Interface())
	}
	var stopCmd tea.Cmd
	busyModel.busyStopwatch, stopCmd = busyModel.busyStopwatch.Update(firstCmd())
	if !busyModel.busyStopwatch.Running() {
		t.Fatalf("expected stopwatch start update to enter the running state, cmd=%v running=%v", stopCmd, busyModel.busyStopwatch.Running())
	}
	if stopCmd := busyModel.finishBusy(); stopCmd == nil {
		t.Fatal("expected finishBusy to stop a running stopwatch")
	}
}
