package components

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbletea/v2"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/zones"

	"cloudflui/internal/api"
	"cloudflui/internal/messages"
	"cloudflui/internal/styles"
)

// recordChrome is the number of rows the records pane spends on its own
// chrome: the zone title, a blank spacer, and the page indicator.
const recordChrome = 3

// RecordsPane lists DNS records for the selected zone.
type RecordsPane struct {
	client  *api.Client
	zone    *zones.Zone
	records []dns.RecordResponse
	cursor  int
	width   int
	height  int
	loading bool
	err     string
}

// NewRecordsPane creates the records pane.
func NewRecordsPane(client *api.Client) *RecordsPane {
	return &RecordsPane{client: client, height: defaultViewportHeight}
}

// SetViewport records the content area the pane may render into.
func (p *RecordsPane) SetViewport(width, height int) {
	p.width = width
	p.height = height
}

// listRows returns how many record rows fit in the current viewport.
func (p *RecordsPane) listRows() int {
	return viewportRows(p.height, recordChrome)
}

// moveCursor sets the cursor, clamped to the list bounds.
func (p *RecordsPane) moveCursor(to int) {
	p.cursor = clampCursor(to, len(p.records))
}

// IsEditing reports whether the pane is in text-entry mode.
func (p *RecordsPane) IsEditing() bool { return false }

// SetZone selects the zone whose records are shown.
func (p *RecordsPane) SetZone(z zones.Zone) {
	p.zone = &z
	p.records = nil
	p.cursor = 0
	p.err = ""
}

// Load fetches records for the selected zone asynchronously.
func (p *RecordsPane) Load() tea.Cmd {
	if p.zone == nil {
		return nil
	}
	p.loading = true
	p.err = ""
	zoneID := p.zone.ID
	return func() tea.Msg {
		records, err := p.client.ListRecords(context.Background(), zoneID)
		return messages.RecordsLoaded{Records: records, Err: err}
	}
}

// Update handles messages for the records pane.
func (p *RecordsPane) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case messages.RecordsLoaded:
		p.loading = false
		if msg.Err != nil {
			p.err = msg.Err.Error()
			return nil
		}
		p.err = ""
		p.records = msg.Records
		// Keep the cursor where it was so a refresh does not lose your place,
		// but pull it back in range if the list shrank.
		p.cursor = clampCursor(p.cursor, len(p.records))
	case messages.RecordSaved:
		if msg.Err == nil {
			return p.Load()
		}
		p.err = msg.Err.Error()
	case messages.RecordDeleted:
		if msg.Err == nil {
			return p.Load()
		}
		p.err = msg.Err.Error()
	case tea.KeyPressMsg:
		if p.zone == nil {
			return nil
		}
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
			p.moveCursor(len(p.records) - 1)
		case tea.KeyEnter:
			if len(p.records) == 0 {
				return nil
			}
			zone := *p.zone
			rec := p.records[p.cursor]
			return func() tea.Msg { return messages.EditRecord{Zone: zone, Record: rec} }
		case 'n':
			zone := *p.zone
			return func() tea.Msg { return messages.NewRecord{Zone: zone} }
		case 'p':
			if len(p.records) == 0 {
				return nil
			}
			return p.toggleProxy()
		case 'd':
			if len(p.records) == 0 {
				return nil
			}
			return p.deleteRecord()
		case tea.KeyEsc:
			return func() tea.Msg { return messages.Navigate{Pane: 0} }
		case 'r':
			return p.Load()
		}
	}
	return nil
}

func (p *RecordsPane) toggleProxy() tea.Cmd {
	zoneID := p.zone.ID
	rec := p.records[p.cursor]
	return func() tea.Msg {
		_, err := p.client.UpdateRecord(
			context.Background(),
			zoneID,
			rec.ID,
			rec.Name,
			string(rec.Type),
			rec.Content,
			int(rec.TTL),
			!rec.Proxied,
			rec.Comment,
		)
		return messages.RecordSaved{Err: err}
	}
}

func (p *RecordsPane) deleteRecord() tea.Cmd {
	zoneID := p.zone.ID
	rec := p.records[p.cursor]
	return func() tea.Msg {
		err := p.client.DeleteRecord(context.Background(), zoneID, rec.ID)
		return messages.RecordDeleted{Err: err}
	}
}

// View renders the records list.
func (p *RecordsPane) View() string {
	var b strings.Builder
	if p.zone == nil {
		b.WriteString(styles.Dimmed.Render("Select a zone to view its DNS records."))
		return b.String()
	}
	b.WriteString(styles.Title.Render("Zone: "+p.zone.Name) + "\n\n")
	if p.loading {
		b.WriteString(styles.Dimmed.Render("Loading records..."))
		return b.String()
	}
	if p.err != "" {
		b.WriteString(styles.ErrorText.Render(p.err))
		return b.String()
	}
	if len(p.records) == 0 {
		b.WriteString(styles.Dimmed.Render("No records. Press n to create one."))
		return b.String()
	}
	rows := p.listRows()
	start, end := visibleWindow(len(p.records), p.cursor, rows)
	for i := start; i < end; i++ {
		r := p.records[i]
		proxy := " "
		if r.Proxied {
			proxy = styles.ProxiedOn.Render("●")
		}
		line := fmt.Sprintf("%s %-6s %-40s %s", proxy, r.Type, r.Name, r.Content)
		b.WriteString(p.renderRow(i, line) + "\n")
	}
	if indicator := pageIndicator(start, end, p.cursor, len(p.records), rows); indicator != "" {
		b.WriteString(styles.HelpBar.Render(indicator))
	}
	return b.String()
}

// renderRow styles a list line, marking the cursor row, and clips the result
// to the pane width so a long value cannot wrap onto a second line.
func (p *RecordsPane) renderRow(index int, line string) string {
	if index == p.cursor {
		return styles.Clip(styles.Selected.Render("> "+line), p.width)
	}
	return styles.Clip(styles.Dimmed.Render("  "+line), p.width)
}
