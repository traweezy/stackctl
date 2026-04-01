package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type theme struct {
	hasDarkBackground bool

	titleForeground      color.Color
	titleBackground      color.Color
	subtitleForeground   color.Color
	metaForeground       color.Color
	sidebarBorder        color.Color
	navForeground        color.Color
	navActiveForeground  color.Color
	navActiveBackground  color.Color
	mainBorder           color.Color
	sectionForeground    color.Color
	subsectionForeground color.Color
	mutedForeground      color.Color
	errorForeground      color.Color
	errorBackground      color.Color
	footerForeground     color.Color
	paletteBorder        color.Color
	paletteInput         color.Color
	listSelectedFg       color.Color
	listSelectedBg       color.Color
	listForeground       color.Color

	statusOK         color.Color
	statusInfo       color.Color
	statusWarn       color.Color
	statusStopped    color.Color
	statusFail       color.Color
	statusDefault    color.Color
	bannerOKFg       color.Color
	bannerOKBg       color.Color
	bannerWarnFg     color.Color
	bannerWarnBg     color.Color
	bannerFailFg     color.Color
	bannerFailBg     color.Color
	bannerInfoFg     color.Color
	bannerInfoBg     color.Color
	chipForeground   color.Color
	chipSuccessBg    color.Color
	chipWarnFg       color.Color
	chipWarnBg       color.Color
	chipFailBg       color.Color
	chipInfoBg       color.Color
	chipNeutralFg    color.Color
	chipNeutralBg    color.Color
	fieldInvalidFg   color.Color
	fieldInvalidBg   color.Color
	fieldSavedFg     color.Color
	fieldSavedBg     color.Color
	fieldPendingFg   color.Color
	fieldPendingBg   color.Color
	subtlePaneBorder color.Color
	strongPaneBorder color.Color
}

var currentTheme = newTheme(true)

func setThemeDark(isDark bool) {
	currentTheme = newTheme(isDark)
}

func activeTheme() theme {
	return currentTheme
}

func newTheme(isDark bool) theme {
	pick := lipgloss.LightDark(isDark)

	return theme{
		hasDarkBackground:    isDark,
		titleForeground:      pick(lipgloss.Color("#0f172a"), lipgloss.Color("#f8fafc")),
		titleBackground:      pick(lipgloss.Color("#bfdbfe"), lipgloss.Color("#1d4ed8")),
		subtitleForeground:   pick(lipgloss.Color("#334155"), lipgloss.Color("#cbd5e1")),
		metaForeground:       pick(lipgloss.Color("#475569"), lipgloss.Color("#93c5fd")),
		sidebarBorder:        pick(lipgloss.Color("#cbd5e1"), lipgloss.Color("#334155")),
		navForeground:        pick(lipgloss.Color("#334155"), lipgloss.Color("#e2e8f0")),
		navActiveForeground:  pick(lipgloss.Color("#0f172a"), lipgloss.Color("#eff6ff")),
		navActiveBackground:  pick(lipgloss.Color("#dbeafe"), lipgloss.Color("#1d4ed8")),
		mainBorder:           pick(lipgloss.Color("#93c5fd"), lipgloss.Color("#3b82f6")),
		sectionForeground:    pick(lipgloss.Color("#1d4ed8"), lipgloss.Color("#7dd3fc")),
		subsectionForeground: pick(lipgloss.Color("#2563eb"), lipgloss.Color("#93c5fd")),
		mutedForeground:      pick(lipgloss.Color("#475569"), lipgloss.Color("#94a3b8")),
		errorForeground:      pick(lipgloss.Color("#7f1d1d"), lipgloss.Color("#fef2f2")),
		errorBackground:      pick(lipgloss.Color("#fecaca"), lipgloss.Color("#b91c1c")),
		footerForeground:     pick(lipgloss.Color("#475569"), lipgloss.Color("#94a3b8")),
		paletteBorder:        pick(lipgloss.Color("#60a5fa"), lipgloss.Color("#60a5fa")),
		paletteInput:         pick(lipgloss.Color("#0f172a"), lipgloss.Color("#f8fafc")),
		listSelectedFg:       pick(lipgloss.Color("#0f172a"), lipgloss.Color("#eff6ff")),
		listSelectedBg:       pick(lipgloss.Color("#dbeafe"), lipgloss.Color("#1d4ed8")),
		listForeground:       pick(lipgloss.Color("#334155"), lipgloss.Color("#e2e8f0")),

		statusOK:      pick(lipgloss.Color("#166534"), lipgloss.Color("#4ade80")),
		statusInfo:    pick(lipgloss.Color("#1d4ed8"), lipgloss.Color("#60a5fa")),
		statusWarn:    pick(lipgloss.Color("#b45309"), lipgloss.Color("#facc15")),
		statusStopped: pick(lipgloss.Color("#c2410c"), lipgloss.Color("#fb923c")),
		statusFail:    pick(lipgloss.Color("#b91c1c"), lipgloss.Color("#f87171")),
		statusDefault: pick(lipgloss.Color("#334155"), lipgloss.Color("#cbd5e1")),

		bannerOKFg:   pick(lipgloss.Color("#f8fafc"), lipgloss.Color("#0f172a")),
		bannerOKBg:   pick(lipgloss.Color("#15803d"), lipgloss.Color("#4ade80")),
		bannerWarnFg: pick(lipgloss.Color("#0f172a"), lipgloss.Color("#0f172a")),
		bannerWarnBg: pick(lipgloss.Color("#facc15"), lipgloss.Color("#fde047")),
		bannerFailFg: pick(lipgloss.Color("#fef2f2"), lipgloss.Color("#fef2f2")),
		bannerFailBg: pick(lipgloss.Color("#b91c1c"), lipgloss.Color("#dc2626")),
		bannerInfoFg: pick(lipgloss.Color("#eff6ff"), lipgloss.Color("#eff6ff")),
		bannerInfoBg: pick(lipgloss.Color("#2563eb"), lipgloss.Color("#1d4ed8")),

		chipForeground: pick(lipgloss.Color("#f8fafc"), lipgloss.Color("#0f172a")),
		chipSuccessBg:  pick(lipgloss.Color("#15803d"), lipgloss.Color("#4ade80")),
		chipWarnFg:     pick(lipgloss.Color("#0f172a"), lipgloss.Color("#0f172a")),
		chipWarnBg:     pick(lipgloss.Color("#facc15"), lipgloss.Color("#fde047")),
		chipFailBg:     pick(lipgloss.Color("#dc2626"), lipgloss.Color("#dc2626")),
		chipInfoBg:     pick(lipgloss.Color("#2563eb"), lipgloss.Color("#1d4ed8")),
		chipNeutralFg:  pick(lipgloss.Color("#0f172a"), lipgloss.Color("#0f172a")),
		chipNeutralBg:  pick(lipgloss.Color("#cbd5e1"), lipgloss.Color("#94a3b8")),

		fieldInvalidFg:   pick(lipgloss.Color("#fef2f2"), lipgloss.Color("#fef2f2")),
		fieldInvalidBg:   pick(lipgloss.Color("#dc2626"), lipgloss.Color("#dc2626")),
		fieldSavedFg:     pick(lipgloss.Color("#0f172a"), lipgloss.Color("#0f172a")),
		fieldSavedBg:     pick(lipgloss.Color("#4ade80"), lipgloss.Color("#4ade80")),
		fieldPendingFg:   pick(lipgloss.Color("#0f172a"), lipgloss.Color("#0f172a")),
		fieldPendingBg:   pick(lipgloss.Color("#fde047"), lipgloss.Color("#fde047")),
		subtlePaneBorder: pick(lipgloss.Color("#cbd5e1"), lipgloss.Color("#334155")),
		strongPaneBorder: pick(lipgloss.Color("#93c5fd"), lipgloss.Color("#3b82f6")),
	}
}
