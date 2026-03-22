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
)

var sections = []section{
	overviewSection,
	servicesSection,
	healthSection,
	connectionsSection,
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
	default:
		return "Unknown"
	}
}

type snapshotMsg struct {
	snapshot Snapshot
	err      error
}

type keyMap struct {
	NextSection   key.Binding
	PrevSection   key.Binding
	Refresh       key.Binding
	ToggleSecrets key.Binding
	ToggleHelp    key.Binding
	Quit          key.Binding
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
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
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
	return []key.Binding{k.NextSection, k.Refresh, k.ToggleSecrets, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextSection, k.PrevSection, k.Refresh},
		{k.ToggleSecrets, k.ToggleHelp, k.Quit},
	}
}

type Model struct {
	width       int
	height      int
	active      section
	loading     bool
	showSecrets bool
	errMessage  string
	snapshot    Snapshot
	loader      Loader
	keys        keyMap
	help        help.Model
	viewport    viewport.Model
}

func NewModel(loader Loader) Model {
	viewportModel := viewport.New()
	helpModel := help.New()
	helpModel.ShowAll = false

	return Model{
		active:   overviewSection,
		loading:  true,
		loader:   loader,
		keys:     defaultKeyMap(),
		help:     helpModel,
		viewport: viewportModel,
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
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
			m.syncLayout()
			return m, nil
		case key.Matches(msg, m.keys.ToggleSecrets):
			m.showSecrets = !m.showSecrets
			m.syncLayout()
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
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
	body := renderBody(m)
	footer := footerStyle().Width(m.width).Render(m.help.View(m.keys))

	view := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
	view.AltScreen = true
	return view
}

func loadSnapshotCmd(loader Loader) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := loader()
		return snapshotMsg{snapshot: snapshot, err: err}
	}
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

	switch m.active {
	case overviewSection:
		return renderOverview(m.snapshot)
	case servicesSection:
		return renderServices(m.snapshot, m.showSecrets)
	case healthSection:
		return renderHealth(m.snapshot)
	case connectionsSection:
		return renderConnections(m.snapshot, m.showSecrets)
	default:
		return ""
	}
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

func renderHeader(m Model) string {
	statusLabel := "Ready"
	if m.loading {
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

	meta := fmt.Sprintf(
		"%s  •  mode: %s  •  secrets: %s  •  updated: %s",
		statusLabel,
		mode,
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

	sidebar := sidebarStyle().Width(sidebarWidth).Height(bodyHeight).Render(renderSidebar(m.active))
	main := mainPanelStyle().Width(mainWidth).Height(bodyHeight).Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

func renderSidebar(active section) string {
	lines := []string{sectionTitleStyle().Render("Sections"), ""}
	for _, candidate := range sections {
		label := candidate.Title()
		if candidate == active {
			lines = append(lines, activeNavItemStyle().Render("▸ "+label))
			continue
		}
		lines = append(lines, navItemStyle().Render("  "+label))
	}

	lines = append(lines, "", mutedStyle().Render("Read-only phase one"))

	return strings.Join(lines, "\n")
}

func renderOverview(snapshot Snapshot) string {
	running, total := runningServiceCount(snapshot.Services)
	lines := []string{
		sectionTitleStyle().Render("Overview"),
		"",
		fmt.Sprintf("Stack: %s", emptyLabel(snapshot.StackName)),
		fmt.Sprintf("Config: %s", emptyLabel(snapshot.ConfigPath)),
		fmt.Sprintf("Stack dir: %s", emptyLabel(snapshot.StackDir)),
		fmt.Sprintf("Compose: %s", emptyLabel(snapshot.ComposePath)),
		fmt.Sprintf("Services running: %d / %d", running, total),
		fmt.Sprintf("Wait on start: %s", onOffLabel(snapshot.WaitForServices)),
		fmt.Sprintf("Startup timeout: %ds", snapshot.StartupTimeoutSec),
	}

	return strings.Join(lines, "\n")
}

func renderServices(snapshot Snapshot, showSecrets bool) string {
	lines := []string{sectionTitleStyle().Render("Services"), ""}
	if snapshot.ServiceError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.ServiceError), "")
	}
	if len(snapshot.Services) == 0 {
		lines = append(lines, mutedStyle().Render("No services loaded."))
		return strings.Join(lines, "\n")
	}

	for idx, service := range snapshot.Services {
		lines = append(lines, fmt.Sprintf("%s  %s", serviceStatusBadge(service.Status), service.DisplayName))
		lines = append(lines, fmt.Sprintf("Status: %s", emptyLabel(service.Status)))
		if service.ContainerName != "" {
			lines = append(lines, fmt.Sprintf("Container: %s", service.ContainerName))
		}
		if service.Image != "" {
			lines = append(lines, fmt.Sprintf("Image: %s", service.Image))
		}
		if service.DataVolume != "" {
			lines = append(lines, fmt.Sprintf("Data volume: %s", service.DataVolume))
		}
		if service.Host != "" {
			lines = append(lines, fmt.Sprintf("Host: %s", service.Host))
		}
		if service.ExternalPort > 0 || service.InternalPort > 0 {
			lines = append(lines, fmt.Sprintf("Port: %s", formatPort(service.ExternalPort, service.InternalPort)))
		}
		if service.Database != "" {
			lines = append(lines, fmt.Sprintf("Database: %s", service.Database))
		}
		if service.MaintenanceDB != "" {
			lines = append(lines, fmt.Sprintf("Maintenance DB: %s", service.MaintenanceDB))
		}
		if service.Email != "" {
			lines = append(lines, fmt.Sprintf("Email: %s", service.Email))
		}
		if service.Username != "" {
			lines = append(lines, fmt.Sprintf("Username: %s", service.Username))
		}
		if service.Password != "" {
			lines = append(lines, fmt.Sprintf("Password: %s", maskSecret(service.Password, showSecrets)))
		}
		if service.AppendOnly != nil {
			lines = append(lines, fmt.Sprintf("Appendonly: %s", enabledDisabled(*service.AppendOnly)))
		}
		if service.SavePolicy != "" {
			lines = append(lines, fmt.Sprintf("Save policy: %s", service.SavePolicy))
		}
		if service.MaxMemoryPolicy != "" {
			lines = append(lines, fmt.Sprintf("Maxmemory policy: %s", service.MaxMemoryPolicy))
		}
		if service.ServerMode != "" {
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

	return strings.Join(lines, "\n")
}

func renderHealth(snapshot Snapshot) string {
	lines := []string{sectionTitleStyle().Render("Health"), ""}
	if snapshot.HealthError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.HealthError), "")
	}
	if len(snapshot.Health) == 0 {
		lines = append(lines, mutedStyle().Render("No health data loaded."))
		return strings.Join(lines, "\n")
	}

	for _, line := range snapshot.Health {
		lines = append(lines, fmt.Sprintf("%s %s", healthStatusIcon(line.Status), line.Message))
	}

	return strings.Join(lines, "\n")
}

func renderConnections(snapshot Snapshot, showSecrets bool) string {
	lines := []string{sectionTitleStyle().Render("Connections"), ""}
	if len(snapshot.Connections) == 0 {
		lines = append(lines, mutedStyle().Render("No connection info loaded."))
		return strings.Join(lines, "\n")
	}

	for idx, entry := range snapshot.Connections {
		lines = append(lines, entry.Name)
		lines = append(lines, "  "+maskConnectionValue(entry.Value, showSecrets))
		if idx < len(snapshot.Connections)-1 {
			lines = append(lines, "")
		}
	}

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

func runningServiceCount(services []Service) (int, int) {
	running := 0
	for _, service := range services {
		if strings.EqualFold(service.Status, "running") {
			running++
		}
	}

	return running, len(services)
}

func serviceStatusBadge(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "●"
	case "stopped", "not installed", "error":
		return "○"
	default:
		return "◌"
	}
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

func formatPort(external, internal int) string {
	if external == 0 {
		return "-"
	}
	if internal == 0 {
		return fmt.Sprintf("%d -> unknown", external)
	}

	return fmt.Sprintf("%d -> %d", external, internal)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
