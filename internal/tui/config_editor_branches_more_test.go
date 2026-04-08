package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

type bogusListItem string

func (b bogusListItem) FilterValue() string { return string(b) }

func withTemporaryConfigFieldSpecs(t *testing.T, groups []string, specs []configFieldSpec) {
	t.Helper()

	originalGroups := append([]string(nil), configFieldGroupOrder...)
	originalSpecs := append([]configFieldSpec(nil), configFieldSpecs...)
	configFieldGroupOrder = append([]string(nil), groups...)
	configFieldSpecs = append([]configFieldSpec(nil), specs...)

	t.Cleanup(func() {
		configFieldGroupOrder = originalGroups
		configFieldSpecs = originalSpecs
	})
}

func TestConfigEditorAdditionalBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTemporaryConfigFieldSpecs(t, []string{"Empty", "Only"}, []configFieldSpec{
		{
			Key:   "flag",
			Group: "Only",
			Label: "Flag",
			Kind:  configFieldBool,
			GetBool: func(cfg configpkg.Config) bool {
				return cfg.Setup.IncludeNATS
			},
			SetBool: func(cfg *configpkg.Config, value bool) error {
				cfg.Setup.IncludeNATS = value
				return nil
			},
		},
		{
			Key:    "secret",
			Group:  "Only",
			Label:  "Secret",
			Kind:   configFieldString,
			Secret: true,
			GetString: func(cfg configpkg.Config) string {
				return cfg.Connection.PostgresPassword
			},
			SetString: func(cfg *configpkg.Config, value string) error {
				cfg.Connection.PostgresPassword = value
				return nil
			},
		},
		{
			Key:   "plain",
			Group: "Only",
			Label: "Plain",
			Kind:  configFieldString,
			GetString: func(cfg configpkg.Config) string {
				return cfg.Connection.Host
			},
			SetString: func(cfg *configpkg.Config, value string) error {
				cfg.Connection.Host = value
				return nil
			},
		},
		{
			Key:   "blocked",
			Group: "Only",
			Label: "Blocked",
			Kind:  configFieldString,
			EditableReason: func(configpkg.Config) string {
				return "blocked in tests"
			},
		},
		{
			Key:   "invalid",
			Group: "Only",
			Label: "Invalid",
			Kind:  configFieldString,
			GetString: func(cfg configpkg.Config) string {
				return cfg.Stack.Name
			},
			SetString: func(cfg *configpkg.Config, value string) error {
				cfg.Stack.Name = value
				return nil
			},
			InputValidate: func(configpkg.Config, string) error {
				return errors.New("invalid value")
			},
		},
		{
			Key:   "setter",
			Group: "Only",
			Label: "Setter",
			Kind:  configFieldString,
			GetString: func(cfg configpkg.Config) string {
				return cfg.Stack.Name
			},
			SetString: func(*configpkg.Config, string) error {
				return errors.New("setter boom")
			},
		},
		{
			Key:   "weird",
			Group: "Only",
			Label: "Weird",
			Kind:  configFieldKind(99),
		},
	})

	manager := configTestManager()
	manager.ValidateConfig = func(configpkg.Config) []configpkg.ValidationIssue {
		return []configpkg.ValidationIssue{{Field: "plain", Message: "dirty warning"}}
	}

	editor := newConfigEditor()
	editor.baseline = cfg
	editor.draft = cfg

	editor.path = "keep"
	editor.syncFromSnapshot(configSnapshot(cfg, ConfigSourceLoaded, ""), nil, false, true)
	if editor.path != "keep" {
		t.Fatalf("expected sync without manager to no-op, path=%q", editor.path)
	}

	editor.draft.Connection.Host = "dirty-host"
	editor.syncFromSnapshot(configSnapshot(cfg, ConfigSourceLoaded, ""), &manager, false, false)
	if editor.path != "keep" || len(editor.issues) != 1 {
		t.Fatalf("expected dirty snapshot sync to keep draft and validate, path=%q issues=%+v", editor.path, editor.issues)
	}

	editor.draft = cfg
	editor.setSize(1, 1, false)
	if editor.width != 40 || editor.height != 4 {
		t.Fatalf("unexpected clamped editor size: %dx%d", editor.width, editor.height)
	}
	if len(editor.fieldList.Items()) == 0 {
		t.Fatal("expected refreshList to populate custom config items")
	}

	if got := editor.selectedFieldIndex([]list.Item{bogusListItem("bogus")}); got != 0 {
		t.Fatalf("expected fallback selected index 0, got %d", got)
	}

	editor.fieldList.SetItems([]list.Item{bogusListItem("bogus")})
	editor.fieldList.Select(0)
	editor.moveSelection(1)
	if editor.selectedKey != "" {
		t.Fatalf("expected bogus item selection to clear selected key, got %q", editor.selectedKey)
	}
	editor.fieldList.SetItems(nil)
	editor.moveSelection(0)

	var rendered strings.Builder
	delegate := configListDelegate{}
	delegateModel := list.New(nil, delegate, 24, 4)
	delegate.Render(&rendered, delegateModel, 0, bogusListItem("bogus"))
	if rendered.Len() != 0 {
		t.Fatalf("expected invalid delegate item to render nothing, got %q", rendered.String())
	}
	rendered.Reset()
	delegate.Render(&rendered, delegateModel, 0, configListItem{kind: configListGroupRow, group: "Only"})
	if !strings.Contains(stripANSITest(rendered.String()), "Only") {
		t.Fatalf("expected group row output, got %q", rendered.String())
	}
	rendered.Reset()
	delegate.Render(&rendered, delegateModel, 0, configListItem{
		kind:    configListFieldRow,
		label:   "Plain",
		value:   "value",
		warning: true,
	})
	if !strings.Contains(stripANSITest(rendered.String()), "!Plain") {
		t.Fatalf("expected warning row prefix, got %q", rendered.String())
	}

	editor.selectedKey = "missing"
	if cmd := editor.beginEdit(false); cmd != nil {
		t.Fatalf("expected missing selected spec to return nil cmd, got %v", cmd)
	}

	editor.selectedKey = "blocked"
	if cmd := editor.beginEdit(false); cmd != nil {
		t.Fatalf("expected blocked field edit to return nil cmd, got %v", cmd)
	}

	editor.selectedKey = "weird"
	if cmd := editor.beginEdit(false); cmd != nil {
		t.Fatalf("expected unsupported field kind to return nil cmd, got %v", cmd)
	}

	editor.draft.Setup.IncludeNATS = false
	editor.selectedKey = "flag"
	_ = editor.beginEdit(false)
	if !editor.draft.Setup.IncludeNATS {
		t.Fatal("expected bool field edit to toggle the draft value")
	}

	editor.selectedKey = "secret"
	if cmd := editor.beginEdit(false); cmd == nil {
		t.Fatal("expected secret field edit to return a focus command")
	}
	if !editor.editing || editor.input.EchoMode != textinput.EchoPassword {
		t.Fatalf("expected password echo mode while editing secrets, editing=%v echo=%v", editor.editing, editor.input.EchoMode)
	}
	editor.cancelEdit()

	editor.selectedKey = "invalid"
	_ = editor.beginEdit(true)
	editor.input.SetValue("bad")
	if err := editor.commitEdit(); err == nil || !strings.Contains(err.Error(), "invalid value") {
		t.Fatalf("expected validation failure while committing edit, got %v", err)
	}

	editor.cancelEdit()
	editor.selectedKey = "setter"
	_ = editor.beginEdit(true)
	editor.input.SetValue("boom")
	if err := editor.commitEdit(); err == nil || !strings.Contains(err.Error(), "setter boom") {
		t.Fatalf("expected setter failure while committing edit, got %v", err)
	}

	editor.cancelEdit()
	editor.selectedKey = "plain"
	if err := editor.commitEdit(); err != nil {
		t.Fatalf("expected noop commit outside edit mode, got %v", err)
	}

	keys := defaultKeyMap()
	editor.selectedKey = "plain"
	_ = editor.beginEdit(true)
	if _, handled := editor.handleKey(tea.KeyPressMsg{Code: 'x', Text: "x"}, keys, &manager, true); !handled {
		t.Fatal("expected text input update to be handled while editing")
	}
	editor.cancelEdit()
	editor.refreshList(true)
	if _, handled := editor.handleKey(tea.KeyPressMsg{Code: 'j', Text: "j"}, keys, &manager, true); !handled {
		t.Fatal("expected next-item key to be handled")
	}
	if _, handled := editor.handleKey(tea.KeyPressMsg{Code: 'k', Text: "k"}, keys, &manager, true); !handled {
		t.Fatal("expected previous-item key to be handled")
	}
	if _, handled := editor.handleKey(tea.KeyPressMsg{Code: 'z', Text: "z"}, keys, &manager, true); handled {
		t.Fatal("expected unrelated key to fall through")
	}

	editor.source = ConfigSourceLoaded
	editor.sourceMessage = "loaded note"
	if got := editor.summaryStatus(); got != "loaded note" {
		t.Fatalf("unexpected loaded summary with message: %q", got)
	}
	editor.source = ConfigSourceMissing
	editor.sourceMessage = "missing note"
	if got := editor.summaryStatus(); got != "missing note" {
		t.Fatalf("unexpected missing summary with message: %q", got)
	}
	editor.source = ConfigSourceUnavailable
	editor.sourceMessage = "unavailable note"
	if got := editor.summaryStatus(); got != "unavailable note" {
		t.Fatalf("unexpected unavailable summary with message: %q", got)
	}

	editor.source = ConfigSourceLoaded
	editor.preview = true
	if got := stripANSITest(editor.renderDetail(true, 20)); !strings.Contains(got, "No unsaved config changes to preview") {
		t.Fatalf("unexpected preview detail output: %q", got)
	}

	editor.preview = false
	editor.selectedKey = "missing"
	if got := stripANSITest(editor.renderDetail(true, 20)); !strings.Contains(got, "No config field is selected") {
		t.Fatalf("unexpected no-selection detail output: %q", got)
	}

	editor.selectedKey = "plain"
	editor.issueIndex = map[string][]configpkg.ValidationIssue{
		"plain": {{Field: "plain", Message: "broken"}},
	}
	editor.editing = true
	editor.input.Err = errors.New("input broken")
	if got := stripANSITest(editor.renderDetail(true, 20)); !strings.Contains(got, "Validation") || !strings.Contains(got, "Enter keeps the draft") {
		t.Fatalf("unexpected detailed field output: %q", got)
	}
	editor.editing = false
	editor.input.Err = nil

	editor.width = 0
	editor.height = 0
	if got := stripANSITest(editor.View(true)); !strings.Contains(got, "Config editor is loading") {
		t.Fatalf("unexpected loading view output: %q", got)
	}

	editor.fieldList.SetItems(nil)
	if got := editor.renderFieldList(2, 40); got != "" {
		t.Fatalf("expected empty field list render, got %q", got)
	}
	editor.fieldList.SetItems([]list.Item{bogusListItem("bogus")})
	if got := editor.renderFieldList(2, 40); strings.Count(got, "\n") < 1 {
		t.Fatalf("expected padded field list render, got %q", got)
	}

	if got := clipAndPadText("content", 0); got != "" {
		t.Fatalf("expected empty clipped content for zero height, got %q", got)
	}
	if got := stripANSITest(renderConfigListRow(configListItem{kind: configListFieldRow, label: "Plain", value: "value", warning: true}, false, 28)); !strings.Contains(got, "!Plain") {
		t.Fatalf("unexpected warning config row output: %q", got)
	}
}

func TestPaletteAndConfigModelAdditionalBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModelSized(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""), 80, 18)
	service := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		ContainerName: "local-postgres",
		Host:          "127.0.0.1",
		ExternalPort:  5432,
		DSN:           "postgres://dev:secret@127.0.0.1:5432/dev?sslmode=disable",
		Username:      "dev",
		Password:      "secret",
		Database:      "dev",
	}
	current.snapshot.Services = []Service{service}
	current.snapshot.Stacks = []StackProfile{{Name: "dev-stack", Current: true, Configured: true, State: "running", Mode: "Managed"}}
	current.snapshot.ConnectText = "stackctl connect"
	current.snapshot.EnvExportText = "export STACKCTL=1"
	current.snapshot.PortsText = "5432->postgres"
	current.selectedService = serviceKey(service)
	current.selectedStack = "dev-stack"
	current.pinnedServices = map[string]struct{}{serviceKey(service): {}}

	actions := current.commandPaletteActions()
	joinedTitles := make([]string, 0, len(actions))
	for _, action := range actions {
		joinedTitles = append(joinedTitles, action.Title)
	}
	actionText := strings.Join(joinedTitles, "\n")
	for _, fragment := range []string{
		"Copy stackctl connect output",
		"Copy stackctl env --export output",
		"Copy stackctl ports output",
		"Watch Postgres logs",
		"Open Postgres shell",
		"Open Postgres db shell",
		"Unpin Postgres",
	} {
		if !strings.Contains(actionText, fragment) {
			t.Fatalf("expected palette actions to contain %q:\n%s", fragment, actionText)
		}
	}

	current.runner = func(ActionID) (ActionReport, error) {
		return ActionReport{Status: output.StatusOK, Message: "ok", Refresh: true}, nil
	}
	actions = current.commandPaletteActions()
	actionText = ""
	for _, action := range actions {
		actionText += action.Title + "\n"
	}
	if !strings.Contains(actionText, "Doctor") && !strings.Contains(actionText, "Start") {
		t.Fatalf("expected runner-backed sidebar actions in command palette:\n%s", actionText)
	}

	current.snapshot.Services = nil
	if actions := current.copyPaletteActions(); actions != nil {
		t.Fatalf("expected nil copy actions without a selected service, got %+v", actions)
	}
	current.snapshot.Services = []Service{service}

	current.snapshot.Services = nil
	current.selectedService = ""
	if cmd := current.openCopyPalette(); cmd == nil || current.banner == nil || current.banner.Message != "select a service before copying values" {
		t.Fatalf("expected copy palette guard banner, cmd=%v banner=%+v", cmd, current.banner)
	}

	current.snapshot.Services = []Service{service}
	current.selectedService = serviceKey(service)
	current.snapshot.Services = []Service{{Name: "postgres", DisplayName: "Postgres"}}
	if cmd := current.openCopyPalette(); cmd == nil || current.banner == nil || current.banner.Message != "no copy targets are available for the selected service" {
		t.Fatalf("expected no-copy-target banner, cmd=%v banner=%+v", cmd, current.banner)
	}

	current.snapshot.Services = []Service{service}
	current.clipboardWriter = func(value string) error {
		if value == "" {
			t.Fatal("expected copy writer to receive a value")
		}
		return nil
	}
	copyMsg := msgOfType[copyDoneMsg](t, current.startCopyAction(paletteAction{
		ServiceKey: serviceKey(service),
		CopyTarget: copyTargetPassword,
	}))
	if copyMsg.err != nil || !strings.Contains(copyMsg.message, "copied Postgres password") {
		t.Fatalf("unexpected copy action message: %+v", copyMsg)
	}

	clearMsg := msgOfType[bannerClearMsg](t, current.startCopyAction(paletteAction{
		ServiceKey: "missing",
		CopyTarget: copyTargetPassword,
	}))
	if clearMsg.id == 0 || current.banner == nil || current.banner.Message != "selected service is no longer available" {
		t.Fatalf("unexpected missing-service copy result: msg=%+v banner=%+v", clearMsg, current.banner)
	}

	copyTextClear := msgOfType[bannerClearMsg](t, current.startCopyTextAction(paletteAction{Title: "Copy connect output"}))
	if copyTextClear.id == 0 || current.banner == nil || current.banner.Message != "copy value is unavailable for this action" {
		t.Fatalf("unexpected empty copy-text result: msg=%+v banner=%+v", copyTextClear, current.banner)
	}

	current.clipboardWriter = func(string) error { return nil }
	copyTextMsg := msgOfType[copyDoneMsg](t, current.startCopyTextAction(paletteAction{
		Title:     "Copy stackctl connect output",
		CopyValue: "stackctl connect",
	}))
	if copyTextMsg.err != nil || !strings.Contains(copyTextMsg.message, "copied stackctl connect output to clipboard") {
		t.Fatalf("unexpected copy-text message: %+v", copyTextMsg)
	}

	current.active = overviewSection
	current.executePaletteAction(paletteAction{Kind: paletteActionSection, Section: servicesSection})
	if current.active != servicesSection {
		t.Fatalf("expected palette section jump to switch sections, active=%v", current.active)
	}

	current.executePaletteAction(paletteAction{Kind: paletteActionJumpStack, StackName: "dev-stack"})
	if current.active != stacksSection || current.selectedStack != "dev-stack" {
		t.Fatalf("expected stack jump action to select stack, active=%v selected=%q", current.active, current.selectedStack)
	}

	current.executePaletteAction(paletteAction{Kind: paletteActionJumpService, ServiceKey: serviceKey(service)})
	if current.active != servicesSection || current.selectedService != serviceKey(service) {
		t.Fatalf("expected service jump action to select service, active=%v selected=%q", current.active, current.selectedService)
	}

	current.executePaletteAction(paletteAction{
		Kind: paletteActionSidebar,
		Action: ActionSpec{
			ID:             ActionDoctor,
			Label:          "Run doctor",
			ConfirmMessage: "confirm doctor",
		},
	})
	if current.confirmation == nil || current.confirmation.Action.ID != ActionDoctor {
		t.Fatalf("expected confirming sidebar action to set confirmation, got %+v", current.confirmation)
	}

	current.layout = expandedLayout
	current.executePaletteAction(paletteAction{Kind: paletteActionToggleLayout})
	if current.layout != compactLayout {
		t.Fatalf("expected layout toggle to switch to compact, got %v", current.layout)
	}

	current.autoRefresh = false
	if cmd := current.executePaletteAction(paletteAction{Kind: paletteActionToggleAutoRefresh}); cmd == nil || !current.autoRefresh {
		t.Fatalf("expected auto-refresh toggle to enable scheduling, autoRefresh=%v cmd=%v", current.autoRefresh, cmd)
	}

	current.configManager = &ConfigManager{
		DefaultConfig: configpkg.Default,
		SaveConfig:    func(string, configpkg.Config) error { return nil },
		ValidateConfig: func(configpkg.Config) []configpkg.ValidationIssue {
			return nil
		},
		MarshalConfig:             configpkg.Marshal,
		ManagedStackNeedsScaffold: func(configpkg.Config) (bool, error) { return false, nil },
		ScaffoldManagedStack: func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg)}, nil
		},
	}
	current.showSecrets = false
	current.configEditor = newConfigEditor()
	current.executePaletteAction(paletteAction{Kind: paletteActionToggleSecrets})
	if !current.showSecrets {
		t.Fatal("expected secrets toggle action to enable secret display")
	}
}

func TestModelHandleConfigKeyAdditionalBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	model := newConfigTestModel(cfg, configTestManager())
	current := loadConfigSnapshotModelSized(t, model, configSnapshot(cfg, ConfigSourceLoaded, ""), 100, 26)
	current = navigateToSection(t, current, configSection)

	current.configManager = nil
	if cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'x', Text: "x"}); handled || cmd != nil {
		t.Fatalf("expected config key handling to noop without a config manager, handled=%v cmd=%v", handled, cmd)
	}

	current.configManager = &ConfigManager{
		DefaultConfig:  configpkg.Default,
		SaveConfig:     func(string, configpkg.Config) error { return nil },
		ValidateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
		MarshalConfig:  configpkg.Marshal,
		ManagedStackNeedsScaffold: func(configpkg.Config) (bool, error) {
			return false, nil
		},
		ScaffoldManagedStack: func(cfg configpkg.Config, force bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{StackDir: cfg.Stack.Dir, ComposePath: configpkg.ComposePath(cfg)}, nil
		},
	}
	current.runningConfigOp = &configOperation{Message: "busy"}
	if cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'g', Text: "g"}); !handled || cmd != nil {
		t.Fatalf("expected scaffold key to be swallowed while busy, handled=%v cmd=%v", handled, cmd)
	}
	current.runningConfigOp = nil

	current.configEditor.draft = current.configEditor.baseline
	if cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'x', Text: "x"}); !handled || cmd == nil || current.banner == nil || current.banner.Message != "config draft is already clean" {
		t.Fatalf("expected reset-clean banner, handled=%v cmd=%v banner=%+v", handled, cmd, current.banner)
	}

	current.configEditor.draft.Connection.Host = "draft-host"
	if cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'u', Text: "u"}); !handled || cmd == nil || current.banner == nil || current.banner.Message != "applied derived defaults to the draft" {
		t.Fatalf("expected apply-defaults banner, handled=%v cmd=%v banner=%+v", handled, cmd, current.banner)
	}

	current.configEditor.draft.Stack.Managed = false
	if cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'g', Text: "g"}); !handled || cmd == nil || current.banner == nil || !strings.Contains(current.banner.Message, "enable a managed stack") {
		t.Fatalf("expected scaffold warning banner, handled=%v cmd=%v banner=%+v", handled, cmd, current.banner)
	}

	current.configEditor.draft = current.configEditor.baseline
	current.configEditor.draft.Connection.Host = "pending-save"
	current.configEditor.path = "/tmp/stackctl/config.yaml"
	if cmd, handled := current.handleConfigKey(tea.KeyPressMsg{Code: 'A', Text: "A"}); !handled || cmd == nil || current.runningConfigOp == nil {
		t.Fatalf("expected apply-config key to start config save flow, handled=%v cmd=%v op=%+v", handled, cmd, current.runningConfigOp)
	}
}
