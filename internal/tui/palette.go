package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	bubblespaginator "charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/traweezy/stackctl/internal/output"
)

type ClipboardWriter func(string) error

type ServiceShellRequest struct {
	Service string
}

type ServiceShellLauncher func(ServiceShellRequest) (tea.ExecCommand, error)

type DBShellRequest struct {
	Service string
}

type DBShellLauncher func(DBShellRequest) (tea.ExecCommand, error)

var newHandoffDoneMsg = func(historyID int, action paletteAction, message string, err error, refresh bool) handoffDoneMsg {
	return handoffDoneMsg{
		historyID: historyID,
		action:    action,
		message:   message,
		err:       err,
		refresh:   refresh,
	}
}

var paletteQueryReplacer = strings.NewReplacer(
	"-", "",
	"_", "",
	":", "",
	"/", "",
	".", "",
	"  ", " ",
)

func handoffDoneCallback(historyID int, action paletteAction, message string, refresh bool) tea.ExecCallback {
	return func(err error) tea.Msg {
		return newHandoffDoneMsg(historyID, action, message, err, refresh)
	}
}

type paletteMode string

const (
	paletteModeCommand paletteMode = "command"
	paletteModeJump    paletteMode = "jump"
	paletteModeCopy    paletteMode = "copy"
)

type paletteActionKind int

const (
	paletteActionSection paletteActionKind = iota
	paletteActionSidebar
	paletteActionJumpStack
	paletteActionJumpService
	paletteActionCopyValue
	paletteActionCopyText
	paletteActionWatchLogs
	paletteActionExecShell
	paletteActionDBShell
	paletteActionPinService
	paletteActionToggleLayout
	paletteActionToggleAutoRefresh
	paletteActionToggleSecrets
)

type copyTargetKind string

const (
	copyTargetDSN       copyTargetKind = "dsn"
	copyTargetURL       copyTargetKind = "url"
	copyTargetEndpoint  copyTargetKind = "endpoint"
	copyTargetHostPort  copyTargetKind = "host-port"
	copyTargetUsername  copyTargetKind = "username"
	copyTargetPassword  copyTargetKind = "password"
	copyTargetMasterKey copyTargetKind = "master-key"
	copyTargetToken     copyTargetKind = "token"
	copyTargetAccessKey copyTargetKind = "access-key"
	copyTargetSecretKey copyTargetKind = "secret-key"
	copyTargetDatabase  copyTargetKind = "database"
	copyTargetEmail     copyTargetKind = "email"
)

type paletteAction struct {
	Kind       paletteActionKind
	Title      string
	Subtitle   string
	Search     string
	Section    section
	Action     ActionSpec
	StackName  string
	ServiceKey string
	CopyTarget copyTargetKind
	CopyValue  string
}

type paletteState struct {
	mode       paletteMode
	title      string
	prompt     string
	input      textinput.Model
	items      []paletteAction
	normalized []string
	filtered   []paletteAction
	paginator  bubblespaginator.Model
	pageSize   int
	selected   int
	offset     int
}

type runningHandoff struct {
	History int
	Action  paletteAction
	Refresh bool
}

type handoffDoneMsg struct {
	historyID int
	action    paletteAction
	message   string
	details   []string
	err       error
	refresh   bool
}

type copyDoneMsg struct {
	action  paletteAction
	message string
	err     error
}

type copyTarget struct {
	Kind    copyTargetKind
	Label   string
	Value   string
	Preview string
	Search  string
}

type scoredPaletteAction struct {
	action paletteAction
	score  int
	index  int
}

func newPaletteState(mode paletteMode, title, prompt string, items []paletteAction) *paletteState {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "Type to filter"
	input.Focus()
	pager := bubblespaginator.New()
	pager.Type = bubblespaginator.Arabic
	pager.ArabicFormat = "page %d/%d"

	preparedItems := append([]paletteAction(nil), items...)
	normalized := make([]string, len(preparedItems))
	for idx, item := range preparedItems {
		normalized[idx] = normalizePaletteQuery(item.searchText())
	}

	state := &paletteState{
		mode:       mode,
		title:      strings.TrimSpace(title),
		prompt:     strings.TrimSpace(prompt),
		input:      input,
		items:      preparedItems,
		normalized: normalized,
		paginator:  pager,
		pageSize:   8,
	}
	state.applyFilter()
	return state
}

func (p *paletteState) applyFilter() {
	query := normalizePaletteQuery(p.input.Value())
	if query == "" {
		p.filtered = append(p.filtered[:0], p.items...)
		p.clampSelection()
		return
	}

	matches := make([]scoredPaletteAction, 0, len(p.items))
	for idx, item := range p.items {
		score, ok := paletteMatchScoreNormalized(query, p.normalized[idx])
		if !ok {
			continue
		}
		matches = append(matches, scoredPaletteAction{
			action: item,
			score:  score,
			index:  idx,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].index < matches[j].index
		}
		return matches[i].score > matches[j].score
	})

	p.filtered = p.filtered[:0]
	for _, match := range matches {
		p.filtered = append(p.filtered, match.action)
	}
	p.clampSelection()
	p.syncPagination()
}

func (p *paletteState) clampSelection() {
	if len(p.filtered) == 0 {
		p.selected = 0
		p.offset = 0
		return
	}
	if p.selected >= len(p.filtered) {
		p.selected = len(p.filtered) - 1
	}
	if p.selected < 0 {
		p.selected = 0
	}
	if p.offset > p.selected {
		p.offset = p.selected
	}
}

func (p *paletteState) move(step int) {
	if len(p.filtered) == 0 {
		p.selected = 0
		p.offset = 0
		return
	}
	p.selected = (p.selected + step + len(p.filtered)) % len(p.filtered)
	p.syncPagination()
}

func (p *paletteState) page(step int) {
	if len(p.filtered) == 0 {
		p.selected = 0
		p.offset = 0
		return
	}
	p.syncPagination()
	target := p.paginator.Page + step
	if target < 0 {
		target = 0
	}
	if maxPage := maxInt(0, p.paginator.TotalPages-1); target > maxPage {
		target = maxPage
	}
	p.paginator.Page = target
	start, _ := p.paginator.GetSliceBounds(len(p.filtered))
	p.selected = start
	p.offset = start
}

func (p *paletteState) selectedAction() (paletteAction, bool) {
	if p == nil || len(p.filtered) == 0 {
		return paletteAction{}, false
	}
	return p.filtered[p.selected], true
}

func (a paletteAction) searchText() string {
	parts := []string{a.Title, a.Subtitle, a.Search}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func (a paletteAction) recentKey() string {
	switch a.Kind {
	case paletteActionSidebar:
		return fmt.Sprintf("sidebar:%s", a.Action.ID)
	case paletteActionJumpStack:
		return fmt.Sprintf("stack:%s", a.StackName)
	case paletteActionCopyValue:
		return fmt.Sprintf("copy:%s:%s", a.ServiceKey, a.CopyTarget)
	case paletteActionCopyText:
		return fmt.Sprintf("copy-text:%s", normalizePaletteQuery(a.Title))
	case paletteActionWatchLogs:
		return fmt.Sprintf("logs:%s", a.ServiceKey)
	case paletteActionExecShell:
		return fmt.Sprintf("exec:%s", a.ServiceKey)
	case paletteActionDBShell:
		return fmt.Sprintf("db:%s", a.ServiceKey)
	default:
		return ""
	}
}

func normalizePaletteQuery(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return paletteQueryReplacer.Replace(trimmed)
}

func paletteMatchScore(query, candidate string) (int, bool) {
	return paletteMatchScoreNormalized(
		normalizePaletteQuery(query),
		normalizePaletteQuery(candidate),
	)
}

func paletteMatchScoreNormalized(query, candidate string) (int, bool) {
	q := query
	c := candidate
	if q == "" {
		return 0, true
	}
	if c == "" {
		return 0, false
	}
	if strings.Contains(c, q) {
		return 1000 - (len(c) - len(q)), true
	}

	score := 0
	qi := 0
	consecutive := 0
	last := -2
	for idx, value := range c {
		if qi >= len(q) {
			break
		}
		if value != rune(q[qi]) {
			consecutive = 0
			continue
		}
		if idx == last+1 {
			consecutive++
		} else {
			consecutive = 0
		}
		score += 10 + consecutive*5
		last = idx
		qi++
	}
	if qi != len(q) {
		return 0, false
	}

	return score, true
}

func renderPalettePanel(state *paletteState, width, height int) string {
	if state == nil {
		return ""
	}

	panelWidth := minInt(88, maxInt(56, width-6))
	panelHeight := minInt(18, maxInt(10, height-4))
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeTheme().paletteBorder).
		Padding(0, 1).
		Width(panelWidth)

	innerWidth := maxInt(24, panelWidth-panelStyle.GetHorizontalFrameSize())
	header := []string{
		subsectionTitleStyle().Render(emptyLabel(state.title)),
	}
	if state.prompt != "" {
		header = append(header, mutedStyle().Render(state.prompt))
	}
	header = append(header, "")
	queryLine := lipgloss.NewStyle().
		Foreground(activeTheme().paletteInput).
		Render(state.input.View())

	lines := append(header, queryLine, "")
	availableRows := maxInt(4, panelHeight-panelStyle.GetVerticalFrameSize()-len(lines)-3)
	state.setPageSize(availableRows)
	if len(state.filtered) == 0 {
		lines = append(lines, mutedStyle().Render("No matching commands."))
	} else {
		start, end := state.paginator.GetSliceBounds(len(state.filtered))
		for idx := start; idx < end; idx++ {
			lines = append(lines, renderPaletteEntry(state.filtered[idx], idx == state.selected, innerWidth))
		}
		if end < len(state.filtered) {
			lines = append(lines, mutedStyle().Render(fmt.Sprintf("+ %d more", len(state.filtered)-end)))
		}
	}

	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render(state.summary()))
	lines = append(lines, mutedStyle().Render("type to filter  •  ↑/↓ choose  •  pgup/pgdn page  •  enter run  •  esc close"))

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		panelStyle.Render(strings.Join(lines, "\n")),
	)
}

func (p *paletteState) setPageSize(size int) {
	if size < 1 {
		size = 1
	}
	p.pageSize = size
	p.syncPagination()
}

func (p *paletteState) syncPagination() {
	if p.pageSize < 1 {
		p.pageSize = 8
	}
	p.paginator.PerPage = p.pageSize
	if len(p.filtered) == 0 {
		p.paginator.Page = 0
		p.paginator.TotalPages = 0
		p.offset = 0
		return
	}
	p.paginator.SetTotalPages(len(p.filtered))
	lastPage := maxInt(0, p.paginator.TotalPages-1)
	page := p.selected / p.pageSize
	if page > lastPage {
		page = lastPage
	}
	if page < 0 {
		page = 0
	}
	p.paginator.Page = page
	start, _ := p.paginator.GetSliceBounds(len(p.filtered))
	p.offset = start
}

func (p *paletteState) summary() string {
	if p == nil || len(p.filtered) == 0 {
		return "0 results"
	}
	start, end := p.paginator.GetSliceBounds(len(p.filtered))
	if p.paginator.TotalPages <= 1 {
		return fmt.Sprintf("%d results", len(p.filtered))
	}
	return fmt.Sprintf("%d-%d of %d  •  %s", start+1, end, len(p.filtered), p.paginator.View())
}

func renderPaletteEntry(action paletteAction, selected bool, width int) string {
	title := action.Title
	if selected {
		title = activeNavItemStyle().Render("▸ " + title)
	} else {
		title = navItemStyle().Render("  " + title)
	}
	if action.Subtitle == "" {
		return lipgloss.NewStyle().Width(width).Render(title)
	}

	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			mutedStyle().Render("    "+action.Subtitle),
		),
	)
}

func (m *Model) normalizePinnedServices() {
	if len(m.pinnedServices) == 0 {
		return
	}
	available := make(map[string]struct{}, len(m.snapshot.Services))
	for _, service := range m.snapshot.Services {
		available[serviceKey(service)] = struct{}{}
	}
	for key := range m.pinnedServices {
		if _, ok := available[key]; !ok {
			delete(m.pinnedServices, key)
		}
	}
}

func (m Model) servicePinned(key string) bool {
	_, ok := m.pinnedServices[key]
	return ok
}

func (m Model) selectedProductivityService() (Service, bool) {
	switch m.active {
	case healthSection:
		return selectedService(m.snapshot, m.selectedHealth)
	default:
		return selectedService(m.snapshot, m.selectedService)
	}
}

func selectedServiceByKey(snapshot Snapshot, key string) (Service, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Service{}, false
	}
	for _, service := range snapshot.Services {
		if serviceKey(service) == key {
			return service, true
		}
	}
	return Service{}, false
}

func serviceEndpointValue(service Service) string {
	if strings.TrimSpace(service.Host) == "" || service.ExternalPort <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", service.Host, service.ExternalPort)
}

func serviceCopyTargets(service Service, showSecrets bool) []copyTarget {
	targets := make([]copyTarget, 0, 8)
	add := func(kind copyTargetKind, label, value, preview string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		targets = append(targets, copyTarget{
			Kind:    kind,
			Label:   label,
			Value:   value,
			Preview: preview,
			Search:  strings.ToLower(label + " " + service.DisplayName),
		})
	}

	if service.DSN != "" {
		add(copyTargetDSN, service.DisplayName+" DSN", service.DSN, maskConnectionValue(service.DSN, showSecrets))
	}
	if service.URL != "" {
		add(copyTargetURL, service.DisplayName+" URL", service.URL, service.URL)
	}
	if endpoint := strings.TrimSpace(service.Endpoint); endpoint != "" {
		add(copyTargetEndpoint, service.DisplayName+" endpoint", endpoint, endpoint)
	} else if endpoint := serviceEndpointValue(service); endpoint != "" {
		add(copyTargetEndpoint, service.DisplayName+" endpoint", endpoint, endpoint)
	}
	if service.ExternalPort > 0 {
		port := strconv.Itoa(service.ExternalPort)
		add(copyTargetHostPort, service.DisplayName+" host port", port, port)
	}
	if service.AccessKey != "" {
		add(copyTargetAccessKey, service.DisplayName+" access key", service.AccessKey, service.AccessKey)
	}
	if service.SecretKey != "" {
		add(copyTargetSecretKey, service.DisplayName+" secret key", service.SecretKey, maskSecret(service.SecretKey, showSecrets))
	}
	if service.Username != "" {
		add(copyTargetUsername, service.DisplayName+" username", service.Username, service.Username)
	}
	if service.Password != "" {
		add(copyTargetPassword, service.DisplayName+" password", service.Password, maskSecret(service.Password, showSecrets))
	}
	if service.MasterKey != "" {
		add(copyTargetMasterKey, service.DisplayName+" master key", service.MasterKey, maskSecret(service.MasterKey, showSecrets))
	}
	if service.Token != "" {
		add(copyTargetToken, service.DisplayName+" token", service.Token, maskSecret(service.Token, showSecrets))
	}
	if service.Database != "" {
		add(copyTargetDatabase, service.DisplayName+" database", service.Database, service.Database)
	}
	if service.Email != "" {
		add(copyTargetEmail, service.DisplayName+" email", service.Email, service.Email)
	}

	return targets
}

func findCopyTarget(service Service, kind copyTargetKind, showSecrets bool) (copyTarget, bool) {
	for _, target := range serviceCopyTargets(service, showSecrets) {
		if target.Kind == kind {
			return target, true
		}
	}
	return copyTarget{}, false
}

func (m Model) recentPaletteActions() []paletteAction {
	actions := make([]paletteAction, 0, 5)
	seen := make(map[string]struct{})
	for idx := len(m.history) - 1; idx >= 0 && len(actions) < 5; idx-- {
		entry := m.history[idx]
		if entry.Recent == nil || entry.CompletedAt.IsZero() {
			continue
		}
		key := entry.Recent.recentKey()
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		action := *entry.Recent
		if entry.Message != "" {
			action.Subtitle = entry.Message
		}
		actions = append(actions, action)
	}
	return actions
}

func (m Model) serviceJumpActions() []paletteAction {
	actions := make([]paletteAction, 0, len(m.snapshot.Services))
	pinned := make([]Service, 0, len(m.snapshot.Services))
	stackServices := make([]Service, 0, len(m.snapshot.Services))
	hostTools := make([]Service, 0, len(m.snapshot.Services))

	for _, service := range m.snapshot.Services {
		key := serviceKey(service)
		if m.servicePinned(key) {
			pinned = append(pinned, service)
			continue
		}
		if isStackService(service) {
			stackServices = append(stackServices, service)
		} else {
			hostTools = append(hostTools, service)
		}
	}

	appendActions := func(group string, services []Service) {
		for _, service := range services {
			subtitle := displayServiceStatus(service)
			if group != "" {
				subtitle = group + "  •  " + subtitle
			}
			actions = append(actions, paletteAction{
				Kind:       paletteActionJumpService,
				Title:      "Go to " + service.DisplayName,
				Subtitle:   subtitle,
				Search:     strings.ToLower(service.DisplayName + " " + subtitle),
				ServiceKey: serviceKey(service),
			})
		}
	}

	appendActions("Pinned", pinned)
	appendActions("Stack service", stackServices)
	appendActions("Host tool", hostTools)
	return actions
}

func (m Model) stackJumpActions() []paletteAction {
	actions := make([]paletteAction, 0, len(m.snapshot.Stacks))
	for _, profile := range m.snapshot.Stacks {
		subtitleParts := make([]string, 0, 3)
		if profile.Current {
			subtitleParts = append(subtitleParts, "Current")
		}
		if profile.Mode != "" && profile.Mode != "-" {
			subtitleParts = append(subtitleParts, profile.Mode)
		}
		if profile.State != "" && profile.State != "-" {
			subtitleParts = append(subtitleParts, profile.State)
		}
		subtitle := strings.Join(subtitleParts, "  •  ")
		actions = append(actions, paletteAction{
			Kind:      paletteActionJumpStack,
			Title:     "Go to stack " + profile.Name,
			Subtitle:  subtitle,
			Search:    strings.ToLower(profile.Name + " stack " + subtitle + " " + profile.ConfigPath),
			StackName: profile.Name,
		})
	}

	return actions
}

func (m Model) commandPaletteActions() []paletteAction {
	actions := make([]paletteAction, 0, 32)
	actions = append(actions, m.recentPaletteActions()...)
	if strings.TrimSpace(m.snapshot.ConnectText) != "" {
		actions = append(actions, paletteAction{
			Kind:      paletteActionCopyText,
			Title:     "Copy stackctl connect output",
			Subtitle:  "Minimal DSNs, URLs, and endpoints",
			Search:    "stackctl connect dsn url endpoint copy",
			CopyValue: m.snapshot.ConnectText,
		})
	}
	if strings.TrimSpace(m.snapshot.EnvExportText) != "" {
		actions = append(actions, paletteAction{
			Kind:      paletteActionCopyText,
			Title:     "Copy stackctl env --export output",
			Subtitle:  "Export-ready environment variables",
			Search:    "stackctl env export environment variables copy",
			CopyValue: m.snapshot.EnvExportText,
		})
	}
	if strings.TrimSpace(m.snapshot.PortsText) != "" {
		actions = append(actions, paletteAction{
			Kind:      paletteActionCopyText,
			Title:     "Copy stackctl ports output",
			Subtitle:  "Host-to-service port mappings",
			Search:    "stackctl ports host port mapping copy",
			CopyValue: m.snapshot.PortsText,
		})
	}

	if service, ok := m.selectedProductivityService(); ok {
		serviceKey := serviceKey(service)
		serviceTitle := service.DisplayName
		actions = append(actions, paletteAction{
			Kind:       paletteActionJumpService,
			Title:      "Go to " + serviceTitle,
			Subtitle:   "Selected service",
			Search:     strings.ToLower(serviceTitle + " selected service jump"),
			ServiceKey: serviceKey,
		})
		if isStackService(service) {
			actions = append(actions, paletteAction{
				Kind:       paletteActionWatchLogs,
				Title:      "Watch " + serviceTitle + " logs",
				Subtitle:   "Open the full compose log stream",
				Search:     strings.ToLower(serviceTitle + " logs watch stream"),
				ServiceKey: serviceKey,
			})
			actions = append(actions, paletteAction{
				Kind:       paletteActionExecShell,
				Title:      "Open " + serviceTitle + " shell",
				Subtitle:   "Run an interactive shell inside the container",
				Search:     strings.ToLower(serviceTitle + " shell exec"),
				ServiceKey: serviceKey,
			})
			if strings.EqualFold(service.Name, "postgres") {
				actions = append(actions, paletteAction{
					Kind:       paletteActionDBShell,
					Title:      "Open Postgres db shell",
					Subtitle:   "Jump straight into psql",
					Search:     "postgres db shell psql",
					ServiceKey: serviceKey,
				})
			}
		}
		if m.servicePinned(serviceKey) {
			actions = append(actions, paletteAction{
				Kind:       paletteActionPinService,
				Title:      "Unpin " + serviceTitle,
				Subtitle:   "Remove it from the pinned group",
				Search:     strings.ToLower(serviceTitle + " unpin"),
				ServiceKey: serviceKey,
			})
		} else {
			actions = append(actions, paletteAction{
				Kind:       paletteActionPinService,
				Title:      "Pin " + serviceTitle,
				Subtitle:   "Keep it at the top of the service lists",
				Search:     strings.ToLower(serviceTitle + " pin"),
				ServiceKey: serviceKey,
			})
		}
		for _, target := range serviceCopyTargets(service, m.showSecrets) {
			actions = append(actions, paletteAction{
				Kind:       paletteActionCopyValue,
				Title:      "Copy " + target.Label,
				Subtitle:   target.Preview,
				Search:     target.Search,
				ServiceKey: serviceKey,
				CopyTarget: target.Kind,
			})
		}
	}

	for _, candidate := range sections {
		actions = append(actions, paletteAction{
			Kind:     paletteActionSection,
			Title:    "Go to " + candidate.Title(),
			Subtitle: "Section",
			Search:   strings.ToLower(candidate.Title() + " section"),
			Section:  candidate,
		})
	}
	actions = append(actions, m.stackJumpActions()...)
	actions = append(actions, m.serviceJumpActions()...)

	if m.runner != nil {
		if m.active != stacksSection {
			if profile, ok := selectedStackProfile(m.snapshot, m.selectedStack); ok {
				for _, action := range availableStackActions(profile, true) {
					actions = append(actions, paletteAction{
						Kind:      paletteActionSidebar,
						Title:     action.Label,
						Subtitle:  strings.TrimSpace(action.Group),
						Search:    strings.ToLower(action.Label + " " + action.Group + " " + profile.Name),
						Action:    action,
						StackName: profile.Name,
					})
				}
			}
		}
		for _, action := range m.availableSidebarActions() {
			actions = append(actions, paletteAction{
				Kind:     paletteActionSidebar,
				Title:    action.Label,
				Subtitle: strings.TrimSpace(action.Group),
				Search:   strings.ToLower(action.Label + " " + action.Group),
				Action:   action,
			})
		}
	}

	actions = append(actions,
		paletteAction{
			Kind:     paletteActionToggleLayout,
			Title:    "Toggle compact layout",
			Subtitle: "Switch between expanded and compact density",
			Search:   "layout compact expanded density",
		},
		paletteAction{
			Kind:     paletteActionToggleAutoRefresh,
			Title:    "Toggle auto-refresh",
			Subtitle: "Pause or resume automatic snapshot refresh",
			Search:   "auto refresh",
		},
		paletteAction{
			Kind:     paletteActionToggleSecrets,
			Title:    "Toggle secrets",
			Subtitle: "Show or hide passwords in the dashboard",
			Search:   "secrets passwords mask",
		},
	)

	return actions
}

func (m Model) copyPaletteActions() []paletteAction {
	service, ok := m.selectedProductivityService()
	if !ok {
		return nil
	}
	actions := make([]paletteAction, 0, 8)
	for _, target := range serviceCopyTargets(service, m.showSecrets) {
		actions = append(actions, paletteAction{
			Kind:       paletteActionCopyValue,
			Title:      "Copy " + target.Label,
			Subtitle:   target.Preview,
			Search:     target.Search,
			ServiceKey: serviceKey(service),
			CopyTarget: target.Kind,
		})
	}
	return actions
}

func (m *Model) openCommandPalette() {
	m.palette = newPaletteState(
		paletteModeCommand,
		"Command palette",
		"Run an action, jump to a section, or open a service helper",
		m.commandPaletteActions(),
	)
}

func (m *Model) openJumpPalette() tea.Cmd {
	if m.active == stacksSection {
		if len(m.snapshot.Stacks) == 0 {
			bannerID := m.setBanner(output.StatusWarn, "no stack profiles are available to jump to")
			m.palette = nil
			return clearBannerCmd(bannerID)
		}
		m.palette = newPaletteState(
			paletteModeJump,
			"Jump to stack",
			"Choose a stack profile",
			m.stackJumpActions(),
		)
		return nil
	}

	if len(m.snapshot.Services) == 0 {
		bannerID := m.setBanner(output.StatusWarn, "no services are available to jump to")
		m.palette = nil
		return clearBannerCmd(bannerID)
	}
	m.palette = newPaletteState(
		paletteModeJump,
		"Jump to service",
		"Choose a service or host tool",
		m.serviceJumpActions(),
	)
	return nil
}

func (m *Model) openCopyPalette() tea.Cmd {
	if service, ok := m.selectedProductivityService(); ok {
		items := m.copyPaletteActions()
		if len(items) == 0 {
			bannerID := m.setBanner(output.StatusWarn, "no copy targets are available for the selected service")
			return clearBannerCmd(bannerID)
		}
		m.palette = newPaletteState(
			paletteModeCopy,
			"Copy value",
			"Choose what to copy from "+service.DisplayName,
			items,
		)
		return nil
	}

	bannerID := m.setBanner(output.StatusWarn, "select a service before copying values")
	return clearBannerCmd(bannerID)
}

func (m *Model) handlePaletteKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if m.palette == nil {
		return nil, false
	}

	switch msg.String() {
	case "ctrl+c":
		return tea.Quit, true
	case "esc":
		m.palette = nil
		return nil, true
	case "enter":
		action, ok := m.palette.selectedAction()
		if !ok {
			return nil, true
		}
		m.palette = nil
		return m.executePaletteAction(action), true
	case "up":
		m.palette.move(-1)
		return nil, true
	case "down":
		m.palette.move(1)
		return nil, true
	case "pgup":
		m.palette.page(-1)
		return nil, true
	case "pgdown":
		m.palette.page(1)
		return nil, true
	}

	var cmd tea.Cmd
	m.palette.input, cmd = m.palette.input.Update(msg)
	m.palette.applyFilter()
	return cmd, true
}

func (m *Model) executePaletteAction(action paletteAction) tea.Cmd {
	switch action.Kind {
	case paletteActionSection:
		m.active = action.Section
		m.resetViewportForActivePanel()
		m.syncLayout()
		return nil
	case paletteActionJumpStack:
		m.active = stacksSection
		m.selectedStack = action.StackName
		m.resetViewportForActivePanel()
		m.syncLayout()
		return nil
	case paletteActionSidebar:
		if action.Action.RequiresConfirmation() {
			m.confirmation = newActionConfirmation(action.Action)
			m.syncLayout()
			return nil
		}
		updated, cmd := m.beginAction(action.Action)
		*m = updated.(Model)
		if len(m.history) > 0 {
			actionCopy := recentPaletteActionForActionSpec(action.Action)
			m.history[len(m.history)-1].Recent = actionCopy
		}
		return cmd
	case paletteActionJumpService:
		m.active = servicesSection
		m.selectedService = action.ServiceKey
		m.resetViewportForActivePanel()
		m.syncLayout()
		return nil
	case paletteActionCopyValue:
		return m.startCopyAction(action)
	case paletteActionCopyText:
		return m.startCopyTextAction(action)
	case paletteActionWatchLogs:
		return m.startServiceLogWatch(action)
	case paletteActionExecShell:
		return m.startServiceShell(action)
	case paletteActionDBShell:
		return m.startDBShell(action)
	case paletteActionPinService:
		return m.togglePinnedService(action.ServiceKey)
	case paletteActionToggleLayout:
		if m.layout == expandedLayout {
			m.layout = compactLayout
		} else {
			m.layout = expandedLayout
		}
		m.syncLayout()
		return nil
	case paletteActionToggleAutoRefresh:
		m.autoRefresh = !m.autoRefresh
		m.autoRefreshID++
		m.syncLayout()
		if m.autoRefresh {
			return autoRefreshCmd(m.autoRefreshID, m.refreshInterval())
		}
		return nil
	case paletteActionToggleSecrets:
		m.showSecrets = !m.showSecrets
		if m.configManager != nil {
			m.configEditor.setSize(m.viewport.Width(), m.viewport.Height(), m.showSecrets)
		}
		m.syncLayout()
		return nil
	default:
		return nil
	}
}

func (m *Model) togglePinnedService(serviceName string) tea.Cmd {
	service, ok := selectedServiceByKey(m.snapshot, serviceName)
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "service is no longer available to pin")
		return clearBannerCmd(bannerID)
	}

	key := serviceKey(service)
	message := "pinned " + service.DisplayName
	if m.servicePinned(key) {
		delete(m.pinnedServices, key)
		message = "unpinned " + service.DisplayName
	} else {
		m.pinnedServices[key] = struct{}{}
	}
	bannerID := m.setBanner(output.StatusInfo, message)
	m.syncLayout()
	return clearBannerCmd(bannerID)
}

func (m *Model) startCopyAction(action paletteAction) tea.Cmd {
	service, ok := selectedServiceByKey(m.snapshot, action.ServiceKey)
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "selected service is no longer available")
		return clearBannerCmd(bannerID)
	}
	target, ok := findCopyTarget(service, action.CopyTarget, m.showSecrets)
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "copy target is unavailable for the selected service")
		return clearBannerCmd(bannerID)
	}

	if m.clipboardWriter == nil {
		return terminalCopyCmd(target.Value, action, fmt.Sprintf("copied %s to clipboard", target.Label))
	}

	return copyValueCmd(m.clipboardWriter, target.Value, action, fmt.Sprintf("copied %s to clipboard", target.Label))
}

func (m *Model) startCopyTextAction(action paletteAction) tea.Cmd {
	if strings.TrimSpace(action.CopyValue) == "" {
		bannerID := m.setBanner(output.StatusWarn, "copy value is unavailable for this action")
		return clearBannerCmd(bannerID)
	}

	successMessage := fmt.Sprintf(
		"copied %s to clipboard",
		strings.ToLower(strings.TrimPrefix(action.Title, "Copy ")),
	)
	if m.clipboardWriter == nil {
		return terminalCopyCmd(action.CopyValue, action, successMessage)
	}

	return copyValueCmd(
		m.clipboardWriter,
		action.CopyValue,
		action,
		successMessage,
	)
}

func copyValueCmd(copyWriter ClipboardWriter, value string, action paletteAction, successMessage string) tea.Cmd {
	return func() tea.Msg {
		err := copyWriter(value)
		return copyDoneMsg{
			action:  action,
			message: successMessage,
			err:     err,
		}
	}
}

func terminalCopyCmd(value string, action paletteAction, successMessage string) tea.Cmd {
	return tea.Batch(
		tea.SetClipboard(value),
		func() tea.Msg {
			return copyDoneMsg{
				action:  action,
				message: successMessage,
			}
		},
	)
}

func (m *Model) startServiceLogWatch(action paletteAction) tea.Cmd {
	service, ok := selectedServiceByKey(m.snapshot, action.ServiceKey)
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "selected service is no longer available")
		return clearBannerCmd(bannerID)
	}
	if !isStackService(service) {
		bannerID := m.setBanner(output.StatusWarn, "live logs are unavailable for host tools")
		return clearBannerCmd(bannerID)
	}
	if m.logWatchLauncher == nil {
		bannerID := m.setBanner(output.StatusWarn, "live log handoff is unavailable in this model")
		return clearBannerCmd(bannerID)
	}

	execCmd, err := m.logWatchLauncher(LogWatchRequest{Service: logWatchServiceName(service)})
	if err != nil {
		bannerID := m.setBanner(output.StatusWarn, watchLogsErrorMessage(service.DisplayName, err))
		return clearBannerCmd(bannerID)
	}

	return m.startHandoffAction(
		action,
		execCmd,
		output.StatusInfo,
		"opening live logs for "+service.DisplayName,
		"closed live logs for "+service.DisplayName,
		true,
	)
}

func (m *Model) startServiceShell(action paletteAction) tea.Cmd {
	service, ok := selectedServiceByKey(m.snapshot, action.ServiceKey)
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "selected service is no longer available")
		return clearBannerCmd(bannerID)
	}
	if !isStackService(service) {
		bannerID := m.setBanner(output.StatusWarn, "service shells are unavailable for host tools")
		return clearBannerCmd(bannerID)
	}
	if m.shellLauncher == nil {
		bannerID := m.setBanner(output.StatusWarn, "service shell handoff is unavailable in this model")
		return clearBannerCmd(bannerID)
	}

	execCmd, err := m.shellLauncher(ServiceShellRequest{Service: logWatchServiceName(service)})
	if err != nil {
		bannerID := m.setBanner(output.StatusWarn, fmt.Sprintf("open %s shell failed: %v", strings.ToLower(service.DisplayName), err))
		return clearBannerCmd(bannerID)
	}

	return m.startHandoffAction(
		action,
		execCmd,
		output.StatusInfo,
		"opening "+service.DisplayName+" shell",
		"closed "+service.DisplayName+" shell",
		true,
	)
}

func (m *Model) startDBShell(action paletteAction) tea.Cmd {
	service, ok := selectedServiceByKey(m.snapshot, action.ServiceKey)
	if !ok {
		bannerID := m.setBanner(output.StatusWarn, "selected service is no longer available")
		return clearBannerCmd(bannerID)
	}
	if !strings.EqualFold(logWatchServiceName(service), "postgres") {
		bannerID := m.setBanner(output.StatusWarn, "db shell is only available for Postgres")
		return clearBannerCmd(bannerID)
	}
	if m.dbShellLauncher == nil {
		bannerID := m.setBanner(output.StatusWarn, "db shell handoff is unavailable in this model")
		return clearBannerCmd(bannerID)
	}

	execCmd, err := m.dbShellLauncher(DBShellRequest{Service: logWatchServiceName(service)})
	if err != nil {
		bannerID := m.setBanner(output.StatusWarn, fmt.Sprintf("open %s db shell failed: %v", strings.ToLower(service.DisplayName), err))
		return clearBannerCmd(bannerID)
	}

	return m.startHandoffAction(
		action,
		execCmd,
		output.StatusInfo,
		"opening "+service.DisplayName+" db shell",
		"closed "+service.DisplayName+" db shell",
		true,
	)
}

func (m *Model) startHandoffAction(action paletteAction, execCmd tea.ExecCommand, status, pendingMessage, doneMessage string, refresh bool) tea.Cmd {
	m.nextHistoryID++
	historyID := m.nextHistoryID
	actionCopy := action
	m.history = append(m.history, historyEntry{
		ID:        historyID,
		Action:    action.Title,
		Status:    status,
		Message:   pendingMessage,
		StartedAt: timeNow(),
		Recent:    &actionCopy,
	})
	m.runningHandoff = &runningHandoff{
		History: historyID,
		Action:  action,
		Refresh: refresh,
	}
	m.setBanner(status, pendingMessage)
	m.syncLayout()

	return tea.Batch(
		m.beginBusy(m.currentBusyBudget()),
		tea.Exec(execCmd, handoffDoneCallback(historyID, action, doneMessage, refresh)),
	)
}

func (m *Model) completeHandoff(msg handoffDoneMsg) tea.Cmd {
	if m.runningHandoff == nil || m.runningHandoff.History != msg.historyID {
		return nil
	}

	entryIndex := -1
	for idx := range m.history {
		if m.history[idx].ID == msg.historyID {
			entryIndex = idx
			break
		}
	}

	status := output.StatusOK
	message := msg.message
	if msg.err != nil {
		status = output.StatusFail
		message = fmt.Sprintf("%s failed: %v", strings.ToLower(msg.action.Title), msg.err)
	}
	if strings.TrimSpace(message) == "" {
		message = msg.action.Title + " completed"
	}
	if entryIndex >= 0 {
		m.history[entryIndex].Status = status
		m.history[entryIndex].Message = message
		m.history[entryIndex].Details = append([]string(nil), msg.details...)
		m.history[entryIndex].CompletedAt = timeNow()
	}

	bannerID := m.setBanner(status, message)
	m.runningHandoff = nil
	if msg.refresh {
		m.loading = true
		m.autoRefreshID++
		return tea.Batch(m.beginBusy(m.currentBusyBudget()), loadSnapshotCmd(m.loader), clearBannerCmd(bannerID))
	}
	return tea.Batch(m.finishBusy(), clearBannerCmd(bannerID))
}

func (m *Model) completeCopy(msg copyDoneMsg) tea.Cmd {
	status := output.StatusOK
	message := msg.message
	if msg.err != nil {
		status = output.StatusFail
		message = fmt.Sprintf("%s failed: %v", strings.ToLower(msg.action.Title), msg.err)
	}
	actionCopy := msg.action
	m.nextHistoryID++
	m.history = append(m.history, historyEntry{
		ID:          m.nextHistoryID,
		Action:      msg.action.Title,
		Status:      status,
		Message:     message,
		StartedAt:   timeNow(),
		CompletedAt: timeNow(),
		Recent:      &actionCopy,
	})
	bannerID := m.setBanner(status, message)
	return clearBannerCmd(bannerID)
}

func recentPaletteActionForActionSpec(action ActionSpec) *paletteAction {
	switch action.ID {
	case "":
		return nil
	default:
		actionCopy := paletteAction{
			Kind:     paletteActionSidebar,
			Title:    action.Label,
			Subtitle: strings.TrimSpace(action.Group),
			Search:   strings.ToLower(action.Label + " " + action.Group),
			Action:   action,
		}
		return &actionCopy
	}
}

func timeNow() time.Time {
	return time.Now()
}

func (m Model) showProductivityHelp() bool {
	return m.active != configSection && len(m.snapshot.Services) > 0
}

func (m Model) showCopyHelp() bool {
	if !m.activeHasServiceSelectionList() {
		return false
	}
	service, ok := m.selectedProductivityService()
	return ok && len(serviceCopyTargets(service, m.showSecrets)) > 0
}

func (m Model) showExecShellHelp() bool {
	if !m.activeHasServiceSelectionList() {
		return false
	}
	service, ok := m.selectedProductivityService()
	return ok && isStackService(service)
}

func (m Model) showDBShellHelp() bool {
	if !m.activeHasServiceSelectionList() {
		return false
	}
	service, ok := m.selectedProductivityService()
	return ok && isStackService(service) && strings.EqualFold(logWatchServiceName(service), "postgres")
}

func (m Model) showPinHelp() bool {
	if !m.activeHasServiceSelectionList() {
		return false
	}
	_, ok := m.selectedProductivityService()
	return ok
}
