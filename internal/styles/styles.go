// Package styles centralizes the lipgloss theme for cloudflui.
package styles

import "charm.land/lipgloss/v2"

// Cloudflare brand palette.
var (
	Accent  = lipgloss.Color("#f6821f")
	Success = lipgloss.Color("#2ecc71")
	Error   = lipgloss.Color("#e74c3c")
	Text    = lipgloss.Color("#e6e6e6")
	Muted   = lipgloss.Color("#8a8a8a")
	Dim     = lipgloss.Color("#5a5a5a")
)

// Title is the app header.
var Title = lipgloss.NewStyle().Bold(true).Foreground(Accent)

// Tab is an inactive pane tab.
var Tab = lipgloss.NewStyle().Foreground(Muted).Padding(0, 1)

// TabActive is the selected pane tab.
var TabActive = lipgloss.NewStyle().Bold(true).Foreground(Text).Background(Accent).Padding(0, 1)

// Pane is the inactive pane frame.
var Pane = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Dim).Padding(0, 1)

// PaneActive is the focused pane frame.
var PaneActive = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Accent).Padding(0, 1)

// Selected highlights the current list item.
var Selected = lipgloss.NewStyle().Bold(true).Foreground(Accent)

// Dimmed renders secondary list items.
var Dimmed = lipgloss.NewStyle().Foreground(Dim)

// ErrorText renders error messages.
var ErrorText = lipgloss.NewStyle().Bold(true).Foreground(Error)

// SuccessText renders success messages.
var SuccessText = lipgloss.NewStyle().Bold(true).Foreground(Success)

// StatusBar is the bottom status line.
var StatusBar = lipgloss.NewStyle().Foreground(Text).Background(Accent).Padding(0, 1)

// HelpBar is the keybinding hint line.
var HelpBar = lipgloss.NewStyle().Foreground(Muted)

// Label renders a form field label.
var Label = lipgloss.NewStyle().Bold(true).Foreground(Text)

// Value renders a form field value.
var Value = lipgloss.NewStyle().Foreground(Text)

// FieldActive highlights the focused form field.
var FieldActive = lipgloss.NewStyle().Bold(true).Foreground(Accent)

// ProxiedOn renders an enabled proxy toggle.
var ProxiedOn = lipgloss.NewStyle().Bold(true).Foreground(Success)

// ProxiedOff renders a disabled proxy toggle.
var ProxiedOff = lipgloss.NewStyle().Foreground(Dim)

// Clip truncates a styled string to width columns, preserving any ANSI
// styling. Long values (TXT record bodies, CNAME targets) would otherwise wrap
// onto a second terminal line and break the pane's height budget. A width of
// zero or less leaves the string untouched.
func Clip(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}
