package components

import (
	"fmt"
	"strings"
	"testing"
)

// TestVisibleWindowKeepsCursorInView verifies the cursor always lands inside
// the rendered window, which is the whole point of the windowing. It sweeps
// every viewport height, including the short ones where a fixed scroll anchor
// would push the cursor out of its own window.
func TestVisibleWindowKeepsCursorInView(t *testing.T) {
	const total = 100

	for _, rows := range []int{1, 2, 3, 4, 5, 7, 10, 33} {
		for cursor := 0; cursor < total; cursor++ {
			start, end := visibleWindow(total, cursor, rows)
			if cursor < start || cursor >= end {
				t.Fatalf("rows %d, cursor %d outside window [%d,%d)", rows, cursor, start, end)
			}
			if end-start > rows {
				t.Fatalf("rows %d, cursor %d: window [%d,%d) larger than %d rows", rows, cursor, start, end, rows)
			}
			if start < 0 || end > total {
				t.Fatalf("rows %d, cursor %d: window [%d,%d) escapes list of %d", rows, cursor, start, end, total)
			}
		}
	}
}

// TestVisibleWindowShortViewportShowsCursor pins the short-viewport case: a
// single-row pane must still render the row the cursor is on, so the user can
// see what they are about to act on.
func TestVisibleWindowShortViewportShowsCursor(t *testing.T) {
	tests := []struct {
		rows      int
		cursor    int
		wantStart int
	}{
		{rows: 1, cursor: 99, wantStart: 99},
		{rows: 1, cursor: 0, wantStart: 0},
		{rows: 2, cursor: 50, wantStart: 49},
		{rows: 3, cursor: 50, wantStart: 48},
	}
	for _, tt := range tests {
		start, end := visibleWindow(100, tt.cursor, tt.rows)
		if start != tt.wantStart || tt.cursor < start || tt.cursor >= end {
			t.Fatalf("visibleWindow(100, %d, %d) = [%d,%d), want start %d with cursor inside",
				tt.cursor, tt.rows, start, end, tt.wantStart)
		}
	}
}

// TestVisibleWindowPositions pins the scroll-anchor behaviour: the window
// stays put near the top and only shifts once the cursor passes the anchor.
func TestVisibleWindowPositions(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		cursor    int
		rows      int
		wantStart int
		wantEnd   int
	}{
		{name: "cursor at top", total: 100, cursor: 0, rows: 10, wantStart: 0, wantEnd: 10},
		{name: "cursor within anchor", total: 100, cursor: 3, rows: 10, wantStart: 0, wantEnd: 10},
		{name: "cursor past anchor scrolls", total: 100, cursor: 8, rows: 10, wantStart: 5, wantEnd: 15},
		{name: "cursor in middle", total: 100, cursor: 50, rows: 10, wantStart: 47, wantEnd: 57},
		{name: "cursor at end clamps window", total: 100, cursor: 99, rows: 10, wantStart: 90, wantEnd: 100},
		{name: "list shorter than window", total: 4, cursor: 2, rows: 10, wantStart: 0, wantEnd: 4},
		{name: "list exactly window size", total: 10, cursor: 9, rows: 10, wantStart: 0, wantEnd: 10},
		{name: "single item", total: 1, cursor: 0, rows: 10, wantStart: 0, wantEnd: 1},
		{name: "empty list", total: 0, cursor: 0, rows: 10, wantStart: 0, wantEnd: 0},
		{name: "zero rows", total: 100, cursor: 5, rows: 0, wantStart: 0, wantEnd: 0},
		{name: "negative rows", total: 100, cursor: 5, rows: -3, wantStart: 0, wantEnd: 0},
		{name: "cursor below end is clamped", total: 20, cursor: 99, rows: 5, wantStart: 15, wantEnd: 20},
		{name: "cursor above start is clamped", total: 20, cursor: -5, rows: 5, wantStart: 0, wantEnd: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visibleWindow(tt.total, tt.cursor, tt.rows)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("visibleWindow(%d, %d, %d) = [%d,%d), want [%d,%d)",
					tt.total, tt.cursor, tt.rows, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// TestVisibleWindowIsStable verifies the window does not jitter while the
// cursor moves within the top of the list.
func TestVisibleWindowIsStable(t *testing.T) {
	var first int
	for cursor := 0; cursor <= scrollAnchor; cursor++ {
		start, _ := visibleWindow(100, cursor, 10)
		if cursor == 0 {
			first = start
			continue
		}
		if start != first {
			t.Fatalf("window scrolled at cursor %d: start %d, want %d", cursor, start, first)
		}
	}
}

// TestViewportRowsNeverNegative verifies short terminals still render a row.
func TestViewportRowsNeverNegative(t *testing.T) {
	tests := []struct {
		height int
		chrome int
		want   int
	}{
		{height: 40, chrome: 1, want: 39},
		{height: 5, chrome: 3, want: 2},
		{height: 3, chrome: 3, want: 1},
		{height: 1, chrome: 3, want: 1},
		{height: 0, chrome: 0, want: 1},
	}
	for _, tt := range tests {
		if got := viewportRows(tt.height, tt.chrome); got != tt.want {
			t.Fatalf("viewportRows(%d, %d) = %d, want %d", tt.height, tt.chrome, got, tt.want)
		}
	}
}

// TestClampCursor verifies cursor bounds for empty, normal and out-of-range
// cursors.
func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		total  int
		want   int
	}{
		{name: "empty list", cursor: 0, total: 0, want: 0},
		{name: "empty list with high cursor", cursor: 9, total: 0, want: 0},
		{name: "in range", cursor: 5, total: 10, want: 5},
		{name: "first item", cursor: 0, total: 10, want: 0},
		{name: "last item", cursor: 9, total: 10, want: 9},
		{name: "past end", cursor: 50, total: 10, want: 9},
		{name: "before start", cursor: -4, total: 10, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampCursor(tt.cursor, tt.total); got != tt.want {
				t.Fatalf("clampCursor(%d, %d) = %d, want %d", tt.cursor, tt.total, got, tt.want)
			}
		})
	}
}

// TestPageCount verifies page math for exact and ragged divisions.
func TestPageCount(t *testing.T) {
	tests := []struct {
		total int
		rows  int
		want  int
	}{
		{total: 0, rows: 10, want: 0},
		{total: 10, rows: 0, want: 0},
		{total: 1, rows: 10, want: 1},
		{total: 10, rows: 10, want: 1},
		{total: 11, rows: 10, want: 2},
		{total: 20, rows: 10, want: 2},
		{total: 21, rows: 10, want: 3},
		{total: 312, rows: 20, want: 16},
	}
	for _, tt := range tests {
		if got := pageCount(tt.total, tt.rows); got != tt.want {
			t.Fatalf("pageCount(%d, %d) = %d, want %d", tt.total, tt.rows, got, tt.want)
		}
	}
}

// TestPageIndicator verifies the footer text, and that short lists show none.
func TestPageIndicator(t *testing.T) {
	tests := []struct {
		name   string
		start  int
		end    int
		cursor int
		total  int
		rows   int
		want   string
	}{
		{name: "fits on one page", start: 0, end: 4, cursor: 0, total: 4, rows: 10, want: ""},
		{name: "empty list", start: 0, end: 0, cursor: 0, total: 0, rows: 10, want: ""},
		{name: "zero rows is safe", start: 0, end: 0, cursor: 0, total: 5, rows: 0, want: ""},
		{name: "first page", start: 0, end: 20, cursor: 0, total: 312, rows: 20, want: "1-20 of 312  (page 1/16)"},
		{name: "middle page", start: 20, end: 40, cursor: 25, total: 312, rows: 20, want: "21-40 of 312  (page 2/16)"},
		{name: "ragged last page", start: 300, end: 312, cursor: 311, total: 312, rows: 20, want: "301-312 of 312  (page 16/16)"},
		{
			// The end of a list whose size is not a multiple of the page size:
			// the window is pulled back to start at total-rows, which is not
			// page-aligned, so the page number must come from the cursor.
			name: "unaligned last page", start: 281, end: 300, cursor: 299,
			total: 300, rows: 19, want: "282-300 of 300  (page 16/16)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pageIndicator(tt.start, tt.end, tt.cursor, tt.total, tt.rows)
			if got != tt.want {
				t.Fatalf("pageIndicator(%d,%d,%d,%d,%d) = %q, want %q",
					tt.start, tt.end, tt.cursor, tt.total, tt.rows, got, tt.want)
			}
		})
	}
}

// TestPageIndicatorPageAlwaysInRange verifies the reported page never exceeds
// the page count, whatever the window and cursor are.
func TestPageIndicatorPageAlwaysInRange(t *testing.T) {
	for _, total := range []int{1, 19, 20, 100, 300, 312, 1001} {
		for _, rows := range []int{1, 2, 3, 19, 20, 25} {
			if rows > total {
				continue
			}
			for cursor := 0; cursor < total; cursor++ {
				start, end := visibleWindow(total, cursor, rows)
				got := pageIndicator(start, end, cursor, total, rows)
				if got == "" {
					continue
				}
				page, pages := parsePage(t, got)
				if page < 1 || page > pages {
					t.Fatalf("total %d rows %d cursor %d: %q reports page %d of %d",
						total, rows, cursor, got, page, pages)
				}
				if pages != pageCount(total, rows) {
					t.Fatalf("total %d rows %d: %q reports %d pages, want %d",
						total, rows, got, pages, pageCount(total, rows))
				}
			}
		}
	}
}

// TestPageIndicatorReachesFinalPage verifies the last item of an unaligned
// list reports the final page, so the footer never claims there is more below.
func TestPageIndicatorReachesFinalPage(t *testing.T) {
	for _, total := range []int{21, 100, 300, 312, 1001} {
		for _, rows := range []int{3, 7, 19, 25} {
			if rows > total {
				continue
			}
			start, end := visibleWindow(total, total-1, rows)
			got := pageIndicator(start, end, total-1, total, rows)
			if got == "" {
				continue
			}
			page, pages := parsePage(t, got)
			if page != pages {
				t.Fatalf("total %d rows %d: last item reports page %d of %d (%q)",
					total, rows, page, pages, got)
			}
		}
	}
}

// parsePage pulls "page N/M" out of an indicator string.
func parsePage(t *testing.T, indicator string) (page, pages int) {
	t.Helper()
	_, err := fmt.Sscanf(indicator[strings.LastIndex(indicator, "(")+1:], "page %d/%d)", &page, &pages)
	if err != nil {
		t.Fatalf("cannot parse indicator %q: %v", indicator, err)
	}
	return page, pages
}

// TestNeedsIndicator verifies the indicator is only shown for overflowing
// lists.
func TestNeedsIndicator(t *testing.T) {
	if needsIndicator(4, 10) {
		t.Fatal("short list should not need an indicator")
	}
	if needsIndicator(10, 10) {
		t.Fatal("exact fit should not need an indicator")
	}
	if !needsIndicator(11, 10) {
		t.Fatal("overflowing list should need an indicator")
	}
}

// TestPageIndicatorMatchesWindow checks the indicator and the window agree
// for a long list, so the reported range is never off by one.
func TestPageIndicatorMatchesWindow(t *testing.T) {
	const total, rows = 312, 20
	for cursor := 0; cursor < total; cursor++ {
		start, end := visibleWindow(total, cursor, rows)
		got := pageIndicator(start, end, cursor, total, rows)
		if got == "" {
			t.Fatalf("cursor %d: expected indicator for overflowing list", cursor)
		}
		if !strings.Contains(got, "of 312") {
			t.Fatalf("cursor %d: indicator missing total: %q", cursor, got)
		}
		if !strings.HasPrefix(got, fmt.Sprintf("%d-%d of 312", start+1, end)) {
			t.Fatalf("cursor %d: indicator range %q does not match window [%d,%d)",
				cursor, got, start, end)
		}
	}
}
