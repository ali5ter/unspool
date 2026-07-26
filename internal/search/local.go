// Package search implements unspool's rationed search (PRD §5.5): a free
// local-cache search over data already synced, and (in the TUI layer) an
// explicit escalation to the quota-costly search.list API for genuine
// web-wide discovery.
package search

import (
	"sort"
	"strings"

	"github.com/ali5ter/unspool/internal/store"
)

// maxLocalResults caps how many local matches are returned, keeping the
// results dialog scannable.
const maxLocalResults = 50

// Result is a single local-cache match. Kind distinguishes a video match
// from a playlist-title match — the latter isn't individually actionable
// (play/queue/like) the way a video is, only "jump to it in Playlists".
type Result struct {
	Video      store.Video
	Channel    string
	Kind       string // "video" | "playlist"
	PlaylistID string // set when Kind == "playlist"
}

// Local searches subscribed-channel cached videos, watch-log entries, and
// playlist titles — the three sources PRD §5.5 names as genuinely free
// ("prefer cheaper paths first: search within the local cache
// (subscriptions, playlists, watch log) for free"). Deliberately does not
// search individual playlist *items*: those aren't mirrored locally and
// would need a live API call per playlist — a documented scope cut, not
// silently pretended to be comprehensive.
func Local(st *store.Store, query string) ([]Result, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}

	subsFile, err := st.LoadSubscriptions()
	if err != nil {
		return nil, err
	}
	channelIDs := make([]string, 0, len(subsFile.Subscriptions))
	channelTitle := map[string]string{}
	for _, sub := range subsFile.Subscriptions {
		channelIDs = append(channelIDs, sub.ChannelID)
		channelTitle[sub.ChannelID] = sub.Title
	}
	videosByChannel, err := st.VideosByChannel(channelIDs)
	if err != nil {
		return nil, err
	}

	var results []Result
	seen := map[string]bool{}
	for _, id := range channelIDs {
		for _, v := range videosByChannel[id] {
			if seen[v.VideoID] {
				continue
			}
			if strings.Contains(strings.ToLower(v.Title), q) {
				seen[v.VideoID] = true
				results = append(results, Result{Video: v, Channel: channelTitle[id], Kind: "video"})
			}
		}
	}

	watchLog, err := st.LoadWatchLog()
	if err != nil {
		return nil, err
	}
	for _, e := range watchLog.Entries {
		if seen[e.VideoID] {
			continue
		}
		if strings.Contains(strings.ToLower(e.Title), q) {
			seen[e.VideoID] = true
			results = append(results, Result{
				Video:   store.Video{VideoID: e.VideoID, Title: e.Title, PublishedAt: e.StartedAt},
				Channel: e.Channel,
				Kind:    "video",
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Video.PublishedAt.After(results[j].Video.PublishedAt)
	})

	playlistsFile, err := st.LoadPlaylistsCache()
	if err != nil {
		return nil, err
	}
	var playlistMatches []Result
	for _, p := range playlistsFile.Playlists {
		if strings.Contains(strings.ToLower(p.Title), q) {
			playlistMatches = append(playlistMatches, Result{
				Video:      store.Video{Title: p.Title},
				Kind:       "playlist",
				PlaylistID: p.PlaylistID,
			})
		}
	}
	results = append(results, playlistMatches...)

	if len(results) > maxLocalResults {
		results = results[:maxLocalResults]
	}
	return results, nil
}
