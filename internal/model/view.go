package model

import (
	"strings"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"cloudflui/internal/styles"
)

var tabNames = []string{"Zones", "Records", "Editor", "Config", "Workers"}

// viewChrome is the number of terminal rows the layout spends on everything
// outside the pane itself: the title, the tab row, the status bar and the
// help bar.
const viewChrome = 4

// paneBorder is the number of rows the pane's rounded border adds on top of
// the height requested from lipgloss, which treats Height as the total.
const paneBorder = 2

// framePadding is the horizontal padding the pane style adds inside its
// border, on each side.
const framePadding = 2

const helpText = "tab/1-5: switch  j/k: move  pgup/pgdn: page  g/G: top/bottom  enter: select  n: new  p: proxy  d: delete  s: save  r: refresh  esc: back  q: quit"

// helpCompact is the fallback shown when the terminal is too narrow for the
// full help line. It is always clipped to the terminal width as a backstop,
// because a wrapped help bar would silently add a row to the layout.
const helpCompact = "j/k move  pgup/pgdn page  g/G ends  n new  p proxy  d del  r reload  q quit"

// helpForWidth returns the help line that fits the terminal.
func helpForWidth(width int) string {
	if width >= lipgloss.Width(helpText) {
		return helpText
	}
	return helpCompact
}

// View renders the full TUI layout.
func (m *Model) View() tea.View {
	content := m.activePane().View()

	// Frame the active pane. MaxHeight is a hard backstop: Height only pads,
	// so without it a pane whose content overflows (for example rows that
	// wrap) would push the status and help bars off the top of the screen.
	// The budget is only applied once the terminal size is known; before that
	// the content is rendered as-is rather than truncated to a guess.
	frame := styles.Pane
	if m.active == PaneZones {
		frame = styles.PaneActive
	}
	style := frame.Width(m.paneWidth())
	if m.height > 0 {
		height := m.paneHeight()
		style = style.Height(height).MaxHeight(height)
	}
	pane := style.Render(content)

	// Tabs.
	var tabs []string
	for i, name := range tabNames {
		if i == m.active {
			tabs = append(tabs, styles.TabActive.Render(name))
		} else {
			tabs = append(tabs, styles.Tab.Render(name))
		}
	}
	// The tab row has a fixed width set by the five labels. Clip it so a
	// narrow terminal hard-wraps into a taller, corrupt layout rather than
	// simply dropping the tail of the row.
	tabRow := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	tabRow = lipgloss.NewStyle().MaxWidth(m.width).Render(tabRow)

	// Header.
	header := lipgloss.JoinVertical(lipgloss.Left,
		styles.Title.Render("cloudflui"),
		tabRow,
	)

	// Status + help. The help bar is clipped to the terminal width so it can
	// never wrap and inflate the rendered height.
	statusStyle := styles.StatusBar
	if m.width > 0 {
		statusStyle = statusStyle.Width(m.width)
	}
	status := statusStyle.MaxWidth(m.width).Render(m.status)
	help := styles.HelpBar.MaxWidth(m.width).Render(helpForWidth(m.width))

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		pane,
		status,
		help,
	)

	v := tea.NewView(body)
	// Take over the whole terminal. In Bubble Tea v2 this is the replacement
	// for v1's tea.WithAltScreen() program option; without it the TUI renders
	// inline in the normal screen buffer and scrolls the user's shell.
	v.AltScreen = true
	return v
}

// paneWidth returns the total width of the framed pane. lipgloss treats Width
// as the total including the border, so this fills the terminal edge to edge.
func (m *Model) paneWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 0
}

// paneContentWidth returns the columns available to pane content once the
// border and the frame's horizontal padding are removed. This is what the
// list panes use to clip their rows.
func (m *Model) paneContentWidth() int {
	w := m.paneWidth() - paneBorder - framePadding
	if w < 1 {
		return 1
	}
	return w
}

// paneHeight returns the total height of the framed pane, including its
// border. It is clamped to a usable minimum so a very short terminal still
// renders something rather than a zero-height box.
func (m *Model) paneHeight() int {
	h := m.height - viewChrome
	if h < paneBorder+1 {
		return paneBorder + 1
	}
	return h
}

// paneContentHeight returns the rows available to pane content once the
// border is removed. This is what gets handed to the list panes so their
// pagination math matches the space actually available.
func (m *Model) paneContentHeight() int {
	h := m.paneHeight() - paneBorder
	if h < 1 {
		return 1
	}
	return h
}

// statusText is a helper for tests and debugging.
func (m *Model) statusText() string {
	return strings.TrimSpace(m.status)
}
