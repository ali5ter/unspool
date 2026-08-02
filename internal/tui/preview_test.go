package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/feed"
	"github.com/ali5ter/unspool/internal/store"
)

// newTestModelForPreview builds a minimal Model with a single Feed-tab
// video selected and a wide enough terminal for the detail column to be
// visible — same "minimal Model in-package" pattern as
// newTestModelForRecommended.
func newTestModelForPreview(t *testing.T, videoID string) Model {
	t.Helper()
	m := Model{
		cfg:            &config.Config{},
		activeTab:      tabFeed,
		width:          160,
		height:         40,
		feedList:       list.New(nil, list.NewDefaultDelegate(), 0, 0),
		thumbSpinner:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		thumbnailCache: map[string]string{},
	}
	m.feedList.SetItems([]list.Item{feedItem{Item: feed.Item{
		Video:   store.Video{VideoID: videoID, Title: "Test Video"},
		Channel: "Test Channel",
	}}})
	return m
}

// TestRefreshPreview_UncachedThumbnail_ReservesPlaceholder covers issue
// #14: selecting a video whose thumbnail isn't cached yet must reserve
// the same row budget a real thumbnail would occupy (via
// thumbnailPlaceholder) rather than leaving it out until the render
// arrives, and must mark thumbLoading so the placeholder animates.
func TestRefreshPreview_UncachedThumbnail_ReservesPlaceholder(t *testing.T) {
	m := newTestModelForPreview(t, "vid123")
	cmd := m.refreshPreview()

	if cmd == nil {
		t.Fatal("refreshPreview() returned a nil cmd, want a debounce+spinner batch")
	}
	if !m.thumbLoading {
		t.Error("thumbLoading = false, want true while the thumbnail is outstanding")
	}
	if !strings.Contains(m.previewContent, "loading thumbnail") {
		t.Errorf("previewContent = %q, want it to contain the loading placeholder", m.previewContent)
	}
	// The placeholder must occupy the same row budget as a real render
	// (previewThumbnailRows-1 newlines) so nothing shifts when
	// handleThumbnailLoaded swaps in the real thumbnail.
	w := columnContentWidth(m.detailColumnOuterWidth())
	placeholder := thumbnailPlaceholder(w, m.thumbSpinner.View())
	if got := strings.Count(placeholder, "\n"); got != previewThumbnailRows-1 {
		t.Errorf("placeholder has %d newlines, want %d (previewThumbnailRows-1)", got, previewThumbnailRows-1)
	}
}

// TestRefreshPreview_CachedThumbnail_SkipsPlaceholder confirms an
// already-cached thumbnail is used directly, with no placeholder/loading
// state.
func TestRefreshPreview_CachedThumbnail_SkipsPlaceholder(t *testing.T) {
	m := newTestModelForPreview(t, "vid123")
	w := columnContentWidth(m.detailColumnOuterWidth())
	m.thumbnailCache[thumbnailCacheKey("vid123", w)] = "CACHED-THUMB"

	m.refreshPreview()

	if m.thumbLoading {
		t.Error("thumbLoading = true, want false — thumbnail was already cached")
	}
	if !strings.Contains(m.previewContent, "CACHED-THUMB") {
		t.Errorf("previewContent = %q, want the cached thumbnail", m.previewContent)
	}
	if strings.Contains(m.previewContent, "loading thumbnail") {
		t.Error("previewContent contains the loading placeholder despite a cache hit")
	}
}

// TestHandleThumbnailLoaded_Success_ClearsLoading confirms a successful
// load both patches in the real thumbnail and stops thumbLoading (so the
// placeholder's spinner tick chain terminates — see the spinner.TickMsg
// case in Update).
func TestHandleThumbnailLoaded_Success_ClearsLoading(t *testing.T) {
	m := newTestModelForPreview(t, "vid123")
	m.refreshPreview()
	if !m.thumbLoading {
		t.Fatal("setup: expected thumbLoading = true before the load resolves")
	}

	w := columnContentWidth(m.detailColumnOuterWidth())
	next, _ := m.handleThumbnailLoaded(thumbnailLoadedMsg{
		videoID: "vid123",
		key:     thumbnailCacheKey("vid123", w),
		output:  "REAL-THUMB",
	})
	m2 := next.(Model)

	if m2.thumbLoading {
		t.Error("thumbLoading = true after a successful load, want false")
	}
	if !strings.Contains(m2.previewContent, "REAL-THUMB") {
		t.Errorf("previewContent = %q, want the real thumbnail", m2.previewContent)
	}
}

// TestHandleThumbnailLoaded_Error_CollapsesPlaceholder confirms a failed
// load also stops thumbLoading and drops the reserved placeholder rows
// (best-effort — no error is surfaced, but the loading state can't be left
// spinning forever over a fetch that will never succeed).
func TestHandleThumbnailLoaded_Error_CollapsesPlaceholder(t *testing.T) {
	m := newTestModelForPreview(t, "vid123")
	m.refreshPreview()
	bodyBeforeLoad := m.previewBody

	next, _ := m.handleThumbnailLoaded(thumbnailLoadedMsg{
		videoID: "vid123",
		err:     errors.New("fetch failed"),
	})
	m2 := next.(Model)

	if m2.thumbLoading {
		t.Error("thumbLoading = true after a failed load, want false")
	}
	if m2.previewContent != bodyBeforeLoad {
		t.Errorf("previewContent = %q, want it collapsed back to the plain body %q", m2.previewContent, bodyBeforeLoad)
	}
}
