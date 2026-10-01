package components

import "fmt"

// defaultViewportHeight is assumed before the terminal has reported its
// size, so the first frame is already paginated rather than unbounded.
const defaultViewportHeight = 24

// scrollAnchor is how many rows from the top of the window the cursor is
// preferred to sit at. Keeping the cursor near the top means the list scrolls
// only once the cursor passes the anchor, instead of shifting on every
// keypress the way a centred cursor would.
const scrollAnchor = 3

// viewportRows returns how many list rows fit in a viewport of the given
// height once a pane's own chrome is subtracted. Never returns less than one,
// so a very short terminal still renders one row rather than none.
func viewportRows(height, chrome int) int {
	rows := height - chrome
	if rows < 1 {
		return 1
	}
	return rows
}

// clampCursor keeps a cursor index inside a list of total items. An empty
// list always yields 0.
func clampCursor(cursor, total int) int {
	switch {
	case total <= 0:
		return 0
	case cursor < 0:
		return 0
	case cursor > total-1:
		return total - 1
	default:
		return cursor
	}
}

// visibleWindow returns the [start, end) bounds of the slice to render for a
// list of total items with the cursor at cursor, in a window of rows. The
// cursor is always inside the returned window, and the window is stable: it
// only moves once the cursor passes the scroll anchor or reaches the end.
func visibleWindow(total, cursor, rows int) (start, end int) {
	if total <= 0 || rows <= 0 {
		return 0, 0
	}
	if rows > total {
		rows = total
	}
	cursor = clampCursor(cursor, total)

	// The anchor must leave room for the cursor itself. In a very short
	// viewport (rows < scrollAnchor+1) an unclamped anchor would scroll the
	// cursor out of its own window.
	anchor := scrollAnchor
	if anchor > rows-1 {
		anchor = rows - 1
	}

	start = cursor - anchor
	if start < 0 {
		start = 0
	}
	// Pull the window back so it never runs past the end of the list.
	if start+rows > total {
		start = total - rows
	}
	if start < 0 {
		start = 0
	}
	return start, start + rows
}

// pageCount returns how many pages of rows items the list spans.
func pageCount(total, rows int) int {
	if total <= 0 || rows <= 0 {
		return 0
	}
	return (total + rows - 1) / rows
}

// needsIndicator reports whether a list is too tall to show at once and so
// needs a page indicator. Short lists stay indicator-free to avoid clutter.
func needsIndicator(total, rows int) bool {
	return total > rows
}

// pageIndicator renders the "showing X-Y of Z" footer for a list window. The
// page number is derived from the cursor rather than the window top, because
// the window is a scrolling viewport that is not page-aligned once it is
// pulled back at the end of a list. It returns an empty string when there is
// nothing worth showing.
func pageIndicator(start, end, cursor, total, rows int) string {
	if rows <= 0 || !needsIndicator(total, rows) {
		return ""
	}
	pages := pageCount(total, rows)
	page := cursor/rows + 1
	if page > pages {
		page = pages
	}
	return fmt.Sprintf("%d-%d of %d  (page %d/%d)", start+1, end, total, page, pages)
}
