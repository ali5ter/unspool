package feed

import (
	"testing"
	"time"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/store"
)

func TestLoadCached(t *testing.T) {
	dir := t.TempDir()
	st := store.New(dir)

	now := time.Now()
	older := now.Add(-24 * time.Hour)

	if err := st.SaveSubscriptions(store.SubscriptionsFile{
		SchemaVersion: 1,
		Subscriptions: []store.Subscription{
			{ChannelID: "chan-kept", Title: "Kept Channel"},
			{ChannelID: "chan-muted", Title: "Muted Channel"},
		},
	}); err != nil {
		t.Fatalf("SaveSubscriptions: %v", err)
	}

	if err := st.SaveMutes(store.MutesFile{SchemaVersion: 1, ChannelIDs: []string{"chan-muted"}}); err != nil {
		t.Fatalf("SaveMutes: %v", err)
	}

	if err := st.SaveVideos("chan-kept", store.VideosFile{
		SchemaVersion: 1,
		Videos: []store.Video{
			{VideoID: "old-video", ChannelID: "chan-kept", Title: "Old Video", PublishedAt: older},
			{VideoID: "new-video", ChannelID: "chan-kept", Title: "New Video", PublishedAt: now},
		},
	}); err != nil {
		t.Fatalf("SaveVideos chan-kept: %v", err)
	}

	if err := st.SaveVideos("chan-muted", store.VideosFile{
		SchemaVersion: 1,
		Videos: []store.Video{
			{VideoID: "muted-video", ChannelID: "chan-muted", Title: "Muted Video", PublishedAt: now},
		},
	}); err != nil {
		t.Fatalf("SaveVideos chan-muted: %v", err)
	}

	if err := st.SaveFeedState(store.FeedStateFile{
		SchemaVersion: 1,
		State: map[string]store.VideoState{
			"new-video": {Seen: true},
		},
	}); err != nil {
		t.Fatalf("SaveFeedState: %v", err)
	}

	result, err := LoadCached(&config.Config{StoreDir: dir})
	if err != nil {
		t.Fatalf("LoadCached: %v", err)
	}

	if got, want := len(result.Items), 2; got != want {
		t.Fatalf("len(Items) = %d, want %d (muted channel's video must be excluded); items: %+v", got, want, result.Items)
	}

	if result.Items[0].Video.VideoID != "new-video" || result.Items[1].Video.VideoID != "old-video" {
		t.Fatalf("items not sorted reverse-chronologically: got [%s, %s]", result.Items[0].Video.VideoID, result.Items[1].Video.VideoID)
	}

	if !result.Items[0].State.Seen {
		t.Fatalf("new-video's feed state (Seen) was not carried over from feed_state.json")
	}

	if result.QuotaSpent != 0 {
		t.Fatalf("QuotaSpent = %d, want 0 for an offline load", result.QuotaSpent)
	}
}
