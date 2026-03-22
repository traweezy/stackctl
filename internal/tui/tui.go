package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/traweezy/stackctl/internal/output"
)

const maskedSecret = "****"

var autoRefreshInterval = 30 * time.Second

type Loader func() (Snapshot, error)

type Snapshot struct {
	ConfigPath        string
	StackName         string
	StackDir          string
	ComposePath       string
	Managed           bool
	WaitForServices   bool
	StartupTimeoutSec int
	LoadedAt          time.Time
	ServiceError      string
	HealthError       string
	Services          []Service
	Health            []HealthLine
	Connections       []Connection
}

type Service struct {
	DisplayName     string
	Status          string
	ContainerName   string
	Image           string
	DataVolume      string
	Host            string
	ExternalPort    int
	InternalPort    int
	PortListening   bool
	Database        string
	MaintenanceDB   string
	Email           string
	Username        string
	Password        string
	AppendOnly      *bool
	SavePolicy      string
	MaxMemoryPolicy string
	ServerMode      string
	URL             string
	DSN             string
}

type HealthLine struct {
	Status  string
	Message string
}

type Connection struct {
	Name  string
	Value string
}

type section int

const (
	overviewSection section = iota
	servicesSection
	healthSection
	connectionsSection
	historySection
)

type layoutMode int

const (
	expandedLayout layoutMode = iota
	compactLayout
)

func (m layoutMode) String() string {
	switch m {
	case compactLayout:
		return "compact"
	default:
		return "expanded"
	}
}

var sections = []section{
	overviewSection,
	servicesSection,
	healthSection,
	connectionsSection,
	historySection,
}

func (s section) Title() string {
	switch s {
	case overviewSection:
		return "Overview"
	case servicesSection:
		return "Services"
	case healthSection:
		return "Health"
	case connectionsSection:
		return "Connections"
	case historySection:
		return "History"
	default:
		return "Unknown"
	}
}

type snapshotMsg struct {
	snapshot Snapshot
	err      error
}

type autoRefreshMsg struct {
	id int
}

type keyMap struct {
	NextSection       key.Binding
	PrevSection       key.Binding
	Action            key.Binding
	Confirm           key.Binding
	Cancel            key.Binding
	Refresh           key.Binding
	ToggleAutoRefresh key.Binding
	ToggleLayout      key.Binding
	ToggleSecrets     key.Binding
	ToggleHelp        key.Binding
	Quit              key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		NextSection: key.NewBinding(
			key.WithKeys("right", "down", "j", "tab", "l"),
			key.WithHelp("tab/j", "next section"),
		),
		PrevSection: key.NewBinding(
			key.WithKeys("left", "up", "k", "shift+tab", "h"),
			key.WithHelp("shift+tab/k", "previous section"),
		),
		Action: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6"),
			key.WithHelp("1-6", "action"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y", "enter"),
			key.WithHelp("y/enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "cancel"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ToggleAutoRefresh: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle auto-refresh"),
		),
		ToggleLayout: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "toggle compact view"),
		),
		ToggleSecrets: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "show or hide secrets"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextSection, k.Action, k.Refresh, k.ToggleAutoRefresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextSection, k.PrevSection, k.Action},
		{k.Confirm, k.Cancel, k.Refresh},
		{k.ToggleAutoRefresh, k.ToggleLayout, k.ToggleSecrets},
		{k.ToggleHelp, k.Quit},
	}
}

type Model struct {
	width         int
	height        int
	active        section
	layout        layoutMode
	loading       bool
	autoRefresh   bool
	autoRefreshID int
	showSecrets   bool
	errMessage    string
	snapshot      Snapshot
	loader        Loader
	runner        ActionRunner
	keys          keyMap
	help          help.Model
	viewport      viewport.Model
	banner        *actionBanner
	confirmation  *confirmationState
	runningAction *runningAction
	history       []historyEntry
	nextHistoryID int
}

func NewModel(loader Loader) Model {
	return newModel(loader, nil)
}

func NewActionModel(loader Loader, runner ActionRunner) Model {
	return newModel(loader, runner)
}

func newModel(loader Loader, runner ActionRunner) Model {
	viewportModel := viewport.New()
	helpModel := help.New()
	helpModel.ShowAll = false

	return Model{
		active:      overviewSection,
		layout:      expandedLayout,
		loading:     true,
		autoRefresh: true,
		loader:      loader,
		runner:      runner,
		keys:        defaultKeyMap(),
		help:        helpModel,
		viewport:    viewportModel,
	}
}

func (m Model) Init() tea.Cmd {
	return loadSnapshotCmd(m.loader)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncLayout()
		return m, nil
	case snapshotMsg:
		m.loading = false
		if msg.err != nil {
			m.errMessage = msg.err.Error()
		} else {
			m.errMessage = ""
			m.snapshot = msg.snapshot
		}
		m.syncLayout()
		if m.autoRefresh {
			m.autoRefreshID++
			return m, autoRefreshCmd(m.autoRefreshID)
		}
		return m, nil
	case actionMsg:
		m.completeAction(msg)
		m.syncLayout()
		if msg.report.Refresh || lifecycleAction(msg.action.ID) {
			m.loading = true
			return m, loadSnapshotCmd(m.loader)
		}
		return m, nil
	case autoRefreshMsg:
		if !m.autoRefresh || msg.id != m.autoRefreshID || m.runningAction != nil {
			return m, nil
		}
		m.loading = true
		return m, loadSnapshotCmd(m.loader)
	case tea.KeyPressMsg:
		if m.confirmation != nil {
			switch {
			case key.Matches(msg, m.keys.Confirm):
				return m.beginAction(m.confirmation.Action)
			case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Quit):
				m.cancelConfirmation()
				m.syncLayout()
				return m, nil
			default:
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Action):
			if m.runner == nil || m.runningAction != nil {
				return m, nil
			}
			index, ok := actionIndex(msg.Text)
			if !ok {
				return m, nil
			}
			actions := availableActions(m.snapshot)
			if index >= len(actions) {
				return m, nil
			}
			action := actions[index]
			if action.RequiresConfirmation() {
				m.confirmation = &confirmationState{Action: action}
				m.syncLayout()
				return m, nil
			}
			return m.beginAction(action)
		case key.Matches(msg, m.keys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
			m.syncLayout()
			return m, nil
		case key.Matches(msg, m.keys.ToggleSecrets):
			m.showSecrets = !m.showSecrets
			m.syncLayout()
			return m, nil
		case key.Matches(msg, m.keys.ToggleLayout):
			if m.layout == expandedLayout {
				m.layout = compactLayout
			} else {
				m.layout = expandedLayout
			}
			m.syncLayout()
			return m, nil
		case key.Matches(msg, m.keys.ToggleAutoRefresh):
			m.autoRefresh = !m.autoRefresh
			m.autoRefreshID++
			m.syncLayout()
			if m.autoRefresh {
				return m, autoRefreshCmd(m.autoRefreshID)
			}
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			if m.runningAction != nil {
				return m, nil
			}
			m.loading = true
			m.autoRefreshID++
			return m, loadSnapshotCmd(m.loader)
		case key.Matches(msg, m.keys.NextSection):
			m.active = nextSection(m.active)
			m.syncLayout()
			return m, nil
		case key.Matches(msg, m.keys.PrevSection):
			m.active = previousSection(m.active)
			m.syncLayout()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		view := tea.NewView("Loading stackctl tui...")
		view.AltScreen = true
		return view
	}

	header := renderHeader(m)
	status := renderGlobalStatus(m, m.width)
	confirmation := renderConfirmationModal(m, m.width)
	body := renderBody(m)
	footer := footerStyle().Width(m.width).Render(m.help.View(m.keys))

	blocks := []string{header}
	if status != "" {
		blocks = append(blocks, status)
	}
	if confirmation != "" {
		blocks = append(blocks, confirmation)
	}
	blocks = append(blocks, body, footer)

	view := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, blocks...))
	view.AltScreen = true
	return view
}

func loadSnapshotCmd(loader Loader) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := loader()
		return snapshotMsg{snapshot: snapshot, err: err}
	}
}

func autoRefreshCmd(id int) tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{id: id}
	})
}

func nextSection(current section) section {
	for idx, candidate := range sections {
		if candidate != current {
			continue
		}
		return sections[(idx+1)%len(sections)]
	}

	return overviewSection
}

func previousSection(current section) section {
	for idx, candidate := range sections {
		if candidate != current {
			continue
		}
		return sections[(idx-1+len(sections))%len(sections)]
	}

	return overviewSection
}

func (m *Model) syncLayout() {
	sidebarWidth := 24
	bodyHeight := m.height - 6
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	mainWidth := m.width - sidebarWidth - 3
	if mainWidth < 36 {
		mainWidth = 36
	}

	panelStyle := mainPanelStyle()
	m.viewport.SetWidth(maxInt(20, mainWidth-panelStyle.GetHorizontalFrameSize()))
	m.viewport.SetHeight(maxInt(4, bodyHeight-panelStyle.GetVerticalFrameSize()))
	m.viewport.SetContent(m.currentContent())
	m.viewport.GotoTop()
}

func (m Model) currentContent() string {
	if m.errMessage != "" && m.snapshot.LoadedAt.IsZero() {
		return renderErrorState(m.errMessage)
	}

	blocks := make([]string, 0, 4)
	switch m.active {
	case overviewSection:
		blocks = append(blocks, renderOverview(m.snapshot, m.layout))
	case servicesSection:
		blocks = append(blocks, renderServices(m.snapshot, m.showSecrets, m.layout))
	case healthSection:
		blocks = append(blocks, renderHealth(m.snapshot))
	case connectionsSection:
		blocks = append(blocks, renderConnections(m.snapshot, m.showSecrets, m.layout))
	case historySection:
		blocks = append(blocks, renderHistory(m.history))
	default:
		return ""
	}

	return strings.Join(blocks, "\n\n")
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("24")).
		Padding(0, 1)
}

func subtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
}

func headerMetaStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))
}

func sidebarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 1)
}

func navItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("251"))
}

func activeNavItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("31")).
		Bold(true).
		Padding(0, 1)
}

func mainPanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("31")).
		Padding(1, 1)
}

func sectionTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("80"))
}

func subsectionTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("117"))
}

func mutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))
}

func errorBannerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("160")).
		Padding(0, 1)
}

func footerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Padding(0, 1)
}

func renderGlobalStatus(m Model, width int) string {
	if m.banner == nil || strings.TrimSpace(m.banner.Message) == "" {
		return ""
	}

	contentWidth := maxInt(20, width-2)
	return bannerStyle(m.banner.Status).Width(contentWidth).Render(m.banner.Message)
}

func renderConfirmationModal(m Model, width int) string {
	if m.confirmation == nil {
		return ""
	}

	modal := renderConfirmation(m.confirmation)
	modalWidth := minInt(maxInt(60, lipgloss.Width(modal)), maxInt(60, width-4))
	return lipgloss.Place(
		width,
		lipgloss.Height(modal),
		lipgloss.Center,
		lipgloss.Top,
		lipgloss.NewStyle().Width(modalWidth).Render(modal),
	)
}

func renderHeader(m Model) string {
	statusLabel := "Ready"
	switch {
	case m.runningAction != nil:
		statusLabel = "Running " + m.runningAction.Action.Label
	case m.confirmation != nil:
		statusLabel = "Awaiting confirmation"
	case m.loading:
		statusLabel = "Refreshing"
	}

	mode := "external"
	if m.snapshot.Managed {
		mode = "managed"
	}

	loadedAt := "not loaded yet"
	if !m.snapshot.LoadedAt.IsZero() {
		loadedAt = m.snapshot.LoadedAt.Format("2006-01-02 15:04:05")
	}

	stackName := m.snapshot.StackName
	if stackName == "" {
		stackName = "stackctl"
	}

	autoRefreshLabel := "off"
	if m.autoRefresh {
		autoRefreshLabel = autoRefreshInterval.String()
	}

	meta := fmt.Sprintf(
		"%s  •  mode: %s  •  layout: %s  •  auto-refresh: %s  •  secrets: %s  •  updated: %s",
		statusLabel,
		mode,
		m.layout.String(),
		autoRefreshLabel,
		onOffLabel(m.showSecrets),
		loadedAt,
	)

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle().Render("stackctl tui")+" "+subtitleStyle().Render(stackName),
		headerMetaStyle().Render(meta),
	)

	if m.errMessage == "" {
		return header
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, errorBannerStyle().Render(m.errMessage))
}

func renderBody(m Model) string {
	sidebarWidth := 24
	bodyHeight := m.height - 6
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	mainWidth := m.width - sidebarWidth - 3
	if mainWidth < 36 {
		mainWidth = 36
	}

	sidebar := sidebarStyle().Width(sidebarWidth).Height(bodyHeight).Render(renderSidebar(m))
	main := mainPanelStyle().Width(mainWidth).Height(bodyHeight).Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

func renderSidebar(m Model) string {
	lines := []string{sectionTitleStyle().Render("Sections"), ""}
	for _, candidate := range sections {
		label := candidate.Title()
		if candidate == m.active {
			lines = append(lines, activeNavItemStyle().Render("▸ "+label))
			continue
		}
		lines = append(lines, navItemStyle().Render("  "+label))
	}

	lines = append(lines, "")
	if actionRail := renderActionRail(m); actionRail != "" {
		lines = append(lines, actionRail)
	} else if m.runner == nil {
		lines = append(lines, mutedStyle().Render("Read-only dashboard"))
	}

	return strings.Join(lines, "\n")
}

func renderOverview(snapshot Snapshot, layout layoutMode) string {
	running, total := runningStackServiceCount(snapshot.Services)
	lines := []string{
		sectionTitleStyle().Render("Overview"),
		"",
		renderOverviewSummary(snapshot.Services),
		"",
		subsectionTitleStyle().Render("Stack"),
		fmt.Sprintf("  Name: %s", emptyLabel(snapshot.StackName)),
		fmt.Sprintf("  Mode: %s", overviewModeLabel(snapshot.Managed)),
		fmt.Sprintf("  Config: %s", emptyLabel(snapshot.ConfigPath)),
		"",
		subsectionTitleStyle().Render("Runtime"),
		fmt.Sprintf("  Stack services: %d / %d running", running, total),
		fmt.Sprintf("  Wait on start: %s", onOffLabel(snapshot.WaitForServices)),
	}
	if host := overviewHost(snapshot.Services); host != "" {
		lines = append(lines, fmt.Sprintf("  Host: %s", host))
	}
	if ports := overviewPortSummary(snapshot.Services); ports != "" {
		lines = append(lines, fmt.Sprintf("  Ports: %s", ports))
	}
	if layout == expandedLayout {
		lines = append(lines,
			"",
			subsectionTitleStyle().Render("Paths"),
			fmt.Sprintf("  Stack dir: %s", emptyLabel(snapshot.StackDir)),
			fmt.Sprintf("  Compose: %s", emptyLabel(snapshot.ComposePath)),
			fmt.Sprintf("  Startup timeout: %ds", snapshot.StartupTimeoutSec),
		)
	}
	lines = append(lines, "")
	lines = append(lines, subsectionTitleStyle().Render("Helpful commands"))
	lines = append(lines, "  "+overviewCommandHints(snapshot.Services))
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render(renderCopyHint(snapshot, overviewSection)))

	return strings.Join(lines, "\n")
}

func renderServices(snapshot Snapshot, showSecrets bool, layout layoutMode) string {
	lines := []string{sectionTitleStyle().Render("Services"), ""}
	if snapshot.ServiceError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.ServiceError), "")
	}
	if len(snapshot.Services) == 0 {
		lines = append(lines, mutedStyle().Render("No services loaded."))
		return strings.Join(lines, "\n")
	}

	for idx, service := range snapshot.Services {
		status := displayServiceStatus(service)
		lines = append(lines, renderServiceHeading(status, service.DisplayName))
		lines = append(lines, renderStatusLine(status))
		if layout == expandedLayout && service.ContainerName != "" {
			lines = append(lines, fmt.Sprintf("Container: %s", service.ContainerName))
		}
		if layout == expandedLayout && service.Image != "" {
			lines = append(lines, fmt.Sprintf("Image: %s", service.Image))
		}
		if layout == expandedLayout && service.DataVolume != "" {
			lines = append(lines, fmt.Sprintf("Data volume: %s", service.DataVolume))
		}
		if service.Host != "" {
			lines = append(lines, fmt.Sprintf("Host: %s", service.Host))
		}
		lines = append(lines, servicePortLines(service)...)
		if service.Database != "" {
			lines = append(lines, fmt.Sprintf("Database: %s", service.Database))
		}
		if layout == expandedLayout && service.MaintenanceDB != "" {
			lines = append(lines, fmt.Sprintf("Maintenance DB: %s", service.MaintenanceDB))
		}
		if service.Email != "" {
			lines = append(lines, fmt.Sprintf("Email: %s", service.Email))
		}
		if service.Username != "" {
			lines = append(lines, fmt.Sprintf("Username: %s", service.Username))
		}
		if layout == expandedLayout && service.Password != "" {
			lines = append(lines, fmt.Sprintf("Password: %s", maskSecret(service.Password, showSecrets)))
		}
		if layout == expandedLayout && service.AppendOnly != nil {
			lines = append(lines, fmt.Sprintf("Appendonly: %s", enabledDisabled(*service.AppendOnly)))
		}
		if layout == expandedLayout && service.SavePolicy != "" {
			lines = append(lines, fmt.Sprintf("Save policy: %s", service.SavePolicy))
		}
		if layout == expandedLayout && service.MaxMemoryPolicy != "" {
			lines = append(lines, fmt.Sprintf("Maxmemory policy: %s", service.MaxMemoryPolicy))
		}
		if layout == expandedLayout && service.ServerMode != "" {
			lines = append(lines, fmt.Sprintf("Server mode: %s", service.ServerMode))
		}
		if service.URL != "" {
			lines = append(lines, fmt.Sprintf("URL: %s", service.URL))
		}
		if service.DSN != "" {
			lines = append(lines, fmt.Sprintf("DSN: %s", maskConnectionValue(service.DSN, showSecrets)))
		}
		if idx < len(snapshot.Services)-1 {
			lines = append(lines, "")
		}
	}
	lines = append(lines, mutedStyle().Render(renderCopyHint(snapshot, servicesSection)))

	return strings.Join(lines, "\n")
}

func renderHealth(snapshot Snapshot) string {
	lines := []string{sectionTitleStyle().Render("Health"), ""}
	if snapshot.HealthError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.HealthError), "")
	}
	if len(snapshot.Services) > 0 {
		lines = append(lines, renderHealthSummary(snapshot.Services))
		lines = append(lines, "")
		for idx, service := range snapshot.Services {
			lines = append(lines, renderHealthBlock(service)...)
			if idx < len(snapshot.Services)-1 {
				lines = append(lines, "")
			}
		}
		return strings.Join(lines, "\n")
	}
	if len(snapshot.Health) == 0 {
		lines = append(lines, mutedStyle().Render("No health data loaded."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, mutedStyle().Render("Live service health is unavailable; showing raw checks instead."))
	lines = append(lines, "")
	for _, line := range snapshot.Health {
		lines = append(lines, healthLineStyle(line.Status).Render(fmt.Sprintf("%s %s", healthStatusIcon(line.Status), line.Message)))
	}

	return strings.Join(lines, "\n")
}

func renderConnections(snapshot Snapshot, showSecrets bool, layout layoutMode) string {
	lines := []string{sectionTitleStyle().Render("Connections"), ""}
	if len(snapshot.Connections) == 0 {
		lines = append(lines, mutedStyle().Render("No connection info loaded."))
		return strings.Join(lines, "\n")
	}

	for idx, entry := range snapshot.Connections {
		if layout == compactLayout {
			lines = append(lines, fmt.Sprintf("%s: %s", entry.Name, maskConnectionValue(entry.Value, showSecrets)))
		} else {
			lines = append(lines, entry.Name)
			lines = append(lines, "  "+maskConnectionValue(entry.Value, showSecrets))
		}
		if idx < len(snapshot.Connections)-1 {
			lines = append(lines, "")
		}
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render(renderCopyHint(snapshot, connectionsSection)))

	return strings.Join(lines, "\n")
}

func renderErrorState(message string) string {
	return strings.Join([]string{
		sectionTitleStyle().Render("Dashboard unavailable"),
		"",
		errorBannerStyle().Render(message),
		"",
		"Fix the configuration issue and press r to retry, or quit with q.",
	}, "\n")
}

func runningStackServiceCount(services []Service) (int, int) {
	running := 0
	total := 0
	for _, service := range services {
		if !isStackService(service) {
			continue
		}
		total++
		if strings.EqualFold(displayServiceStatus(service), "running") {
			running++
		}
	}

	return running, total
}

func lookupService(services []Service, displayName string) (Service, bool) {
	for _, service := range services {
		if service.DisplayName == displayName {
			return service, true
		}
	}

	return Service{}, false
}

func isStackService(service Service) bool {
	return strings.TrimSpace(service.ContainerName) != ""
}

func displayServiceStatus(service Service) string {
	status := strings.TrimSpace(strings.ToLower(service.Status))
	switch {
	case status == "" && isStackService(service):
		return "not running"
	case status == "missing" && isStackService(service):
		return "not running"
	case status == "":
		return "-"
	default:
		return status
	}
}

func servicePortLines(service Service) []string {
	lines := make([]string, 0, 2)
	if service.ExternalPort > 0 {
		lines = append(lines, fmt.Sprintf("Host port: %d", service.ExternalPort))
	}
	if service.InternalPort > 0 {
		lines = append(lines, fmt.Sprintf("Container port: %d", service.InternalPort))
	}

	return lines
}

func renderServiceHeading(status, displayName string) string {
	return statusStyle(status).Render(fmt.Sprintf("%s  %s", serviceStatusBadge(status), displayName))
}

func renderStatusLine(status string) string {
	return statusStyle(status).Render(fmt.Sprintf("Status: %s", emptyLabel(status)))
}

func renderStatusSummaryLine(label, status string) string {
	return statusStyle(status).Render(fmt.Sprintf("%s: %s", label, emptyLabel(status)))
}

func renderCopyHint(snapshot Snapshot, active section) string {
	hints := copyHintTargets(snapshot, active)
	if len(hints) == 0 {
		return "Copy placeholders: no DSNs or URLs available yet."
	}

	return "Copy placeholders: " + strings.Join(hints, "  •  ")
}

func renderOverviewSummary(services []Service) string {
	running := 0
	stopped := 0
	attention := 0

	for _, service := range services {
		if !isStackService(service) {
			continue
		}
		switch displayServiceStatus(service) {
		case "running":
			running++
		case "stopped", "not running", "missing":
			stopped++
		default:
			attention++
		}
	}

	parts := []string{
		statusStyle("healthy").Render(fmt.Sprintf("Running: %d", running)),
		statusStyle("not running").Render(fmt.Sprintf("Stopped: %d", stopped)),
		statusStyle("warning").Render(fmt.Sprintf("Attention: %d", attention)),
	}
	if cockpit, ok := lookupService(services, "Cockpit"); ok {
		parts = append(parts, renderStatusSummaryLine("Cockpit", displayServiceStatus(cockpit)))
	}

	return strings.Join(parts, "  ")
}

func overviewModeLabel(managed bool) string {
	if managed {
		return "managed"
	}

	return "external"
}

func overviewHost(services []Service) string {
	for _, service := range services {
		if strings.TrimSpace(service.Host) != "" {
			return service.Host
		}
	}

	return ""
}

func overviewPortSummary(services []Service) string {
	parts := make([]string, 0, len(services))
	for _, service := range services {
		if service.ExternalPort <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", service.DisplayName, service.ExternalPort))
	}

	return strings.Join(parts, "  •  ")
}

func overviewCommandHints(services []Service) string {
	running, _ := runningStackServiceCount(services)
	if running > 0 {
		return "stackctl services  •  stackctl health  •  stackctl connect"
	}

	return "stackctl start  •  stackctl services  •  stackctl health"
}

func renderHealthSummary(services []Service) string {
	healthy := 0
	warning := 0
	notRunning := 0

	for _, service := range services {
		switch classifyServiceHealth(service) {
		case output.StatusOK:
			healthy++
		case "not running":
			notRunning++
		default:
			warning++
		}
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusStyle(output.StatusOK).Render(fmt.Sprintf("Healthy: %d", healthy)),
		"  ",
		statusStyle(output.StatusWarn).Render(fmt.Sprintf("Warnings: %d", warning)),
		"  ",
		statusStyle("not running").Render(fmt.Sprintf("Not running: %d", notRunning)),
	)
}

func renderHealthBlock(service Service) []string {
	lines := make([]string, 0, 8)
	status := classifyServiceHealth(service)
	lines = append(lines, renderServiceHeading(status, service.DisplayName))
	lines = append(lines, renderStatusLine(healthStatusLabel(service)))
	lines = append(lines, fmt.Sprintf("Runtime: %s", emptyLabel(displayServiceStatus(service))))

	reachability := healthReachabilityLabel(service)
	if reachability != "" {
		lines = append(lines, fmt.Sprintf("Reachability: %s", reachability))
	}
	if note := healthNote(service); note != "" {
		lines = append(lines, mutedStyle().Render(note))
	}
	if service.URL != "" {
		lines = append(lines, fmt.Sprintf("URL: %s", service.URL))
	}

	return lines
}

func classifyServiceHealth(service Service) string {
	status := displayServiceStatus(service)
	switch {
	case strings.EqualFold(status, "running") && serviceHasReachablePort(service):
		return output.StatusOK
	case transitionalServiceStatus(status):
		return output.StatusWarn
	case strings.EqualFold(status, "running"):
		return output.StatusWarn
	case service.PortListening:
		return output.StatusWarn
	default:
		return "not running"
	}
}

func healthStatusLabel(service Service) string {
	switch classifyServiceHealth(service) {
	case output.StatusOK:
		return "healthy"
	case "not running":
		if strings.EqualFold(displayServiceStatus(service), "missing") {
			return "not installed"
		}
		return "not running"
	default:
		if transitionalServiceStatus(displayServiceStatus(service)) {
			return "changing"
		}
		return "needs attention"
	}
}

func healthReachabilityLabel(service Service) string {
	if service.ExternalPort <= 0 {
		return ""
	}

	target := fmt.Sprintf("%s:%d", emptyLabel(service.Host), service.ExternalPort)
	if service.PortListening {
		return target + " is accepting connections"
	}

	return target + " is not responding"
}

func healthNote(service Service) string {
	status := displayServiceStatus(service)
	switch {
	case transitionalServiceStatus(status):
		return "This service is changing state. Wait for the action to finish, then refresh."
	case strings.EqualFold(status, "running") && !serviceHasReachablePort(service):
		return "The service reports running, but its host port is not reachable yet."
	case !strings.EqualFold(status, "running") && service.PortListening:
		return "The host port is active even though this service is not running. Another process may be using it."
	case strings.EqualFold(status, "missing"):
		if isStackService(service) {
			return "The managed container is not present yet."
		}
		return "This service is not installed."
	default:
		return ""
	}
}

func serviceHasReachablePort(service Service) bool {
	if service.ExternalPort <= 0 {
		return strings.EqualFold(displayServiceStatus(service), "running")
	}

	return service.PortListening
}

func copyHintTargets(snapshot Snapshot, active section) []string {
	targets := make([]string, 0, 4)
	seen := make(map[string]struct{})

	add := func(label string) {
		if strings.TrimSpace(label) == "" {
			return
		}
		if _, ok := seen[label]; ok {
			return
		}
		seen[label] = struct{}{}
		targets = append(targets, label)
	}

	switch active {
	case servicesSection:
		for _, service := range snapshot.Services {
			if service.DSN != "" {
				add(service.DisplayName + " DSN")
			}
			if service.URL != "" {
				add(service.DisplayName + " URL")
			}
		}
	case overviewSection, connectionsSection:
		for _, entry := range snapshot.Connections {
			add(entry.Name)
		}
	}

	return targets
}

func serviceStatusBadge(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "ok", "healthy":
		return "●"
	case "starting", "stopping", "restarting", "info":
		return "◐"
	case "stopped", "not running", "not installed", "error", "warn", "warning", "fail", "miss":
		return "○"
	default:
		return "◌"
	}
}

func statusStyle(status string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "ok", "healthy":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	case "starting", "stopping", "restarting", "info", "health":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	case "warning", "warn":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Bold(true)
	case "stopped", "not running", "missing", "not installed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case "error", "fail", "miss":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	}
}

func healthLineStyle(status string) lipgloss.Style {
	return statusStyle(status)
}

func healthStatusIcon(status string) string {
	switch status {
	case output.StatusOK:
		return "✅"
	case output.StatusWarn:
		return "⚠️"
	case output.StatusFail, output.StatusMiss:
		return "❌"
	default:
		return "•"
	}
}

func maskConnectionValue(value string, showSecrets bool) string {
	if showSecrets || strings.TrimSpace(value) == "" {
		return value
	}

	parsed, err := url.Parse(value)
	if err == nil && parsed.User != nil {
		if password, ok := parsed.User.Password(); ok && password != "" {
			maskedUser := parsed.User.Username()
			if maskedUser != "" {
				maskedUser += ":" + maskedSecret
			} else {
				maskedUser = ":" + maskedSecret
			}
			return strings.Replace(value, parsed.User.String(), maskedUser, 1)
		}
	}

	return value
}

func maskSecret(value string, showSecrets bool) string {
	if showSecrets || strings.TrimSpace(value) == "" {
		return value
	}

	return maskedSecret
}

func emptyLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

func onOffLabel(value bool) string {
	if value {
		return "on"
	}

	return "off"
}

func enabledDisabled(value bool) string {
	if value {
		return "enabled"
	}

	return "disabled"
}

func transitionalServiceStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "starting", "stopping", "restarting":
		return true
	default:
		return false
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
