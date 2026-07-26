package tui

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/thumbnail"
)

// previewMinWidth is the terminal width below which the preview pane is
// dropped entirely and the list takes the full width (PRD §7.1 "Narrow").
const previewMinWidth = 90

// previewThumbnailRows is the fixed row budget a thumbnail occupies in the
// preview pane. thumbnail.Render pads its output to span exactly this many
// embedded newlines, so once spliced into previewContent the existing
// line-counting scroll math (windowLines/previewScrollMax) accounts for it
// automatically — no separate height-budget special-casing needed here.
const previewThumbnailRows = 10

// previewFraction is the preview pane's share of the content width, mirroring
// wwlog's list/detail 1/3–2/3 split (there the list is the narrow pane; here
// the preview is, since the list is unspool's primary content surface).
func previewWidth(totalWidth int) int {
	return totalWidth / 3
}

// refreshPreview regenerates the cached preview content when the selected
// item, or the detail column's width, has changed. Glamour rendering isn't
// cheap enough to redo on every View() call (which fires on every message,
// including spinner ticks), so the result is cached and only recomputed on
// an actual change. previewWidthUsed is keyed on the detail column's own
// outer width, not m.width directly — Feed/Queue/Liked's detail column and
// the Playlists tab's third column are sized differently at the same
// terminal width, so comparing against raw m.width could wrongly reuse
// content wrapped for a different width after switching tabs.
//
// Returns a non-nil tea.Cmd only when a thumbnail needs fetching/rendering
// for the newly-selected video — the caller (Update) must batch it in,
// since refreshPreview itself can't return through the normal Update/cmd
// path (it's called after updateInner already produced one).
func (m *Model) refreshPreview() tea.Cmd {
	video, channel, ok := m.selectedVideo()
	if !ok {
		m.previewVideoID = ""
		m.previewContent = styleMeta.Render("Nothing selected.")
		return nil
	}
	outerW := m.detailColumnOuterWidth()
	if outerW == 0 {
		// No detail column visible at this width/tab — nothing to render,
		// and no point paying for a Glamour pass no one will see.
		m.previewVideoID = video.VideoID
		m.previewWidthUsed = outerW
		m.previewContent = ""
		return nil
	}
	if video.VideoID == m.previewVideoID && m.previewWidthUsed == outerW {
		return nil
	}
	m.previewVideoID = video.VideoID
	m.previewWidthUsed = outerW
	m.previewScroll = 0

	w := columnContentWidth(outerW)
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
	// The video ID/URL isn't shown anywhere else in the UI — surfaced here
	// so it's copy-able for manual debugging (e.g. testing a
	// classifier.inspect_command script directly against the same video
	// `i` would inspect).
	if video.VideoID != "" {
		lines = append(lines, styleMeta.Render("https://www.youtube.com/watch?v="+video.VideoID))
	}

	if desc := renderDescription(video.Description, w); desc != "" {
		lines = append(lines, "", desc)
	}

	m.previewBody = lipgloss.JoinVertical(lipgloss.Left, lines...)

	if m.thumbnailsDisabled || video.VideoID == "" {
		m.previewContent = m.previewBody
		return nil
	}
	key := thumbnailCacheKey(video.VideoID, w)
	if thumb, ok := m.thumbnailCache[key]; ok {
		m.previewContent = composePreviewWithThumbnail(thumb, m.previewBody)
		return nil
	}
	m.previewContent = m.previewBody
	return loadThumbnailCmd(m.cfg, video.VideoID, w)
}

// thumbnailCacheKey is keyed on cols only (rows is the fixed
// previewThumbnailRows constant) — a terminal resize that changes the
// preview column's width naturally invalidates a stale cached render.
func thumbnailCacheKey(videoID string, cols int) string {
	return videoID + "@" + strconv.Itoa(cols)
}

func composePreviewWithThumbnail(thumb, body string) string {
	return lipgloss.JoinVertical(lipgloss.Left, thumb, "", body)
}

// thumbnailLoadedMsg carries the result of a loadThumbnailCmd back to the
// model. key is the cache key the render was requested for; videoID is the
// staleness guard, compared against m.previewVideoID at receipt — same
// shape as inspectDoneMsg's videoID check.
type thumbnailLoadedMsg struct {
	videoID string
	key     string
	output  string
	err     error
}

// loadThumbnailCmd fetches (and caches on disk) videoID's thumbnail, then
// renders it via chafa at cols wide / previewThumbnailRows tall. Runs off
// the update loop — network plus a subprocess, so it must not block —
// matching how sync/inspect/search-escalation are already handled.
func loadThumbnailCmd(cfg *config.Config, videoID string, cols int) tea.Cmd {
	return func() tea.Msg {
		key := thumbnailCacheKey(videoID, cols)
		ctx := context.Background()
		cacheDir := filepath.Join(cfg.StoreDir, "thumbnails")
		path, err := thumbnail.Fetch(ctx, videoID, cacheDir)
		if err != nil {
			return thumbnailLoadedMsg{videoID: videoID, key: key, err: err}
		}
		out, err := thumbnail.Render(ctx, path, cols, previewThumbnailRows, cfg.Thumbnails)
		return thumbnailLoadedMsg{videoID: videoID, key: key, output: out, err: err}
	}
}

// handleThumbnailLoaded caches a rendered thumbnail (success only — a
// failure isn't cached, so the next time this video is selected it's
// simply retried rather than remembered as permanently broken) and, if the
// video is still the one selected, patches it into the already-rendered
// preview immediately rather than waiting for some unrelated change to
// pick it up — same idea as handleInspectDone's patchInspectedBadge.
func (m Model) handleThumbnailLoaded(msg thumbnailLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil // best-effort — a missing/failed thumbnail is never surfaced to the user
	}
	if m.thumbnailCache == nil {
		m.thumbnailCache = map[string]string{}
	}
	m.thumbnailCache[msg.key] = msg.output
	if msg.videoID == m.previewVideoID {
		m.previewContent = composePreviewWithThumbnail(msg.output, m.previewBody)
	}
	return m, nil
}

// renderDescription renders a video description as Glamour-styled markdown,
// wrapped to width. Returns "" for an empty description rather than
// rendering an empty block.
//
// Always uses the dark style explicitly (styles.DarkStyle), never
// glamour.WithAutoStyle(). unspool's own chrome (styles.go) is a fixed dark
// palette regardless of the terminal's actual background — it never adapts
// — so there was never a reason to detect the terminal's background at all.
// WithAutoStyle calls termenv.HasDarkBackground(), which queries the
// terminal over OSC 11 on the same stdin Bubble Tea's own input reader is
// concurrently blocked reading from. The two compete for the same bytes:
// Bubble Tea's reader can consume the terminal's OSC reply before termenv
// sees it, so termenv hangs waiting (up to its 5s timeout) while Bubble
// Tea's own reader may now be desynced reading a stray reply as if it were
// keyboard input. A single cached query (tried first) still executes this
// race once — that was enough to freeze all key handling for ~13s and, in
// a later report, apparently wedge the read loop badly enough that only
// ctrl+c's SIGINT (delivered by the tty driver, not through the byte
// stream both sides read) could break the program out of it. Never query
// the terminal at all while Bubble Tea owns stdin — hardcoding the style
// removes the race entirely rather than narrowing its window.
func renderDescription(desc string, width int) string {
	if desc == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(styles.DarkStyle), glamour.WithWordWrap(width))
	if err != nil {
		return styleMeta.Render(desc)
	}
	out, err := r.Render(desc)
	if err != nil {
		return styleMeta.Render(desc)
	}
	return out
}

// renderDetailContent windows the cached preview content to height
// starting at m.previewScroll (see keys.Up/Down, intercepted by
// handleGlobalKey while the detail column has focus) — the box itself
// (width, border, focus color) is applied by the caller via columnBox,
// the same as every other column, not here.
//
// A long description (which is unbounded, unlike everything else in the
// preview) could otherwise grow past the column's height budget and push
// the status bar (and everything below it) off the bottom of the
// terminal — windowLines truncates at the content level, and columnBox's
// own MaxHeight is a hard backstop in case the rendered result still ends
// up taller than the raw line count suggests.
//
// The clamp here is local to this render, not written back to
// m.previewScroll — View() must not mutate model state. handleGlobalKey
// clamps the stored value too when adjusting it, so this is a defensive
// second pass (e.g. covering a resize that shrank height since the last
// scroll keypress), not the only place clamping happens.
func (m Model) renderDetailContent(height int) string {
	return windowLines(m.previewContent, m.previewScroll, height)
}

// windowLines returns at most n lines of s starting at offset (clamped into
// range), marking either edge with "…" when scrolling would reveal more
// content in that direction.
func windowLines(s string, offset, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}

	maxOffset := len(lines) - n
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + n

	visible := append([]string{}, lines[offset:end]...)
	if offset > 0 {
		visible[0] = styleMeta.Render("…")
	}
	if end < len(lines) {
		visible[len(visible)-1] = styleMeta.Render("…")
	}
	return strings.Join(visible, "\n")
}

// previewScrollMax returns the largest valid previewScroll for the given
// content and visible height — clamping here (rather than only inside
// windowLines) keeps the stored value itself always valid, so a repeated
// ScrollDown press near the bottom is a no-op instead of silently
// accumulating an offset far past what's ever displayed.
func previewScrollMax(content string, height int) int {
	if height <= 0 {
		return 0
	}
	lines := strings.Count(content, "\n") + 1
	max := lines - height
	if max < 0 {
		return 0
	}
	return max
}
