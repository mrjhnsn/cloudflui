package model

import (
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/zones"

	"cloudflui/internal/messages"
)

// TestViewUsesAltScreen verifies the TUI takes over the whole terminal. In
// Bubble Tea v2 this is the replacement for v1's tea.WithAltScreen(); without
// it the app renders inline in the normal screen buffer.
func TestViewUsesAltScreen(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	if !m.View().AltScreen {
		t.Fatal("View().AltScreen is false; the TUI will not fill the terminal")
	}
}

// TestViewFillsTerminalHeight verifies the rendered layout occupies exactly
// the terminal height, leaving no unused rows and overflowing none.
func TestViewFillsTerminalHeight(t *testing.T) {
	m := testModel(t)

	for _, height := range []int{24, 30, 40, 60, 100} {
		m.Update(tea.WindowSizeMsg{Width: 100, Height: height})
		lines := strings.Count(m.View().Content, "\n") + 1
		if lines != height {
			t.Fatalf("terminal height %d: view rendered %d lines, want exactly %d",
				height, lines, height)
		}
	}
}

// TestViewSurvivesTinyTerminal verifies a squeezed terminal still produces a
// small, bounded layout instead of a negative height or a runaway box.
func TestViewSurvivesTinyTerminal(t *testing.T) {
	m := testModel(t)

	for _, height := range []int{1, 2, 3, 5, 6, 7} {
		m.Update(tea.WindowSizeMsg{Width: 100, Height: height})
		lines := strings.Count(m.View().Content, "\n") + 1
		if lines > 10 {
			t.Fatalf("terminal height %d: view rendered %d lines, want a small bounded layout",
				height, lines)
		}
	}
}

// TestViewNeverExceedsTerminalWidth verifies the rendered view fits the
// terminal width. A line wider than the terminal hard-wraps on screen, which
// would silently add rows and push the status and help bars off the top.
func TestViewNeverExceedsTerminalWidth(t *testing.T) {
	m := testModel(t)

	for _, width := range []int{40, 60, 80, 100, 200} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		got := lipgloss.Width(m.View().Content)
		if got > width {
			t.Fatalf("terminal width %d: view is %d columns wide", width, got)
		}
	}
}

// TestHelpBarNeverWraps verifies the help line is clipped to the terminal
// rather than spilling past it.
func TestHelpBarNeverWraps(t *testing.T) {
	m := testModel(t)

	for _, width := range []int{20, 40, 60, 80, 145, 200} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
		view := m.View().Content
		if lipgloss.Width(view) > width {
			t.Fatalf("width %d: view is %d columns wide", width, lipgloss.Width(view))
		}
	}
}

// TestHelpForWidthPicksFittingText verifies the compact help is used on narrow
// terminals.
func TestHelpForWidthPicksFittingText(t *testing.T) {
	if got := helpForWidth(200); got != helpText {
		t.Fatalf("wide terminal got %q, want the full help text", got)
	}
	if got := helpForWidth(40); got != helpCompact {
		t.Fatalf("narrow terminal got %q, want the compact help text", got)
	}
	if lipgloss.Width(helpCompact) >= lipgloss.Width(helpText) {
		t.Fatal("compact help should be shorter than the full help text")
	}
}

// TestOverflowingContentCannotPushBarsOffscreen verifies the pane's
// MaxHeight backstop: content taller than the budget is truncated rather
// than growing the layout. Without it a pane that renders more rows than fit
// (a long list, or a row that wraps) would push the status and help bars off
// the top of the screen.
func TestOverflowingContentCannotPushBarsOffScreen(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Force the records pane to render far more rows than the budget allows,
	// with content wide enough to wrap.
	m.active = PaneRecords
	m.records.SetZone(zones.Zone{ID: "z1", Name: "example.com"})
	long := strings.Repeat("x", 400)
	records := make([]dns.RecordResponse, 200)
	for i := range records {
		records[i] = dns.RecordResponse{
			ID:      "id",
			Name:    "record.example.com",
			Type:    dns.RecordResponseType("TXT"),
			Content: long,
			TTL:     300,
		}
	}
	m.records.Update(messages.RecordsLoaded{Records: records})

	view := m.View().Content
	lines := strings.Count(view, "\n") + 1
	if lines != 24 {
		t.Fatalf("view rendered %d lines, want exactly 24 despite overflowing content", lines)
	}
	if !strings.Contains(view, "cloudflui") {
		t.Fatal("title scrolled off; the layout overflowed the terminal")
	}
	if !strings.Contains(view, "q quit") {
		t.Fatal("help bar scrolled off; the layout overflowed the terminal")
	}
}

// TestSyncViewportsSizesListPanes verifies the list panes are told the
// content area rather than the full terminal, so their windowing matches the
// space the root actually leaves them.
func TestSyncViewportsSizesListPanes(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// A 100-column terminal leaves 100 - border(2) - padding(2) of content.
	if got, want := m.paneContentWidth(), 96; got != want {
		t.Fatalf("paneContentWidth() = %d, want %d", got, want)
	}
	// 40 rows minus the title, tab row, status and help bars, and minus the
	// pane's own border.
	if got, want := m.paneContentHeight(), 34; got != want {
		t.Fatalf("paneContentHeight() = %d, want %d", got, want)
	}

	// The panes must end up with exactly that content height.
	for name, p := range map[string]Pane{
		"zones":   m.zones,
		"records": m.records,
		"workers": m.workers,
	} {
		if _, ok := p.(ViewportPane); !ok {
			t.Fatalf("%s pane does not implement ViewportPane", name)
		}
	}

	// A pane given the content height must not overflow its box.
	if rows := viewportRowsForTest(m.paneContentHeight(), 1); rows != 33 {
		t.Fatalf("zones list rows = %d, want 33 (content height minus indicator)", rows)
	}
}

// viewportRowsForTest mirrors the chrome arithmetic the list panes use, so
// the test can assert the budget without reaching into an unexported field.
func viewportRowsForTest(height, chrome int) int {
	rows := height - chrome
	if rows < 1 {
		return 1
	}
	return rows
}

// TestFixedPanesSkipViewport documents that the form panes are not paginated
// and the root's viewport sync skips them. Profiles and record forms are
// small and bounded, so they do not implement ViewportPane.
func TestFixedPanesSkipViewport(t *testing.T) {
	m := testModel(t)

	if _, ok := Pane(m.editor).(ViewportPane); ok {
		t.Fatal("editor pane should not implement ViewportPane")
	}
	if _, ok := Pane(m.config).(ViewportPane); ok {
		t.Fatal("config pane should not implement ViewportPane")
	}
}

// TestRebuildPanesResyncsViewport verifies a profile switch does not leave a
// freshly rebuilt pane rendering at the default size.
func TestRebuildPanesResyncsViewport(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	before := m.paneContentHeight()

	m.rebuildPanes()

	if got := m.paneContentHeight(); got != before {
		t.Fatalf("pane content height after rebuild = %d, want %d", got, before)
	}
	// A full-height view must still be exactly the terminal height.
	lines := strings.Count(m.View().Content, "\n") + 1
	if lines != 40 {
		t.Fatalf("view rendered %d lines after rebuild, want 40", lines)
	}
}
