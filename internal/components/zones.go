// Package components contains the five TUI panes.
package components

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbletea/v2"
	"github.com/cloudflare/cloudflare-go/v7/zones"

	"cloudflui/internal/api"
	"cloudflui/internal/messages"
	"cloudflui/internal/styles"
)

// zoneChrome is the number of rows the zones pane spends on its own chrome:
// the page indicator below the list.
const zoneChrome = 1

// ZonesPane lists Cloudflare zones.
type ZonesPane struct {
	client  *api.Client
	zones   []zones.Zone
	cursor  int
	width   int
	height  int
	loading bool
	err     string
}

// NewZonesPane creates the zones pane.
func NewZonesPane(client *api.Client) *ZonesPane {
	return &ZonesPane{client: client, height: defaultViewportHeight}
}

// SetViewport records the content area the pane may render into.
func (p *ZonesPane) SetViewport(width, height int) {
	p.width = width
	p.height = height
}

// listRows returns how many zone rows fit in the current viewport.
func (p *ZonesPane) listRows() int {
	return viewportRows(p.height, zoneChrome)
}

// IsEditing reports whether the pane is in text-entry mode.
func (p *ZonesPane) IsEditing() bool { return false }

// Load fetches zones asynchronously.
func (p *ZonesPane) Load() tea.Cmd {
	if p.client == nil {
		p.err = "No API token configured. Add a profile in the Config pane."
		return nil
	}
	p.loading = true
	p.err = ""
	return func() tea.Msg {
		zones, err := p.client.ListZones(context.Background())
		return messages.ZonesLoaded{Zones: zones, Err: err}
	}
}

// Update handles messages for the zones pane.
func (p *ZonesPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case messages.ZonesLoaded:
		p.loading = false
		if msg.Err != nil {
			p.err = msg.Err.Error()
			return nil
		}
		p.err = ""
		p.zones = msg.Zones
		// Keep the cursor where it was so a refresh does not lose your place,
		// but pull it back in range if the list shrank.
		p.cursor = clampCursor(p.cursor, len(p.zones))
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			p.moveCursor(p.cursor - 1)
		case tea.KeyDown, 'j':
			p.moveCursor(p.cursor + 1)
		case tea.KeyPgUp:
			p.moveCursor(p.cursor - p.listRows())
		case tea.KeyPgDown:
			p.moveCursor(p.cursor + p.listRows())
		case 'g', tea.KeyHome:
			p.moveCursor(0)
		case 'G', tea.KeyEnd:
			p.moveCursor(len(p.zones) - 1)
		case tea.KeyEnter:
			if len(p.zones) == 0 {
				return nil
			}
			zone := p.zones[p.cursor]
			return func() tea.Msg { return messages.ZoneSelected{Zone: zone} }
		case 'r':
			return p.Load()
		}
	}
	return nil
}

// moveCursor sets the cursor, clamped to the list bounds.
func (p *ZonesPane) moveCursor(to int) {
	p.cursor = clampCursor(to, len(p.zones))
}

// View renders the zones list.
func (p *ZonesPane) View() string {
	var b strings.Builder
	if p.loading {
		b.WriteString(styles.Dimmed.Render("Loading zones..."))
		return b.String()
	}
	if p.err != "" {
		b.WriteString(styles.ErrorText.Render(p.err))
		return b.String()
	}
	if len(p.zones) == 0 {
		b.WriteString(styles.Dimmed.Render("No zones found. Press r to refresh."))
		return b.String()
	}
	rows := p.listRows()
	start, end := visibleWindow(len(p.zones), p.cursor, rows)
	for i := start; i < end; i++ {
		line := fmt.Sprintf(" %s  %s", statusDot(p.zones[i].Status), p.zones[i].Name)
		if i == p.cursor {
			b.WriteString(styles.Clip(styles.Selected.Render("> "+line), p.width))
		} else {
			b.WriteString(styles.Clip(styles.Dimmed.Render("  "+line), p.width))
		}
		b.WriteString("\n")
	}
	if indicator := pageIndicator(start, end, p.cursor, len(p.zones), rows); indicator != "" {
		b.WriteString(styles.HelpBar.Render(indicator))
	}
	return b.String()
}

func statusDot(status zones.ZoneStatus) string {
	switch status {
	case zones.ZoneStatusActive:
		return styles.SuccessText.Render("●")
	case zones.ZoneStatusPending:
		return styles.ProxiedOff.Render("◌")
	default:
		return styles.Dimmed.Render("○")
	}
}
