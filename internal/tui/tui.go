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

const transientBannerDuration = 4 * time.Second

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
	DoctorError       string
	Services          []Service
	Health            []HealthLine
	DoctorSummary     DoctorSummary
	DoctorChecks      []DoctorCheck
	Connections       []Connection
}

type Service struct {
	Name            string
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

type LogWatchRequest struct {
	Service string
}

type LogWatchLauncher func(LogWatchRequest) (tea.ExecCommand, error)

type section int

const (
	overviewSection section = iota
	servicesSection
	portsSection
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
	portsSection,
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
	case portsSection:
		return "Ports"
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

type logWatchDoneMsg struct {
	Service string
	Err     error
}

type keyMap struct {
	NextSection       key.Binding
	PrevSection       key.Binding
	Action            key.Binding
	Confirm           key.Binding
	Cancel            key.Binding
	Refresh           key.Binding
	PrevItem          key.Binding
	NextItem          key.Binding
	WatchLogs         key.Binding
	ToggleAutoRefresh key.Binding
	ToggleLayout      key.Binding
	ToggleSecrets     key.Binding
	ToggleHelp        key.Binding
	Quit              key.Binding
}

type helpBindings struct {
	short []key.Binding
	full  [][]key.Binding
}

func (h helpBindings) ShortHelp() []key.Binding {
	return h.short
}

func (h helpBindings) FullHelp() [][]key.Binding {
	return h.full
}

func defaultKeyMap() keyMap {
	return keyMap{
		NextSection: key.NewBinding(
			key.WithKeys("right", "tab", "l"),
			key.WithHelp("tab/l", "next section"),
		),
		PrevSection: key.NewBinding(
			key.WithKeys("left", "shift+tab", "h"),
			key.WithHelp("shift+tab/h", "previous section"),
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
		PrevItem: key.NewBinding(
			key.WithKeys("up", "k", "["),
			key.WithHelp("k/[", "previous item"),
		),
		NextItem: key.NewBinding(
			key.WithKeys("down", "j", "]"),
			key.WithHelp("j/]", "next item"),
		),
		WatchLogs: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "watch logs"),
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

type Model struct {
	width            int
	height           int
	active           section
	layout           layoutMode
	loading          bool
	autoRefresh      bool
	autoRefreshID    int
	showSecrets      bool
	errMessage       string
	snapshot         Snapshot
	loader           Loader
	logWatchLauncher LogWatchLauncher
	runner           ActionRunner
	keys             keyMap
	help             help.Model
	viewport         viewport.Model
	banner           *actionBanner
	confirmation     *confirmationState
	runningAction    *runningAction
	history          []historyEntry
	nextHistoryID    int
	nextBannerID     int
	selectedService  string
	selectedPort     string
	selectedHealth   string
}

func NewModel(loader Loader) Model {
	return newModel(loader, nil, nil)
}

func NewActionModel(loader Loader, runner ActionRunner) Model {
	return newModel(loader, nil, runner)
}

func NewInspectionModel(loader Loader, logWatchLauncher LogWatchLauncher, runner ActionRunner) Model {
	return newModel(loader, logWatchLauncher, runner)
}

func newModel(loader Loader, logWatchLauncher LogWatchLauncher, runner ActionRunner) Model {
	viewportModel := viewport.New()
	helpModel := help.New()
	helpModel.ShowAll = false

	return Model{
		active:           overviewSection,
		layout:           expandedLayout,
		loading:          true,
		autoRefresh:      true,
		loader:           loader,
		logWatchLauncher: logWatchLauncher,
		runner:           runner,
		keys:             defaultKeyMap(),
		help:             helpModel,
		viewport:         viewportModel,
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
			m.normalizeSelections()
		}
		m.syncLayout()
		if m.autoRefresh {
			m.autoRefreshID++
			return m, autoRefreshCmd(m.autoRefreshID, m.refreshInterval())
		}
		return m, nil
	case logWatchDoneMsg:
		m.loading = true
		m.autoRefreshID++
		cmds := []tea.Cmd{loadSnapshotCmd(m.loader)}
		if msg.Err != nil {
			bannerID := m.setBanner(output.StatusWarn, watchLogsErrorMessage(msg.Service, msg.Err))
			cmds = append(cmds, clearBannerCmd(bannerID))
		}
		m.syncLayout()
		if m.autoRefresh {
			cmds = append(cmds, autoRefreshCmd(m.autoRefreshID, m.refreshInterval()))
		}
		return m, tea.Batch(cmds...)
	case actionMsg:
		bannerCmd := m.completeAction(msg)
		m.syncLayout()
		if msg.report.Refresh || lifecycleAction(msg.action.ID) {
			m.loading = true
			return m, tea.Batch(loadSnapshotCmd(m.loader), bannerCmd)
		}
		return m, bannerCmd
	case bannerClearMsg:
		if m.banner != nil && m.banner.ID == msg.id {
			m.banner = nil
			m.syncLayout()
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
				clearCmd := m.cancelConfirmation()
				m.syncLayout()
				return m, clearCmd
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
				return m, autoRefreshCmd(m.autoRefreshID, m.refreshInterval())
			}
			return m, nil
		case key.Matches(msg, m.keys.WatchLogs):
			if m.logWatchLauncher == nil || m.runningAction != nil {
				return m, nil
			}
			cmd := m.startSelectedLogWatch()
			m.syncLayout()
			return m, cmd
		case key.Matches(msg, m.keys.Refresh):
			if m.runningAction != nil {
				return m, nil
			}
			m.loading = true
			m.autoRefreshID++
			return m, loadSnapshotCmd(m.loader)
		case key.Matches(msg, m.keys.NextItem):
			if !m.activeHasSelectionList() {
				return m, nil
			}
			cmd := m.cycleActiveSelection(1)
			m.autoRefreshID++
			m.syncLayout()
			m.resetViewportForActivePanel()
			if m.autoRefresh {
				return m, tea.Batch(cmd, autoRefreshCmd(m.autoRefreshID, m.refreshInterval()))
			}
			return m, cmd
		case key.Matches(msg, m.keys.PrevItem):
			if !m.activeHasSelectionList() {
				return m, nil
			}
			cmd := m.cycleActiveSelection(-1)
			m.autoRefreshID++
			m.syncLayout()
			m.resetViewportForActivePanel()
			if m.autoRefresh {
				return m, tea.Batch(cmd, autoRefreshCmd(m.autoRefreshID, m.refreshInterval()))
			}
			return m, cmd
		case key.Matches(msg, m.keys.NextSection):
			m.active = nextSection(m.active)
			m.autoRefreshID++
			m.syncLayout()
			m.resetViewportForActivePanel()
			if m.autoRefresh {
				return m, autoRefreshCmd(m.autoRefreshID, m.refreshInterval())
			}
			return m, nil
		case key.Matches(msg, m.keys.PrevSection):
			m.active = previousSection(m.active)
			m.autoRefreshID++
			m.syncLayout()
			m.resetViewportForActivePanel()
			if m.autoRefresh {
				return m, autoRefreshCmd(m.autoRefreshID, m.refreshInterval())
			}
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
	body := renderBody(m)
	footer := footerStyle().Width(m.width).Render(m.help.View(m.helpBindings()))

	blocks := []string{header}
	if status != "" {
		blocks = append(blocks, status)
	}
	blocks = append(blocks, body, footer)

	view := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, blocks...))
	view.AltScreen = true
	return view
}

func (m Model) helpBindings() helpBindings {
	short := []key.Binding{m.keys.NextSection}
	if m.runner != nil {
		short = append(short, m.keys.Action)
	}
	if m.activeHasSelectionList() {
		short = append(short, m.keys.NextItem)
	}
	if m.showWatchLogsHelp() {
		short = append(short, m.keys.WatchLogs)
	}
	short = append(short, m.keys.Refresh, m.keys.Quit)

	row1 := []key.Binding{m.keys.NextSection, m.keys.PrevSection}
	if m.runner != nil {
		row1 = append(row1, m.keys.Action)
	}

	row2 := []key.Binding{m.keys.Confirm, m.keys.Cancel, m.keys.Refresh}

	row3 := []key.Binding{}
	if m.activeHasSelectionList() {
		row3 = append(row3, m.keys.PrevItem, m.keys.NextItem)
	}
	if m.showWatchLogsHelp() {
		row3 = append(row3, m.keys.WatchLogs)
	}

	full := [][]key.Binding{row1, row2}
	if len(row3) > 0 {
		full = append(full, row3)
	}
	full = append(full,
		[]key.Binding{m.keys.ToggleAutoRefresh, m.keys.ToggleLayout, m.keys.ToggleSecrets},
		[]key.Binding{m.keys.ToggleHelp, m.keys.Quit},
	)

	return helpBindings{short: short, full: full}
}

func loadSnapshotCmd(loader Loader) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := loader()
		return snapshotMsg{snapshot: snapshot, err: err}
	}
}

func watchLogsCmd(launcher LogWatchLauncher, request LogWatchRequest, displayName string) tea.Cmd {
	execCmd, err := launcher(request)
	if err != nil {
		return func() tea.Msg {
			return logWatchDoneMsg{Service: displayName, Err: err}
		}
	}

	return tea.Exec(execCmd, func(err error) tea.Msg {
		return logWatchDoneMsg{Service: displayName, Err: err}
	})
}

func autoRefreshCmd(id int, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return autoRefreshMsg{id: id}
	})
}

func clearBannerCmd(id int) tea.Cmd {
	return tea.Tick(transientBannerDuration, func(time.Time) tea.Msg {
		return bannerClearMsg{id: id}
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

func (m Model) refreshInterval() time.Duration {
	return autoRefreshInterval
}

func (m *Model) normalizeSelections() {
	m.selectedService = pickSelectedName(m.selectedService, selectableServiceNames(m.snapshot))
	m.selectedPort = pickSelectedName(m.selectedPort, selectablePortNames(m.snapshot))
	m.selectedHealth = pickSelectedName(m.selectedHealth, selectableServiceNames(m.snapshot))
}

func (m Model) activeHasSelectionList() bool {
	switch m.active {
	case servicesSection, portsSection, healthSection:
		return true
	default:
		return false
	}
}

func (m *Model) resetViewportForActivePanel() {
	m.viewport.GotoTop()
}

func (m *Model) cycleActiveSelection(step int) tea.Cmd {
	switch m.active {
	case servicesSection:
		m.selectedService = cycleSelectedName(m.selectedService, selectableServiceNames(m.snapshot), step)
		return nil
	case portsSection:
		m.selectedPort = cycleSelectedName(m.selectedPort, selectablePortNames(m.snapshot), step)
		return nil
	case healthSection:
		m.selectedHealth = cycleSelectedName(m.selectedHealth, selectableServiceNames(m.snapshot), step)
		return nil
	}

	return nil
}

func (m *Model) startSelectedLogWatch() tea.Cmd {
	service, ok := m.selectedLogWatchService()
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "select a service, port, or health target to watch logs")
		return clearBannerCmd(bannerID)
	}
	if !isStackService(service) {
		bannerID := m.setBanner(output.StatusWarn, "live logs are unavailable for host tools")
		return clearBannerCmd(bannerID)
	}

	return watchLogsCmd(
		m.logWatchLauncher,
		LogWatchRequest{Service: logWatchServiceName(service)},
		service.DisplayName,
	)
}

func (m Model) selectedLogWatchService() (Service, bool) {
	switch m.active {
	case servicesSection:
		return selectedService(m.snapshot, m.selectedService)
	case portsSection:
		return selectedPortService(m.snapshot, m.selectedPort)
	case healthSection:
		return selectedService(m.snapshot, m.selectedHealth)
	default:
		return Service{}, false
	}
}

func (m Model) showWatchLogsHelp() bool {
	service, ok := m.selectedLogWatchService()
	return ok && isStackService(service)
}

func logWatchServiceName(service Service) string {
	if strings.TrimSpace(service.Name) != "" {
		return strings.TrimSpace(service.Name)
	}

	return serviceKey(service)
}

func watchLogsErrorMessage(service string, err error) string {
	label := strings.TrimSpace(service)
	if label == "" {
		label = "selected service"
	}

	return fmt.Sprintf("watch logs for %s failed: %v", strings.ToLower(label), err)
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
	sidebarWidth := 26
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
}

func (m Model) currentContent() string {
	if m.errMessage != "" && m.snapshot.LoadedAt.IsZero() {
		return renderErrorState(m.errMessage)
	}

	blocks := make([]string, 0, 4)
	contentWidth := maxInt(20, m.viewport.Width())
	switch m.active {
	case overviewSection:
		blocks = append(blocks, renderOverview(m.snapshot, m.layout))
	case servicesSection:
		blocks = append(blocks, renderServices(m.snapshot, m.showSecrets, m.layout, m.selectedService, contentWidth))
	case portsSection:
		blocks = append(blocks, renderPorts(m.snapshot, m.selectedPort, contentWidth))
	case healthSection:
		blocks = append(blocks, renderHealth(m.snapshot, m.selectedHealth, contentWidth))
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

func headerStatusStyle(m Model) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true)

	switch {
	case m.runningAction != nil:
		return style.Foreground(lipgloss.Color("81"))
	case m.confirmation != nil:
		return style.Foreground(lipgloss.Color("221"))
	case m.loading:
		return style.Foreground(lipgloss.Color("117"))
	default:
		return style.Foreground(lipgloss.Color("78"))
	}
}

func headerShellStyle() lipgloss.Style {
	return lipgloss.NewStyle().PaddingLeft(1)
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

	contentWidth := maxInt(20, width-3)
	return headerShellStyle().Render(
		bannerStyle(m.banner.Status).Width(contentWidth).Render(m.banner.Message),
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
		autoRefreshLabel = m.refreshInterval().String()
	}

	meta := fmt.Sprintf(
		"%s  •  mode: %s  •  layout: %s  •  auto-refresh: %s  •  secrets: %s  •  updated: %s",
		headerStatusStyle(m).Render(statusLabel),
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
	header = headerShellStyle().Render(header)

	if m.errMessage == "" {
		return header
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, errorBannerStyle().Render(m.errMessage))
}

func renderBody(m Model) string {
	sidebarWidth := 26
	bodyHeight := m.height - 6
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	mainWidth := m.width - sidebarWidth - 3
	if mainWidth < 36 {
		mainWidth = 36
	}
	panelStyle := mainPanelStyle()
	mainInnerWidth := maxInt(20, mainWidth-panelStyle.GetHorizontalFrameSize())
	mainInnerHeight := maxInt(4, bodyHeight-panelStyle.GetVerticalFrameSize())

	sidebar := sidebarStyle().Width(sidebarWidth).Height(bodyHeight).Render(renderSidebar(m))
	mainContent := m.viewport.View()
	if m.confirmation != nil {
		mainContent = renderConfirmationPanel(m.confirmation, mainInnerWidth, mainInnerHeight)
	}
	main := panelStyle.Width(mainWidth).Height(bodyHeight).Render(mainContent)

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

func renderConfirmationPanel(state *confirmationState, width, height int) string {
	if state == nil {
		return ""
	}

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		renderConfirmation(state),
	)
}

func renderOverview(snapshot Snapshot, layout layoutMode) string {
	stackServices, hostTools := splitServices(snapshot.Services)
	running, total := runningStackServiceCount(stackServices)
	lines := []string{
		sectionTitleStyle().Render("Overview"),
		"",
		renderOverviewSummary(stackServices),
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
	if host := overviewHost(stackServices); host != "" {
		lines = append(lines, fmt.Sprintf("  Host: %s", host))
	}
	if ports := overviewPortSummary(stackServices); ports != "" {
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
	if len(hostTools) > 0 {
		lines = append(lines, subsectionTitleStyle().Render("Host tools"))
		lines = append(lines, mutedStyle().Render("External to stack start, stop, and restart."))
		for _, service := range hostTools {
			lines = append(lines, "  "+renderStatusSummaryLine(service.DisplayName, displayServiceStatus(service)))
			if service.URL != "" {
				lines = append(lines, fmt.Sprintf("  URL: %s", service.URL))
			}
		}
		lines = append(lines, "")
	}
	lines = append(lines, subsectionTitleStyle().Render("Helpful commands"))
	lines = append(lines, "  "+overviewCommandHints(stackServices))

	return strings.Join(lines, "\n")
}

func renderServiceBlock(service Service, showSecrets bool, layout layoutMode, hostTool bool) []string {
	lines := make([]string, 0, 16)
	status := displayServiceStatus(service)
	lines = append(lines, renderServiceHeading(status, service.DisplayName))
	lines = append(lines, renderStatusLine(status))
	if hostTool {
		lines = append(lines, mutedStyle().Render("Lifecycle: external to stack lifecycle"))
	}
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

	return lines
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

func splitServices(services []Service) ([]Service, []Service) {
	stackServices := make([]Service, 0, len(services))
	hostTools := make([]Service, 0, len(services))
	for _, service := range services {
		if isStackService(service) {
			stackServices = append(stackServices, service)
			continue
		}
		hostTools = append(hostTools, service)
	}

	return stackServices, hostTools
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
		return "accepting on " + target
	}

	return "no response on " + target
}

func healthNote(service Service) string {
	status := displayServiceStatus(service)
	switch {
	case transitionalServiceStatus(status):
		return "Service is changing state. Refresh when the action finishes."
	case strings.EqualFold(status, "running") && !serviceHasReachablePort(service):
		return "Container is running, but the host port is not reachable yet."
	case !strings.EqualFold(status, "running") && service.PortListening:
		return "Host port is busy outside this stack."
	case strings.EqualFold(status, "missing"):
		if isStackService(service) {
			return "Managed container is not present yet."
		}
		return "Service is not installed."
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
	case portsSection:
		for _, service := range snapshot.Services {
			if service.ExternalPort > 0 {
				add(fmt.Sprintf("%s port %d", service.DisplayName, service.ExternalPort))
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
