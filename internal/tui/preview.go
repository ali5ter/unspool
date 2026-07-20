package tui

import (
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/termenv"
)

// previewMinWidth is the terminal width below which the preview pane is
// dropped entirely and the list takes the full width (PRD §7.1 "Narrow").
const previewMinWidth = 90

// previewFraction is the preview pane's share of the content width, mirroring
// wwlog's list/detail 1/3–2/3 split (there the list is the narrow pane; here
// the preview is, since the list is unspool's primary content surface).
func previewWidth(totalWidth int) int {
	return totalWidth / 3
}

// refreshPreview regenerates the cached preview content when the selected
// item has changed. Glamour rendering isn't cheap enough to redo on every
// View() call (which fires on every message, including spinner ticks), so
// the result is cached and only recomputed on an actual selection change.
func (m *Model) refreshPreview() {
	video, channel, ok := m.selectedVideo()
	if !ok {
		m.previewVideoID = ""
		m.previewContent = styleMeta.Render("Nothing selected.")
		return
	}
	if video.VideoID == m.previewVideoID && m.previewWidthUsed == m.width {
		return
	}
	m.previewVideoID = video.VideoID
	m.previewWidthUsed = m.width

	w := previewWidth(m.width) - 4 // account for padding
	if w < 20 {
		w = 20
	}

	var lines []string
	lines = append(lines, styleTitle.Render(video.Title))
	if channel != "" {
		lines = append(lines, styleMeta.Render(channel))
	}
	meta := humanAge(video.PublishedAt) + " ago · " + humanDuration(video.DurationSeconds)
	if video.ContainsSyntheticMedia {
		meta += "  ◆ synthetic media"
	}
	lines = append(lines, styleMeta.Render(meta))

	if desc := renderDescription(video.Description, w); desc != "" {
		lines = append(lines, "", desc)
	}

	m.previewContent = lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// glamourStyleName resolves once, on first use, to "dark" or "light" and is
// cached for the rest of the process. glamour.WithAutoStyle() re-detects
// this on every call via termenv.HasDarkBackground(), which queries the
// terminal (OSC 11) and blocks synchronously waiting for a reply — up to
// termenv's 5s OSCTimeout. renderDescription runs on the main Update()
// goroutine every time the selected video changes (i.e. on ordinary up/down
// navigation), so re-querying per keystroke could freeze all key handling
// for seconds at a time — confirmed live via an asciinema recording showing
// a ~13s stall after one such query went unanswered by the terminal in
// time. The terminal's background doesn't change mid-session, so querying
// once and reusing the answer is both correct and the actual fix.
var (
	styleNameOnce sync.Once
	styleName     string
)

func glamourStyleName() string {
	styleNameOnce.Do(func() {
		if termenv.HasDarkBackground() {
			styleName = styles.DarkStyle
		} else {
			styleName = styles.LightStyle
		}
	})
	return styleName
}

// renderDescription renders a video description as Glamour-styled markdown,
// wrapped to width. Returns "" for an empty description rather than
// rendering an empty block.
func renderDescription(desc string, width int) string {
	if desc == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(glamourStyleName()), glamour.WithWordWrap(width))
	if err != nil {
		return styleMeta.Render(desc)
	}
	out, err := r.Render(desc)
	if err != nil {
		return styleMeta.Render(desc)
	}
	return out
}

// renderPreviewPane wraps the cached preview content in the pane's fixed
// width and padding, clipped to height. lipgloss's Height() only pads short
// content up to the minimum — it doesn't truncate long content — so a long
// description (which is unbounded, unlike everything else in the preview)
// could otherwise grow the pane past its budget and push the status bar
// (and everything below it) off the bottom of the terminal entirely.
// clipLines handles this at the content level (with a visible "…" marker);
// MaxHeight is a hard backstop in case the styled/padded/bordered render
// still ends up taller than the raw line count suggests.
func (m Model) renderPreviewPane(height int) string {
	content := clipLines(m.previewContent, height)
	return lipgloss.NewStyle().
		Width(previewWidth(m.width)).
		Height(height).
		MaxHeight(height).
		Padding(0, 2, 0, 1).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(colorLine).
		Render(content)
}

// clipLines truncates s to at most n lines, marking the cut with an
// ellipsis on its own line when content was actually dropped.
func clipLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	kept := lines[:n-1]
	kept = append(kept, styleMeta.Render("…"))
	return strings.Join(kept, "\n")
}
