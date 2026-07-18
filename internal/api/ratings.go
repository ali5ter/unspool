package api

import (
	"context"
	"fmt"

	"github.com/ali5ter/unspool/internal/store"
)

// RateVideo sets or clears a video's like rating (videos.rate, 50 units).
// rating is "like" or "none" (YouTube also supports "dislike", unused here).
func (c *Client) RateVideo(ctx context.Context, videoID, rating string) error {
	if err := c.yt.Videos.Rate(videoID, rating).Context(ctx).Do(); err != nil {
		return fmt.Errorf("rate video %s as %q: %w", videoID, rating, err)
	}
	c.Quota.Spend(CostWrite)
	return nil
}

// ListLikedVideos fetches the authenticated user's liked videos via
// videos.list(myRating=like), paginating 50 at a time (1 unit/page).
func (c *Client) ListLikedVideos(ctx context.Context) ([]store.Video, error) {
	var videos []store.Video
	pageToken := ""

	for {
		call := c.yt.Videos.List([]string{"snippet", "contentDetails"}).MyRating("like").MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list liked videos: %w", err)
		}
		c.Quota.Spend(CostListPage)

		for _, item := range resp.Items {
			if item.Snippet == nil {
				continue
			}
			var duration int
			if item.ContentDetails != nil {
				duration = parseISO8601Duration(item.ContentDetails.Duration)
			}
			videos = append(videos, store.Video{
				VideoID:         item.Id,
				ChannelID:       item.Snippet.ChannelId,
				ChannelTitle:    item.Snippet.ChannelTitle,
				Title:           item.Snippet.Title,
				PublishedAt:     parseAPITimestamp(item.Snippet.PublishedAt),
				DurationSeconds: duration,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return videos, nil
}
