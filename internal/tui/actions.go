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
	ActionOpenCockpit ActionID = "open-cockpit"
	ActionOpenPgAdmin ActionID = "open-pgadmin"
	ActionDoctor      ActionID = "doctor"
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

type confirmationState struct {
	Action ActionSpec
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
	case '1', '2', '3', '4', '5', '6':
		return int(keyText[0] - '1'), true
	default:
		return 0, false
	}
}

func availableActions(snapshot Snapshot) []ActionSpec {
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

	actions := make([]ActionSpec, 0, 5)
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
		return false
	}
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

	actions := availableActions(m.snapshot)
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
		lines = append(lines, mutedStyle().Render("Confirm "+m.confirmation.Action.Label))
	}

	return strings.Join(lines, "\n")
}

func renderConfirmation(state *confirmationState) string {
	if state == nil {
		return ""
	}

	lines := []string{
		subsectionTitleStyle().Render("Confirm action"),
		"",
		state.Action.Label,
		"",
		state.Action.ConfirmMessage,
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
	case m.confirmation != nil && m.confirmation.Action.ID == action.ID:
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
