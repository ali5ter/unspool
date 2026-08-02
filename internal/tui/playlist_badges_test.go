package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"

	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/store"
)

// TestLikedRowDescription_PlaylistBadge covers issue #13: a liked video's
// row must advertise which playlist(s) it's already in — no badge when
// unknown/none, the playlist name when there's exactly one, and a count
// once it's in several (keeping the row from growing unbounded).
func TestLikedRowDescription_PlaylistBadge(t *testing.T) {
	base := store.Video{ChannelTitle: "Chan", DurationSeconds: 60}

	tests := []struct {
		name        string
		inPlaylists []string
		want        string
		notWant     string
	}{
		{name: "none known", inPlaylists: nil, notWant: "▤"},
		{name: "one playlist", inPlaylists: []string{"DIY"}, want: "▤ DIY"},
		{name: "several playlists", inPlaylists: []string{"DIY", "Favorites"}, want: "▤ in 2 playlists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := likedRow{video: base, inPlaylists: tt.inPlaylists}
			desc := row.Description()
			if tt.want != "" && !strings.Contains(desc, tt.want) {
				t.Errorf("Description() = %q, want it to contain %q", desc, tt.want)
			}
			if tt.notWant != "" && strings.Contains(desc, tt.notWant) {
				t.Errorf("Description() = %q, want it to NOT contain %q", desc, tt.notWant)
			}
		})
	}
}

// TestPlaylistItemRowDescription_LikedBadge covers the reverse direction:
// a playlist item's row must show when the video is already liked.
func TestPlaylistItemRowDescription_LikedBadge(t *testing.T) {
	row := playlistItemRow{
		ref:     api.PlaylistItemRef{Title: "Some Video"},
		channel: "Chan",
		liked:   true,
	}
	if desc := row.Description(); !strings.Contains(desc, "♥ liked") {
		t.Errorf("Description() = %q, want it to contain the liked badge", desc)
	}

	row.liked = false
	if desc := row.Description(); strings.Contains(desc, "♥") {
		t.Errorf("Description() = %q, want no liked badge when not liked", desc)
	}
}

// TestHandlePlaylistMembershipLoaded_PatchesRenderedLikedRows covers the
// case where the Liked tab's rows are already on screen (loadLikedCmd
// resolved first) by the time the membership index arrives — it must
// patch the existing rows in place rather than requiring some unrelated
// reload to pick the badge up.
func TestHandlePlaylistMembershipLoaded_PatchesRenderedLikedRows(t *testing.T) {
	m := Model{likedList: list.New(nil, list.NewDefaultDelegate(), 0, 0)}
	m.likedList.SetItems([]list.Item{
		likedRow{video: store.Video{VideoID: "vid1", Title: "One"}},
		likedRow{video: store.Video{VideoID: "vid2", Title: "Two"}},
	})

	next, _ := m.handlePlaylistMembershipLoaded(playlistMembershipLoadedMsg{
		membership: map[string][]string{"vid1": {"DIY"}},
	})
	m2 := next.(Model)

	if !m2.playlistMembershipLoaded {
		t.Error("playlistMembershipLoaded = false, want true")
	}
	items := m2.likedList.Items()
	row1 := items[0].(likedRow)
	if len(row1.inPlaylists) != 1 || row1.inPlaylists[0] != "DIY" {
		t.Errorf("vid1.inPlaylists = %v, want [DIY]", row1.inPlaylists)
	}
	row2 := items[1].(likedRow)
	if len(row2.inPlaylists) != 0 {
		t.Errorf("vid2.inPlaylists = %v, want empty", row2.inPlaylists)
	}
}
