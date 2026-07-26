package recommend

import (
	"testing"
	"time"

	"github.com/ali5ter/unspool/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	return store.New(t.TempDir())
}

func TestBuild_MoreFromLastWatchedChannel(t *testing.T) {
	st := newTestStore(t)
	mustSave(t, st.SaveSubscriptions(store.SubscriptionsFile{Subscriptions: []store.Subscription{
		{ChannelID: "chan1", Title: "Chan One"},
	}}))
	mustSave(t, st.SaveVideos("chan1", store.VideosFile{Videos: []store.Video{
		{VideoID: "v1", ChannelID: "chan1", Title: "Old video", PublishedAt: time.Now().Add(-48 * time.Hour)},
		{VideoID: "v2", ChannelID: "chan1", Title: "New video", PublishedAt: time.Now()},
	}}))
	mustSave(t, st.AppendWatchLog(store.WatchLogEntry{VideoID: "v1", Title: "Old video", Channel: "Chan One", StartedAt: time.Now()}))

	items, err := Build(st)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one recommendation")
	}
	if items[0].Video.VideoID != "v2" {
		t.Errorf("items[0].Video.VideoID = %q, want v2 (newest unseen from last-watched channel)", items[0].Video.VideoID)
	}
	if items[0].Reason == "" {
		t.Errorf("expected a plain-language reason, got empty string")
	}
}

func TestBuild_DedupesAcrossSignals(t *testing.T) {
	st := newTestStore(t)
	mustSave(t, st.SaveSubscriptions(store.SubscriptionsFile{Subscriptions: []store.Subscription{
		{ChannelID: "chan1", Title: "Chan One"},
	}}))
	mustSave(t, st.SaveVideos("chan1", store.VideosFile{Videos: []store.Video{
		{VideoID: "only", ChannelID: "chan1", Title: "Only unseen video", PublishedAt: time.Now()},
	}}))
	// Watch chan1 several times, so it qualifies for both "more from the
	// last-watched channel" (signal 1) and "you watch a lot of X" (signal
	// 2) — the single unseen video should appear once, not twice.
	for i := 0; i < 3; i++ {
		mustSave(t, st.AppendWatchLog(store.WatchLogEntry{VideoID: "watched", Title: "Watched", Channel: "Chan One", StartedAt: time.Now()}))
	}

	items, err := Build(st)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	count := 0
	for _, it := range items {
		if it.Video.VideoID == "only" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("video appeared %d times across signals, want exactly once", count)
	}
}

func TestBuild_SkipsMutedChannels(t *testing.T) {
	st := newTestStore(t)
	mustSave(t, st.SaveSubscriptions(store.SubscriptionsFile{Subscriptions: []store.Subscription{
		{ChannelID: "chan1", Title: "Muted Channel"},
	}}))
	mustSave(t, st.SaveVideos("chan1", store.VideosFile{Videos: []store.Video{
		{VideoID: "v1", ChannelID: "chan1", Title: "Video", PublishedAt: time.Now()},
	}}))
	mustSave(t, st.MuteChannel("chan1"))

	items, err := Build(st)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no recommendations from a muted channel, got %d", len(items))
	}
}

func mustSave(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}
