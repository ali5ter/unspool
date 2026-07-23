package tui

import "charm.land/lipgloss/v2"

// columnKind distinguishes columns whose Up/Down should navigate a
// list.Model from the read-only detail/preview column, whose Up/Down
// instead scrolls it. See (Model).focusedColumnKind and
// handleGlobalKey's Up/Down interception.
type columnKind int

const (
	columnList columnKind = iota
	columnDetail
)

// columnBorderWidth/columnBorderHeight are how much space columnBox's
// border and padding cost a column's content budget: 1 border + 1 padding
// cell per horizontal side, 1 border row per vertical side (no vertical
// padding — that would cost rows for no real benefit here). lipgloss's
// Width()/Height() are border-box (outer-inclusive: Width(20) with a
// 1-cell border and 1-cell padding on each side renders content into the
// remaining 16 columns, not 20), but they don't reflow pre-rendered
// multi-line content (a list.Model's own View() output) to fit — short
// lines get padded, long/tall content overflows. So callers must size
// their content (list.Model.SetSize, preview word-wrap/windowing) to
// columnContentWidth/columnContentHeight themselves before handing it to
// columnBox.
const (
	columnBorderWidth  = 4
	columnBorderHeight = 2
)

// columnBox wraps content in a focus-aware rounded box — colorTeal border
// when this column has keyboard focus, colorLine otherwise. Every column
// (list or detail) gets the same treatment, so which one currently has
// focus is always visually unambiguous. A full box (all four sides,
// rounded corners — matching the modal/splash dialogs' own
// RoundedBorder) reads as "this is one focused thing" more clearly than
// the left/right-only vertical rules used originally: two adjacent
// columns' abutting rules were easy to misread as belonging to either
// side, whereas a rounded corner unambiguously belongs to one box. The
// tradeoff is 2 rows of vertical content space per column, previously
// free — accepted after live feedback that clarity mattered more here.
//
// Deliberately NOT colorAccent: a list's own selected row already uses
// colorAccent (styleSelected, set in New()) and keeps showing it on
// whichever row the cursor sits on regardless of column focus — reusing
// the same color for "this column has focus" made the two indicators
// read as one signal instead of two, especially since an unfocused
// column's list still shows its own accent-colored selected row.
func columnBox(content string, outerWidth, outerHeight int, focused bool) string {
	c := colorLine
	if focused {
		c = colorTeal
	}
	return lipgloss.NewStyle().
		Width(outerWidth).
		Height(outerHeight).
		MaxHeight(outerHeight).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c).
		Render(content)
}

// columnContentWidth returns how wide a column's own content (a
// list.Model's SetSize width, or the detail pane's word-wrap width) needs
// to be so it fits exactly inside columnBox's border and padding.
func columnContentWidth(outerWidth int) int {
	w := outerWidth - columnBorderWidth
	if w < 0 {
		return 0
	}
	return w
}

// columnContentHeight returns how tall a column's own content (a
// list.Model's SetSize height, or the detail pane's windowed line count)
// needs to be so it fits exactly inside columnBox's top/bottom border.
func columnContentHeight(outerHeight int) int {
	h := outerHeight - columnBorderHeight
	if h < 0 {
		return 0
	}
	return h
}

// playlistsDetailMinWidth is the "remaining" width (total minus the
// playlists column) below which Playlists' third column (video detail) is
// dropped. Deliberately its own, lower threshold rather than reusing
// listWidthFor/previewWidth — those are tuned for Feed/Queue/Liked's
// two-column split (their own internal gate is previewMinWidth, 90), and
// reusing them here meant the third column stayed hidden until a much
// wider terminal than actually needed, since the playlists column had
// already eaten into what was left. A dedicated, smaller minimum lets a
// typical ~110-120 column terminal show all three.
const playlistsDetailMinWidth = 60

// playlistsColumnWidths returns the outer width of each of the Playlists
// tab's visible columns (playlists, that playlist's videos, selected video
// detail) for the given total terminal width. Narrower terminals drop
// columns from the right — the same degrade-gracefully approach
// Feed/Queue/Liked already use for their preview pane below
// previewMinWidth.
func playlistsColumnWidths(totalWidth int) []int {
	if totalWidth < previewMinWidth {
		return []int{totalWidth}
	}

	plWidth := totalWidth / 4
	switch {
	case plWidth < 22:
		plWidth = 22
	case plWidth > 40:
		plWidth = 40
	}
	remaining := totalWidth - plWidth

	if remaining < playlistsDetailMinWidth {
		return []int{plWidth, remaining}
	}
	detailWidth := remaining / 3
	if detailWidth < 20 {
		detailWidth = 20
	}
	itemsWidth := remaining - detailWidth
	return []int{plWidth, itemsWidth, detailWidth}
}

// activeColumnCount returns how many columns the active tab currently
// shows, given the terminal width.
func (m Model) activeColumnCount() int {
	if m.activeTab == tabPlaylists {
		return len(playlistsColumnWidths(m.width))
	}
	if m.width >= previewMinWidth {
		return 2
	}
	return 1
}

// focusedColumnKind reports whether the currently focused column is a
// navigable list (Up/Down moves its selection) or the read-only
// detail/preview column (Up/Down scrolls it instead) — see
// handleGlobalKey.
func (m Model) focusedColumnKind() columnKind {
	n := m.activeColumnCount()
	if m.activeTab == tabPlaylists {
		if n == 3 && m.focusedColumn == 2 {
			return columnDetail
		}
		return columnList
	}
	if n == 2 && m.focusedColumn == 1 {
		return columnDetail
	}
	return columnList
}

// detailColumnOuterWidth returns the current tab's detail/preview
// column's outer width, or 0 if none is visible at the current terminal
// width — used both to size wrapped preview content and to render its box.
func (m Model) detailColumnOuterWidth() int {
	if m.activeTab == tabPlaylists {
		w := playlistsColumnWidths(m.width)
		if len(w) == 3 {
			return w[2]
		}
		return 0
	}
	if m.activeColumnCount() == 2 {
		return previewWidth(m.width)
	}
	return 0
}
