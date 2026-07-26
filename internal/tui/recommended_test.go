package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"

	"github.com/ali5ter/unspool/config"
)

func newTestModelForRecommended(t *testing.T, recommendationsEnabled bool) Model {
	t.Helper()
	cfg := &config.Config{}
	cfg.Recommendations.Enabled = recommendationsEnabled
	return Model{
		cfg:             cfg,
		activeTab:       tabRecommended,
		recommendedList: list.New(nil, list.NewDefaultDelegate(), 0, 0),
		spinner:         spinner.New(),
	}
}

// TestOnTabChanged_RecommendationsDisabled covers the disabled-in-config
// notice row — confirmed live via VHS this session, this test pins the
// exact behavior down so a future change can't silently regress it.
func TestOnTabChanged_RecommendationsDisabled(t *testing.T) {
	m := newTestModelForRecommended(t, false)
	m.onTabChanged()

	if !m.recommendedLoaded {
		t.Errorf("recommendedLoaded = false, want true (disabled state should not wait on a load)")
	}
	if m.busy {
		t.Errorf("busy = true, want false — disabled state should never dispatch a load command")
	}
	items := m.recommendedList.Items()
	if len(items) != 1 {
		t.Fatalf("got %d items, want exactly 1 notice row", len(items))
	}
	row, ok := items[0].(noticeRow)
	if !ok {
		t.Fatalf("item is %T, want noticeRow", items[0])
	}
	const want = "Recommendations disabled — enable [recommendations].enabled in config.toml"
	if row.text != want {
		t.Errorf("notice text = %q, want %q", row.text, want)
	}
}

// TestOnTabChanged_RecommendationsEnabled_DispatchesLoad confirms the
// enabled path takes the async-load branch instead (busy + a command),
// rather than also short-circuiting into a static notice.
func TestOnTabChanged_RecommendationsEnabled_DispatchesLoad(t *testing.T) {
	m := newTestModelForRecommended(t, true)
	cmd := m.onTabChanged()

	if m.recommendedLoaded {
		t.Errorf("recommendedLoaded = true, want false — still waiting on loadRecommendedCmd")
	}
	if !m.busy {
		t.Errorf("busy = false, want true — enabled path should be async")
	}
	if cmd == nil {
		t.Errorf("onTabChanged returned a nil command, want a batch including loadRecommendedCmd")
	}
}

// TestHandleRecommendedLoaded_Empty covers the empty-recommendations
// notice row (a fresh/low-history account, or one where every subscribed
// channel's videos are already seen) — the counterpart to the
// disabled-in-config case above, confirmed live via VHS there but not
// practical to reproduce live here without a second, artificially-empty
// account, so pinned down at the unit level instead.
func TestHandleRecommendedLoaded_Empty(t *testing.T) {
	m := newTestModelForRecommended(t, true)
	m.busy = true

	next, _ := m.handleRecommendedLoaded(recommendedLoadedMsg{items: nil})
	got := next.(Model)

	if !got.recommendedLoaded {
		t.Errorf("recommendedLoaded = false, want true")
	}
	if got.busy {
		t.Errorf("busy = true, want false")
	}
	items := got.recommendedList.Items()
	if len(items) != 1 {
		t.Fatalf("got %d items, want exactly 1 notice row", len(items))
	}
	row, ok := items[0].(noticeRow)
	if !ok {
		t.Fatalf("item is %T, want noticeRow", items[0])
	}
	const want = "No recommendations yet — watch a few videos and sync again."
	if row.text != want {
		t.Errorf("notice text = %q, want %q", row.text, want)
	}
}
