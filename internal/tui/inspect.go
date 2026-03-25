package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/traweezy/stackctl/internal/output"
)

const (
	splitPaneMinWidth   = 96
	defaultListPaneMinW = 34
	defaultListPaneMaxW = 42
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

func selectableStackNames(snapshot Snapshot) []string {
	names := make([]string, 0, len(snapshot.Stacks))
	for _, profile := range snapshot.Stacks {
		if strings.TrimSpace(profile.Name) == "" {
			continue
		}
		names = append(names, profile.Name)
	}

	return names
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

func selectedStackProfile(snapshot Snapshot, selected string) (StackProfile, bool) {
	name := pickSelectedName(selected, selectableStackNames(snapshot))
	if name == "" {
		return StackProfile{}, false
	}
	for _, profile := range snapshot.Stacks {
		if profile.Name == name {
			return profile, true
		}
	}

	return StackProfile{}, false
}

func splitPaneWidths(width int, minListPaneWidth int, maxListPaneWidth int) (int, int, bool) {
	if width < splitPaneMinWidth {
		return 0, 0, true
	}

	leftWidth := minInt(maxListPaneWidth, maxInt(minListPaneWidth, width*2/5))
	rightWidth := maxInt(24, width-leftWidth-3)
	return leftWidth, rightWidth, false
}

func splitPane(left, right string, width int, minListPaneWidth int, maxListPaneWidth int) string {
	leftWidth, rightWidth, stacked := splitPaneWidths(width, minListPaneWidth, maxListPaneWidth)
	if stacked {
		return strings.TrimSpace(left) + "\n\n" + strings.TrimSpace(right)
	}

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

func doctorSummaryLine(summary DoctorSummary) string {
	return strings.Join([]string{
		statusChip(fmt.Sprintf("ok %d", summary.OK), output.StatusOK),
		statusChip(fmt.Sprintf("warn %d", summary.Warn), output.StatusWarn),
		statusChip(fmt.Sprintf("miss %d", summary.Miss), output.StatusMiss),
		statusChip(fmt.Sprintf("fail %d", summary.Fail), output.StatusFail),
	}, " ")
}

func renderStacks(snapshot Snapshot, selected string, layout layoutMode, width int) string {
	lines := []string{sectionTitleStyle().Render("Stacks"), ""}
	if len(snapshot.Stacks) == 0 {
		lines = append(lines, mutedStyle().Render("No stack profiles are available."))
		return strings.Join(lines, "\n")
	}

	profile, ok := selectedStackProfile(snapshot, selected)
	if !ok {
		lines = append(lines, mutedStyle().Render("No stack detail is available."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, renderStackSummary(snapshot.Stacks))
	left := renderStackListPane(snapshot, profile.Name)
	right := renderStackDetailPane(profile, layout)
	lines = append(lines, splitPane(left, right, width, 30, 40))

	return strings.Join(lines, "\n")
}

func renderServices(snapshot Snapshot, showSecrets bool, layout layoutMode, selected string, width int, pinned map[string]struct{}) string {
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

	stackServices, _ := splitServices(snapshot.Services)
	lines = append(lines, renderOverviewSummary(stackServices))

	left := renderServiceListPane(snapshot, serviceKey(selectedService), pinned)
	right := renderServiceDetailPane(selectedService, showSecrets, layout, pinned)
	lines = append(lines, splitPane(left, right, width, defaultListPaneMinW, defaultListPaneMaxW))

	return strings.Join(lines, "\n")
}

func renderStackListPane(snapshot Snapshot, selected string) string {
	lines := []string{detailHeading("Stack list"), ""}
	for _, profile := range snapshot.Stacks {
		lines = append(lines, listItem(selected == profile.Name, stackListLabel(profile), statusChip(profile.State, profile.State)))
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch stack  •  g or / jump"))

	return strings.Join(lines, "\n")
}

func renderStackDetailPane(profile StackProfile, layout layoutMode) string {
	status := profile.State
	if strings.TrimSpace(status) == "" {
		status = "unknown"
	}

	lines := []string{
		renderServiceHeading(status, stackHeading(profile)),
		renderStatusLine(status),
	}

	identity := []string{
		fmt.Sprintf("Current: %s", yesNoLabel(profile.Current)),
		fmt.Sprintf("Configured: %s", yesNoLabel(profile.Configured)),
	}
	if profile.Mode != "" && profile.Mode != "-" {
		identity = append(identity, fmt.Sprintf("Mode: %s", profile.Mode))
	}

	location := []string{
		fmt.Sprintf("Config: %s", emptyLabel(profile.ConfigPath)),
	}
	if profile.Services != "" && profile.Services != "-" {
		location = append(location, fmt.Sprintf("Running services: %s", profile.Services))
	}

	workflow := stackWorkflowLines(profile, layout)

	lines = appendDetailGroup(lines, "Profile", identity)
	lines = appendDetailGroup(lines, "Location", location)
	lines = appendDetailGroup(lines, "Workflow", workflow)

	return strings.Join(lines, "\n")
}

func renderServiceListPane(snapshot Snapshot, selected string, pinned map[string]struct{}) string {
	lines := []string{detailHeading("Service list"), ""}
	pinnedServices := make([]Service, 0, len(snapshot.Services))
	stackServices := make([]Service, 0, len(snapshot.Services))
	hostTools := make([]Service, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		if _, ok := pinned[serviceKey(service)]; ok {
			pinnedServices = append(pinnedServices, service)
			continue
		}
		if isStackService(service) {
			stackServices = append(stackServices, service)
			continue
		}
		hostTools = append(hostTools, service)
	}
	if len(pinnedServices) > 0 {
		lines = append(lines, mutedStyle().Render("Pinned"))
		for _, service := range pinnedServices {
			lines = append(lines, listItem(selected == serviceKey(service), serviceListLabel(service), statusChip(displayServiceStatus(service), displayServiceStatus(service))))
		}
	}
	if len(stackServices) > 0 {
		if len(pinnedServices) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, mutedStyle().Render("Stack services"))
		for _, service := range stackServices {
			lines = append(lines, listItem(selected == serviceKey(service), serviceListLabel(service), statusChip(displayServiceStatus(service), displayServiceStatus(service))))
		}
	}
	if len(hostTools) > 0 {
		if len(stackServices) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, mutedStyle().Render("Host tools"))
		lines = append(lines, mutedStyle().Render("Managed outside stack lifecycle."))
		for _, service := range hostTools {
			lines = append(lines, listItem(selected == serviceKey(service), serviceListLabel(service), statusChip(displayServiceStatus(service), displayServiceStatus(service))))
		}
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch service  •  g or / jump"))

	return strings.Join(lines, "\n")
}

func renderStackSummary(profiles []StackProfile) string {
	current := 0
	configured := 0
	running := 0
	for _, profile := range profiles {
		if profile.Current {
			current++
		}
		if profile.Configured {
			configured++
		}
		if strings.EqualFold(strings.TrimSpace(profile.State), "running") {
			running++
		}
	}

	return strings.Join([]string{
		statusChip(fmt.Sprintf("Current: %d", current), output.StatusInfo),
		statusChip(fmt.Sprintf("Configured: %d", configured), output.StatusOK),
		statusChip(fmt.Sprintf("Running: %d", running), "running"),
	}, "  ")
}

func stackListLabel(profile StackProfile) string {
	label := profile.Name
	if strings.TrimSpace(label) == "" {
		label = "-"
	}
	if profile.Current {
		label += "  " + statusChip("current", output.StatusInfo)
	}

	return label
}

func stackHeading(profile StackProfile) string {
	if profile.Current {
		return profile.Name + " (current)"
	}

	return profile.Name
}

func stackWorkflowLines(profile StackProfile, layout layoutMode) []string {
	lines := make([]string, 0, 6)
	switch {
	case !profile.Configured:
		lines = append(lines, "Save defaults from Config to create this stack profile.")
	case profile.Current:
		lines = append(lines, "Open Config to edit this stack.")
		lines = append(lines, "Use the action rail to manage this stack directly.")
	default:
		lines = append(lines, "Use the action rail to switch the dashboard to this stack.")
		switch normalizedStackState(profile.State) {
		case "running", "stopped":
			lines = append(lines, "You can also start, stop, or restart this stack here without switching first.")
			lines = append(lines, "Use it first if you want the rest of the dashboard to follow this stack.")
		default:
			lines = append(lines, "After switching, open Config to edit it.")
		}
	}

	if profile.Configured {
		lines = append(lines, "Delete removes the profile.")
		if profile.Mode == "managed" {
			lines = append(lines, "Managed profiles also purge stackctl-owned local data.")
		}
	}
	if layout == expandedLayout {
		lines = append(lines, "The command palette includes stack-profile jump targets too.")
	}

	return lines
}

func yesNoLabel(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

func renderServiceDetailPane(service Service, showSecrets bool, layout layoutMode, pinned map[string]struct{}) string {
	lines := []string{
		detailHeading("Service detail"),
		"",
	}
	lines = append(lines, renderServiceBlock(service, showSecrets, layout, !isStackService(service))...)
	lines = append(lines, "")
	lines = append(lines, renderProductivityHint(service, pinned))

	return strings.Join(lines, "\n")
}

func renderHealth(snapshot Snapshot, selected string, width int, pinned map[string]struct{}) string {
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
		if footer := renderHealthDoctorFooter(snapshot, width); footer != "" {
			lines = append(lines, "")
			lines = append(lines, footer)
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
	left := renderHealthListPane(snapshot, serviceKey(selectedService), pinned)
	right := renderHealthDetailPane(snapshot, selectedService, pinned)
	lines = append(lines, splitPane(left, right, width, defaultListPaneMinW, defaultListPaneMaxW))
	if footer := renderHealthDoctorFooter(snapshot, width); footer != "" {
		lines = append(lines, "")
		lines = append(lines, footer)
	}

	return strings.Join(lines, "\n")
}

func renderHealthListPane(snapshot Snapshot, selected string, pinned map[string]struct{}) string {
	lines := []string{detailHeading("Health targets"), ""}
	pinnedServices := make([]Service, 0, len(snapshot.Services))
	stackServices := make([]Service, 0, len(snapshot.Services))
	hostTools := make([]Service, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		if _, ok := pinned[serviceKey(service)]; ok {
			pinnedServices = append(pinnedServices, service)
			continue
		}
		if isStackService(service) {
			stackServices = append(stackServices, service)
			continue
		}
		hostTools = append(hostTools, service)
	}
	if len(pinnedServices) > 0 {
		lines = append(lines, mutedStyle().Render("Pinned"))
		for _, service := range pinnedServices {
			lines = append(lines, listItem(selected == serviceKey(service), serviceListLabel(service), statusChip(healthStatusLabel(service), classifyServiceHealth(service))))
		}
	}
	if len(stackServices) > 0 {
		if len(pinnedServices) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, mutedStyle().Render("Stack services"))
		for _, service := range stackServices {
			lines = append(lines, listItem(selected == serviceKey(service), serviceListLabel(service), statusChip(healthStatusLabel(service), classifyServiceHealth(service))))
		}
	}
	if len(hostTools) > 0 {
		if len(stackServices) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, mutedStyle().Render("Host tools"))
		for _, service := range hostTools {
			lines = append(lines, listItem(selected == serviceKey(service), serviceListLabel(service), statusChip(healthStatusLabel(service), classifyServiceHealth(service))))
		}
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle().Render("j/k or [ ] switch target  •  g or / jump"))

	return strings.Join(lines, "\n")
}

func renderHealthDetailPane(_ Snapshot, service Service, pinned map[string]struct{}) string {
	lines := []string{detailHeading("Health detail"), ""}
	lines = append(lines, renderHealthBlock(service)...)
	lines = append(lines, "")
	lines = append(lines, renderProductivityHint(service, pinned))

	return strings.Join(lines, "\n")
}

func renderHealthDoctorFooter(snapshot Snapshot, width int) string {
	if len(snapshot.DoctorChecks) == 0 && snapshot.DoctorSummary == (DoctorSummary{}) {
		return ""
	}

	lines := []string{
		detailHeading("Doctor findings"),
		"",
		doctorSummaryLine(snapshot.DoctorSummary),
	}

	findings := doctorFindings(snapshot.DoctorChecks)
	if len(findings) == 0 {
		lines = append(lines, "", mutedStyle().Render("No doctor findings recorded."))
	} else {
		lines = append(lines, "")
		for _, check := range findings {
			lines = append(lines, fmt.Sprintf("%s  %s", statusChip(strings.ToLower(check.Status), check.Status), check.Message))
		}
	}

	panelWidth := maxInt(24, width)
	return subPaneStyle(doctorFooterColor(snapshot.DoctorSummary)).Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func doctorFindings(checks []DoctorCheck) []DoctorCheck {
	findings := make([]DoctorCheck, 0, len(checks))
	for _, check := range checks {
		if check.Status == output.StatusOK {
			continue
		}
		findings = append(findings, check)
	}

	return findings
}

func doctorFooterColor(summary DoctorSummary) string {
	switch {
	case summary.Fail > 0 || summary.Miss > 0:
		return "160"
	case summary.Warn > 0:
		return "221"
	default:
		return "28"
	}
}

func renderProductivityHint(service Service, pinned map[string]struct{}) string {
	parts := make([]string, 0, 6)
	if len(serviceCopyTargets(service, false)) > 0 {
		parts = append(parts, "c copy")
	}
	if isStackService(service) {
		parts = append(parts, "w logs", "e shell")
		if strings.EqualFold(logWatchServiceName(service), "postgres") {
			parts = append(parts, "d db shell")
		}
	}
	if _, ok := pinned[serviceKey(service)]; ok {
		parts = append(parts, "p unpin")
	} else {
		parts = append(parts, "p pin")
	}
	parts = append(parts, "g jump", ": palette")
	return mutedStyle().Render("Actions: " + strings.Join(parts, "  •  "))
}

func serviceListLabel(service Service) string {
	label := service.DisplayName
	if service.ExternalPort > 0 {
		label += fmt.Sprintf("  :%d", service.ExternalPort)
	}

	return label
}
