// Package queue manages the locally-owned Queue (Watch Later replacement)
// and its auto-mirror to a real YouTube playlist. Local order is
// authoritative; the mirror keeps set membership in sync but does not
// enforce remote ordering (a deliberate M2 scope cut — full reorder-sync
// would cost a write-quota unit per item on every sync for little benefit,
// since the local TUI is the primary interface).
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/store"
)

// mirrorPlaylistTitle is created on first use when no playlist is configured
// or already adopted.
const mirrorPlaylistTitle = "▶ unspool Queue"

// Add appends videoID to the local queue if not already present.
func Add(st *store.Store, videoID string) error {
	qf, err := st.LoadQueue()
	if err != nil {
		return err
	}
	for _, id := range qf.VideoIDs {
		if id == videoID {
			return nil
		}
	}
	qf.VideoIDs = append(qf.VideoIDs, videoID)
	return st.SaveQueue(qf)
}

// Remove removes videoID from the local queue.
func Remove(st *store.Store, videoID string) error {
	qf, err := st.LoadQueue()
	if err != nil {
		return err
	}
	out := qf.VideoIDs[:0]
	for _, id := range qf.VideoIDs {
		if id != videoID {
			out = append(out, id)
		}
	}
	qf.VideoIDs = out
	return st.SaveQueue(qf)
}

// ensureMirrorPlaylist resolves the playlist ID to mirror the Queue into:
// the configured mirror_playlist_id if set, else one already adopted in
// queue.json, else a freshly created "▶ unspool Queue" playlist.
func ensureMirrorPlaylist(ctx context.Context, client *api.Client, st *store.Store, cfg *config.Config) (string, error) {
	if cfg.Queue.MirrorPlaylistID != "" {
		return cfg.Queue.MirrorPlaylistID, nil
	}

	qf, err := st.LoadQueue()
	if err != nil {
		return "", err
	}
	if qf.MirrorPlaylist != "" {
		return qf.MirrorPlaylist, nil
	}

	id, err := client.CreatePlaylist(ctx, mirrorPlaylistTitle)
	if err != nil {
		return "", err
	}
	qf.MirrorPlaylist = id
	if err := st.SaveQueue(qf); err != nil {
		return "", err
	}
	return id, nil
}

// SyncMirror reconciles the local queue with its mirrored remote playlist:
// inserts videos present locally but not remotely, removes videos present
// remotely but not locally. A no-op when mirroring is disabled in config.
func SyncMirror(ctx context.Context, client *api.Client, st *store.Store, cfg *config.Config) error {
	if !cfg.Queue.Mirror {
		return nil
	}

	playlistID, err := ensureMirrorPlaylist(ctx, client, st, cfg)
	if err != nil {
		return fmt.Errorf("ensure mirror playlist: %w", err)
	}

	qf, err := st.LoadQueue()
	if err != nil {
		return err
	}
	local := make(map[string]bool, len(qf.VideoIDs))
	for _, id := range qf.VideoIDs {
		local[id] = true
	}

	// A just-created playlist is occasionally not yet queryable via
	// playlistItems.list for a few seconds (observed directly against the
	// live API, not hypothetical) — retry briefly before giving up.
	remoteRefs, err := listPlaylistItemRefsWithRetry(ctx, client, playlistID)
	if err != nil {
		return fmt.Errorf("list mirror playlist items: %w", err)
	}
	remoteByVideo := make(map[string]api.PlaylistItemRef, len(remoteRefs))
	for _, ref := range remoteRefs {
		remoteByVideo[ref.VideoID] = ref
	}

	for _, videoID := range qf.VideoIDs {
		if _, ok := remoteByVideo[videoID]; ok {
			continue
		}
		if _, err := client.AddPlaylistItem(ctx, playlistID, videoID); err != nil {
			return fmt.Errorf("mirror add %s: %w", videoID, err)
		}
	}
	for videoID, ref := range remoteByVideo {
		if local[videoID] {
			continue
		}
		if err := client.RemovePlaylistItem(ctx, ref.PlaylistItemID); err != nil {
			return fmt.Errorf("mirror remove %s: %w", videoID, err)
		}
	}

	return nil
}

// listPlaylistItemRefsWithRetry retries ListPlaylistItemRefs a few times on
// failure. A just-created playlist can 404 on playlistItems.list for a few
// seconds before YouTube's backend catches up.
func listPlaylistItemRefsWithRetry(ctx context.Context, client *api.Client, playlistID string) ([]api.PlaylistItemRef, error) {
	const maxAttempts = 3
	backoff := 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		refs, err := client.ListPlaylistItemRefs(ctx, playlistID)
		if err == nil {
			return refs, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, lastErr
}
