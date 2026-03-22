package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/traweezy/stackctl/internal/output"
)

const (
	logTailLines       = 200
	logFollowInterval  = 3 * time.Second
	splitPaneMinWidth  = 96
	defaultListPaneW   = 26
	defaultFilterPaneW = 20
)

type DoctorCheck struct {
	Status  string
	Message string
}

type DoctorSummary struct {
	OK   int
	Warn int
	Miss int
	Fail int
}

type LogRequest struct {
	Service string
	Tail    int
}

type LogSnapshot struct {
	Service  string
	Output   string
	LoadedAt time.Time
}

type LogLoader func(LogRequest) (LogSnapshot, error)

type logPanelState struct {
	Service   string
	Follow    bool
	Loading   bool
	Error     string
	Output    string
	LoadedAt  time.Time
	RequestID int
}

type logSnapshotMsg struct {
	RequestID int
	Snapshot  LogSnapshot
	Err       error
}

type logFilter struct {
	Name        string
	DisplayName string
	Status      string
}

func serviceKey(service Service) string {
	if strings.TrimSpace(service.Name) != "" {
		return strings.TrimSpace(service.Name)
	}

	var builder strings.Builder
	for _, value := range strings.ToLower(service.DisplayName) {
		if (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') {
			builder.WriteRune(value)
		}
	}

	return builder.String()
}

func selectableServiceNames(snapshot Snapshot) []string {
	names := make([]string, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		key := serviceKey(service)
		if key == "" {
			continue
		}
		names = append(names, key)
	}

	return names
}

func selectablePortNames(snapshot Snapshot) []string {
	names := make([]string, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		key := serviceKey(service)
		if key == "" || service.ExternalPort <= 0 {
			continue
		}
		names = append(names, key)
	}

	return names
}

func logFilters(snapshot Snapshot) []logFilter {
	filters := []logFilter{{
		Name:        "",
		DisplayName: "All services",
		Status:      output.StatusLogs,
	}}
	for _, service := range snapshot.Services {
		key := serviceKey(service)
		if !isStackService(service) || key == "" {
			continue
		}
		filters = append(filters, logFilter{
			Name:        key,
			DisplayName: service.DisplayName,
			Status:      displayServiceStatus(service),
		})
	}

	return filters
}

func pickSelectedName(selected string, available []string) string {
	if len(available) == 0 {
		return ""
	}
	for _, candidate := range available {
		if candidate == selected {
			return candidate
		}
	}

	return available[0]
}

func cycleSelectedName(selected string, available []string, step int) string {
	if len(available) == 0 {
		return ""
	}
	selected = pickSelectedName(selected, available)
	index := 0
	for idx, candidate := range available {
		if candidate == selected {
			index = idx
			break
		}
	}

	index = (index + step + len(available)) % len(available)
	return available[index]
}

func pickSelectedFilter(selected string, filters []logFilter) string {
	if len(filters) == 0 {
		return ""
	}
	for _, filter := range filters {
		if filter.Name == selected {
			return filter.Name
		}
	}

	return filters[0].Name
}

func cycleSelectedFilter(selected string, filters []logFilter, step int) string {
	if len(filters) == 0 {
		return ""
	}
	selected = pickSelectedFilter(selected, filters)
	index := 0
	for idx, filter := range filters {
		if filter.Name == selected {
			index = idx
			break
		}
	}

	index = (index + step + len(filters)) % len(filters)
	return filters[index].Name
}

func selectedService(snapshot Snapshot, selected string) (Service, bool) {
	name := pickSelectedName(selected, selectableServiceNames(snapshot))
	if name == "" {
		return Service{}, false
	}
	for _, service := range snapshot.Services {
		if serviceKey(service) == name {
			return service, true
		}
	}

	return Service{}, false
}

func selectedPortService(snapshot Snapshot, selected string) (Service, bool) {
	name := pickSelectedName(selected, selectablePortNames(snapshot))
	if name == "" {
		return Service{}, false
	}
	for _, service := range snapshot.Services {
		if serviceKey(service) == name {
			return service, true
		}
	}

	return Service{}, false
}

func splitPane(left, right string, width int, listPaneWidth int) string {
	if width < splitPaneMinWidth {
		return strings.TrimSpace(left) + "\n\n" + strings.TrimSpace(right)
	}

	leftWidth := minInt(listPaneWidth, maxInt(20, width/3))
	rightWidth := maxInt(24, width-leftWidth-3)

	leftPane := subPaneStyle("238").Width(leftWidth).Render(left)
	rightPane := subPaneStyle("31").Width(rightWidth).Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func subPaneStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 1)
}

func detailHeading(title string) string {
	return subsectionTitleStyle().Render(title)
}

func listItem(selected bool, label string, chip string) string {
	prefix := "  "
	if selected {
		prefix = "▸ "
	}

	line := prefix + label
	if chip != "" {
		line += "  " + chip
	}

	if selected {
		return activeNavItemStyle().Render(line)
	}

	return navItemStyle().Render(line)
}

func statusChip(label, status string) string {
	if strings.TrimSpace(label) == "" {
		return ""
	}

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "ok", "healthy":
		style = style.Background(lipgloss.Color("28"))
	case output.StatusWarn, "warn", "warning", "needs attention", "changing":
		style = style.Foreground(lipgloss.Color("16")).Background(lipgloss.Color("221"))
	case output.StatusFail, output.StatusMiss, "not running", "stopped", "missing", "not installed":
		style = style.Background(lipgloss.Color("160"))
	case output.StatusInfo, output.StatusLogs, "starting", "stopping", "restarting":
		style = style.Background(lipgloss.Color("24"))
	default:
		style = style.Foreground(lipgloss.Color("16")).Background(lipgloss.Color("245"))
	}

	return style.Render(strings.ToUpper(label))
}

func formatLoadedAt(value time.Time) string {
	if value.IsZero() {
		return "not loaded yet"
	}

	return value.Format("15:04:05")
}

func doctorSummaryLine(summary DoctorSummary) string {
	return strings.Join([]string{
		statusChip(fmt.Sprintf("ok %d", summary.OK), output.StatusOK),
		statusChip(fmt.Sprintf("warn %d", summary.Warn), output.StatusWarn),
		statusChip(fmt.Sprintf("miss %d", summary.Miss), output.StatusMiss),
		statusChip(fmt.Sprintf("fail %d", summary.Fail), output.StatusFail),
	}, " ")
}

func renderServices(snapshot Snapshot, showSecrets bool, layout layoutMode, selected string, width int) string {
	lines := []string{sectionTitleStyle().Render("Services"), ""}
	if snapshot.ServiceError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.ServiceError), "")
	}
	if len(snapshot.Services) == 0 {
		lines = append(lines, mutedStyle().Render("No services loaded."))
		return strings.Join(lines, "\n")
	}

	selectedService, ok := selectedService(snapshot, selected)
	if !ok {
		lines = append(lines, mutedStyle().Render("No service detail is available."))
		return strings.Join(lines, "\n")
	}

	left := renderServiceListPane(snapshot, serviceKey(selectedService))
	right := renderServiceDetailPane(selectedService, showSecrets, layout)
	lines = append(lines, splitPane(left, right, width, defaultListPaneW))
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render(renderCopyHint(snapshot, servicesSection)))

	return strings.Join(lines, "\n")
}

func renderServiceListPane(snapshot Snapshot, selected string) string {
	lines := []string{detailHeading("Service list"), ""}
	stackServices, hostTools := splitServices(snapshot.Services)
	if len(stackServices) > 0 {
		lines = append(lines, mutedStyle().Render("Stack services"))
		for _, service := range stackServices {
			lines = append(lines, listItem(selected == serviceKey(service), service.DisplayName, statusChip(displayServiceStatus(service), displayServiceStatus(service))))
		}
	}
	if len(hostTools) > 0 {
		if len(stackServices) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, mutedStyle().Render("Host tools"))
		lines = append(lines, mutedStyle().Render("Managed outside stack lifecycle."))
		for _, service := range hostTools {
			lines = append(lines, listItem(selected == serviceKey(service), service.DisplayName, statusChip(displayServiceStatus(service), displayServiceStatus(service))))
		}
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch service"))

	return strings.Join(lines, "\n")
}

func renderServiceDetailPane(service Service, showSecrets bool, layout layoutMode) string {
	lines := []string{
		detailHeading("Service detail"),
		"",
	}
	lines = append(lines, renderServiceBlock(service, showSecrets, layout, !isStackService(service))...)
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("Logs shows scrollback. Ports shows host mappings."))

	return strings.Join(lines, "\n")
}

func renderPorts(snapshot Snapshot, selected string, width int) string {
	lines := []string{sectionTitleStyle().Render("Ports"), ""}
	portNames := selectablePortNames(snapshot)
	if len(portNames) == 0 {
		lines = append(lines, mutedStyle().Render("No exposed host ports are configured."))
		return strings.Join(lines, "\n")
	}

	service, ok := selectedPortService(snapshot, selected)
	if !ok {
		lines = append(lines, mutedStyle().Render("No port detail is available."))
		return strings.Join(lines, "\n")
	}

	left := renderPortListPane(snapshot, serviceKey(service))
	right := renderPortDetailPane(service)
	lines = append(lines, splitPane(left, right, width, defaultListPaneW))
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render(renderCopyHint(snapshot, portsSection)))

	return strings.Join(lines, "\n")
}

func renderPortListPane(snapshot Snapshot, selected string) string {
	lines := []string{detailHeading("Exposed ports"), ""}
	for _, service := range snapshot.Services {
		if service.ExternalPort <= 0 {
			continue
		}
		label := fmt.Sprintf("%s  %d", service.DisplayName, service.ExternalPort)
		lines = append(lines, listItem(selected == serviceKey(service), label, statusChip(displayServiceStatus(service), displayServiceStatus(service))))
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch port"))

	return strings.Join(lines, "\n")
}

func renderPortDetailPane(service Service) string {
	lines := []string{
		detailHeading("Port detail"),
		"",
		renderServiceHeading(displayServiceStatus(service), service.DisplayName),
		fmt.Sprintf("Host: %s", emptyLabel(service.Host)),
	}
	lines = append(lines, servicePortLines(service)...)
	if reachability := healthReachabilityLabel(service); reachability != "" {
		lines = append(lines, fmt.Sprintf("Reachability: %s", reachability))
	}
	if note := healthNote(service); note != "" {
		lines = append(lines, mutedStyle().Render(note))
	}
	if service.URL != "" {
		lines = append(lines, fmt.Sprintf("URL: %s", service.URL))
	}
	if service.DSN != "" {
		lines = append(lines, fmt.Sprintf("DSN: %s", maskConnectionValue(service.DSN, false)))
	}
	if !isStackService(service) {
		lines = append(lines, mutedStyle().Render("Lifecycle: external to stack lifecycle"))
	}

	return strings.Join(lines, "\n")
}

func renderHealth(snapshot Snapshot, selected string, width int) string {
	lines := []string{sectionTitleStyle().Render("Health"), ""}
	if snapshot.HealthError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.HealthError), "")
	}
	if snapshot.DoctorError != "" {
		lines = append(lines, errorBannerStyle().Render(snapshot.DoctorError), "")
	}
	if len(snapshot.Services) == 0 {
		if len(snapshot.Health) == 0 && len(snapshot.DoctorChecks) == 0 {
			lines = append(lines, mutedStyle().Render("No health data loaded."))
			return strings.Join(lines, "\n")
		}

		lines = append(lines, mutedStyle().Render("Live service health is unavailable; showing raw checks instead."), "")
		for _, line := range snapshot.Health {
			lines = append(lines, healthLineStyle(line.Status).Render(fmt.Sprintf("%s %s", healthStatusIcon(line.Status), line.Message)))
		}
		findings := make([]DoctorCheck, 0, len(snapshot.DoctorChecks))
		for _, check := range snapshot.DoctorChecks {
			if check.Status == output.StatusOK {
				continue
			}
			findings = append(findings, check)
		}
		if len(findings) > 0 {
			lines = append(lines, "")
			lines = append(lines, detailHeading("Doctor findings"))
			for _, check := range findings {
				lines = append(lines, fmt.Sprintf("%s  %s", statusChip(strings.ToLower(check.Status), check.Status), check.Message))
			}
		}
		return strings.Join(lines, "\n")
	}

	selectedService, ok := selectedService(snapshot, selected)
	if !ok {
		lines = append(lines, mutedStyle().Render("No health detail is available."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, renderHealthSummary(snapshot.Services))
	lines = append(lines, "")
	left := renderHealthListPane(snapshot, serviceKey(selectedService))
	right := renderHealthDetailPane(snapshot, selectedService)
	lines = append(lines, splitPane(left, right, width, defaultListPaneW))

	return strings.Join(lines, "\n")
}

func renderHealthListPane(snapshot Snapshot, selected string) string {
	lines := []string{detailHeading("Health targets"), ""}
	stackServices, hostTools := splitServices(snapshot.Services)
	if len(stackServices) > 0 {
		lines = append(lines, mutedStyle().Render("Stack services"))
		for _, service := range stackServices {
			lines = append(lines, listItem(selected == serviceKey(service), service.DisplayName, statusChip(healthStatusLabel(service), classifyServiceHealth(service))))
		}
	}
	if len(hostTools) > 0 {
		if len(stackServices) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, mutedStyle().Render("Host tools"))
		for _, service := range hostTools {
			lines = append(lines, listItem(selected == serviceKey(service), service.DisplayName, statusChip(healthStatusLabel(service), classifyServiceHealth(service))))
		}
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch target"))
	lines = append(lines, "")
	lines = append(lines, detailHeading("Doctor summary"))
	lines = append(lines, doctorSummaryLine(snapshot.DoctorSummary))

	return strings.Join(lines, "\n")
}

func renderHealthDetailPane(snapshot Snapshot, service Service) string {
	lines := []string{detailHeading("Health detail"), ""}
	lines = append(lines, renderHealthBlock(service)...)
	lines = append(lines, "")
	lines = append(lines, detailHeading("Doctor findings"))
	findings := make([]DoctorCheck, 0, len(snapshot.DoctorChecks))
	for _, check := range snapshot.DoctorChecks {
		if check.Status == output.StatusOK {
			continue
		}
		findings = append(findings, check)
	}
	if len(findings) == 0 {
		lines = append(lines, mutedStyle().Render("No doctor findings recorded."))
		return strings.Join(lines, "\n")
	}

	for _, check := range findings {
		lines = append(lines, fmt.Sprintf("%s  %s", statusChip(strings.ToLower(check.Status), check.Status), check.Message))
	}

	return strings.Join(lines, "\n")
}

func renderLogs(snapshot Snapshot, logs logPanelState, width int) string {
	lines := []string{sectionTitleStyle().Render("Logs"), ""}
	filters := logFilters(snapshot)
	left := renderLogFilterPane(filters, logs)
	right := renderLogDetailPane(filters, logs)
	lines = append(lines, splitPane(left, right, width, defaultFilterPaneW))

	return strings.Join(lines, "\n")
}

func renderLogFilterPane(filters []logFilter, logs logPanelState) string {
	lines := []string{detailHeading("Log filters"), ""}
	for _, filter := range filters {
		label := filter.DisplayName
		if filter.Name == "" {
			label = "All services"
		}
		lines = append(lines, listItem(logs.Service == filter.Name, label, statusChip(filterChipLabel(filter), filter.Status)))
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch filter"))
	lines = append(lines, mutedStyle().Render("f toggle follow"))
	if logs.Follow {
		lines = append(lines, mutedStyle().Render("Follow interval: "+logFollowInterval.String()))
	}

	return strings.Join(lines, "\n")
}

func renderLogDetailPane(filters []logFilter, logs logPanelState) string {
	filterLabel := "All services"
	for _, filter := range filters {
		if filter.Name != logs.Service {
			continue
		}
		filterLabel = filter.DisplayName
		break
	}

	lines := []string{
		detailHeading("Log output"),
		"",
		fmt.Sprintf("Filter: %s", filterLabel),
		fmt.Sprintf("Tail: %d lines", logTailLines),
		fmt.Sprintf("Follow: %s", onOffLabel(logs.Follow)),
		fmt.Sprintf("Loaded: %s", formatLoadedAt(logs.LoadedAt)),
	}
	if logs.Loading {
		lines = append(lines, mutedStyle().Render("Loading logs..."))
	}
	if logs.Error != "" {
		lines = append(lines, errorBannerStyle().Render(logs.Error))
	}
	if strings.TrimSpace(logs.Output) == "" {
		lines = append(lines, "", mutedStyle().Render("No logs captured for this selection yet."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "", logs.Output)
	return strings.Join(lines, "\n")
}

func filterChipLabel(filter logFilter) string {
	if filter.Name == "" {
		return "all"
	}

	return filter.Name
}
