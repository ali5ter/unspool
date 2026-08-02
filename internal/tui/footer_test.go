package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestStatusLine_NarrowWidth_KeepsNoticeVisible covers issue #13: at a
// terminal width too narrow for the full hint legend + quota meter to fit
// on one row, the combined line used to wrap onto a second terminal row,
// silently pushing renderNotice()'s row (where "added to <playlist>" and
// similar confirmations render) out of the footer's fixed row budget —
// computed correctly but never actually visible. statusLine must instead
// drop trailing hints so line one still fits, keeping the notice on its
// own visible row.
func TestStatusLine_NarrowWidth_KeepsNoticeVisible(t *testing.T) {
	m := Model{
		activeTab:   tabLiked,
		width:       60,
		quotaSpent:  17,
		quotaBudget: 10000,
		statusMsg:   "added to DIY",
	}

	lines := strings.SplitN(m.statusLine(), "\n", 2)
	if len(lines) != 2 {
		t.Fatalf("statusLine() = %d lines, want exactly 2 (hints+quota, then notice)", len(lines))
	}
	line1, notice := lines[0], lines[1]

	if w := lipgloss.Width(line1); w > m.width-2 {
		t.Errorf("line1 width = %d, want <= %d (m.width - styleStatusBar padding)", w, m.width-2)
	}
	if !strings.Contains(notice, "added to DIY") {
		t.Errorf("notice row = %q, want it to contain the full status message", notice)
	}
}

// TestStatusLine_WideWidth_KeepsAllHints is the regression guard the
// narrow-width fix above needs: plenty of room must not drop any hints.
func TestStatusLine_WideWidth_KeepsAllHints(t *testing.T) {
	m := Model{
		activeTab:   tabLiked,
		width:       200,
		quotaSpent:  17,
		quotaBudget: 10000,
		statusMsg:   "loaded liked videos",
	}

	line1 := strings.SplitN(m.statusLine(), "\n", 2)[0]
	for _, h := range m.footerHints() {
		if !strings.Contains(line1, h.label) {
			t.Errorf("line1 = %q, want it to still contain hint %q at a wide width", line1, h.label)
		}
	}
}
