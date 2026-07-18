// Package feed orchestrates a sync: resolving subscriptions, pulling each
// channel's Shorts-free uploads (RSS incrementally, playlistItems.list for
// first-run backfill), batching video-detail lookups, and merging the
// result into the local store.
package feed

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/auth"
	"github.com/ali5ter/unspool/internal/queue"
	"github.com/ali5ter/unspool/internal/store"
)

// backfillItems is how many videos to pull on a channel's first sync, when
// the RSS feed's ~15-item window may not be enough.
const backfillItems = 30

// Item is a single feed row: a video plus its channel and mutable state.
type Item struct {
	Video   store.Video
	Channel string
	State   store.VideoState
}

// Result is the outcome of a Sync.
type Result struct {
	Items           []Item
	QuotaSpent      int
	QuotaBudget     int
	SkippedChannels []string // channels whose fetch failed this run; sync continued
	MirrorErr       error    // non-nil if the Queue mirror reconciliation failed this run
}

// Sync refreshes subscriptions and per-channel video caches from the API,
// merges the result into the local store, and returns the merged feed
// sorted reverse-chronologically. A single channel failing to fetch does
// not abort the whole sync — it's recorded in Result.SkippedChannels.
func Sync(ctx context.Context, cfg *config.Config) (*Result, error) {
	hc, err := auth.Client(ctx, cfg.OAuthClientSecretFile)
	if err != nil {
		return nil, err
	}
	client, err := api.NewClient(ctx, hc)
	if err != nil {
		return nil, err
	}
	st := store.New(cfg.StoreDir)

	subsFile, err := st.LoadSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("load subscriptions: %w", err)
	}
	if len(subsFile.Subscriptions) == 0 {
		resolved, err := client.ResolveSubscriptions(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve subscriptions: %w", err)
		}
		subsFile.Subscriptions = resolved
	}

	mutesFile, err := st.LoadMutes()
	if err != nil {
		return nil, fmt.Errorf("load mutes: %w", err)
	}
	muted := toSet(mutesFile.ChannelIDs)

	feedState, err := st.LoadFeedState()
	if err != nil {
		return nil, fmt.Errorf("load feed state: %w", err)
	}

	var items []Item
	var skipped []string

	for i := range subsFile.Subscriptions {
		sub := &subsFile.Subscriptions[i]
		if muted[sub.ChannelID] {
			continue
		}

		kept, err := syncChannel(ctx, client, st, cfg, sub.ChannelID, sub.UploadsLFPlaylistID)
		if err != nil {
			skipped = append(skipped, sub.Title)
			continue
		}

		sub.LastSeen = time.Now()
		for _, v := range kept {
			if _, seen := feedState.State[v.VideoID]; !seen {
				feedState.State[v.VideoID] = store.VideoState{}
			}
			items = append(items, Item{Video: v, Channel: sub.Title, State: feedState.State[v.VideoID]})
		}
	}

	if err := st.SaveSubscriptions(subsFile); err != nil {
		return nil, fmt.Errorf("save subscriptions: %w", err)
	}
	if err := st.SaveFeedState(feedState); err != nil {
		return nil, fmt.Errorf("save feed state: %w", err)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Video.PublishedAt.After(items[j].Video.PublishedAt)
	})

	// Mirror drift is recoverable on the next sync — don't fail the whole
	// feed refresh over it.
	mirrorErr := queue.SyncMirror(ctx, client, st, cfg)

	return &Result{
		Items:           items,
		QuotaSpent:      client.Quota.Spent(),
		QuotaBudget:     api.DailyQuota,
		SkippedChannels: skipped,
		MirrorErr:       mirrorErr,
	}, nil
}

// syncChannel fetches new videos for one channel (RSS incrementally, or a
// playlistItems.list backfill on first sync), merges them with the cached
// set, batches detail lookups for anything missing duration, applies the
// Shorts fallback guard, and persists the result.
func syncChannel(ctx context.Context, client *api.Client, st *store.Store, cfg *config.Config, channelID, uploadsLFPlaylistID string) ([]store.Video, error) {
	cached, err := st.LoadVideos(channelID)
	if err != nil {
		return nil, err
	}

	backfill := len(cached.Videos) == 0
	fresh, err := fetchChannelVideos(ctx, client, uploadsLFPlaylistID, channelID, backfill)
	if err != nil {
		// Not every channel has a UULF playlist (observed in practice, not
		// just a theoretical concern) — fall back to the full uploads
		// playlist and lean on the duration-based Shorts guard below.
		fresh, err = fetchChannelVideos(ctx, client, api.UploadsPlaylistID(channelID), channelID, backfill)
		if err != nil {
			return nil, err
		}
	}

	merged, needDetails := mergeVideos(cached.Videos, fresh)

	if len(needDetails) > 0 {
		details, derr := client.FetchVideoDetails(ctx, needDetails)
		if derr == nil {
			needSet := make(map[string]bool, len(needDetails))
			for _, id := range needDetails {
				needSet[id] = true
			}
			for j := range merged {
				if !needSet[merged[j].VideoID] {
					continue
				}
				// Mark fetched even if this ID is absent from the response
				// (e.g. a deleted/private video) — otherwise it would be
				// retried on every future sync indefinitely.
				merged[j].DetailsFetched = true
				if d, ok := details[merged[j].VideoID]; ok {
					merged[j].DurationSeconds = d.DurationSeconds
					merged[j].ContainsSyntheticMedia = d.ContainsSyntheticMedia
					merged[j].Description = d.Description
				}
			}
		}
	}

	kept := merged
	if cfg.Filters.HideShorts {
		kept = kept[:0]
		for _, v := range merged {
			if api.IsLikelyShort(v.DurationSeconds) {
				continue
			}
			kept = append(kept, v)
		}
	}

	if err := st.SaveVideos(channelID, store.VideosFile{Videos: kept}); err != nil {
		return nil, err
	}
	return kept, nil
}

// fetchChannelVideos pulls a playlist's videos: playlistItems.list on a
// channel's first sync (RSS's ~15-item window isn't enough for a backfill),
// otherwise the quota-free RSS feed.
func fetchChannelVideos(ctx context.Context, client *api.Client, playlistID, channelID string, backfill bool) ([]store.Video, error) {
	if backfill {
		return client.FetchPlaylistItems(ctx, playlistID, channelID, backfillItems)
	}
	return api.FetchRSSFeed(ctx, playlistID, channelID)
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
