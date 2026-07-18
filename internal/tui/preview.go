package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
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

// renderDescription renders a video description as Glamour-styled markdown,
// wrapped to width. Returns "" for an empty description rather than
// rendering an empty block.
func renderDescription(desc string, width int) string {
	if desc == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width))
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
// width and padding.
func (m Model) renderPreviewPane(height int) string {
	return lipgloss.NewStyle().
		Width(previewWidth(m.width)).
		Height(height).
		Padding(0, 2, 0, 1).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(colorLine).
		Render(m.previewContent)
}
