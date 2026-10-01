package components

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/workers"
	"github.com/cloudflare/cloudflare-go/v7/zones"

	"cloudflui/internal/messages"
)

// makeZones builds n zones named zone-0000..zone-NNNN.
func makeZones(n int) []zones.Zone {
	out := make([]zones.Zone, n)
	for i := range out {
		out[i] = zones.Zone{
			ID:     fmt.Sprintf("zoneid-%04d", i),
			Name:   fmt.Sprintf("zone-%04d", i),
			Status: zones.ZoneStatusActive,
		}
	}
	return out
}

// makeRecords builds n records named rec-0000..rec-NNNN.
func makeRecords(n int) []dns.RecordResponse {
	out := make([]dns.RecordResponse, n)
	for i := range out {
		out[i] = dns.RecordResponse{
			ID:      fmt.Sprintf("recid-%04d", i),
			Name:    fmt.Sprintf("rec-%04d", i),
			Type:    dns.RecordResponseType("A"),
			Content: "192.0.2.1",
			Proxied: i%2 == 0,
			TTL:     300,
		}
	}
	return out
}

// makeWorkersScripts builds n scripts whose IDs carry a 4-digit index.
func makeWorkersScripts(n int) []workers.ScriptListResponse {
	out := make([]workers.ScriptListResponse, n)
	for i := range out {
		out[i] = workers.ScriptListResponse{
			ID:        fmt.Sprintf("script-%04d", i),
			CreatedOn: time.Unix(int64(i)*86400, 0).UTC(),
		}
	}
	return out
}

// countRendered reports how many of n possible items appear in the view.
func countRendered(view string, n int) int {
	found := 0
	for i := 0; i < n; i++ {
		if strings.Contains(view, fmt.Sprintf("%04d", i)) {
			found++
		}
	}
	return found
}

// TestZonesPaneWindowsLongList verifies a large zone list is windowed to the
// viewport instead of rendering every row.
func TestZonesPaneWindowsLongList(t *testing.T) {
	const total = 300
	p := NewZonesPane(nil)
	p.SetViewport(80, 20)
	p.Update(messages.ZonesLoaded{Zones: makeZones(total)})

	view := p.View()
	// 20 rows of viewport minus the 1 row of pane chrome.
	if got, want := countRendered(view, total), 19; got != want {
		t.Fatalf("rendered %d zone rows, want %d", got, want)
	}
	if !strings.Contains(view, "1-19 of 300") {
		t.Fatalf("expected first-page indicator, got:\n%s", view)
	}
}

// TestZonesPaneEndKeyShowsLastPage verifies 'G' jumps to the final item and
// the window follows it there.
func TestZonesPaneEndKeyShowsLastPage(t *testing.T) {
	const total = 300
	p := NewZonesPane(nil)
	p.SetViewport(80, 20)
	p.Update(messages.ZonesLoaded{Zones: makeZones(total)})

	p.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})

	view := p.View()
	if !strings.Contains(view, "zone-0299") {
		t.Fatalf("expected the last zone to be visible, got:\n%s", view)
	}
	if !strings.Contains(view, "of 300") {
		t.Fatalf("expected indicator to report the full total, got:\n%s", view)
	}
	// The reported range must end at the last item, not past it.
	if !strings.Contains(view, "of 300") || !strings.Contains(view, "-300 ") {
		t.Fatalf("expected the indicator to end at item 300, got:\n%s", view)
	}
	// Still only a viewport's worth of rows.
	if got := countRendered(view, total); got != 19 {
		t.Fatalf("rendered %d rows at end of list, want 19", got)
	}
}

// TestZonesPanePageDownAdvancesAWindow verifies PageDown moves by a window
// and the indicator follows.
func TestZonesPanePageDownAdvancesAWindow(t *testing.T) {
	const total = 300
	p := NewZonesPane(nil)
	p.SetViewport(80, 20)
	p.Update(messages.ZonesLoaded{Zones: makeZones(total)})

	p.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})

	view := p.View()
	if !strings.Contains(view, "of 300") {
		t.Fatalf("expected an indicator after paging, got:\n%s", view)
	}
	// The first page's top item must have scrolled away.
	if strings.Contains(view, "zone-0000") {
		t.Fatalf("expected the window to scroll past zone-0000, got:\n%s", view)
	}
}

// TestZonesPanePageUpAtTopIsSafe verifies paging up from the top clamps
// instead of going negative.
func TestZonesPanePageUpAtTopIsSafe(t *testing.T) {
	p := NewZonesPane(nil)
	p.SetViewport(80, 20)
	p.Update(messages.ZonesLoaded{Zones: makeZones(300)})

	p.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})

	view := p.View()
	if !strings.Contains(view, "zone-0000") {
		t.Fatalf("expected to stay at the top of the list, got:\n%s", view)
	}
	if got := countRendered(view, 300); got != 19 {
		t.Fatalf("rendered %d rows, want 19", got)
	}
}

// TestZonesPaneShortListHasNoIndicator verifies small lists are not cluttered
// with a pointless page footer.
func TestZonesPaneShortListHasNoIndicator(t *testing.T) {
	p := NewZonesPane(nil)
	p.SetViewport(80, 40)
	p.Update(messages.ZonesLoaded{Zones: makeZones(4)})

	view := p.View()
	if got := countRendered(view, 4); got != 4 {
		t.Fatalf("rendered %d rows, want all 4", got)
	}
	if strings.Contains(view, "page") {
		t.Fatalf("expected no page indicator for a short list, got:\n%s", view)
	}
}

// TestZonesPaneNarrowTerminalStillRenders verifies a cramped terminal renders
// at least one row rather than an empty pane.
func TestZonesPaneNarrowTerminalStillRenders(t *testing.T) {
	p := NewZonesPane(nil)
	p.SetViewport(20, 1)
	p.Update(messages.ZonesLoaded{Zones: makeZones(300)})

	if got := countRendered(p.View(), 300); got < 1 {
		t.Fatal("expected at least one row to render in a tiny viewport")
	}
}

// TestRecordsPaneClipsLongRows verifies a long record value (TXT bodies are
// routinely hundreds of characters) is clipped to the pane width instead of
// wrapping onto a second terminal line and breaking the height budget.
func TestRecordsPaneClipsLongRows(t *testing.T) {
	p := NewRecordsPane(nil)
	p.SetViewport(80, 30)
	p.SetZone(zones.Zone{ID: "z1", Name: "example.com"})

	long := makeRecords(5)
	for i := range long {
		long[i].Content = strings.Repeat("v", 300)
	}
	p.Update(messages.RecordsLoaded{Records: long})

	view := p.View()
	if got := lipgloss.Width(view); got > 80 {
		t.Fatalf("view is %d columns wide, want at most 80", got)
	}
	// Each record must still occupy exactly one line, so the pane keeps its
	// full row budget rather than losing rows to wrapped content.
	for _, rec := range long {
		if !strings.Contains(view, rec.Name) {
			t.Fatalf("record %q was not rendered; wrapping consumed the row budget", rec.Name)
		}
	}
	// 5 records fit in the viewport, so the pane emits the zone title, a
	// spacer and one line per record. Any extra line would mean a record's
	// content wrapped instead of being clipped.
	lines := strings.Count(strings.TrimRight(view, "\n"), "\n") + 1
	if lines != 7 {
		t.Fatalf("view rendered %d lines, want 7 (title, spacer, 5 records)", lines)
	}
}

// TestRecordsPaneWindowsLongList verifies records are windowed too.
func TestRecordsPaneWindowsLongList(t *testing.T) {
	const total = 500
	p := NewRecordsPane(nil)
	p.SetViewport(120, 30)
	p.SetZone(zones.Zone{ID: "z1", Name: "example.com"})
	p.Update(messages.RecordsLoaded{Records: makeRecords(total)})

	view := p.View()
	// 30 rows minus title, spacer and indicator.
	if got, want := countRendered(view, total), 27; got != want {
		t.Fatalf("rendered %d record rows, want %d", got, want)
	}
	if !strings.Contains(view, "1-27 of 500") {
		t.Fatalf("expected first-page indicator, got:\n%s", view)
	}
}

// TestRecordsPaneKeepsCursorOnRefresh verifies a reload keeps your place in
// a long list instead of snapping back to the top.
func TestRecordsPaneKeepsCursorOnRefresh(t *testing.T) {
	p := NewRecordsPane(nil)
	p.SetViewport(120, 30)
	p.SetZone(zones.Zone{ID: "z1", Name: "example.com"})
	p.Update(messages.RecordsLoaded{Records: makeRecords(500)})

	for i := 0; i < 50; i++ {
		p.Update(tea.KeyPressMsg{Code: tea.KeyDown, Text: "j"})
	}
	p.Update(messages.RecordsLoaded{Records: makeRecords(500)})

	view := p.View()
	if strings.Contains(view, "1-27 of 500") {
		t.Fatalf("expected the window to stay near row 50, got:\n%s", view)
	}
	if !strings.Contains(view, "0050") {
		t.Fatalf("expected the cursor to stay at record 0050, got:\n%s", view)
	}
}

// TestRecordsPaneCursorClampsWhenListShrinks verifies the cursor is pulled
// back in range when a refresh returns fewer records.
func TestRecordsPaneCursorClampsWhenListShrinks(t *testing.T) {
	p := NewRecordsPane(nil)
	p.SetViewport(120, 30)
	p.SetZone(zones.Zone{ID: "z1", Name: "example.com"})
	p.Update(messages.RecordsLoaded{Records: makeRecords(500)})

	for i := 0; i < 100; i++ {
		p.Update(tea.KeyPressMsg{Code: tea.KeyDown, Text: "j"})
	}
	p.Update(messages.RecordsLoaded{Records: makeRecords(3)})

	// Enter must not panic on an out-of-range cursor.
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if p.cursor != 2 {
		t.Fatalf("cursor = %d after shrink, want 2", p.cursor)
	}
}

// TestRecordsPaneNoZoneIsSafe verifies paging keys are inert before a zone is
// chosen.
func TestRecordsPaneNoZoneIsSafe(t *testing.T) {
	p := NewRecordsPane(nil)
	p.SetViewport(120, 30)

	p.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	p.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown, Text: "j"})

	if p.cursor != 0 {
		t.Fatalf("cursor = %d with no zone selected, want 0", p.cursor)
	}
}

// TestWorkersPaneWindowsLongList verifies the workers script list paginates.
func TestWorkersPaneWindowsLongList(t *testing.T) {
	const total = 200
	p := NewWorkersPane(nil)
	p.SetViewport(100, 20)
	p.scripts = makeWorkersScripts(total)
	p.cursor = 0
	p.loading = false

	view := p.View()
	if !strings.Contains(view, "of 200") {
		t.Fatalf("expected a page indicator for the scripts list, got:\n%s", view)
	}
	if got := countRendered(view, total); got != 17 {
		t.Fatalf("rendered %d script rows, want 17 (20 minus title, spacer, indicator)", got)
	}
}
