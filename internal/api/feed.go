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
// rate-limited or briefly unavailable (PRD §2.4).
const rssTimeout = 10 * time.Second

// FetchRSSFeed fetches the quota-free Atom feed for a Shorts-free uploads
// playlist (UULF-prefixed). Returns only the latest ~15 items — a
// "new since last sync" mechanism, not a backfill (PRD §2.4).
func FetchRSSFeed(ctx context.Context, uploadsLFPlaylistID, channelID string) ([]store.Video, error) {
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
}

// FetchVideoDetails batches videos.list calls (50 IDs/call, 1 unit/call —
// PRD §6.4) to fetch duration and provenance metadata. Duration feeds the
// Shorts fallback guard (≤180s + portrait — PRD §5.1) in case the UULF
// convention ever breaks.
func (c *Client) FetchVideoDetails(ctx context.Context, videoIDs []string) (map[string]VideoDetail, error) {
	out := make(map[string]VideoDetail, len(videoIDs))

	for i := 0; i < len(videoIDs); i += 50 {
		end := min(i+50, len(videoIDs))
		batch := videoIDs[i:end]

		resp, err := c.yt.Videos.List([]string{"contentDetails", "status"}).Id(batch...).Context(ctx).Do()
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
			out[item.Id] = detail
		}
	}

	return out, nil
}

// IsLikelyShort applies the fallback guard from PRD §5.1: short duration is
// the reliable half of the signal available without a real aspect-ratio
// read (the API doesn't expose one directly on videos.list).
func IsLikelyShort(durationSeconds int) bool {
	return durationSeconds > 0 && durationSeconds <= 180
}
