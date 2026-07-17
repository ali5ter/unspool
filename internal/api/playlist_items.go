package api

import (
	"context"
	"fmt"

	"github.com/ali5ter/unspool/internal/store"
)

// FetchPlaylistItems pulls up to maxItems videos from a playlist via
// playlistItems.list (1 unit/page — PRD §5.1). Used as the backfill path
// when the RSS feed's ~15-item window isn't enough, e.g. a channel's first
// sync.
func (c *Client) FetchPlaylistItems(ctx context.Context, playlistID, channelID string, maxItems int) ([]store.Video, error) {
	var videos []store.Video
	pageToken := ""

	for len(videos) < maxItems {
		call := c.yt.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(playlistID).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list playlist items for %s: %w", playlistID, err)
		}
		c.Quota.Spend(CostListPage)

		for _, item := range resp.Items {
			if item.ContentDetails == nil {
				continue
			}
			videos = append(videos, store.Video{
				VideoID:     item.ContentDetails.VideoId,
				ChannelID:   channelID,
				Title:       item.Snippet.Title,
				PublishedAt: parseAPITimestamp(item.ContentDetails.VideoPublishedAt),
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" || len(resp.Items) == 0 {
			break
		}
	}

	if len(videos) > maxItems {
		videos = videos[:maxItems]
	}
	return videos, nil
}
