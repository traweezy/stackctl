package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/help"
	bubblesstopwatch "charm.land/bubbles/v2/stopwatch"
	bubblestimer "charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func TestModelHandleConfigKeyCoversSaveResetPreviewAndScaffoldBranches(t *testing.T) {
	cfg := configpkg.Default()
	manager := configTestManager()
	saveCalls := 0
	manager.SaveConfig = func(string, configpkg.Config) error {
		saveCalls++
		return nil
	}

	model := newConfigTestModel(cfg, manager)
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))
	current = navigateToSection(t, current, configSection)

	cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if !handled || cmd == nil {
		t.Fatalf("expected clean reset to be handled with a banner command")
	}
	if current.banner == nil || current.banner.Message != "config draft is already clean" {
		t.Fatalf("unexpected clean reset banner: %+v", current.banner)
	}

	current.configEditor.preview = false
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if !handled || cmd != nil || !current.configEditor.preview {
		t.Fatalf("expected preview toggle to flip state, handled=%v cmd=%v preview=%v", handled, cmd, current.configEditor.preview)
	}

	current.configEditor.draft.Connection.Host = ""
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if !handled || cmd == nil {
		t.Fatalf("expected apply-defaults to be handled with a banner clear command")
	}
	if current.configEditor.draft.Connection.Host != "localhost" {
		t.Fatalf("expected apply defaults to restore derived host, got %q", current.configEditor.draft.Connection.Host)
	}
	if current.banner == nil || current.banner.Message != "applied derived defaults to the draft" {
		t.Fatalf("unexpected apply-defaults banner: %+v", current.banner)
	}

	current.runningConfigOp = &configOperation{Message: "busy"}
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if !handled || cmd != nil {
		t.Fatalf("expected scaffold to be swallowed while busy, handled=%v cmd=%v", handled, cmd)
	}
	current.runningConfigOp = nil

	current.configEditor.draft.Stack.Managed = false
	current.configEditor.draft.Setup.ScaffoldDefaultStack = false
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if !handled || cmd == nil {
		t.Fatalf("expected invalid scaffold request to produce a banner clear command")
	}
	if current.banner == nil || !strings.Contains(current.banner.Message, "enable a managed stack before scaffolding") {
		t.Fatalf("unexpected scaffold warning banner: %+v", current.banner)
	}

	current.configEditor.draft = cfg
	current.configEditor.syncValidation(current.configManager)
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if !handled || cmd == nil || current.runningConfigOp == nil {
		t.Fatalf("expected scaffold command to start, handled=%v cmd=%v op=%+v", handled, cmd, current.runningConfigOp)
	}
	if current.runningConfigOp.Message != "Scaffolding managed stack" {
		t.Fatalf("unexpected scaffold pending message: %+v", current.runningConfigOp)
	}
	result := msgOfType[configOperationMsg](t, cmd)
	if result.Status != output.StatusOK || !result.Reload {
		t.Fatalf("unexpected scaffold result: %+v", result)
	}

	current.runningConfigOp = nil
	cmd, handled = current.handleConfigKey(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if !handled || cmd == nil || current.runningConfigOp == nil {
		t.Fatalf("expected force scaffold command to start, handled=%v cmd=%v op=%+v", handled, cmd, current.runningConfigOp)
	}
	if current.runningConfigOp.Message != "Refreshing managed scaffold" {
		t.Fatalf("unexpected force scaffold pending message: %+v", current.runningConfigOp)
	}

	current.runningConfigOp = nil
	saveCalls = 0
	cmd, handled = current.startConfigSaveFlow()
	if !handled || cmd == nil || current.banner == nil || current.banner.Message != "config draft already matches disk" {
		t.Fatalf("expected clean save flow to short-circuit with a banner, handled=%v cmd=%v banner=%+v", handled, cmd, current.banner)
	}
	if saveCalls != 0 {
		t.Fatalf("expected clean save flow not to write config, got %d calls", saveCalls)
	}

	current.banner = nil
	current.configEditor.source = ConfigSourceMissing
	current.configEditor.draft.Connection.Host = "db.internal"
	current.configEditor.syncValidation(current.configManager)
	cmd, handled = current.startConfigSaveFlow()
	if !handled || cmd == nil || current.runningConfigOp == nil {
		t.Fatalf("expected dirty save flow to start, handled=%v cmd=%v op=%+v", handled, cmd, current.runningConfigOp)
	}
	if current.runningConfigOp.Message != "Saving config" {
		t.Fatalf("unexpected save-flow pending message: %+v", current.runningConfigOp)
	}
	result = msgOfType[configOperationMsg](t, cmd)
	if result.Status != output.StatusOK || !result.Reload {
		t.Fatalf("unexpected save-flow result: %+v", result)
	}
	if saveCalls == 0 {
		t.Fatal("expected save flow to call SaveConfig")
	}
}

func TestConfigEditorCommitAndSaveFlowCoversValidationAndCommitBranches(t *testing.T) {
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
		t.Fatal("expected postgres port edit to start")
	}
	current.configEditor.input.SetValue("invalid-port")

	if err := current.configEditor.commitEdit(); err == nil {
		t.Fatal("expected invalid commitEdit to fail")
	}
	if saveCalls != 0 {
		t.Fatalf("expected invalid edit not to save, got %d calls", saveCalls)
	}

	current.configEditor.input.SetValue("15432")
	if err := current.configEditor.commitEdit(); err != nil {
		t.Fatalf("expected valid commitEdit to succeed, got %v", err)
	}
	current.configEditor.syncValidation(current.configManager)
	current.configEditor.refreshList(false)
	cmd, handled := current.startConfigSaveFlow()
	if !handled || cmd == nil || current.runningConfigOp == nil {
		t.Fatalf("expected committed save flow to start, handled=%v cmd=%v op=%+v", handled, cmd, current.runningConfigOp)
	}
	if current.configEditor.editing {
		t.Fatal("expected valid commitEdit to leave edit mode before saving")
	}
	result := msgOfType[configOperationMsg](t, cmd)
	if result.Status != output.StatusOK || !result.Reload {
		t.Fatalf("unexpected committed save result: %+v", result)
	}
	if saveCalls != 1 {
		t.Fatalf("expected one committed save call, got %d", saveCalls)
	}
}

func TestModelUpdateCoversBusyStateAndReloadBranches(t *testing.T) {
	cfg := configpkg.Default()
	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModel(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""))

	updated, cmd := current.Update(tea.BackgroundColorMsg{})
	current = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command for background update, got %v", cmd)
	}

	enhancements := tea.KeyboardEnhancementsMsg{}
	updated, _ = current.Update(enhancements)
	current = updated.(Model)
	if current.keyboardFeatures != enhancements {
		t.Fatalf("expected keyboard enhancements to persist, got %+v", current.keyboardFeatures)
	}

	updated, cmd = current.Update(QuitBlockedMsg{Reason: "busy"})
	current = updated.(Model)
	if cmd == nil || current.banner == nil || current.banner.Message != "busy" {
		t.Fatalf("expected quit-blocked update to set a banner, cmd=%v banner=%+v", cmd, current.banner)
	}

	current.runningConfigOp = &configOperation{Message: "Applying config changes"}
	current.beginBusy(15 * time.Second)
	updated, cmd = current.Update(configOperationMsg{
		Status:  output.StatusOK,
		Message: "saved config",
		Reload:  true,
	})
	current = updated.(Model)
	if current.runningConfigOp != nil || !current.loading {
		t.Fatalf("expected reload operation to clear busy op and enter loading, op=%+v loading=%v", current.runningConfigOp, current.loading)
	}
	if cmd == nil {
		t.Fatal("expected reload operation to issue follow-up commands")
	}

	current.loading = false
	current.runningConfigOp = &configOperation{Message: "Saving config"}
	current.beginBusy(0)
	updated, cmd = current.Update(configOperationMsg{
		Status:  output.StatusFail,
		Message: "save failed",
		Err:     errors.New("disk full"),
	})
	current = updated.(Model)
	if current.runningConfigOp != nil {
		t.Fatalf("expected failed config operation to clear the running op, got %+v", current.runningConfigOp)
	}
	if current.banner == nil || current.banner.Status != output.StatusFail {
		t.Fatalf("expected failed config operation to show a failure banner, got %+v", current.banner)
	}
	if cmd == nil {
		t.Fatal("expected failed config operation to still return cleanup commands")
	}

	current.autoRefresh = true
	current.autoRefreshID = 7
	current.configEditor.draft.Connection.Host = "db.internal"
	updated, cmd = current.Update(autoRefreshMsg{id: 7})
	current = updated.(Model)
	if cmd != nil || current.loading {
		t.Fatalf("expected dirty draft to block auto-refresh, loading=%v cmd=%v", current.loading, cmd)
	}

	current.configEditor.draft = current.configEditor.baseline
	current.configEditor.syncValidation(current.configManager)
	updated, cmd = current.Update(autoRefreshMsg{id: 7})
	current = updated.(Model)
	if !current.loading || cmd == nil {
		t.Fatalf("expected clean auto-refresh to trigger loading, loading=%v cmd=%v", current.loading, cmd)
	}
}

func TestBusyAndSidebarHelpersCoverCompactBranches(t *testing.T) {
	model := Model{
		help: help.New(),
		snapshot: Snapshot{
			WaitForServices:   true,
			StartupTimeoutSec: 30,
		},
		runningAction: &runningAction{Action: ActionSpec{ID: ActionRestart}},
	}

	cmd := model.beginBusy(10 * time.Second)
	if cmd == nil || model.busyBudget != 10*time.Second {
		t.Fatalf("expected beginBusy to initialize timers, cmd=%v budget=%s", cmd, model.busyBudget)
	}
	if model.busyTimer.Timeout != 10*time.Second {
		t.Fatalf("expected busy timer timeout to match the budget, got %s", model.busyTimer.Timeout)
	}

	model.busyStartedAt = time.Now().Add(-3 * time.Second)
	if got := renderBusyProgress(model, 120); !strings.Contains(got, "left") {
		t.Fatalf("expected busy progress to include remaining time, got %q", got)
	}

	stopCmd := model.finishBusy()
	if stopCmd == nil || model.busyBudget != 0 || !model.busyStartedAt.IsZero() {
		t.Fatalf("expected finishBusy to reset state, cmd=%v budget=%s started=%s", stopCmd, model.busyBudget, model.busyStartedAt)
	}

	noBudgetModel := Model{}
	noBudgetModel.busyStopwatch = bubblesstopwatch.New()
	noBudgetModel.busyTimer = bubblestimer.Model{}
	if got := renderBusyProgress(noBudgetModel, 120); got != "" {
		t.Fatalf("expected idle busy progress to stay blank, got %q", got)
	}

	if got := sidebarHelpLabel(Model{}); got != "short" {
		t.Fatalf("expected short help label, got %q", got)
	}
	if got := sidebarHelpLabel(Model{help: helpModelWithFullHelp(true)}); got != "full" {
		t.Fatalf("expected full help label, got %q", got)
	}

	if got := displayServiceStatus(Service{ContainerName: "stack-postgres"}); got != "not running" {
		t.Fatalf("expected missing stack service status to render as not running, got %q", got)
	}
	if got := displayServiceStatus(Service{Status: "missing", ContainerName: "stack-postgres"}); got != "not running" {
		t.Fatalf("expected missing stack container to render as not running, got %q", got)
	}
	if got := displayServiceStatus(Service{}); got != "-" {
		t.Fatalf("expected host tool without status to render as -, got %q", got)
	}

	if got := renderConfirmationPanel(nil, 80, 20); got != "" {
		t.Fatalf("expected nil confirmation panel to render empty, got %q", got)
	}
	panel := stripANSITest(renderConfirmationPanel(newConfigResetConfirmation(), 80, 20))
	if !strings.Contains(panel, "Reset draft") {
		t.Fatalf("expected confirmation panel to render reset confirmation, got %q", panel)
	}
}

func helpModelWithFullHelp(showAll bool) help.Model {
	model := help.New()
	model.ShowAll = showAll
	return model
}
