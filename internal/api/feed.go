package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ali5ter/unspool/internal/store"
)

// rssTimeout bounds a single feed fetch; native feeds are occasionally
// rate-limited or briefly unavailable.
const rssTimeout = 10 * time.Second

// FetchRSSFeed fetches the quota-free Atom feed for a Shorts-free uploads
// playlist (UULF-prefixed). Returns only the latest ~15 items — a
// "new since last sync" mechanism, not a backfill.
//
// Retries a few times with backoff on failure: this consumer-facing feed
// endpoint (youtube.com, not the quota-tracked googleapis.com Data API) is
// not built for bulk concurrent access and can throttle a burst of parallel
// requests — observed directly by fetching ~1160 subscribed channels'
// feeds concurrently, which produced a wave of spurious 404s (confirmed via
// a direct curl of one such URL, not a parse-error guess) across roughly
// half the account's channels. A single request retried a few times, on
// its own schedule, is far less likely to land inside another burst's
// throttle window than a bare unretried call.
func FetchRSSFeed(ctx context.Context, uploadsLFPlaylistID, channelID string) ([]store.Video, error) {
	var videos []store.Video
	err := retryRSS(ctx, func() error {
		v, err := fetchRSSFeedOnce(ctx, uploadsLFPlaylistID, channelID)
		if err != nil {
			return err
		}
		videos = v
		return nil
	})
	return videos, err
}

func fetchRSSFeedOnce(ctx context.Context, uploadsLFPlaylistID, channelID string) ([]store.Video, error) {
	url := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?playlist_id=%s", uploadsLFPlaylistID)

	client := &http.Client{Timeout: rssTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch RSS feed for %s: %w", channelID, err)
	}
	defer resp.Body.Close()

	feed, err := gofeed.NewParser().Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse RSS feed for %s: %w", channelID, err)
	}

	videos := make([]store.Video, 0, len(feed.Items))
	for _, item := range feed.Items {
		videoID := extensionValue(item, "yt", "videoId")
		if videoID == "" {
			continue
		}
		videos = append(videos, store.Video{
			VideoID:     videoID,
			ChannelID:   channelID,
			Title:       item.Title,
			PublishedAt: publishedTime(item),
		})
	}
	return videos, nil
}

// retryRSS retries fn a few times with backoff. Every failure mode here —
// network error, non-2xx status, or a parse failure (this endpoint returns
// an HTML error page rather than a clean HTTP error status when throttling,
// confirmed directly) — is treated as potentially transient, unlike
// retryTransient's narrower googleapi.Error code check: this endpoint
// doesn't return structured API errors at all.
func retryRSS(ctx context.Context, fn func() error) error {
	const maxAttempts = 3
	backoff := 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxAttempts {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
		}
	}
	return lastErr
}

func extensionValue(item *gofeed.Item, namespace, key string) string {
	if item.Extensions == nil {
		return ""
	}
	ns, ok := item.Extensions[namespace]
	if !ok {
		return ""
	}
	vals, ok := ns[key]
	if !ok || len(vals) == 0 {
		return ""
	}
	return vals[0].Value
}

func publishedTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	return time.Time{}
}

// VideoDetail augments an RSS stub with metadata only the API provides.
type VideoDetail struct {
	DurationSeconds        int
	ContainsSyntheticMedia bool
	Description            string
	ChannelTitle           string
	PublishedAt            time.Time
}

// FetchVideoDetails batches videos.list calls (50 IDs/call, 1 unit/call) to
// fetch duration, provenance, and metadata for videos not already known
// from a feed sync — e.g. arbitrary playlist contents, which can be any
// video from any channel, not just subscribed ones. Duration feeds the
// Shorts fallback guard (IsLikelyShort) in case the UULF convention ever
// breaks; description powers the preview pane; ChannelTitle/PublishedAt
// are for callers (playlist item rows) that have no other source for them.
func (c *Client) FetchVideoDetails(ctx context.Context, videoIDs []string) (map[string]VideoDetail, error) {
	out := make(map[string]VideoDetail, len(videoIDs))

	for i := 0; i < len(videoIDs); i += 50 {
		end := min(i+50, len(videoIDs))
		batch := videoIDs[i:end]

		resp, err := c.yt.Videos.List([]string{"snippet", "contentDetails", "status"}).Id(batch...).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list video details: %w", err)
		}
		c.Quota.Spend(CostListPage)

		for _, item := range resp.Items {
			var detail VideoDetail
			if item.ContentDetails != nil {
				detail.DurationSeconds = parseISO8601Duration(item.ContentDetails.Duration)
			}
			if item.Status != nil {
				detail.ContainsSyntheticMedia = item.Status.ContainsSyntheticMedia
			}
			if item.Snippet != nil {
				detail.Description = item.Snippet.Description
				detail.ChannelTitle = item.Snippet.ChannelTitle
				detail.PublishedAt = parseAPITimestamp(item.Snippet.PublishedAt)
			}
			out[item.Id] = detail
		}
	}

	return out, nil
}

// IsLikelyShort is a fallback Shorts guard: short duration is the reliable
// half of the signal available without a real aspect-ratio read (the API
// doesn't expose one directly on videos.list).
func IsLikelyShort(durationSeconds int) bool {
	return durationSeconds > 0 && durationSeconds <= 180
}
