package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rogpeppe/go-internal/diff"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

var (
	configEditorAnalyzeConfigImpact                 = analyzeConfigImpact
	configEditorCurrentPackageManagerRecommendation = system.CurrentPackageManagerRecommendation
	configEditorFormatPackageManagerRecommendation  = system.FormatPackageManagerRecommendation
	configEditorMarshal                             = configpkg.Marshal
	configEditorMarshalConfig                       = configMarshalConfig
	configEditorSpecificFieldEffect                 = specificFieldEffect
	configEditorEffectFollowUp                      = effectFollowUp
)

type ConfigSourceState string

const (
	ConfigSourceLoaded      ConfigSourceState = "loaded"
	ConfigSourceMissing     ConfigSourceState = "missing"
	ConfigSourceUnavailable ConfigSourceState = "unavailable"
)

type ConfigManager struct {
	DefaultConfig             func() configpkg.Config
	SaveConfig                func(string, configpkg.Config) error
	ValidateConfig            func(configpkg.Config) []configpkg.ValidationIssue
	MarshalConfig             func(configpkg.Config) ([]byte, error)
	ManagedStackNeedsScaffold func(configpkg.Config) (bool, error)
	ScaffoldManagedStack      func(configpkg.Config, bool) (configpkg.ScaffoldResult, error)
}

type configOperation struct {
	Message string
}

type configOperationMsg struct {
	Status  string
	Message string
	Err     error
	Reload  bool
}

type configApplyPlan struct {
	Allowed       bool
	Reason        string
	Save          bool
	Scaffold      bool
	ForceScaffold bool
	Restart       bool
	RunningStack  int
}

func (p configApplyPlan) pendingMessage() string {
	switch {
	case p.Restart:
		return "Applying config changes"
	case p.Scaffold:
		return "Saving and scaffolding config"
	case p.Save:
		return "Applying config changes"
	default:
		return "Checking config changes"
	}
}

type configFieldKind int

const (
	configFieldString configFieldKind = iota
	configFieldInt
	configFieldBool
)

type configFieldSpec struct {
	Key             string
	Group           string
	Label           string
	Description     string
	DescriptionFor  func(configpkg.Config) string
	SuggestionTitle string
	Kind            configFieldKind
	Secret          bool
	GetString       func(configpkg.Config) string
	SetString       func(*configpkg.Config, string) error
	GetBool         func(configpkg.Config) bool
	SetBool         func(*configpkg.Config, bool) error
	InputValidate   func(configpkg.Config, string) error
	EditableReason  func(configpkg.Config) string
	Suggestions     func(configpkg.Config) []string
}

type configListItemKind int

const (
	configListGroupRow configListItemKind = iota
	configListFieldRow
)

type configListItem struct {
	kind    configListItemKind
	group   string
	spec    configFieldSpec
	label   string
	value   string
	warning bool
}

func (i configListItem) FilterValue() string {
	if i.kind == configListGroupRow {
		return i.group
	}
	return i.spec.Key + " " + i.label + " " + i.group + " " + i.value
}

func (i configListItem) selectable() bool {
	return i.kind == configListFieldRow
}

type configListDelegate struct{}

func (configListDelegate) Height() int  { return 1 }
func (configListDelegate) Spacing() int { return 0 }
func (configListDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (configListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	row, ok := item.(configListItem)
	if !ok || m.Width() <= 0 {
		return
	}
	lineWidth := maxInt(8, m.Width())

	if row.kind == configListGroupRow {
		_, _ = fmt.Fprint(w, subsectionTitleStyle().Width(lineWidth).MaxWidth(lineWidth).Render(truncateEnd(row.group, lineWidth)))
		return
	}

	prefix := "  "
	if index == m.Index() {
		prefix = "▸ "
	}
	bodyWidth := maxInt(8, lineWidth-len(prefix))
	labelWidth := minInt(18, maxInt(10, bodyWidth/2))
	label := truncateEnd(row.label, labelWidth)
	valueWidth := maxInt(4, bodyWidth-labelWidth-2)
	value := truncateMiddle(row.value, valueWidth)
	if row.warning {
		label = "!" + truncateEnd(row.label, maxInt(3, labelWidth-1))
	}

	line := prefix + fmt.Sprintf("%-*s %s", labelWidth, label, value)
	style := lipgloss.NewStyle().Width(lineWidth).MaxWidth(lineWidth)
	if index == m.Index() {
		style = style.Foreground(activeTheme().listSelectedFg).Background(activeTheme().listSelectedBg).Bold(true)
	} else {
		style = style.Foreground(activeTheme().listForeground)
	}
	_, _ = fmt.Fprint(w, style.Render(line))
}

type configEditor struct {
	path            string
	source          ConfigSourceState
	sourceMessage   string
	baseline        configpkg.Config
	draft           configpkg.Config
	issues          []configpkg.ValidationIssue
	issueIndex      map[string][]configpkg.ValidationIssue
	needsScaffold   bool
	scaffoldProblem string
	width           int
	height          int
	fieldList       list.Model
	selectedKey     string
	input           textinput.Model
	editing         bool
	preview         bool
	runningStack    int
	totalStack      int
}

const configListChromeLines = 2

type configEditorLayout struct {
	statusStrip       string
	leftWidth         int
	rightWidth        int
	stacked           bool
	listContentHeight int
	detailContentHgt  int
	fieldListHeight   int
	workflowStrip     string
}

func newConfigEditor() configEditor {
	fieldList := list.New([]list.Item{}, configListDelegate{}, 0, 0)
	fieldList.DisableQuitKeybindings()
	fieldList.SetFilteringEnabled(false)
	fieldList.SetShowTitle(false)
	fieldList.SetShowStatusBar(false)
	fieldList.SetShowPagination(false)
	fieldList.SetShowHelp(false)
	fieldList.SetShowFilter(false)

	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 256

	return configEditor{
		source:     ConfigSourceMissing,
		fieldList:  fieldList,
		input:      input,
		issueIndex: make(map[string][]configpkg.ValidationIssue),
	}
}

func (e *configEditor) dirty() bool {
	return !configsEqual(e.baseline, e.draft)
}

func (e *configEditor) needsSave() bool {
	return e.dirty() || e.source != ConfigSourceLoaded
}

func (e *configEditor) syncFromSnapshot(snapshot Snapshot, manager *ConfigManager, showSecrets bool, force bool) {
	if manager == nil {
		return
	}
	if !force && e.dirty() {
		e.syncValidation(manager)
		e.refreshList(showSecrets)
		return
	}

	e.path = snapshot.ConfigPath
	e.source = snapshot.ConfigSource
	e.sourceMessage = strings.TrimSpace(snapshot.ConfigProblem)
	e.baseline = snapshot.ConfigData
	e.draft = snapshot.ConfigData
	e.needsScaffold = snapshot.ConfigNeedsScaffold
	e.scaffoldProblem = strings.TrimSpace(snapshot.ConfigScaffoldProblem)
	e.preview = false
	e.editing = false
	e.input.Blur()
	e.runningStack, e.totalStack = runningStackServiceCount(snapshot.Services)
	e.syncValidation(manager)
	e.refreshList(showSecrets)
}

func (e *configEditor) setSize(width, height int, showSecrets bool) {
	if width < 40 {
		width = 40
	}
	if height < 4 {
		height = 4
	}
	e.width = width
	e.height = height
	layout := e.layoutMetrics()
	listInnerWidth := maxInt(8, layout.leftWidth-subPaneStyle(lipgloss.Color("238")).GetHorizontalFrameSize())
	e.fieldList.SetSize(listInnerWidth, layout.fieldListHeight)
	e.refreshList(showSecrets)
}

func (e configEditor) layoutMetrics() configEditorLayout {
	width := maxInt(40, e.width)
	height := maxInt(4, e.height)

	leftWidth, rightWidth, stacked := splitPaneWidths(width, 42, 54)
	if stacked {
		leftWidth = maxInt(20, width-2)
		rightWidth = maxInt(20, width-2)
	}

	listStyle := subPaneStyle(lipgloss.Color("238")).Width(leftWidth)
	detailStyle := subPaneStyle(lipgloss.Color("31")).Width(rightWidth)
	minListTotal := listStyle.GetVerticalFrameSize() + configListChromeLines + 1
	minDetailTotal := detailStyle.GetVerticalFrameSize() + 4
	bodyHeight := height
	statusStrip := e.statusPanel(width)
	statusHeight := lipgloss.Height(statusStrip)
	minBodyHeight := maxInt(minListTotal, minDetailTotal)
	if stacked {
		minBodyHeight = minListTotal + minDetailTotal + 1
	}
	if height-statusHeight >= minBodyHeight {
		bodyHeight -= statusHeight
	} else {
		statusStrip = ""
	}

	workflowWidth := rightWidth
	if stacked {
		workflowWidth = width
	}
	workflowStrip := e.workflowPanel(workflowWidth)
	workflowHeight := lipgloss.Height(workflowStrip)
	topHeight := bodyHeight
	minTopHeight := minDetailTotal
	if stacked {
		minTopHeight += minListTotal
	}
	if bodyHeight-workflowHeight >= minTopHeight {
		topHeight -= workflowHeight
	} else {
		workflowStrip = mutedStyle().Render(e.workflowMiniStripContent(workflowWidth))
		workflowHeight = lipgloss.Height(workflowStrip)
		if bodyHeight-workflowHeight >= minTopHeight {
			topHeight -= workflowHeight
		} else {
			workflowStrip = ""
			workflowHeight = 0
		}
	}

	if stacked {
		usableHeight := topHeight
		listTotal := maxInt(minListTotal, (usableHeight-1)/2)
		detailTotal := maxInt(minDetailTotal, usableHeight-listTotal-1)
		if listTotal+detailTotal+1 > usableHeight {
			listTotal = maxInt(minListTotal, usableHeight-detailTotal-1)
		}

		listContentHeight := maxInt(configListChromeLines+1, listTotal-listStyle.GetVerticalFrameSize())
		detailContentHeight := maxInt(4, detailTotal-detailStyle.GetVerticalFrameSize())
		return configEditorLayout{
			statusStrip:       statusStrip,
			leftWidth:         leftWidth,
			rightWidth:        rightWidth,
			stacked:           true,
			listContentHeight: listContentHeight,
			detailContentHgt:  detailContentHeight,
			fieldListHeight:   maxInt(1, listContentHeight-configListChromeLines),
			workflowStrip:     workflowStrip,
		}
	}

	listContentHeight := maxInt(configListChromeLines+1, bodyHeight-listStyle.GetVerticalFrameSize())
	detailTotal := bodyHeight
	if workflowStrip != "" {
		detailTotal -= workflowHeight
	}
	detailContentHeight := maxInt(4, detailTotal-detailStyle.GetVerticalFrameSize())

	return configEditorLayout{
		statusStrip:       statusStrip,
		leftWidth:         leftWidth,
		rightWidth:        rightWidth,
		stacked:           false,
		listContentHeight: listContentHeight,
		detailContentHgt:  detailContentHeight,
		fieldListHeight:   maxInt(1, listContentHeight-configListChromeLines),
		workflowStrip:     workflowStrip,
	}
}

func (e *configEditor) refreshList(showSecrets bool) {
	items := make([]list.Item, 0, len(configFieldSpecs)+8)
	for _, group := range configFieldGroupOrder {
		specs := groupedConfigFieldSpecs(group)
		if len(specs) == 0 {
			continue
		}
		items = append(items, configListItem{kind: configListGroupRow, group: group})
		for _, spec := range specs {
			items = append(items, configListItem{
				kind:    configListFieldRow,
				group:   group,
				spec:    spec,
				label:   configFieldLabel(spec),
				value:   fieldItemDescription(spec, e.draft, showSecrets, e.issueIndex[spec.Key]),
				warning: len(e.issueIndex[spec.Key]) > 0,
			})
		}
	}
	e.fieldList.SetItems(items)
	index := e.selectedFieldIndex(items)
	e.fieldList.Select(index)
	e.syncSelectedKey()
}

func (e *configEditor) syncSelectedKey() {
	item, ok := e.fieldList.SelectedItem().(configListItem)
	if !ok || !item.selectable() {
		e.selectedKey = ""
		return
	}
	e.selectedKey = item.spec.Key
}

func (e configEditor) selectedFieldIndex(items []list.Item) int {
	if e.selectedKey != "" {
		for index, item := range items {
			row, ok := item.(configListItem)
			if ok && row.selectable() && row.spec.Key == e.selectedKey {
				return index
			}
		}
	}

	for index, item := range items {
		row, ok := item.(configListItem)
		if ok && row.selectable() {
			return index
		}
	}

	return 0
}

func (e *configEditor) moveSelection(step int) {
	items := e.fieldList.Items()
	if len(items) == 0 || step == 0 {
		return
	}

	current := e.fieldList.Index()
	for attempts := 0; attempts < len(items); attempts++ {
		next := current + step
		if next < 0 || next >= len(items) {
			break
		}
		current = next
		e.fieldList.Select(current)
		row, ok := items[current].(configListItem)
		if ok && row.selectable() {
			break
		}
	}

	e.syncSelectedKey()
}

func (e *configEditor) syncValidation(manager *ConfigManager) {
	if manager == nil {
		e.issues = nil
		e.issueIndex = make(map[string][]configpkg.ValidationIssue)
		e.needsScaffold = false
		e.scaffoldProblem = ""
		return
	}

	e.issues = manager.ValidateConfig(e.draft)
	e.issueIndex = make(map[string][]configpkg.ValidationIssue, len(e.issues))
	for _, issue := range e.issues {
		e.issueIndex[issue.Field] = append(e.issueIndex[issue.Field], issue)
	}

	if e.draft.Stack.Managed && e.draft.Setup.ScaffoldDefaultStack {
		needsScaffold, err := manager.ManagedStackNeedsScaffold(e.draft)
		e.needsScaffold = needsScaffold
		if err != nil {
			e.scaffoldProblem = err.Error()
		} else {
			e.scaffoldProblem = ""
		}
		return
	}

	e.needsScaffold = false
	e.scaffoldProblem = ""
}

func (e configEditor) selectedSpec() (configFieldSpec, bool) {
	for _, spec := range configFieldSpecs {
		if spec.Key == e.selectedKey {
			return spec, true
		}
	}

	return configFieldSpec{}, false
}

func (e *configEditor) beginEdit(showSecrets bool) tea.Cmd {
	spec, ok := e.selectedSpec()
	if !ok {
		return nil
	}
	if reason := selectedFieldEditBlock(spec, e.draft); reason != "" {
		return nil
	}

	switch spec.Kind {
	case configFieldBool:
		current := false
		if spec.GetBool != nil {
			current = spec.GetBool(e.draft)
		}
		if spec.SetBool != nil {
			_ = spec.SetBool(&e.draft, !current)
		}
		e.preview = false
		return nil
	case configFieldString, configFieldInt:
		e.editing = true
		e.preview = false
		e.input.Reset()
		e.input.Err = nil
		e.input.Validate = func(value string) error {
			if spec.InputValidate == nil {
				return nil
			}
			return spec.InputValidate(e.draft, value)
		}
		suggestions := selectedFieldSuggestions(spec, e.draft)
		e.input.SetSuggestions(suggestions)
		e.input.ShowSuggestions = len(suggestions) > 0
		e.input.Placeholder = spec.Label
		e.input.EchoMode = textinput.EchoNormal
		if spec.Secret && !showSecrets {
			e.input.EchoMode = textinput.EchoPassword
			e.input.EchoCharacter = '•'
		}
		e.input.SetValue(selectedFieldValue(spec, e.draft))
		e.input.SetWidth(maxInt(12, e.detailWidth()-6))
		return e.input.Focus()
	default:
		return nil
	}
}

func (e *configEditor) commitEdit() error {
	spec, ok := e.selectedSpec()
	if !ok || !e.editing {
		return nil
	}
	value := e.input.Value()
	if spec.InputValidate != nil {
		if err := spec.InputValidate(e.draft, value); err != nil {
			e.input.Err = err
			return err
		}
	}
	if spec.SetString != nil {
		if err := spec.SetString(&e.draft, value); err != nil {
			e.input.Err = err
			return err
		}
	}
	e.input.Err = nil
	e.input.Blur()
	e.editing = false
	return nil
}

func (e *configEditor) cancelEdit() {
	e.input.Blur()
	e.input.Err = nil
	e.editing = false
}

func (e *configEditor) applyDerivedDefaults(manager *ConfigManager, showSecrets bool) {
	e.draft.ApplyDerivedFields()
	if e.draft.Stack.Managed {
		if managedDir, err := configpkg.ManagedStackDir(e.draft.Stack.Name); err == nil {
			e.draft.Stack.Dir = managedDir
			e.draft.Stack.ComposeFile = configpkg.DefaultComposeFileName
		}
	}
	e.preview = false
	e.syncValidation(manager)
	e.refreshList(showSecrets)
}

func (e *configEditor) resetDraft(manager *ConfigManager, showSecrets bool) {
	e.draft = e.baseline
	e.preview = false
	e.cancelEdit()
	e.syncValidation(manager)
	e.refreshList(showSecrets)
}

func (e *configEditor) togglePreview(showSecrets bool) {
	e.preview = !e.preview
	e.refreshList(showSecrets)
}

func (e *configEditor) canScaffold() (bool, string) {
	if !e.draft.Stack.Managed {
		return false, "enable a managed stack before scaffolding"
	}
	return true, ""
}

func (e configEditor) showScaffoldAction() bool {
	ok, _ := e.canScaffold()
	return ok && (e.needsSave() || e.needsScaffold)
}

func (e configEditor) applyPlan() configApplyPlan {
	plan := configApplyPlan{RunningStack: e.runningStack}

	if len(e.issues) > 0 {
		plan.Reason = "fix validation issues before applying"
		return plan
	}
	if strings.TrimSpace(e.scaffoldProblem) != "" {
		plan.Reason = "resolve the managed scaffold problem before applying"
		return plan
	}

	impact := configEditorAnalyzeConfigImpact(e.baseline, e.draft)
	saveNeeded := e.needsSave()

	if impact.stackTarget {
		plan.Reason = "stack target changes are save-only"
		return plan
	}
	if impact.manualFollowUp {
		plan.Reason = "save first, then handle this stack change manually"
		return plan
	}

	plan.Save = saveNeeded
	if e.needsScaffold {
		plan.Scaffold = true
		plan.ForceScaffold = true
	}

	if impact.composeTemplate && e.draft.Stack.Managed {
		if !e.draft.Setup.ScaffoldDefaultStack {
			plan.Reason = "managed compose changes require manual compose updates"
			return configApplyPlan{Reason: plan.Reason}
		}
		plan.Scaffold = true
		if saveNeeded {
			plan.ForceScaffold = true
		}
	}

	if plan.Scaffold && e.runningStack > 0 {
		plan.Restart = true
	}

	if plan.Save && !plan.Scaffold && !plan.Restart {
		plan.Reason = "use ctrl+s to save config-only changes"
		return plan
	}
	if !plan.Save && !plan.Scaffold && !plan.Restart {
		plan.Reason = "nothing new needs to be applied"
		return plan
	}

	plan.Allowed = true
	return plan
}

func (e *configEditor) handleKey(msg tea.KeyPressMsg, keys keyMap, manager *ConfigManager, showSecrets bool) (tea.Cmd, bool) {
	if e.editing {
		switch {
		case key.Matches(msg, keys.EditField):
			if err := e.commitEdit(); err != nil {
				return nil, true
			}
			e.syncValidation(manager)
			e.refreshList(showSecrets)
			return nil, true
		case key.Matches(msg, keys.CancelEdit):
			e.cancelEdit()
			e.refreshList(showSecrets)
			return nil, true
		default:
			var cmd tea.Cmd
			e.input, cmd = e.input.Update(msg)
			return cmd, true
		}
	}

	if key.Matches(msg, keys.EditField) || msg.Text == " " {
		cmd := e.beginEdit(showSecrets)
		e.syncValidation(manager)
		e.refreshList(showSecrets)
		return cmd, true
	}
	if key.Matches(msg, keys.PrevItem) {
		e.moveSelection(-1)
		return nil, true
	}
	if key.Matches(msg, keys.NextItem) {
		e.moveSelection(1)
		return nil, true
	}

	return nil, false
}

func (e configEditor) summaryStatus() string {
	switch e.source {
	case ConfigSourceLoaded:
		if e.sourceMessage != "" {
			return e.sourceMessage
		}
		return "Loaded from disk."
	case ConfigSourceMissing:
		if e.sourceMessage != "" {
			return e.sourceMessage
		}
		return "No config file exists yet. Review the defaults and save when ready."
	default:
		if e.sourceMessage != "" {
			return e.sourceMessage
		}
		return "The current config could not be loaded. Saving will replace it."
	}
}

func (e configEditor) View(showSecrets bool) string {
	if e.width == 0 || e.height == 0 {
		return mutedStyle().Render("Config editor is loading…")
	}

	layout := e.layoutMetrics()
	leftWidth := layout.leftWidth
	rightWidth := layout.rightWidth

	listPane := subPaneStyle(lipgloss.Color("238")).Width(leftWidth)
	detailPane := subPaneStyle(lipgloss.Color("31")).Width(rightWidth)

	listContent := clipAndPadText(strings.Join([]string{
		detailHeading("Config fields"),
		mutedStyle().Render(e.listSummary()),
		e.renderFieldList(layout.fieldListHeight, layout.leftWidth-subPaneStyle(lipgloss.Color("238")).GetHorizontalFrameSize()),
	}, "\n"), layout.listContentHeight)

	detailContent := clipAndPadText(e.renderDetail(showSecrets, layout.detailContentHgt), layout.detailContentHgt)

	if layout.stacked {
		listRendered := listPane.Height(layout.listContentHeight).Render(listContent)
		detailRendered := detailPane.Height(layout.detailContentHgt).Render(detailContent)
		blocks := make([]string, 0, 4)
		if layout.statusStrip != "" {
			blocks = append(blocks, layout.statusStrip)
		}
		blocks = append(blocks, listRendered, detailRendered)
		if layout.workflowStrip == "" {
			return lipgloss.JoinVertical(lipgloss.Left, blocks...)
		}
		blocks = append(blocks, layout.workflowStrip)
		return lipgloss.JoinVertical(lipgloss.Left, blocks...)
	}

	rightColumn := lipgloss.JoinVertical(
		lipgloss.Left,
		detailPane.Height(layout.detailContentHgt).Render(detailContent),
	)
	if layout.workflowStrip != "" {
		rightColumn = lipgloss.JoinVertical(lipgloss.Left, rightColumn, layout.workflowStrip)
	}
	leftPaneRendered := listPane.Height(layout.listContentHeight).Render(listContent)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPaneRendered, rightColumn)
	if layout.statusStrip == "" {
		return body
	}
	return lipgloss.JoinVertical(lipgloss.Left, layout.statusStrip, body)
}

func (e configEditor) renderDetail(showSecrets bool, availableHeight int) string {
	if e.preview {
		return e.renderPreview(showSecrets)
	}

	spec, ok := e.selectedSpec()
	if !ok {
		return strings.Join([]string{
			detailHeading("Config detail"),
			"",
			mutedStyle().Render("No config field is selected."),
		}, "\n")
	}

	lines := []string{
		detailHeading("Config detail"),
		e.renderConfigFieldHeading(spec),
	}
	appendSection := func(title string, body []string) {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, detailHeading(title))
		lines = append(lines, body...)
	}
	compact := availableHeight <= 9
	if !compact {
		lines = append(lines, mutedStyle().Render(spec.Key))
	}

	valueRows := [][2]string{
		{"Draft", displayFieldValue(spec, e.draft, showSecrets)},
	}
	if e.source == ConfigSourceLoaded {
		valueRows = append(valueRows, [2]string{"Saved", displayFieldValue(spec, e.baseline, showSecrets)})
	} else {
		valueRows = append(valueRows, [2]string{"Saved", "not written to disk yet"})
	}
	appendSection("Values", e.detailRows(false, valueRows...))

	summaryRows := make([][2]string, 0, 4)
	if !compact {
		summaryRows = append(summaryRows, [2]string{"Purpose", fieldDescription(spec, e.draft)})
	}
	summaryRows = append(summaryRows, [2]string{"Effect", selectedFieldEffect(spec, e.draft)})

	if reason := selectedFieldEditBlock(spec, e.draft); reason != "" {
		summaryRows = append(summaryRows, [2]string{"Editing", reason})
	}
	appendSection("Field", e.detailRows(true, summaryRows...))
	if suggestionRows := e.suggestionRows(spec); len(suggestionRows) > 0 {
		appendSection(spec.suggestionHeading(), e.detailRows(true, suggestionRows...))
	}

	if len(e.issueIndex[spec.Key]) > 0 {
		lines = append(lines, "", detailHeading("Validation"))
		for _, issue := range e.issueIndex[spec.Key] {
			lines = append(lines, errorBannerStyle().Render(fmt.Sprintf("%s: %s", issue.Field, issue.Message)))
		}
	}

	if e.editing {
		lines = append(lines, "", detailHeading("Edit field"), e.input.View())
		if e.input.Err != nil {
			lines = append(lines, errorBannerStyle().Render(e.input.Err.Error()))
		}
		if hints := e.editSuggestionHints(spec); len(hints) > 0 {
			for _, hint := range hints {
				lines = append(lines, mutedStyle().Render(hint))
			}
		}
		lines = append(lines, mutedStyle().Render("Enter keeps the draft. Esc cancels. Ctrl+S saves."))
	}

	return strings.Join(lines, "\n")
}

func fieldDescription(spec configFieldSpec, cfg configpkg.Config) string {
	if spec.DescriptionFor != nil {
		if description := strings.TrimSpace(spec.DescriptionFor(cfg)); description != "" {
			return description
		}
	}
	return spec.Description
}

func (e configEditor) renderFieldList(visibleRows int, width int) string {
	items := e.fieldList.Items()
	if len(items) == 0 || visibleRows <= 0 {
		return ""
	}

	selected := e.fieldList.Index()
	start := selected - visibleRows/2
	if start < 0 {
		start = 0
	}
	maxStart := maxInt(0, len(items)-visibleRows)
	if start > maxStart {
		start = maxStart
	}
	end := minInt(len(items), start+visibleRows)

	lines := make([]string, 0, visibleRows)
	for index := start; index < end; index++ {
		row, ok := items[index].(configListItem)
		if !ok {
			continue
		}
		lines = append(lines, renderConfigListRow(row, index == selected, width))
	}

	for len(lines) < visibleRows {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func clipAndPadText(content string, height int) string {
	if height <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderConfigListRow(row configListItem, selected bool, width int) string {
	lineWidth := maxInt(8, width)
	if row.kind == configListGroupRow {
		return subsectionTitleStyle().Width(lineWidth).MaxWidth(lineWidth).Render(truncateEnd(row.group, lineWidth))
	}

	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	bodyWidth := maxInt(8, lineWidth-len(prefix))
	labelWidth := minInt(18, maxInt(10, bodyWidth/2))
	label := truncateEnd(row.label, labelWidth)
	valueWidth := maxInt(4, bodyWidth-labelWidth-2)
	value := truncateMiddle(row.value, valueWidth)
	if row.warning {
		label = "!" + truncateEnd(row.label, maxInt(3, labelWidth-1))
	}

	line := prefix + fmt.Sprintf("%-*s %s", labelWidth, label, value)
	style := lipgloss.NewStyle().Width(lineWidth).MaxWidth(lineWidth)
	if selected {
		style = style.Foreground(activeTheme().listSelectedFg).Background(activeTheme().listSelectedBg).Bold(true)
	} else {
		style = style.Foreground(activeTheme().listForeground)
	}

	return style.Render(line)
}

func (e configEditor) renderPreview(showSecrets bool) string {
	lines := []string{
		detailHeading("Config diff"),
		"",
	}
	diffText, err := e.diffText(showSecrets)
	if err != nil {
		lines = append(lines, errorBannerStyle().Render(err.Error()))
		return strings.Join(lines, "\n")
	}
	if strings.TrimSpace(diffText) == "" {
		lines = append(lines, mutedStyle().Render("No unsaved config changes to preview."))
		lines = append(lines, "")
		lines = append(lines, mutedStyle().Render(e.summaryStatus()))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, diffText)
	return strings.Join(lines, "\n")
}

func (e configEditor) diffText(showSecrets bool) (string, error) {
	oldConfig := e.baseline
	if e.source != ConfigSourceLoaded {
		oldConfig = configpkg.Config{}
	}
	if !showSecrets {
		oldConfig = redactConfigSecrets(oldConfig)
	}
	oldData := []byte{}
	var err error
	if e.source == ConfigSourceLoaded {
		oldData, err = configEditorMarshalConfig(oldConfig)
		if err != nil {
			return "", err
		}
	}

	newConfig := e.draft
	if !showSecrets {
		newConfig = redactConfigSecrets(newConfig)
	}
	newData, err := configEditorMarshalConfig(newConfig)
	if err != nil {
		return "", err
	}

	return string(diff.Diff("saved", oldData, "draft", newData)), nil
}

func (e configEditor) detailWidth() int {
	frameWidth := subPaneStyle(lipgloss.Color("31")).GetHorizontalFrameSize()
	_, rightWidth, stacked := splitPaneWidths(e.width, 42, 54)
	if stacked {
		return maxInt(20, e.width-frameWidth)
	}
	return maxInt(20, rightWidth-frameWidth)
}

func (e configEditor) workflowPanel(width int) string {
	color := lipgloss.Color("240")
	innerWidth := maxInt(24, width-subPaneStyle(color).GetHorizontalFrameSize())
	line := truncateEnd(fmt.Sprintf("Keys    %s", e.workflowKeysSummary()), innerWidth)
	return subPaneStyle(color).Width(width).Render(line)
}

func (e configEditor) workflowMiniStripContent(width int) string {
	return wrapText("Keys: j/k move • ctrl+s save/apply • e edit • ? more", maxInt(24, width-2))
}

func (e configEditor) statusPanel(width int) string {
	color := lipgloss.Color(e.statusPanelColor())
	innerWidth := maxInt(24, width-subPaneStyle(color).GetHorizontalFrameSize())
	line := truncateEnd(fmt.Sprintf("Status  %s", e.workflowStatusLine()), innerWidth)
	return subPaneStyle(color).Width(width).Render(line)
}

func (e configEditor) statusPanelColor() string {
	switch {
	case e.editing && e.input.Err != nil:
		return "160"
	case len(e.issues) > 0:
		return "160"
	case e.dirty():
		return "220"
	case e.source != ConfigSourceLoaded:
		return "81"
	default:
		return "78"
	}
}

func (e configEditor) workflowStatusLine() string {
	plan := e.savePlan()
	prefix := "Saved"
	switch {
	case e.source == ConfigSourceMissing:
		prefix = "Draft not saved yet"
	case e.source != ConfigSourceLoaded:
		prefix = "Draft replaces unreadable config"
	case e.dirty():
		prefix = "Unsaved draft"
	}

	return prefix + " • " + e.compactNextStepSummary(plan)
}

func (e configEditor) compactNextStepSummary(plan configApplyPlan) string {
	switch {
	case e.editing && e.input.Err != nil:
		return "ctrl+s is blocked until this field is fixed"
	case e.editing:
		return "press Enter to keep this edit in the draft"
	case len(e.issues) > 0:
		return fmt.Sprintf("fix %d validation issue(s) before ctrl+s", len(e.issues))
	case plan.Allowed && plan.Restart:
		return "ctrl+s saves, refreshes compose, and restarts running services"
	case plan.Allowed && plan.Scaffold:
		return "ctrl+s saves and refreshes compose for the next start"
	case plan.Reason == "use ctrl+s to save config-only changes":
		if !e.draft.Stack.Managed {
			return "ctrl+s writes config only; external compose stays unchanged"
		}
		return "ctrl+s writes config only"
	case plan.Reason == "nothing new needs to be applied":
		if !e.draft.Stack.Managed {
			return "ctrl+s would only update stackctl metadata here"
		}
		return "ctrl+s would not apply anything new right now"
	case plan.Reason == "stack target changes are save-only":
		return "ctrl+s saves only; stack target changes do not restart the current stack"
	case plan.Reason == "save first, then handle this stack change manually":
		return "ctrl+s saves now, then you finish the stack change manually"
	case plan.Reason == "managed compose changes require manual compose updates":
		return "ctrl+s saves only; update compose manually after"
	case plan.Reason == "resolve the managed scaffold problem before applying":
		return "fix the scaffold problem before ctrl+s"
	default:
		return "ctrl+s saves this draft"
	}
}

func (e configEditor) workflowKeysSummary() string {
	if e.editing {
		return "enter keep • esc cancel • ctrl+s save/apply • ? more"
	}

	keys := []string{"j/k move", "e edit", "ctrl+s save/apply"}
	if e.needsSave() {
		keys = append(keys, "x reset")
	}
	keys = append(keys, "p diff", "? more")
	return strings.Join(keys, " • ")
}

func (e configEditor) detailRows(muted bool, rows ...[2]string) []string {
	return renderRowsForWidth(e.detailWidth(), muted, rows...)
}

func renderRowsForWidth(width int, muted bool, rows ...[2]string) []string {
	maxLabelWidth := 0
	for _, row := range rows {
		if len(row[0]) > maxLabelWidth {
			maxLabelWidth = len(row[0])
		}
	}

	valueWidth := maxInt(16, width-maxLabelWidth-4)
	lines := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		value := strings.TrimSpace(row[1])
		if value == "" {
			value = "(empty)"
		}

		for index, line := range strings.Split(wrapText(value, valueWidth), "\n") {
			label := ""
			if index == 0 {
				label = row[0]
			}
			rendered := fmt.Sprintf("%-*s  %s", maxLabelWidth, label, line)
			if muted {
				rendered = mutedStyle().Render(rendered)
			}
			lines = append(lines, rendered)
		}
	}

	return lines
}

func (e configEditor) suggestionRows(spec configFieldSpec) [][2]string {
	values := selectedFieldSuggestions(spec, e.draft)
	if len(values) == 0 {
		return nil
	}
	return [][2]string{{"Values", strings.Join(values, ", ")}}
}

func (e configEditor) editSuggestionHints(spec configFieldSpec) []string {
	values := selectedFieldSuggestions(spec, e.draft)
	if len(values) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf("%s: %s", spec.suggestionHeading(), strings.Join(values, ", ")),
	}
}

func saveConfigCmd(manager *ConfigManager, path string, previous configpkg.Config, cfg configpkg.Config, runningStack int) tea.Cmd {
	return func() tea.Msg {
		if err := manager.SaveConfig(path, cfg); err != nil {
			return configOperationMsg{
				Status:  output.StatusFail,
				Message: fmt.Sprintf("save config failed: %v", err),
				Err:     err,
			}
		}
		issues := manager.ValidateConfig(cfg)
		status := output.StatusOK
		message := fmt.Sprintf("saved config to %s", path)
		if len(issues) > 0 {
			status = output.StatusWarn
			message = fmt.Sprintf("saved config to %s with %d validation issue(s)", path, len(issues))
		}
		if followUp := saveFollowUpMessage(previous, cfg, runningStack); followUp != "" {
			message += "  •  " + followUp
		}
		return configOperationMsg{
			Status:  status,
			Message: message,
			Reload:  true,
		}
	}
}

func scaffoldConfigCmd(manager *ConfigManager, path string, cfg configpkg.Config, force bool, runningStack int) tea.Cmd {
	return func() tea.Msg {
		if err := manager.SaveConfig(path, cfg); err != nil {
			return configOperationMsg{
				Status:  output.StatusFail,
				Message: fmt.Sprintf("save config failed: %v", err),
				Err:     err,
			}
		}
		result, err := manager.ScaffoldManagedStack(cfg, force)
		if err != nil {
			return configOperationMsg{
				Status:  output.StatusFail,
				Message: fmt.Sprintf("scaffold failed: %v", err),
				Err:     err,
			}
		}
		message := scaffoldResultMessage(result)
		status := output.StatusOK
		if strings.TrimSpace(message) == "" {
			message = "managed stack scaffold is up to date"
		}
		if runningStack > 0 {
			message += "  •  restart the stack to apply updated compose changes"
		}
		return configOperationMsg{
			Status:  status,
			Message: message,
			Reload:  true,
		}
	}
}

func applyConfigCmd(manager *ConfigManager, runner ActionRunner, path string, previous configpkg.Config, cfg configpkg.Config, plan configApplyPlan) tea.Cmd {
	return func() tea.Msg {
		issues := manager.ValidateConfig(cfg)
		if len(issues) > 0 {
			return configOperationMsg{
				Status:  output.StatusWarn,
				Message: fmt.Sprintf("apply blocked by %d validation issue(s)", len(issues)),
			}
		}

		steps := make([]string, 0, 4)
		reload := false

		if plan.Save {
			if err := manager.SaveConfig(path, cfg); err != nil {
				return configOperationMsg{
					Status:  output.StatusFail,
					Message: fmt.Sprintf("save config failed: %v", err),
					Err:     err,
				}
			}
			reload = true
			steps = append(steps, fmt.Sprintf("saved config to %s", path))
		}

		if plan.Scaffold {
			result, err := manager.ScaffoldManagedStack(cfg, plan.ForceScaffold)
			if err != nil {
				return configOperationMsg{
					Status:  output.StatusFail,
					Message: joinOperationSteps(append(append([]string(nil), steps...), fmt.Sprintf("scaffold failed: %v", err))...),
					Reload:  reload,
				}
			}
			reload = true
			message := scaffoldResultMessage(result)
			if strings.TrimSpace(message) == "" {
				message = "managed stack scaffold is up to date"
			}
			steps = append(steps, message)
		}

		if plan.Restart {
			if runner == nil {
				return configOperationMsg{
					Status:  output.StatusFail,
					Message: joinOperationSteps(append(append([]string(nil), steps...), "restart is unavailable in this model")...),
					Reload:  reload,
				}
			}
			report, err := runner(ActionRestart)
			if err != nil {
				return configOperationMsg{
					Status:  output.StatusFail,
					Message: joinOperationSteps(append(append([]string(nil), steps...), fmt.Sprintf("restart failed: %v", err))...),
					Reload:  true,
				}
			}
			status := report.Status
			if strings.TrimSpace(status) == "" {
				status = output.StatusOK
			}
			message := strings.TrimSpace(report.Message)
			if message == "" {
				message = "stack restarted"
			}
			steps = append(steps, message)
			return configOperationMsg{
				Status:  status,
				Message: joinOperationSteps(steps...),
				Reload:  true,
			}
		}

		if len(steps) == 0 {
			return configOperationMsg{
				Status:  output.StatusInfo,
				Message: "nothing new needed to be applied",
			}
		}

		if followUp := applyFollowUpMessage(previous, cfg, plan); followUp != "" {
			steps = append(steps, followUp)
		}

		return configOperationMsg{
			Status:  output.StatusOK,
			Message: joinOperationSteps(steps...),
			Reload:  reload,
		}
	}
}

func scaffoldResultMessage(result configpkg.ScaffoldResult) string {
	parts := make([]string, 0, 6)
	if result.CreatedDir {
		parts = append(parts, fmt.Sprintf("created managed stack directory %s", result.StackDir))
	}
	if result.WroteCompose {
		parts = append(parts, fmt.Sprintf("wrote managed compose file %s", result.ComposePath))
	}
	if result.WroteNATSConfig {
		parts = append(parts, fmt.Sprintf("wrote managed nats config file %s", result.NATSConfigPath))
	}
	if result.WroteRedisACL {
		parts = append(parts, fmt.Sprintf("wrote managed redis ACL file %s", result.RedisACLPath))
	}
	if result.WrotePgAdminServers {
		parts = append(parts, fmt.Sprintf("wrote managed pgAdmin server bootstrap file %s", result.PgAdminServersPath))
	}
	if result.WrotePGPass {
		parts = append(parts, fmt.Sprintf("wrote managed pgpass file %s", result.PGPassPath))
	}
	if len(parts) == 0 && result.AlreadyPresent {
		parts = append(parts, fmt.Sprintf("managed stack already exists at %s", result.ComposePath))
	}
	return strings.Join(parts, "  •  ")
}

func joinOperationSteps(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "  •  ")
}

func applyFollowUpMessage(previous configpkg.Config, next configpkg.Config, plan configApplyPlan) string {
	switch {
	case plan.Restart:
		return ""
	case plan.Scaffold && plan.RunningStack > 0:
		return "restart the stack to load the refreshed compose file"
	case plan.Scaffold && plan.RunningStack == 0:
		return "ready for the next stack start"
	case plan.Save:
		return saveFollowUpMessage(previous, next, plan.RunningStack)
	default:
		return ""
	}
}

func saveFollowUpMessage(previous configpkg.Config, next configpkg.Config, runningStack int) string {
	impact := configEditorAnalyzeConfigImpact(previous, next)
	if !impact.changed {
		return "running services are unchanged"
	}

	switch {
	case impact.stackTarget:
		return "future stackctl commands use the new stack target"
	case impact.composeTemplate && next.Stack.Managed && next.Setup.ScaffoldDefaultStack:
		if runningStack > 0 {
			return "save also needs a compose refresh and restart to update running services"
		}
		return "save also needs a compose refresh before the next start"
	case impact.composeTemplate && next.Stack.Managed:
		return "update the managed compose file yourself before restart"
	case impact.composeTemplate:
		return "external compose files were not changed"
	case impact.localOnly:
		return "running services were not changed"
	default:
		return "review the config impact before the next restart"
	}
}

type configImpact struct {
	changed         bool
	localOnly       bool
	stackTarget     bool
	composeTemplate bool
	manualFollowUp  bool
}

func analyzeConfigImpact(previous configpkg.Config, next configpkg.Config) configImpact {
	impact := configImpact{}
	for _, spec := range configFieldSpecs {
		if !configFieldChanged(spec, previous, next) {
			continue
		}
		impact.changed = true
		classifyConfigImpact(&impact, spec.Key, previous, next)
	}

	return impact
}

func configFieldChanged(spec configFieldSpec, previous configpkg.Config, next configpkg.Config) bool {
	switch spec.Kind {
	case configFieldBool:
		if spec.GetBool == nil {
			return false
		}
		return spec.GetBool(previous) != spec.GetBool(next)
	default:
		return selectedFieldValue(spec, previous) != selectedFieldValue(spec, next)
	}
}

func classifyConfigImpact(impact *configImpact, key string, previous configpkg.Config, next configpkg.Config) {
	switch {
	case key == "connection.host", key == "services.postgres.maintenance_database", key == "setup.include_cockpit", key == "setup.install_cockpit", strings.HasPrefix(key, "behavior."), strings.HasPrefix(key, "tui."), strings.HasPrefix(key, "system."):
		impact.localOnly = true
	case key == "stack.name":
		if previous.Stack.Managed || next.Stack.Managed {
			impact.stackTarget = true
			impact.manualFollowUp = true
			return
		}
		impact.localOnly = true
	case key == "stack.managed", key == "stack.dir", key == "stack.compose_file":
		impact.stackTarget = true
		impact.manualFollowUp = true
	case key == "setup.scaffold_default_stack":
		if previous.Stack.Managed || next.Stack.Managed {
			impact.composeTemplate = true
			impact.manualFollowUp = true
			return
		}
		impact.localOnly = true
	default:
		if next.Stack.Managed {
			impact.composeTemplate = true
			if !next.Setup.ScaffoldDefaultStack {
				impact.manualFollowUp = true
			}
			return
		}
		impact.composeTemplate = true
		impact.localOnly = true
	}
}

func selectedFieldEffect(spec configFieldSpec, cfg configpkg.Config) string {
	base := configEditorSpecificFieldEffect(spec, cfg)
	followUp := configEditorEffectFollowUp(spec, cfg)
	switch {
	case base == "":
		return followUp
	case followUp == "":
		return base
	default:
		return base + " " + followUp
	}
}

func specificFieldEffect(spec configFieldSpec, cfg configpkg.Config) string {
	switch spec.Key {
	case "stack.name":
		return "Renames the stack target and, in managed mode, changes the derived stack directory name."
	case "stack.managed":
		return "Switches between a stackctl-managed compose stack and an external compose stack."
	case "stack.dir":
		return "Changes the stack directory stackctl targets."
	case "stack.compose_file":
		return "Changes which compose file stackctl targets inside the stack directory."
	case "setup.scaffold_default_stack":
		return "Controls whether stackctl keeps the managed compose file in sync with the embedded template."
	case "connection.host":
		return "Changes the host name stackctl uses when it builds URLs and DSNs."
	case "setup.include_postgres":
		return "Adds or removes Postgres from stackctl-managed service handling."
	case "services.postgres_container":
		return "Changes the Postgres service and container name stackctl targets."
	case "services.postgres.image":
		return "Changes the Postgres image tag used by the managed stack template."
	case "services.postgres.data_volume":
		return "Changes the Postgres volume name used for database storage."
	case "services.postgres.maintenance_database":
		return "Changes the maintenance database stackctl uses for database commands."
	case "services.postgres.max_connections":
		return "Changes the Postgres max_connections setting in the managed stack template."
	case "services.postgres.shared_buffers":
		return "Changes the Postgres shared_buffers setting in the managed stack template."
	case "services.postgres.log_min_duration_statement_ms":
		return "Changes the Postgres slow-query duration logging threshold in the managed stack template."
	case "services.redis_container":
		return "Changes the Redis service and container name stackctl targets."
	case "services.redis.image":
		return "Changes the Redis image tag used by the managed stack template."
	case "services.redis.data_volume":
		return "Changes the Redis volume name used for persistence."
	case "services.redis.appendonly":
		return "Turns Redis appendonly persistence on or off."
	case "services.redis.save_policy":
		return "Changes the Redis snapshot save policy passed to the container."
	case "services.redis.maxmemory_policy":
		return "Changes the Redis eviction policy passed to the container."
	case "setup.include_redis":
		return "Adds or removes Redis from stackctl-managed service handling."
	case "setup.include_nats":
		return "Adds or removes NATS from the managed stack."
	case "services.nats_container":
		return "Changes the NATS service and container name stackctl targets."
	case "services.nats.image":
		return "Changes the NATS image tag used by the managed stack template."
	case "setup.include_meilisearch":
		return "Adds or removes Meilisearch from the managed stack."
	case "services.meilisearch_container":
		return "Changes the Meilisearch service and container name stackctl targets."
	case "services.meilisearch.image":
		return "Changes the Meilisearch image tag used by the managed stack template."
	case "services.meilisearch.data_volume":
		return "Changes the Meilisearch volume name used for index storage."
	case "setup.include_pgadmin":
		return "Adds or removes pgAdmin from the managed stack."
	case "services.pgadmin_container":
		return "Changes the pgAdmin service and container name stackctl targets."
	case "services.pgadmin.image":
		return "Changes the pgAdmin image tag used by the managed stack template."
	case "services.pgadmin.data_volume":
		return "Changes the pgAdmin volume name used for state and storage."
	case "services.pgadmin.server_mode":
		return "Turns pgAdmin server mode on or off."
	case "services.pgadmin.bootstrap_postgres_server":
		return "Controls whether stackctl bootstraps pgAdmin with the managed Postgres server."
	case "services.pgadmin.bootstrap_server_name":
		return "Changes the saved pgAdmin connection name used for Postgres bootstrap."
	case "services.pgadmin.bootstrap_server_group":
		return "Changes the pgAdmin server group used for Postgres bootstrap."
	case "ports.postgres":
		return "Changes the host port published for Postgres."
	case "ports.redis":
		return "Changes the host port published for Redis."
	case "ports.nats":
		return "Changes the host port published for NATS."
	case "ports.meilisearch":
		return "Changes the host port published for Meilisearch."
	case "ports.pgadmin":
		return "Changes the host port published for pgAdmin."
	case "ports.cockpit":
		return "Changes the host port stackctl uses when it opens Cockpit."
	case "setup.include_cockpit":
		return "Adds or removes Cockpit from stackctl helper output and dashboard actions."
	case "connection.postgres_database":
		return "Changes the default Postgres database in helpers and the managed stack bootstrap environment."
	case "connection.postgres_username":
		return "Changes the Postgres username in helpers and the managed stack bootstrap environment."
	case "connection.postgres_password":
		return "Changes the Postgres password in helpers and the managed stack bootstrap environment."
	case "connection.redis_password":
		return "Adds or removes Redis authentication in helpers and the managed stack runtime arguments."
	case "connection.redis_acl_username":
		return "Changes the Redis ACL username used in helpers and the managed ACL bootstrap file."
	case "connection.redis_acl_password":
		return "Changes the Redis ACL password used in helpers and the managed ACL bootstrap file."
	case "connection.nats_token":
		return "Changes the NATS token in helpers and the managed NATS configuration."
	case "connection.meilisearch_master_key":
		return "Changes the Meilisearch master key used by helpers and the managed stack environment."
	case "connection.pgadmin_email":
		return "Changes the default pgAdmin login email in helpers and the managed stack environment."
	case "connection.pgadmin_password":
		return "Changes the default pgAdmin login password in helpers and the managed stack environment."
	case "behavior.wait_for_services_on_start":
		return "Controls whether start and restart wait for services to become reachable."
	case "behavior.startup_timeout_seconds":
		return "Controls how long stackctl waits for services to become ready."
	case "tui.auto_refresh_interval_seconds":
		return "Changes the default auto-refresh cadence for future TUI sessions."
	case "setup.install_cockpit":
		if reason := configpkg.CockpitInstallEnableReasonForConfig(cfg); reason != "" {
			return reason
		}
		return "Controls whether setup and doctor fix install and enable Cockpit automatically."
	case "system.package_manager":
		recommendation := configEditorCurrentPackageManagerRecommendation()
		if recommendation.Name == "" {
			return "Controls which package manager setup and doctor fix use for host package installs."
		}
		return fmt.Sprintf(
			"Controls which package manager setup and doctor fix use for host package installs. %s",
			configEditorFormatPackageManagerRecommendation(recommendation),
		)
	default:
		return ""
	}
}

func effectFollowUp(spec configFieldSpec, cfg configpkg.Config) string {
	switch {
	case spec.Key == "connection.host":
		return "Saving updates helpers and dashboards only; it does not rewrite containers."
	case spec.Key == "services.postgres.maintenance_database":
		return "Saving affects future database commands only; running services are unchanged."
	case strings.HasPrefix(spec.Key, "tui."):
		return "Saving affects future stackctl tui sessions only; it does not stop, start, scaffold, or restart services."
	case spec.Key == "setup.install_cockpit" || spec.Key == "setup.include_cockpit" || strings.HasPrefix(spec.Key, "behavior.") || strings.HasPrefix(spec.Key, "system."):
		return "Saving affects future stackctl commands only; it does not stop, start, scaffold, or restart services."
	case spec.Key == "stack.name" || spec.Key == "stack.managed" || spec.Key == "stack.dir" || spec.Key == "stack.compose_file":
		return "Saving changes which stack future stackctl commands target; it does not move files or restart services automatically."
	case spec.Key == "setup.scaffold_default_stack":
		if !cfg.Stack.Managed {
			return "External stacks ignore managed scaffolding."
		}
		return "Saving changes whether stackctl can refresh the managed compose file for you."
	case !cfg.Stack.Managed:
		return "For external stacks, saving updates stackctl metadata and helper commands only; it does not rewrite your compose file."
	case !cfg.Setup.ScaffoldDefaultStack:
		return "For managed stacks with scaffold disabled, save updates the config only; update the compose file yourself before restarting."
	default:
		return "For managed stacks, saving refreshes compose automatically and restarts running services when needed."
	}
}

func (e configEditor) runtimeImpactLines() []string {
	lines := []string{"Saving writes the draft first. stackctl only restarts services automatically when the change can be applied safely."}

	impact := analyzeConfigImpact(e.baseline, e.draft)
	if !impact.changed {
		switch {
		case e.source != ConfigSourceLoaded:
			lines = append(lines, "Nothing is running yet. Save, scaffold if needed, then start when ready.")
		default:
			lines = append(lines, "No runtime changes are pending right now.")
		}
		return lines
	}

	switch {
	case impact.stackTarget:
		lines = append(lines, "This draft changes which stack future stackctl commands target.")
	case impact.composeTemplate && e.draft.Stack.Managed && e.draft.Setup.ScaffoldDefaultStack:
		if e.runningStack > 0 {
			lines = append(lines, "Saving refreshes the managed compose file and restarts the running stack automatically.")
		} else {
			lines = append(lines, "Saving refreshes the managed compose file for the next start.")
		}
	case impact.composeTemplate && e.draft.Stack.Managed:
		lines = append(lines, "After saving, update the managed compose file yourself before the next restart.")
	default:
		lines = append(lines, "This only updates stackctl metadata and helper commands. Your compose file stays untouched.")
		if !e.draft.Stack.Managed {
			lines = append(lines, "External compose services keep running until you change them yourself.")
		}
	}

	if impact.manualFollowUp {
		lines = append(lines, "Some changes still need manual follow-up outside the TUI.")
	}

	return lines
}

func configMarshalConfig(cfg configpkg.Config) ([]byte, error) {
	cfg.ApplyDerivedFields()
	data, err := configEditorMarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config preview: %w", err)
	}
	return data, nil
}

func configsEqual(left, right configpkg.Config) bool {
	left.ApplyDerivedFields()
	right.ApplyDerivedFields()
	return reflect.DeepEqual(left, right)
}

func (e configEditor) listSummary() string {
	switch {
	case e.source != ConfigSourceLoaded:
		return "Draft values • not saved yet"
	case e.dirty():
		return "Draft values • unsaved"
	default:
		return "Draft values • saved"
	}
}

func (e configEditor) savePlan() configApplyPlan {
	plan := e.applyPlan()
	if plan.Allowed {
		return plan
	}
	return configApplyPlan{Reason: plan.Reason}
}

func redactConfigSecrets(cfg configpkg.Config) configpkg.Config {
	if strings.TrimSpace(cfg.Connection.PostgresPassword) != "" {
		cfg.Connection.PostgresPassword = maskedSecret
	}
	if strings.TrimSpace(cfg.Connection.RedisPassword) != "" {
		cfg.Connection.RedisPassword = maskedSecret
	}
	if strings.TrimSpace(cfg.Connection.RedisACLPassword) != "" {
		cfg.Connection.RedisACLPassword = maskedSecret
	}
	if strings.TrimSpace(cfg.Connection.NATSToken) != "" {
		cfg.Connection.NATSToken = maskedSecret
	}
	if strings.TrimSpace(cfg.Connection.SeaweedFSSecretKey) != "" {
		cfg.Connection.SeaweedFSSecretKey = maskedSecret
	}
	if strings.TrimSpace(cfg.Connection.MeilisearchMasterKey) != "" {
		cfg.Connection.MeilisearchMasterKey = maskedSecret
	}
	if strings.TrimSpace(cfg.Connection.PgAdminPassword) != "" {
		cfg.Connection.PgAdminPassword = maskedSecret
	}
	return cfg
}

var configFieldGroupOrder = []string{
	"Stack",
	"Postgres",
	"Redis",
	"NATS",
	"SeaweedFS",
	"Meilisearch",
	"pgAdmin",
	"Cockpit",
	"Behavior",
	"TUI",
	"System",
}

func groupedConfigFieldSpecs(group string) []configFieldSpec {
	specs := make([]configFieldSpec, 0, len(configFieldSpecs))
	for _, spec := range configFieldSpecs {
		if configFieldGroup(spec) == group {
			specs = append(specs, spec)
		}
	}
	return specs
}

func configFieldHeader(spec configFieldSpec) string {
	return configFieldGroup(spec) + " / " + configFieldLabel(spec)
}

func configFieldGroup(spec configFieldSpec) string {
	switch {
	case strings.HasPrefix(spec.Key, "stack."), spec.Key == "setup.scaffold_default_stack", spec.Key == "connection.host":
		return "Stack"
	case spec.Key == "setup.include_postgres", strings.HasPrefix(spec.Key, "services.postgres"), strings.HasPrefix(spec.Key, "ports.postgres"), strings.HasPrefix(spec.Key, "connection.postgres_"):
		return "Postgres"
	case spec.Key == "setup.include_redis", strings.HasPrefix(spec.Key, "services.redis"), strings.HasPrefix(spec.Key, "ports.redis"), strings.HasPrefix(spec.Key, "connection.redis_"):
		return "Redis"
	case spec.Key == "setup.include_nats", strings.HasPrefix(spec.Key, "services.nats"), strings.HasPrefix(spec.Key, "ports.nats"), strings.HasPrefix(spec.Key, "connection.nats_"):
		return "NATS"
	case spec.Key == "setup.include_seaweedfs", strings.HasPrefix(spec.Key, "services.seaweedfs"), strings.HasPrefix(spec.Key, "ports.seaweedfs"), strings.HasPrefix(spec.Key, "connection.seaweedfs_"):
		return "SeaweedFS"
	case spec.Key == "setup.include_meilisearch", strings.HasPrefix(spec.Key, "services.meilisearch"), strings.HasPrefix(spec.Key, "ports.meilisearch"), strings.HasPrefix(spec.Key, "connection.meilisearch_"):
		return "Meilisearch"
	case spec.Key == "setup.include_pgadmin", strings.HasPrefix(spec.Key, "services.pgadmin"), strings.HasPrefix(spec.Key, "ports.pgadmin"), strings.HasPrefix(spec.Key, "connection.pgadmin_"):
		return "pgAdmin"
	case spec.Key == "setup.include_cockpit", spec.Key == "setup.install_cockpit", strings.HasPrefix(spec.Key, "ports.cockpit"):
		return "Cockpit"
	case strings.HasPrefix(spec.Key, "behavior."):
		return "Behavior"
	case strings.HasPrefix(spec.Key, "tui."):
		return "TUI"
	case strings.HasPrefix(spec.Key, "system."):
		return "System"
	default:
		return spec.Group
	}
}

func configFieldLabel(spec configFieldSpec) string {
	switch spec.Key {
	case "stack.name":
		return "Name"
	case "stack.managed":
		return "Managed mode"
	case "stack.dir":
		return "Directory"
	case "setup.scaffold_default_stack":
		return "Scaffold compose"
	case "connection.host":
		return "Host name"
	case "setup.include_postgres":
		return "Enabled"
	case "setup.include_redis":
		return "Enabled"
	case "setup.include_nats":
		return "Enabled"
	case "setup.include_seaweedfs":
		return "Enabled"
	case "setup.include_meilisearch":
		return "Enabled"
	case "setup.include_pgadmin":
		return "Enabled"
	case "setup.include_cockpit":
		return "Enabled"
	case "setup.install_cockpit":
		return "Install"
	case "behavior.wait_for_services_on_start":
		return "Wait on start"
	case "tui.auto_refresh_interval_seconds":
		return "Auto refresh interval"
	}

	switch configFieldGroup(spec) {
	case "Postgres":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "Postgres "))
	case "Redis":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "Redis "))
	case "NATS":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "NATS "))
	case "SeaweedFS":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "SeaweedFS "))
	case "Meilisearch":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "Meilisearch "))
	case "pgAdmin":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "pgAdmin "))
	case "Cockpit":
		return titleCaseLabel(strings.TrimPrefix(spec.Label, "Cockpit "))
	default:
		return spec.Label
	}
}

func fieldItemDescription(spec configFieldSpec, cfg configpkg.Config, showSecrets bool, _ []configpkg.ValidationIssue) string {
	return compactFieldValue(spec, cfg, showSecrets)
}

func (spec configFieldSpec) suggestionHeading() string {
	if strings.TrimSpace(spec.SuggestionTitle) != "" {
		return spec.SuggestionTitle
	}
	return "Suggested values"
}

func compactFieldValue(spec configFieldSpec, cfg configpkg.Config, showSecrets bool) string {
	value := displayFieldValue(spec, cfg, showSecrets)
	switch spec.Kind {
	case configFieldBool:
		return value
	default:
		return truncateMiddle(value, 28)
	}
}

func truncateMiddle(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit || limit < 5 {
		return value
	}

	left := (limit - 1) / 2
	right := limit - left - 1
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func truncateEnd(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit || limit < 2 {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func titleCaseLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	runes := []rune(trimmed)
	first := strings.ToUpper(string(runes[0]))
	if len(runes) == 1 {
		return first
	}
	return first + string(runes[1:])
}

func (e configEditor) renderConfigFieldHeading(spec configFieldSpec) string {
	state := e.configFieldState(spec)
	chip := configFieldStateChip(state)
	if chip == "" {
		return subsectionTitleStyle().Render(configFieldHeader(spec))
	}
	return fmt.Sprintf("%s  %s", chip, subsectionTitleStyle().Render(configFieldHeader(spec)))
}

func (e configEditor) configFieldState(spec configFieldSpec) string {
	switch {
	case e.editing && e.input.Err != nil:
		return "invalid"
	case e.editing:
		return "editing"
	case len(e.issueIndex[spec.Key]) > 0:
		return "invalid"
	case e.source != ConfigSourceLoaded:
		return "edited"
	case configFieldChanged(spec, e.baseline, e.draft):
		return "edited"
	default:
		return "clean"
	}
}

func configFieldStateLabel(state string) string {
	switch state {
	case "invalid":
		return "invalid"
	case "editing":
		return "editing"
	case "edited":
		return "edited"
	default:
		return ""
	}
}

func configFieldStateChip(state string) string {
	label := configFieldStateLabel(state)
	if label == "" {
		return ""
	}
	return configFieldStateStyle(state).Render(" " + label + " ")
}

func configFieldStateStyle(state string) lipgloss.Style {
	switch state {
	case "invalid":
		return lipgloss.NewStyle().Foreground(activeTheme().fieldInvalidFg).Background(activeTheme().fieldInvalidBg).Bold(true)
	case "editing":
		return lipgloss.NewStyle().Foreground(activeTheme().fieldSavedFg).Background(activeTheme().fieldSavedBg).Bold(true)
	case "edited":
		return lipgloss.NewStyle().Foreground(activeTheme().fieldPendingFg).Background(activeTheme().fieldPendingBg).Bold(true)
	default:
		return lipgloss.NewStyle()
	}
}

func displayFieldValue(spec configFieldSpec, cfg configpkg.Config, showSecrets bool) string {
	switch spec.Kind {
	case configFieldBool:
		if spec.GetBool != nil {
			return onOffLabel(spec.GetBool(cfg))
		}
	case configFieldInt, configFieldString:
		value := selectedFieldValue(spec, cfg)
		if spec.Secret && !showSecrets && strings.TrimSpace(value) != "" {
			return maskedSecret
		}
		if strings.TrimSpace(value) == "" {
			return "(empty)"
		}
		return value
	}
	return "(unknown)"
}

func selectedFieldValue(spec configFieldSpec, cfg configpkg.Config) string {
	if spec.GetString == nil {
		return ""
	}
	return spec.GetString(cfg)
}

func selectedFieldSuggestions(spec configFieldSpec, cfg configpkg.Config) []string {
	if spec.Suggestions == nil {
		return nil
	}
	return spec.Suggestions(cfg)
}

func redisMaxMemoryPolicySuggestions(cfg configpkg.Config) []string {
	values := []string{
		"noeviction",
		"allkeys-lru",
		"allkeys-lfu",
		"allkeys-random",
		"volatile-lru",
		"volatile-lfu",
		"volatile-random",
		"volatile-ttl",
	}
	if redisImageSupportsLRMPolicies(cfg.Services.Redis.Image) {
		values = append(values, "allkeys-lrm", "volatile-lrm")
	}
	return values
}

func redisImageSupportsLRMPolicies(image string) bool {
	major, minor, ok := parseImageVersionTag(image)
	if !ok {
		return false
	}
	return major > 8 || (major == 8 && minor >= 6)
}

func parseImageVersionTag(image string) (int, int, bool) {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return 0, 0, false
	}

	tag := trimmed
	lastSlash := strings.LastIndex(tag, "/")
	lastColon := strings.LastIndex(tag, ":")
	if lastColon > lastSlash {
		tag = tag[lastColon+1:]
	} else {
		return 0, 0, false
	}

	tag = strings.SplitN(tag, "-", 2)[0]
	tag = strings.SplitN(tag, "@", 2)[0]
	parts := strings.Split(tag, ".")

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}

	minor := 0
	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}

	return major, minor, true
}

func selectedFieldEditBlock(spec configFieldSpec, cfg configpkg.Config) string {
	if spec.EditableReason == nil {
		return ""
	}
	return spec.EditableReason(cfg)
}

func wrapText(value string, width int) string {
	if width < 20 {
		return value
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		current := lines[len(lines)-1]
		if len(current)+1+len(word) <= width {
			lines[len(lines)-1] = current + " " + word
			continue
		}
		lines = append(lines, word)
	}
	return strings.Join(lines, "\n")
}

func requiredText(_ configpkg.Config, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value must not be empty")
	}
	return nil
}

func validStackNameText(_ configpkg.Config, value string) error {
	if err := requiredText(configpkg.Config{}, value); err != nil {
		return err
	}
	return configpkg.ValidateStackName(value)
}

func validPortText(_ configpkg.Config, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("port must not be empty")
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("enter a valid port")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func positiveIntText(_ configpkg.Config, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("value must not be empty")
	}
	number, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("enter a valid number")
	}
	if number <= 0 {
		return fmt.Errorf("value must be greater than zero")
	}
	return nil
}

func parsePostgresLogDurationSetting(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("value must not be empty")
	}
	if trimmed == "-1" {
		return -1, nil
	}
	number, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("enter -1 or a positive number")
	}
	if number <= 0 {
		return 0, fmt.Errorf("enter -1 or a positive number")
	}
	return number, nil
}

func validPostgresLogDurationSettingText(_ configpkg.Config, value string) error {
	_, err := parsePostgresLogDurationSetting(value)
	return err
}

func minLengthText(length int) func(configpkg.Config, string) error {
	return func(_ configpkg.Config, value string) error {
		if len(strings.TrimSpace(value)) < length {
			return fmt.Errorf("value must be at least %d characters", length)
		}
		return nil
	}
}

func stringSetter(target func(*configpkg.Config) *string) func(*configpkg.Config, string) error {
	return func(cfg *configpkg.Config, value string) error {
		*target(cfg) = strings.TrimSpace(value)
		return nil
	}
}

func intSetter(target func(*configpkg.Config) *int) func(*configpkg.Config, string) error {
	return func(cfg *configpkg.Config, value string) error {
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("enter a valid number")
		}
		*target(cfg) = parsed
		return nil
	}
}

var configFieldSpecs = []configFieldSpec{
	{
		Key:         "stack.name",
		Group:       "Stack",
		Label:       "Stack name",
		Description: "The logical stack name shown in the UI and used to derive the managed stack path.",
		Kind:        configFieldString,
		GetString:   func(cfg configpkg.Config) string { return cfg.Stack.Name },
		SetString: func(cfg *configpkg.Config, value string) error {
			cfg.Stack.Name = strings.TrimSpace(value)
			if cfg.Stack.Managed {
				managedDir, err := configpkg.ManagedStackDir(cfg.Stack.Name)
				if err != nil {
					return err
				}
				cfg.Stack.Dir = managedDir
				cfg.Stack.ComposeFile = configpkg.DefaultComposeFileName
				cfg.Setup.ScaffoldDefaultStack = true
			}
			return nil
		},
		InputValidate: validStackNameText,
	},
	{
		Key:         "stack.managed",
		Group:       "Stack",
		Label:       "Managed stack",
		Description: "Turn this on to keep the compose file under the default XDG data directory and let stackctl scaffold it for you.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Stack.Managed },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Stack.Managed = value
			if value {
				managedDir, err := configpkg.ManagedStackDir(cfg.Stack.Name)
				if err != nil {
					return err
				}
				cfg.Stack.Dir = managedDir
				cfg.Stack.ComposeFile = configpkg.DefaultComposeFileName
				cfg.Setup.ScaffoldDefaultStack = true
				return nil
			}
			cfg.Setup.ScaffoldDefaultStack = false
			return nil
		},
	},
	{
		Key:         "stack.dir",
		Group:       "Stack",
		Label:       "Stack directory",
		Description: "The directory that contains the compose file for external stacks. Managed stacks derive this path automatically.",
		Kind:        configFieldString,
		GetString:   func(cfg configpkg.Config) string { return cfg.Stack.Dir },
		SetString: func(cfg *configpkg.Config, value string) error {
			resolved, err := filepath.Abs(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("resolve stack directory: %w", err)
			}
			cfg.Stack.Dir = resolved
			return nil
		},
		InputValidate: requiredText,
		EditableReason: func(cfg configpkg.Config) string {
			if cfg.Stack.Managed {
				return "Managed stacks derive the stack directory from the stack name. Turn off managed mode to edit this path."
			}
			return ""
		},
	},
	{
		Key:           "stack.compose_file",
		Group:         "Stack",
		Label:         "Compose file",
		Description:   "The compose file name inside the stack directory. Managed stacks always use the default embedded compose filename.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Stack.ComposeFile },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Stack.ComposeFile }),
		InputValidate: requiredText,
		EditableReason: func(cfg configpkg.Config) string {
			if cfg.Stack.Managed {
				return "Managed stacks always use the embedded compose filename. Turn off managed mode to edit it."
			}
			return ""
		},
	},
	{
		Key:         "setup.include_postgres",
		Group:       "Services",
		Label:       "Include Postgres",
		Description: "Enable the Postgres service and its connection settings in the managed stack.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.IncludePostgres },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludePostgres = value
			return nil
		},
	},
	{
		Key:           "services.postgres_container",
		Group:         "Services",
		Label:         "Postgres container",
		Description:   "The compose service and container name used for the Postgres runtime.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.PostgresContainer },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.PostgresContainer }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.postgres.image",
		Group:         "Services",
		Label:         "Postgres image",
		Description:   "The container image used for Postgres.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Postgres.Image },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Postgres.Image }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.postgres.data_volume",
		Group:         "Services",
		Label:         "Postgres data volume",
		Description:   "The named volume used for Postgres data persistence.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Postgres.DataVolume },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Postgres.DataVolume }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.postgres.maintenance_database",
		Group:         "Services",
		Label:         "Postgres maintenance DB",
		Description:   "The administrative database used for maintenance commands.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Postgres.MaintenanceDatabase },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Postgres.MaintenanceDatabase }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.postgres.max_connections",
		Group:         "Services",
		Label:         "Postgres max connections",
		Description:   "The max_connections setting passed into the managed Postgres server.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Services.Postgres.MaxConnections) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Services.Postgres.MaxConnections }),
		InputValidate: positiveIntText,
	},
	{
		Key:           "services.postgres.shared_buffers",
		Group:         "Services",
		Label:         "Postgres shared buffers",
		Description:   "The shared_buffers setting passed into the managed Postgres server.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Postgres.SharedBuffers },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Postgres.SharedBuffers }),
		InputValidate: requiredText,
	},
	{
		Key:         "services.postgres.log_min_duration_statement_ms",
		Group:       "Services",
		Label:       "Postgres log min duration",
		Description: "Set to -1 to disable query duration logging, or a positive threshold in milliseconds.",
		Kind:        configFieldInt,
		GetString: func(cfg configpkg.Config) string {
			return strconv.Itoa(cfg.Services.Postgres.LogMinDurationStatementMS)
		},
		SetString: func(cfg *configpkg.Config, value string) error {
			parsed, err := parsePostgresLogDurationSetting(value)
			if err != nil {
				return err
			}
			cfg.Services.Postgres.LogMinDurationStatementMS = parsed
			return nil
		},
		InputValidate: validPostgresLogDurationSettingText,
	},
	{
		Key:         "setup.include_redis",
		Group:       "Services",
		Label:       "Include Redis",
		Description: "Enable the Redis service and its connection settings in the managed stack.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.IncludeRedis },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludeRedis = value
			return nil
		},
	},
	{
		Key:           "services.redis_container",
		Group:         "Services",
		Label:         "Redis container",
		Description:   "The compose service and container name used for Redis.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.RedisContainer },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.RedisContainer }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.redis.image",
		Group:         "Services",
		Label:         "Redis image",
		Description:   "The container image used for Redis.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Redis.Image },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Redis.Image }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.redis.data_volume",
		Group:         "Services",
		Label:         "Redis data volume",
		Description:   "The named volume used for Redis persistence.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Redis.DataVolume },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Redis.DataVolume }),
		InputValidate: requiredText,
	},
	{
		Key:         "services.redis.appendonly",
		Group:       "Services",
		Label:       "Redis appendonly",
		Description: "Enable appendonly persistence for Redis.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Services.Redis.AppendOnly },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Services.Redis.AppendOnly = value
			return nil
		},
	},
	{
		Key:           "services.redis.save_policy",
		Group:         "Services",
		Label:         "Redis save policy",
		Description:   "The Redis snapshot save policy string passed into the container.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Redis.SavePolicy },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Redis.SavePolicy }),
		InputValidate: requiredText,
	},
	{
		Key:             "services.redis.maxmemory_policy",
		Group:           "Services",
		Label:           "Redis maxmemory policy",
		Description:     "The eviction policy used when Redis reaches its memory limit.",
		SuggestionTitle: "Redis policies",
		Kind:            configFieldString,
		GetString:       func(cfg configpkg.Config) string { return cfg.Services.Redis.MaxMemoryPolicy },
		SetString:       stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Redis.MaxMemoryPolicy }),
		InputValidate:   requiredText,
		Suggestions:     redisMaxMemoryPolicySuggestions,
	},
	{
		Key:         "setup.include_nats",
		Group:       "Services",
		Label:       "Include NATS",
		Description: "Enable the NATS service and its connection settings in the managed stack.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.IncludeNATS },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludeNATS = value
			return nil
		},
	},
	{
		Key:           "services.nats_container",
		Group:         "Services",
		Label:         "NATS container",
		Description:   "The compose service and container name used for NATS.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.NATSContainer },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.NATSContainer }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.nats.image",
		Group:         "Services",
		Label:         "NATS image",
		Description:   "The container image used for NATS.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.NATS.Image },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.NATS.Image }),
		InputValidate: requiredText,
	},
	{
		Key:         "setup.include_seaweedfs",
		Group:       "Services",
		Label:       "Include SeaweedFS",
		Description: "Enable the SeaweedFS S3-compatible object storage service in the managed stack.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.IncludeSeaweedFS },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludeSeaweedFS = value
			return nil
		},
	},
	{
		Key:           "services.seaweedfs_container",
		Group:         "Services",
		Label:         "SeaweedFS container",
		Description:   "The compose service and container name used for SeaweedFS.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.SeaweedFSContainer },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.SeaweedFSContainer }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.seaweedfs.image",
		Group:         "Services",
		Label:         "SeaweedFS image",
		Description:   "The container image used for the SeaweedFS S3 service.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.SeaweedFS.Image },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.SeaweedFS.Image }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.seaweedfs.data_volume",
		Group:         "Services",
		Label:         "SeaweedFS data volume",
		Description:   "The named volume used for SeaweedFS filer and object data.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.SeaweedFS.DataVolume },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.SeaweedFS.DataVolume }),
		InputValidate: requiredText,
	},
	{
		Key:             "services.seaweedfs.volume_size_limit_mb",
		Group:           "Services",
		Label:           "SeaweedFS volume size limit",
		Description:     "The per-volume size cap, in MB, passed to the local SeaweedFS server.",
		SuggestionTitle: "Common values",
		Kind:            configFieldInt,
		GetString:       func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Services.SeaweedFS.VolumeSizeLimitMB) },
		SetString:       intSetter(func(cfg *configpkg.Config) *int { return &cfg.Services.SeaweedFS.VolumeSizeLimitMB }),
		InputValidate:   positiveIntText,
		Suggestions: func(configpkg.Config) []string {
			return []string{"512", "1024", "2048"}
		},
	},
	{
		Key:         "setup.include_meilisearch",
		Group:       "Services",
		Label:       "Include Meilisearch",
		Description: "Enable the Meilisearch service and its connection settings in the managed stack.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.IncludeMeilisearch },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludeMeilisearch = value
			return nil
		},
	},
	{
		Key:           "services.meilisearch_container",
		Group:         "Services",
		Label:         "Meilisearch container",
		Description:   "The compose service and container name used for Meilisearch.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.MeilisearchContainer },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.MeilisearchContainer }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.meilisearch.image",
		Group:         "Services",
		Label:         "Meilisearch image",
		Description:   "The container image used for Meilisearch.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Meilisearch.Image },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Meilisearch.Image }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.meilisearch.data_volume",
		Group:         "Services",
		Label:         "Meilisearch data volume",
		Description:   "The named volume used for Meilisearch index storage.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.Meilisearch.DataVolume },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.Meilisearch.DataVolume }),
		InputValidate: requiredText,
	},
	{
		Key:         "setup.include_pgadmin",
		Group:       "Services",
		Label:       "Include pgAdmin",
		Description: "Enable the pgAdmin service and its connection settings in the managed stack.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.IncludePgAdmin },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludePgAdmin = value
			return nil
		},
	},
	{
		Key:           "services.pgadmin_container",
		Group:         "Services",
		Label:         "pgAdmin container",
		Description:   "The compose service and container name used for pgAdmin.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.PgAdminContainer },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.PgAdminContainer }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.pgadmin.image",
		Group:         "Services",
		Label:         "pgAdmin image",
		Description:   "The container image used for pgAdmin.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.PgAdmin.Image },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.PgAdmin.Image }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.pgadmin.data_volume",
		Group:         "Services",
		Label:         "pgAdmin data volume",
		Description:   "The named volume used for pgAdmin state and storage.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.PgAdmin.DataVolume },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.PgAdmin.DataVolume }),
		InputValidate: requiredText,
	},
	{
		Key:         "services.pgadmin.server_mode",
		Group:       "Services",
		Label:       "pgAdmin server mode",
		Description: "Enable pgAdmin server mode for multi-user workflows.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Services.PgAdmin.ServerMode },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Services.PgAdmin.ServerMode = value
			return nil
		},
	},
	{
		Key:         "services.pgadmin.bootstrap_postgres_server",
		Group:       "Services",
		Label:       "pgAdmin bootstrap Postgres",
		Description: "Generate pgAdmin bootstrap files for the managed Postgres service.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Services.PgAdmin.BootstrapPostgresServer },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Services.PgAdmin.BootstrapPostgresServer = value
			return nil
		},
	},
	{
		Key:           "services.pgadmin.bootstrap_server_name",
		Group:         "Services",
		Label:         "pgAdmin bootstrap server name",
		Description:   "The saved pgAdmin connection name used for the managed Postgres bootstrap entry.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.PgAdmin.BootstrapServerName },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.PgAdmin.BootstrapServerName }),
		InputValidate: requiredText,
	},
	{
		Key:           "services.pgadmin.bootstrap_server_group",
		Group:         "Services",
		Label:         "pgAdmin bootstrap server group",
		Description:   "The pgAdmin server group used for the managed Postgres bootstrap entry.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Services.PgAdmin.BootstrapServerGroup },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Services.PgAdmin.BootstrapServerGroup }),
		InputValidate: requiredText,
	},
	{
		Key:           "ports.postgres",
		Group:         "Ports",
		Label:         "Postgres port",
		Description:   "The host port exposed for Postgres.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.Postgres) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.Postgres }),
		InputValidate: validPortText,
	},
	{
		Key:           "ports.redis",
		Group:         "Ports",
		Label:         "Redis port",
		Description:   "The host port exposed for Redis.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.Redis) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.Redis }),
		InputValidate: validPortText,
	},
	{
		Key:           "ports.nats",
		Group:         "Ports",
		Label:         "NATS port",
		Description:   "The host port exposed for NATS.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.NATS) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.NATS }),
		InputValidate: validPortText,
	},
	{
		Key:           "ports.seaweedfs",
		Group:         "Ports",
		Label:         "SeaweedFS port",
		Description:   "The host port exposed for the SeaweedFS S3 endpoint.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.SeaweedFS) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.SeaweedFS }),
		InputValidate: validPortText,
	},
	{
		Key:           "ports.meilisearch",
		Group:         "Ports",
		Label:         "Meilisearch port",
		Description:   "The host port exposed for the Meilisearch API and preview interface.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.Meilisearch) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.Meilisearch }),
		InputValidate: validPortText,
	},
	{
		Key:           "ports.pgadmin",
		Group:         "Ports",
		Label:         "pgAdmin port",
		Description:   "The host port exposed for pgAdmin.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.PgAdmin) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.PgAdmin }),
		InputValidate: validPortText,
	},
	{
		Key:           "ports.cockpit",
		Group:         "Ports",
		Label:         "Cockpit port",
		Description:   "The host port exposed for Cockpit.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Ports.Cockpit) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Ports.Cockpit }),
		InputValidate: validPortText,
	},
	{
		Key:           "connection.host",
		Group:         "Connections",
		Label:         "Host name",
		Description:   "The host name used to derive URLs and DSNs for the local stack.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.Host },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.Host }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.postgres_database",
		Group:         "Connections",
		Label:         "Postgres database",
		Description:   "The default Postgres database name used by stackctl connection helpers.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.PostgresDatabase },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.PostgresDatabase }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.postgres_username",
		Group:         "Connections",
		Label:         "Postgres username",
		Description:   "The Postgres username used for connection strings and bootstrap settings.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.PostgresUsername },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.PostgresUsername }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.postgres_password",
		Group:         "Connections",
		Label:         "Postgres password",
		Description:   "The Postgres password used for connection strings and bootstrap settings.",
		Kind:          configFieldString,
		Secret:        true,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.PostgresPassword },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.PostgresPassword }),
		InputValidate: requiredText,
	},
	{
		Key:         "connection.redis_password",
		Group:       "Connections",
		Label:       "Redis password",
		Description: "Leave this empty to disable Redis auth, or set a password to require authentication.",
		Kind:        configFieldString,
		Secret:      true,
		GetString:   func(cfg configpkg.Config) string { return cfg.Connection.RedisPassword },
		SetString:   stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.RedisPassword }),
	},
	{
		Key:         "connection.redis_acl_username",
		Group:       "Connections",
		Label:       "Redis ACL username",
		Description: "Optional named Redis ACL username for app connections. Leave empty to keep DSNs password-only.",
		Kind:        configFieldString,
		GetString:   func(cfg configpkg.Config) string { return cfg.Connection.RedisACLUsername },
		SetString:   stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.RedisACLUsername }),
	},
	{
		Key:         "connection.redis_acl_password",
		Group:       "Connections",
		Label:       "Redis ACL password",
		Description: "Optional named Redis ACL password. Set this together with the Redis ACL username.",
		Kind:        configFieldString,
		Secret:      true,
		GetString:   func(cfg configpkg.Config) string { return cfg.Connection.RedisACLPassword },
		SetString:   stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.RedisACLPassword }),
	},
	{
		Key:           "connection.nats_token",
		Group:         "Connections",
		Label:         "NATS token",
		Description:   "The token used for NATS client authentication and the managed NATS config file.",
		Kind:          configFieldString,
		Secret:        true,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.NATSToken },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.NATSToken }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.seaweedfs_access_key",
		Group:         "Connections",
		Label:         "SeaweedFS access key",
		Description:   "The S3 access key used by SeaweedFS clients.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.SeaweedFSAccessKey },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.SeaweedFSAccessKey }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.seaweedfs_secret_key",
		Group:         "Connections",
		Label:         "SeaweedFS secret key",
		Description:   "The S3 secret key used by SeaweedFS clients.",
		Kind:          configFieldString,
		Secret:        true,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.SeaweedFSSecretKey },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.SeaweedFSSecretKey }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.meilisearch_master_key",
		Group:         "Connections",
		Label:         "Meilisearch master key",
		Description:   "The Meilisearch master key used for local-dev auth and exported API key helpers.",
		Kind:          configFieldString,
		Secret:        true,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.MeilisearchMasterKey },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.MeilisearchMasterKey }),
		InputValidate: minLengthText(16),
	},
	{
		Key:           "connection.pgadmin_email",
		Group:         "Connections",
		Label:         "pgAdmin email",
		Description:   "The default pgAdmin login email.",
		Kind:          configFieldString,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.PgAdminEmail },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.PgAdminEmail }),
		InputValidate: requiredText,
	},
	{
		Key:           "connection.pgadmin_password",
		Group:         "Connections",
		Label:         "pgAdmin password",
		Description:   "The default pgAdmin login password.",
		Kind:          configFieldString,
		Secret:        true,
		GetString:     func(cfg configpkg.Config) string { return cfg.Connection.PgAdminPassword },
		SetString:     stringSetter(func(cfg *configpkg.Config) *string { return &cfg.Connection.PgAdminPassword }),
		InputValidate: requiredText,
	},
	{
		Key:         "behavior.wait_for_services_on_start",
		Group:       "Behavior",
		Label:       "Wait for services on start",
		Description: "Wait for configured services to become reachable after stackctl start and restart.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Behavior.WaitForServicesStart },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Behavior.WaitForServicesStart = value
			return nil
		},
	},
	{
		Key:           "behavior.startup_timeout_seconds",
		Group:         "Behavior",
		Label:         "Startup timeout",
		Description:   "The maximum wait, in seconds, for stack services to become ready.",
		Kind:          configFieldInt,
		GetString:     func(cfg configpkg.Config) string { return strconv.Itoa(cfg.Behavior.StartupTimeoutSec) },
		SetString:     intSetter(func(cfg *configpkg.Config) *int { return &cfg.Behavior.StartupTimeoutSec }),
		InputValidate: positiveIntText,
	},
	{
		Key:             "tui.auto_refresh_interval_seconds",
		Group:           "TUI",
		Label:           "Auto refresh interval",
		Description:     "The number of seconds between automatic snapshot refreshes while auto-refresh is on.",
		SuggestionTitle: "Common values",
		Kind:            configFieldInt,
		GetString:       func(cfg configpkg.Config) string { return strconv.Itoa(cfg.TUI.AutoRefreshIntervalSec) },
		SetString:       intSetter(func(cfg *configpkg.Config) *int { return &cfg.TUI.AutoRefreshIntervalSec }),
		InputValidate:   positiveIntText,
		Suggestions: func(configpkg.Config) []string {
			return []string{"5", "10", "30", "60"}
		},
	},
	{
		Key:         "setup.include_cockpit",
		Group:       "Setup",
		Label:       "Include Cockpit",
		Description: configpkg.CurrentCockpitHelperDescription(),
		DescriptionFor: func(cfg configpkg.Config) string {
			return configpkg.CockpitHelperDescriptionForConfig(cfg)
		},
		Kind:    configFieldBool,
		GetBool: func(cfg configpkg.Config) bool { return cfg.Setup.IncludeCockpit },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.IncludeCockpit = value
			configpkg.NormalizeCockpitSettings(cfg)
			return nil
		},
	},
	{
		Key:         "setup.install_cockpit",
		Group:       "Setup",
		Label:       "Install Cockpit",
		Description: configpkg.CurrentCockpitInstallDescription(),
		DescriptionFor: func(cfg configpkg.Config) string {
			return configpkg.CockpitInstallDescriptionForConfig(cfg)
		},
		Kind:    configFieldBool,
		GetBool: func(cfg configpkg.Config) bool { return cfg.Setup.InstallCockpit },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			if value {
				if reason := configpkg.CockpitInstallEnableReasonForConfig(*cfg); reason != "" {
					return fmt.Errorf("%s", reason)
				}
			}
			cfg.Setup.InstallCockpit = value
			configpkg.NormalizeCockpitSettings(cfg)
			return nil
		},
		EditableReason: func(cfg configpkg.Config) string {
			if !cfg.Setup.IncludeCockpit {
				return "Turn on Cockpit helpers first to change install behavior."
			}
			return ""
		},
	},
	{
		Key:         "setup.scaffold_default_stack",
		Group:       "Setup",
		Label:       "Scaffold managed stack",
		Description: "Keep the managed compose file in sync with the embedded stack template.",
		Kind:        configFieldBool,
		GetBool:     func(cfg configpkg.Config) bool { return cfg.Setup.ScaffoldDefaultStack },
		SetBool: func(cfg *configpkg.Config, value bool) error {
			cfg.Setup.ScaffoldDefaultStack = value
			return nil
		},
		EditableReason: func(cfg configpkg.Config) string {
			if !cfg.Stack.Managed {
				return "External stacks do not use the managed scaffold flow."
			}
			return ""
		},
	},
	{
		Key:         "system.package_manager",
		Group:       "System",
		Label:       "Package manager",
		Description: configpkg.CurrentPackageManagerFieldDescription(),
		DescriptionFor: func(cfg configpkg.Config) string {
			return configpkg.PackageManagerFieldDescriptionForConfig(cfg)
		},
		SuggestionTitle: "Common values",
		Kind:            configFieldString,
		GetString:       func(cfg configpkg.Config) string { return cfg.System.PackageManager },
		SetString: func(cfg *configpkg.Config, value string) error {
			cfg.System.PackageManager = strings.TrimSpace(value)
			configpkg.NormalizeCockpitSettings(cfg)
			return nil
		},
		InputValidate: requiredText,
		Suggestions: func(cfg configpkg.Config) []string {
			return packageManagerConfigSuggestions(cfg.System.PackageManager)
		},
	},
}

func packageManagerConfigSuggestions(current string) []string {
	values := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)

	appendValue := func(value string) {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		values = append(values, normalized)
	}

	appendValue(system.CurrentPackageManagerRecommendation().Name)
	appendValue(current)
	for _, value := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "brew"} {
		appendValue(value)
	}

	return values
}
