package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/traweezy/stackctl/internal/output"
)

type ActionID string

const (
	ActionStart       ActionID = "start"
	ActionStop        ActionID = "stop"
	ActionRestart     ActionID = "restart"
	ActionUseStack    ActionID = "use-stack"
	ActionOpenCockpit ActionID = "open-cockpit"
	ActionOpenPgAdmin ActionID = "open-pgadmin"
	ActionDoctor      ActionID = "doctor"
)

const (
	actionStartServicePrefix   = "start-service:"
	actionStopServicePrefix    = "stop-service:"
	actionRestartServicePrefix = "restart-service:"
	actionUseStackPrefix       = "use-stack:"
	actionDeleteStackPrefix    = "delete-stack:"
)

type ActionRunner func(ActionID) (ActionReport, error)

type ActionReport struct {
	Status  string
	Message string
	Details []string
	Refresh bool
}

type ActionSpec struct {
	ID             ActionID
	Label          string
	Group          string
	ConfirmMessage string
	PendingMessage string
	PendingStatus  string
	DefaultStatus  string
}

func (s ActionSpec) RequiresConfirmation() bool {
	return strings.TrimSpace(s.ConfirmMessage) != ""
}

type actionBanner struct {
	ID      int
	Status  string
	Message string
}

type confirmationKind int

const (
	confirmationAction confirmationKind = iota
	confirmationConfigReset
)

type confirmationState struct {
	Kind    confirmationKind
	Title   string
	Message string
	Action  ActionSpec
}

type runningAction struct {
	Action   ActionSpec
	History  int
	Previous Snapshot
}

type historyEntry struct {
	ID          int
	Action      string
	Status      string
	Message     string
	Details     []string
	StartedAt   time.Time
	CompletedAt time.Time
	Recent      *paletteAction
}

type actionMsg struct {
	historyID int
	action    ActionSpec
	report    ActionReport
	err       error
}

type bannerClearMsg struct {
	id int
}

func actionIndex(keyText string) (int, bool) {
	if len(keyText) != 1 {
		return 0, false
	}

	switch keyText[0] {
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return int(keyText[0] - '1'), true
	default:
		return 0, false
	}
}

func availableActions(snapshot Snapshot, selected Service, hasSelected bool) []ActionSpec {
	running, total := runningStackServiceCount(snapshot.Services)

	addLifecycleActions := func(actions []ActionSpec) []ActionSpec {
		switch {
		case running == 0:
			actions = append(actions, actionStartSpec())
		case running < total:
			actions = append(actions, actionStartSpec(), actionRestartSpec(), actionStopSpec())
		default:
			actions = append(actions, actionRestartSpec(), actionStopSpec())
		}

		return actions
	}

	actions := make([]ActionSpec, 0, 8)
	if hasSelected && isStackService(selected) {
		actions = append(actions, selectedServiceActions(selected)...)
	}
	actions = addLifecycleActions(actions)
	actions = append(actions, actionDoctorSpec())
	if includeOpenCockpit(snapshot.Services) {
		actions = append(actions, actionOpenCockpitSpec())
	}
	if includeOpenPgAdmin(snapshot.Services) {
		actions = append(actions, actionOpenPgAdminSpec())
	}

	return actions
}

func selectedServiceActions(service Service) []ActionSpec {
	switch strings.ToLower(displayServiceStatus(service)) {
	case "running":
		return []ActionSpec{
			actionRestartServiceSpec(service),
			actionStopServiceSpec(service),
		}
	default:
		return []ActionSpec{actionStartServiceSpec(service)}
	}
}

func availableStackActions(profile StackProfile, hasSelected bool) []ActionSpec {
	if !hasSelected {
		return nil
	}

	actions := make([]ActionSpec, 0, 2)
	if !profile.Current {
		actions = append(actions, actionUseStackSpec(profile))
	}
	if profile.Configured {
		actions = append(actions, actionDeleteStackSpec(profile))
	}

	return actions
}

func actionStartSpec() ActionSpec {
	return ActionSpec{
		ID:             ActionStart,
		Label:          "Start",
		Group:          "Stack",
		PendingMessage: "starting stack...",
		PendingStatus:  output.StatusStart,
		DefaultStatus:  output.StatusOK,
	}
}

func actionStartServiceSpec(service Service) ActionSpec {
	return ActionSpec{
		ID:             ActionID(actionStartServicePrefix + service.Name),
		Label:          "Start " + service.DisplayName,
		Group:          "Service",
		PendingMessage: "starting " + strings.ToLower(service.DisplayName) + "...",
		PendingStatus:  output.StatusStart,
		DefaultStatus:  output.StatusOK,
	}
}

func actionUseStackSpec(profile StackProfile) ActionSpec {
	return ActionSpec{
		ID:             ActionID(actionUseStackPrefix + profile.Name),
		Label:          "Use " + profile.Name,
		Group:          "Stack profile",
		PendingMessage: "switching to " + profile.Name + "...",
		PendingStatus:  output.StatusInfo,
		DefaultStatus:  output.StatusOK,
	}
}

func actionDeleteStackSpec(profile StackProfile) ActionSpec {
	message := "Delete stack " + profile.Name + " now?"
	switch {
	case profile.Mode == "managed":
		message += " This removes the config and stackctl-managed local data for that profile."
	case profile.Current:
		message += " The dashboard will fall back to dev-stack afterwards."
	default:
		message += " This removes the saved stack profile."
	}

	return ActionSpec{
		ID:             ActionID(actionDeleteStackPrefix + profile.Name),
		Label:          "Delete " + profile.Name,
		Group:          "Stack profile",
		ConfirmMessage: message,
		PendingMessage: "deleting " + profile.Name + "...",
		PendingStatus:  output.StatusReset,
		DefaultStatus:  output.StatusOK,
	}
}

func actionStopSpec() ActionSpec {
	return ActionSpec{
		ID:             ActionStop,
		Label:          "Stop",
		Group:          "Stack",
		ConfirmMessage: "Stop the local stack now? Running services will be interrupted.",
		PendingMessage: "stopping stack...",
		PendingStatus:  output.StatusStop,
		DefaultStatus:  output.StatusOK,
	}
}

func actionStopServiceSpec(service Service) ActionSpec {
	return ActionSpec{
		ID:             ActionID(actionStopServicePrefix + service.Name),
		Label:          "Stop " + service.DisplayName,
		Group:          "Service",
		ConfirmMessage: "Stop " + service.DisplayName + " now? Running work on that service will be interrupted.",
		PendingMessage: "stopping " + strings.ToLower(service.DisplayName) + "...",
		PendingStatus:  output.StatusStop,
		DefaultStatus:  output.StatusOK,
	}
}

func actionRestartSpec() ActionSpec {
	return ActionSpec{
		ID:             ActionRestart,
		Label:          "Restart",
		Group:          "Stack",
		ConfirmMessage: "Restart the local stack now? Running services will be interrupted.",
		PendingMessage: "restarting stack...",
		PendingStatus:  output.StatusRestart,
		DefaultStatus:  output.StatusOK,
	}
}

func actionRestartServiceSpec(service Service) ActionSpec {
	return ActionSpec{
		ID:             ActionID(actionRestartServicePrefix + service.Name),
		Label:          "Restart " + service.DisplayName,
		Group:          "Service",
		ConfirmMessage: "Restart " + service.DisplayName + " now? Running work on that service will be interrupted.",
		PendingMessage: "restarting " + strings.ToLower(service.DisplayName) + "...",
		PendingStatus:  output.StatusRestart,
		DefaultStatus:  output.StatusOK,
	}
}

func actionOpenCockpitSpec() ActionSpec {
	return ActionSpec{
		ID:             ActionOpenCockpit,
		Label:          "Open Cockpit",
		Group:          "Open",
		PendingMessage: "opening Cockpit...",
		PendingStatus:  output.StatusInfo,
		DefaultStatus:  output.StatusOK,
	}
}

func actionOpenPgAdminSpec() ActionSpec {
	return ActionSpec{
		ID:             ActionOpenPgAdmin,
		Label:          "Open pgAdmin",
		Group:          "Open",
		PendingMessage: "opening pgAdmin...",
		PendingStatus:  output.StatusInfo,
		DefaultStatus:  output.StatusOK,
	}
}

func actionDoctorSpec() ActionSpec {
	return ActionSpec{
		ID:             ActionDoctor,
		Label:          "Doctor",
		Group:          "Stack",
		PendingMessage: "running doctor diagnostics...",
		PendingStatus:  output.StatusHealth,
		DefaultStatus:  output.StatusOK,
	}
}

func lifecycleAction(action ActionID) bool {
	switch action {
	case ActionStart, ActionStop, ActionRestart:
		return true
	default:
		return strings.HasPrefix(string(action), actionStartServicePrefix) ||
			strings.HasPrefix(string(action), actionStopServicePrefix) ||
			strings.HasPrefix(string(action), actionRestartServicePrefix)
	}
}

func serviceActionTarget(action ActionID) (string, string, bool) {
	value := string(action)
	switch {
	case strings.HasPrefix(value, actionStartServicePrefix):
		return "start", strings.TrimPrefix(value, actionStartServicePrefix), true
	case strings.HasPrefix(value, actionStopServicePrefix):
		return "stop", strings.TrimPrefix(value, actionStopServicePrefix), true
	case strings.HasPrefix(value, actionRestartServicePrefix):
		return "restart", strings.TrimPrefix(value, actionRestartServicePrefix), true
	default:
		return "", "", false
	}
}

func ServiceActionTarget(action ActionID) (string, string, bool) {
	return serviceActionTarget(action)
}

func stackActionTarget(action ActionID) (string, string, bool) {
	value := string(action)
	switch {
	case strings.HasPrefix(value, actionUseStackPrefix):
		return "use", strings.TrimPrefix(value, actionUseStackPrefix), true
	case strings.HasPrefix(value, actionDeleteStackPrefix):
		return "delete", strings.TrimPrefix(value, actionDeleteStackPrefix), true
	default:
		return "", "", false
	}
}

func StackActionTarget(action ActionID) (string, string, bool) {
	return stackActionTarget(action)
}

func includeOpenCockpit(services []Service) bool {
	for _, service := range services {
		if !strings.EqualFold(service.DisplayName, "Cockpit") {
			continue
		}
		return strings.TrimSpace(service.URL) != ""
	}

	return false
}

func includeOpenPgAdmin(services []Service) bool {
	for _, service := range services {
		if !strings.EqualFold(service.DisplayName, "pgAdmin") {
			continue
		}
		if strings.TrimSpace(service.URL) == "" {
			return false
		}
		return strings.EqualFold(displayServiceStatus(service), "running")
	}

	return false
}

func renderActionRail(m Model) string {
	if m.runner == nil {
		return ""
	}

	actions := m.availableSidebarActions()
	if len(actions) == 0 {
		return ""
	}

	lines := []string{sectionTitleStyle().Render("Actions")}
	currentGroup := ""
	for idx, action := range actions {
		if action.Group != "" && action.Group != currentGroup {
			lines = append(lines, "")
			lines = append(lines, subsectionTitleStyle().Render(action.Group))
			currentGroup = action.Group
		}
		label := fmt.Sprintf("[%d] %s", idx+1, action.Label)
		if m.runningAction != nil && m.runningAction.Action.ID == action.ID {
			label += "…"
		}
		lines = append(lines, "  "+actionChipStyle(action, m).Render(label))
	}

	switch {
	case m.runningAction != nil:
		lines = append(lines, "")
		lines = append(lines, mutedStyle().Render("Running "+m.runningAction.Action.Label))
	case m.confirmation != nil:
		lines = append(lines, "")
		lines = append(lines, mutedStyle().Render(m.confirmationSidebarLabel()))
	}

	return strings.Join(lines, "\n")
}

func (m Model) availableSidebarActions() []ActionSpec {
	if m.active == stacksSection {
		profile, ok := selectedStackProfile(m.snapshot, m.selectedStack)
		return availableStackActions(profile, ok)
	}

	selected, hasSelected := m.selectedLifecycleService()
	return availableActions(m.snapshot, selected, hasSelected)
}

func renderConfirmation(state *confirmationState) string {
	if state == nil {
		return ""
	}

	lines := []string{
		subsectionTitleStyle().Render(emptyLabel(state.Title)),
		"",
		emptyLabel(state.confirmationLabel()),
		"",
		state.confirmationMessage(),
		"",
		mutedStyle().Render("y / enter confirm  •  n / esc cancel"),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("221")).
		Width(56).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func renderHistory(history []historyEntry) string {
	lines := []string{sectionTitleStyle().Render("History"), ""}
	if len(history) == 0 {
		lines = append(lines, mutedStyle().Render("No actions have run in this session yet."))
		return strings.Join(lines, "\n")
	}

	for idx := len(history) - 1; idx >= 0; idx-- {
		entry := history[idx]
		lines = append(lines, statusStyle(entry.Status).Render(fmt.Sprintf("%s  %s", serviceStatusBadge(entry.Status), entry.Action)))
		lines = append(lines, fmt.Sprintf("Status: %s", historyStatusLabel(entry)))
		lines = append(lines, fmt.Sprintf("When: %s", historyTimestamp(entry)))
		lines = append(lines, fmt.Sprintf("Message: %s", emptyLabel(entry.Message)))
		for _, detail := range entry.Details {
			lines = append(lines, mutedStyle().Render("  "+detail))
		}
		if idx > 0 {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

func historyStatusLabel(entry historyEntry) string {
	if entry.CompletedAt.IsZero() {
		return "in progress"
	}

	switch entry.Status {
	case output.StatusOK:
		return "completed"
	case output.StatusWarn:
		return "completed with warnings"
	case output.StatusFail, output.StatusMiss:
		return "failed"
	default:
		return strings.ToLower(strings.TrimSpace(entry.Status))
	}
}

func historyTimestamp(entry historyEntry) string {
	if entry.CompletedAt.IsZero() {
		return entry.StartedAt.Format("2006-01-02 15:04:05")
	}

	return entry.CompletedAt.Format("2006-01-02 15:04:05")
}

func bannerStyle(status string) lipgloss.Style {
	switch status {
	case output.StatusOK:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("28")).Padding(0, 1)
	case output.StatusWarn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("221")).Padding(0, 1)
	case output.StatusFail, output.StatusMiss:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160")).Padding(0, 1)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1)
	}
}

func actionChipStyle(action ActionSpec, m Model) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true)

	switch {
	case m.confirmation != nil && m.confirmation.Kind == confirmationAction && m.confirmation.Action.ID == action.ID:
		return style.Foreground(lipgloss.Color("221"))
	case m.runningAction != nil && m.runningAction.Action.ID == action.ID:
		return style.Foreground(lipgloss.Color("81"))
	default:
		return style.Foreground(lipgloss.Color("252"))
	}
}

func (m Model) beginAction(action ActionSpec) (tea.Model, tea.Cmd) {
	if m.runner == nil {
		return m, nil
	}

	m.confirmation = nil
	m.setBanner(action.PendingStatus, action.PendingMessage)
	m.autoRefreshID++
	m.nextHistoryID++
	historyID := m.nextHistoryID
	m.history = append(m.history, historyEntry{
		ID:        historyID,
		Action:    action.Label,
		Status:    action.PendingStatus,
		Message:   action.PendingMessage,
		StartedAt: time.Now(),
		Recent:    recentPaletteActionForActionSpec(action),
	})
	m.runningAction = &runningAction{
		Action:   action,
		History:  historyID,
		Previous: m.snapshot,
	}
	m.snapshot = applyOptimisticUpdate(m.snapshot, action.ID)
	m.syncLayout()

	return m, runActionCmd(m.runner, action, historyID)
}

func runActionCmd(runner ActionRunner, action ActionSpec, historyID int) tea.Cmd {
	return func() tea.Msg {
		report, err := runner(action.ID)
		if strings.TrimSpace(report.Status) == "" {
			report.Status = action.DefaultStatus
		}
		if strings.TrimSpace(report.Message) == "" {
			report.Message = action.Label + " completed"
		}
		return actionMsg{
			historyID: historyID,
			action:    action,
			report:    report,
			err:       err,
		}
	}
}

func (m *Model) cancelConfirmation() tea.Cmd {
	if m.confirmation == nil {
		return nil
	}

	if m.confirmation.Kind != confirmationAction {
		title := strings.ToLower(strings.TrimSpace(m.confirmation.Title))
		if title == "" {
			title = "confirmation"
		}
		m.confirmation = nil
		bannerID := m.setBanner(output.StatusWarn, title+" cancelled")
		return clearBannerCmd(bannerID)
	}

	action := m.confirmation.Action
	m.confirmation = nil
	bannerID := m.setBanner(output.StatusWarn, strings.ToLower(action.Label)+" cancelled")
	m.nextHistoryID++
	m.history = append(m.history, historyEntry{
		ID:          m.nextHistoryID,
		Action:      action.Label,
		Status:      output.StatusWarn,
		Message:     strings.ToLower(action.Label) + " cancelled",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	})

	return clearBannerCmd(bannerID)
}

func (m *Model) completeAction(msg actionMsg) tea.Cmd {
	if m.runningAction == nil || m.runningAction.History != msg.historyID {
		return nil
	}

	entryIndex := -1
	for idx := range m.history {
		if m.history[idx].ID == msg.historyID {
			entryIndex = idx
			break
		}
	}

	status := msg.report.Status
	message := msg.report.Message
	details := append([]string(nil), msg.report.Details...)
	if msg.err != nil {
		status = output.StatusFail
		message = fmt.Sprintf("%s failed: %v", strings.ToLower(msg.action.Label), msg.err)
		details = nil
		m.snapshot = m.runningAction.Previous
	}

	if entryIndex >= 0 {
		m.history[entryIndex].Status = status
		m.history[entryIndex].Message = message
		m.history[entryIndex].Details = details
		m.history[entryIndex].CompletedAt = time.Now()
	}

	bannerID := m.setBanner(status, message)
	m.runningAction = nil

	return clearBannerCmd(bannerID)
}

func (m *Model) setBanner(status, message string) int {
	m.nextBannerID++
	m.banner = &actionBanner{
		ID:      m.nextBannerID,
		Status:  status,
		Message: message,
	}

	return m.nextBannerID
}

func applyOptimisticUpdate(snapshot Snapshot, action ActionID) Snapshot {
	updated := snapshot
	updated.Services = append([]Service(nil), snapshot.Services...)
	for idx, service := range updated.Services {
		if !isStackService(service) {
			continue
		}

		switch action {
		case ActionStart:
			updated.Services[idx].Status = "starting"
		case ActionStop:
			updated.Services[idx].Status = "stopping"
			updated.Services[idx].PortListening = false
		case ActionRestart:
			updated.Services[idx].Status = "restarting"
			updated.Services[idx].PortListening = false
		}
	}

	return updated
}

func newActionConfirmation(action ActionSpec) *confirmationState {
	return &confirmationState{
		Kind:    confirmationAction,
		Title:   "Confirm action",
		Message: action.ConfirmMessage,
		Action:  action,
	}
}

func newConfigResetConfirmation() *confirmationState {
	return &confirmationState{
		Kind:    confirmationConfigReset,
		Title:   "Reset draft",
		Message: "Discard the unsaved config changes and restore the last loaded values?",
	}
}

func (c confirmationState) confirmationLabel() string {
	if c.Kind == confirmationAction {
		return c.Action.Label
	}
	return c.Title
}

func (c confirmationState) confirmationMessage() string {
	if strings.TrimSpace(c.Message) != "" {
		return c.Message
	}
	if c.Kind == confirmationAction {
		return c.Action.ConfirmMessage
	}
	return ""
}

func (m Model) confirmationSidebarLabel() string {
	if m.confirmation == nil {
		return ""
	}
	if m.confirmation.Kind == confirmationAction {
		return "Confirm " + m.confirmation.Action.Label
	}
	return "Confirm " + m.confirmation.Title
}
