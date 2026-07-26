// Package recommend synthesises a "Recommended" tab (PRD §5.8) entirely
// from data unspool already has locally — watch log, subscriptions, cached
// per-channel videos, feed state, and mutes. No network or quota cost, and
// explicitly not a claim to reproduce YouTube's own recommendation feed.
package recommend

import (
	"sort"
	"time"

	"github.com/ali5ter/unspool/internal/store"
)

// maxItems caps the total number of recommendations returned, keeping the
// tab snappy and the list scannable rather than exhaustive.
const maxItems = 30

// resurfaceAfter is how long a subscribed channel can go unwatched before
// it qualifies for the "haven't watched X in a while" signal.
const resurfaceAfter = 30 * 24 * time.Hour

// topChannelsLimit bounds how many of the most-watched channels contribute
// a "you watch a lot of X" recommendation.
const topChannelsLimit = 5

// Item is a single recommendation: a video, its channel, and a
// plain-language reason it was surfaced (PRD §5.8: "no black box").
type Item struct {
	Video   store.Video
	Channel string
	Reason  string
}

// Build computes the current set of recommendations from the local store.
func Build(st *store.Store) ([]Item, error) {
	subsFile, err := st.LoadSubscriptions()
	if err != nil {
		return nil, err
	}
	watchLog, err := st.LoadWatchLog()
	if err != nil {
		return nil, err
	}
	mutesFile, err := st.LoadMutes()
	if err != nil {
		return nil, err
	}
	feedState, err := st.LoadFeedState()
	if err != nil {
		return nil, err
	}

	muted := make(map[string]bool, len(mutesFile.ChannelIDs))
	for _, id := range mutesFile.ChannelIDs {
		muted[id] = true
	}

	channelIDs := make([]string, 0, len(subsFile.Subscriptions))
	channelTitle := map[string]string{}     // channel ID -> title
	titleToChannelID := map[string]string{} // title -> channel ID (watch log only has titles)
	for _, sub := range subsFile.Subscriptions {
		if muted[sub.ChannelID] {
			continue
		}
		channelIDs = append(channelIDs, sub.ChannelID)
		channelTitle[sub.ChannelID] = sub.Title
		titleToChannelID[sub.Title] = sub.ChannelID
	}

	videosByChannel, err := st.VideosByChannel(channelIDs)
	if err != nil {
		return nil, err
	}

	watchCount := map[string]int{}        // channel ID -> times watched
	lastWatched := map[string]time.Time{} // channel ID -> most recent watch
	for _, e := range watchLog.Entries {
		id, ok := titleToChannelID[e.Channel]
		if !ok {
			continue
		}
		watchCount[id]++
		if e.StartedAt.After(lastWatched[id]) {
			lastWatched[id] = e.StartedAt
		}
	}

	unseen := func(channelID string) []store.Video {
		var out []store.Video
		for _, v := range videosByChannel[channelID] {
			if !feedState.State[v.VideoID].Seen {
				out = append(out, v)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
		return out
	}

	var items []Item
	seen := map[string]bool{}
	add := func(v store.Video, channelID, reason string) {
		if seen[v.VideoID] {
			return
		}
		seen[v.VideoID] = true
		items = append(items, Item{Video: v, Channel: channelTitle[channelID], Reason: reason})
	}

	// 1. More from the channel of the most recent watch-log entry.
	if len(watchLog.Entries) > 0 {
		last := watchLog.Entries[len(watchLog.Entries)-1]
		if id, ok := titleToChannelID[last.Channel]; ok {
			for _, v := range unseen(id) {
				add(v, id, "more from "+channelTitle[id]+" — you just watched \""+last.Title+"\"")
				break
			}
		}
	}

	// 2. Top-watched channels' newest unseen video.
	type channelCount struct {
		id    string
		count int
	}
	var ranked []channelCount
	for id, c := range watchCount {
		ranked = append(ranked, channelCount{id, c})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })
	if len(ranked) > topChannelsLimit {
		ranked = ranked[:topChannelsLimit]
	}
	for _, rc := range ranked {
		for _, v := range unseen(rc.id) {
			add(v, rc.id, "you watch a lot of "+channelTitle[rc.id])
			break
		}
	}

	// 3. Subscribed channels unwatched in resurfaceAfter or more.
	now := time.Now()
	for _, id := range channelIDs {
		if last, ok := lastWatched[id]; ok && now.Sub(last) < resurfaceAfter {
			continue
		}
		for _, v := range unseen(id) {
			add(v, id, "haven't watched "+channelTitle[id]+" in a while")
			break
		}
	}

	// 4. Anything else unseen from a subscribed channel.
	for _, id := range channelIDs {
		for _, v := range unseen(id) {
			add(v, id, "new from a channel you follow")
			if len(items) >= maxItems {
				break
			}
		}
		if len(items) >= maxItems {
			break
		}
	}

	if len(items) > maxItems {
		items = items[:maxItems]
	}
	return items, nil
}
