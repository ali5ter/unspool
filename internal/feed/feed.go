// Package feed orchestrates a sync: resolving subscriptions, pulling each
// channel's Shorts-free uploads (RSS incrementally, playlistItems.list for
// first-run backfill), batching video-detail lookups, and merging the
// result into the local store.
package feed

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/api"
	"github.com/ali5ter/unspool/internal/auth"
	"github.com/ali5ter/unspool/internal/classifier"
	"github.com/ali5ter/unspool/internal/queue"
	"github.com/ali5ter/unspool/internal/store"
)

// backfillItems is how many videos to pull on a channel's first sync, when
// the RSS feed's ~15-item window may not be enough.
const backfillItems = 30

// channelSyncConcurrency bounds how many channels are synced in parallel.
// Sequential syncing (one blocking network round-trip per channel) scales
// linearly with subscription count — observed taking 80+ seconds on a
// ~1160-channel account, which reads as a hung app since the splash's only
// feedback is a spinner.
//
// This was originally 20, which cut that 80s sync to ~2s — but on a live
// ~1160-channel account it also triggered throttling on the RSS feed
// endpoint (youtube.com/feeds/videos.xml): a burst of 20 concurrent
// requests got a wave of spurious 404s back (confirmed via a direct curl
// of one such URL, not inferred from a parse error), affecting roughly
// half the account's channels on the next sync. Unlike the quota-tracked
// googleapis.com Data API, this consumer-facing endpoint isn't built for
// bulk concurrent access and has no documented budget to stay under. 6 is
// a deliberately conservative retreat — still ~3x faster than sequential
// per channel, but a much smaller burst. FetchRSSFeed also now retries
// transiently on its own (internal/api/feed.go), so an occasional
// individual failure recovers without a full resync.
const channelSyncConcurrency = 6

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
	AutoInspected   int      // videos run through the tier-1 transcript classifier this run
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

	var (
		mu                sync.Mutex // guards items, skipped, feedState.State, and newChannelSamples below
		items             []Item
		skipped           []string
		newChannelSamples []store.Video // one newest video per brand-new channel this sync — tier-1 candidates
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(channelSyncConcurrency)

	for i := range subsFile.Subscriptions {
		sub := &subsFile.Subscriptions[i]
		if muted[sub.ChannelID] {
			continue
		}

		g.Go(func() error {
			kept, scores, sample, err := syncChannel(gctx, client, st, cfg, sub.ChannelID, sub.UploadsLFPlaylistID)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				skipped = append(skipped, sub.Title)
				return nil
			}
			sub.LastSeen = time.Now()
			for _, v := range kept {
				state := feedState.State[v.VideoID]
				// Scored, not "is this a new video", gates the heuristic
				// score write — otherwise every video already present in
				// feed_state.json from before this feature shipped would
				// stay unscored forever, since presence in the map (not
				// Seen) is what the old check keyed on.
				if !state.Scored {
					state.AIScore = scores[v.VideoID]
					state.Scored = true
					feedState.State[v.VideoID] = state
				}
				items = append(items, Item{Video: v, Channel: sub.Title, State: feedState.State[v.VideoID]})
			}
			if sample != nil {
				newChannelSamples = append(newChannelSamples, *sample)
			}
			return nil
		})
	}
	_ = g.Wait() // syncChannel already reports its own errors via skipped; nothing to propagate

	if err := st.SaveSubscriptions(subsFile); err != nil {
		return nil, fmt.Errorf("save subscriptions: %w", err)
	}
	if err := st.SaveFeedState(feedState); err != nil {
		return nil, fmt.Errorf("save feed state: %w", err)
	}

	// Tier-1 auto-inspection (PRD §5.2.4) runs here, after every channel
	// sync has finished — deliberately not inline inside the per-channel
	// goroutines above. backfill (see syncChannel) is true for every
	// channel on a cold-start sync, so gating tier 1 on it there would
	// launch one yt-dlp-plus-classifier subprocess pair per subscription
	// (documented at ~1160 channels for this account) with no timeout
	// anywhere in the chain — turning a sync re-engineered from 80s→~2s
	// back into potentially tens of minutes, plus real LLM spend, on
	// literally the first run. runTier1 bounds this to a small constant
	// regardless of account size, cold-start or steady-state.
	autoInspected := 0
	if cfg.Classifier.AutoInspectNewChannels && cfg.Classifier.TranscriptCommand != "" {
		autoInspected = runTier1(ctx, cfg, st, newChannelSamples)
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
		AutoInspected:   autoInspected,
	}, nil
}

// LoadCached assembles a Result entirely from the local store — no OAuth
// client, no network calls, no quota spend. It's the read path for
// `--offline`: the same Item shape and sort order Sync produces, sourced
// from whatever a prior Sync already cached. QuotaSpent, SkippedChannels,
// MirrorErr, and AutoInspected are all meaningless without a network round
// trip, so they're left at their zero values rather than faked.
func LoadCached(cfg *config.Config) (*Result, error) {
	st := store.New(cfg.StoreDir)

	subsFile, err := st.LoadSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("load subscriptions: %w", err)
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

	channelIDs := make([]string, 0, len(subsFile.Subscriptions))
	channelTitles := make(map[string]string, len(subsFile.Subscriptions))
	for _, sub := range subsFile.Subscriptions {
		if muted[sub.ChannelID] {
			continue
		}
		channelIDs = append(channelIDs, sub.ChannelID)
		channelTitles[sub.ChannelID] = sub.Title
	}

	videosByChannel, err := st.VideosByChannel(channelIDs)
	if err != nil {
		return nil, fmt.Errorf("load cached videos: %w", err)
	}

	var items []Item
	for _, channelID := range channelIDs {
		for _, v := range videosByChannel[channelID] {
			items = append(items, Item{Video: v, Channel: channelTitles[channelID], State: feedState.State[v.VideoID]})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Video.PublishedAt.After(items[j].Video.PublishedAt)
	})

	return &Result{
		Items:       items,
		QuotaSpent:  0,
		QuotaBudget: api.DailyQuota,
	}, nil
}

// syncChannel fetches new videos for one channel (RSS incrementally, or a
// playlistItems.list backfill on first sync), merges them with the cached
// set, batches detail lookups for anything missing duration, applies the
// Shorts fallback guard, computes each kept video's heuristic AI-slop score
// (PRD §5.2.2), and persists the result.
//
// sample is non-nil only when this channel had no cached videos before this
// sync (backfill) — the single newest kept video, offered as a tier-1
// auto-inspection candidate. It's returned rather than acted on here since
// syncChannel runs inside a bounded-concurrency errgroup goroutine, where
// blocking on an external yt-dlp-plus-classifier round-trip per channel
// would defeat the whole point of that concurrency bound — see runTier1.
func syncChannel(ctx context.Context, client *api.Client, st *store.Store, cfg *config.Config, channelID, uploadsLFPlaylistID string) (kept []store.Video, scores map[string]float64, sample *store.Video, err error) {
	cached, err := st.LoadVideos(channelID)
	if err != nil {
		return nil, nil, nil, err
	}

	backfill := len(cached.Videos) == 0
	fresh, err := fetchChannelVideos(ctx, client, uploadsLFPlaylistID, channelID, backfill)
	if err != nil {
		// Not every channel has a UULF playlist (observed in practice, not
		// just a theoretical concern) — fall back to the full uploads
		// playlist and lean on the duration-based Shorts guard below.
		fresh, err = fetchChannelVideos(ctx, client, api.UploadsPlaylistID(channelID), channelID, backfill)
		if err != nil {
			return nil, nil, nil, err
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

	kept = merged
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
		return nil, nil, nil, err
	}

	scores = make(map[string]float64, len(kept))
	age := channelAgeDays(kept)
	var newest *store.Video
	for i := range kept {
		v := &kept[i]
		scores[v.VideoID], _ = classifier.Score(*v, uploadCadence(kept, *v), age)
		if newest == nil || v.PublishedAt.After(newest.PublishedAt) {
			newest = v
		}
	}
	if backfill && newest != nil {
		sampleCopy := *newest
		sample = &sampleCopy
	}

	return kept, scores, sample, nil
}

// uploadCadenceWindow is how wide a window (each side of a video's own
// publish date) counts as "recent" for the high-upload-cadence heuristic
// signal (PRD §5.2.2).
const uploadCadenceWindow = 7 * 24 * time.Hour

// uploadCadence counts how many of a channel's other kept videos were
// published within uploadCadenceWindow of v's own publish date.
func uploadCadence(kept []store.Video, v store.Video) int {
	count := 0
	for _, other := range kept {
		if other.VideoID == v.VideoID {
			continue
		}
		diff := v.PublishedAt.Sub(other.PublishedAt)
		if diff < 0 {
			diff = -diff
		}
		if diff <= uploadCadenceWindow {
			count++
		}
	}
	return count
}

// channelAgeDays approximates a channel's age from its earliest cached
// upload — not the channel's real creation date, but a cheap, already-
// available proxy for "brand-new channel" (PRD §5.2.2). Returns -1 if kept
// is empty (unknown).
func channelAgeDays(kept []store.Video) int {
	if len(kept) == 0 {
		return -1
	}
	earliest := kept[0].PublishedAt
	for _, v := range kept[1:] {
		if v.PublishedAt.Before(earliest) {
			earliest = v.PublishedAt
		}
	}
	if earliest.IsZero() {
		return -1
	}
	return int(time.Since(earliest).Hours() / 24)
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

// maxTier1PerSync caps how many videos get an opportunistic tier-1
// transcript classification per sync run, regardless of how many brand-new
// channels were discovered — see the comment where this is called in Sync.
const maxTier1PerSync = 5

// tier1Concurrency is deliberately separate from channelSyncConcurrency:
// tier 1 shells out to yt-dlp (a transcript fetch) plus an external
// classifier command per video, both far slower than a single API call, so
// it gets its own much smaller pool.
const tier1Concurrency = 2

// tier1Timeout bounds a single video's transcript-fetch-plus-classify
// round-trip — nothing upstream of runTier1 has a context deadline of its
// own, so a hung yt-dlp or classifier process would otherwise have nothing
// to cancel it besides quitting the whole app.
const tier1Timeout = 30 * time.Second

// runTier1 opportunistically classifies up to maxTier1PerSync of this
// sync's newly-discovered channels' newest video (PRD §5.2.4 tier 1),
// caching any resulting verdicts, and returns how many were actually
// inspected. Runs after every per-channel sync has already finished — see
// the call site in Sync for why this can't run inline per-channel.
func runTier1(ctx context.Context, cfg *config.Config, st *store.Store, samples []store.Video) int {
	if len(samples) > maxTier1PerSync {
		samples = samples[:maxTier1PerSync]
	}
	if len(samples) == 0 {
		return 0
	}

	var (
		mu       sync.Mutex
		verdicts = map[string]store.Verdict{}
	)
	g := new(errgroup.Group)
	g.SetLimit(tier1Concurrency)
	for _, v := range samples {
		v := v
		g.Go(func() error {
			vctx, cancel := context.WithTimeout(ctx, tier1Timeout)
			defer cancel()

			transcript, err := classifier.FetchTranscript(vctx, v.VideoID)
			if err != nil {
				return nil // best-effort — a missing/failed transcript just skips this video
			}
			cv, err := classifier.RunTranscriptClassifier(vctx, cfg.Classifier.TranscriptCommand, transcript)
			if err != nil {
				return nil
			}

			mu.Lock()
			verdicts[v.VideoID] = store.Verdict{
				Score:          cv.Score,
				LikelyAI:       cv.LikelyAI,
				Reasoning:      cv.Reasoning,
				SuspectedTools: cv.SuspectedTools,
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	if len(verdicts) == 0 {
		return 0
	}
	vf, err := st.LoadVerdicts()
	if err != nil {
		return 0
	}
	for id, v := range verdicts {
		v.VideoID = id
		v.CheckedAt = time.Now()
		vf.Verdicts[id] = v
	}
	if err := st.SaveVerdicts(vf); err != nil {
		return 0
	}
	return len(verdicts)
}
