package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"personalcrm/internal/domain"
)

var (
	colorPrimary = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
	colorAccent  = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C084FC"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	colorWarn    = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colorBorder  = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"}

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 1)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 1)

	styleBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#111827")).
			Background(colorWarn).
			Padding(0, 2)

	stylePane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	stylePaneFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleSectionHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	styleBadge = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleTag = lipgloss.NewStyle().
			Foreground(colorPrimary)

	styleFooter = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleToast = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)
)

func renderStatusPill(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	var c lipgloss.AdaptiveColor
	switch s {
	case "active", "ready", "done", "promoted", "finished":
		c = colorSuccess
	case "waiting", "in_progress", "exploring":
		c = colorPrimary
	case "on_hold", "inbox", "parked", "blocked", "to_consume":
		c = colorWarn
	case "rejected", "discarded", "cancelled":
		c = colorDanger
	default:
		c = colorMuted
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render("[" + s + "]")
}

func renderPriorityPill(p domain.Priority) string {
	switch p {
	case domain.PriorityHigh:
		return lipgloss.NewStyle().Foreground(colorDanger).Bold(true).Render("HIGH")
	case domain.PriorityMedium:
		return lipgloss.NewStyle().Foreground(colorWarn).Render("MED")
	default:
		return lipgloss.NewStyle().Foreground(colorMuted).Render("LOW")
	}
}

func renderPluginBadges(badges []string) string {
	if len(badges) == 0 {
		return ""
	}
	styled := make([]string, len(badges))
	for i, b := range badges {
		styled[i] = styleBadge.Render(b)
	}
	return " " + strings.Join(styled, " ")
}

func truncate(s string, max int) string {
	if max <= 3 {
		return s
	}
	r := []rune(strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", " "))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}
