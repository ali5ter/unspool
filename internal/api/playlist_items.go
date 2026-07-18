package api

import (
	"context"
	"fmt"

	"google.golang.org/api/youtube/v3"

	"github.com/ali5ter/unspool/internal/store"
)

// PlaylistItemRef is a playlist entry with the playlist-item ID needed to
// remove or reorder it — distinct from the video ID, which YouTube's API
// requires for delete/update calls.
type PlaylistItemRef struct {
	PlaylistItemID string
	VideoID        string
	Title          string
}

// FetchPlaylistItems pulls up to maxItems videos from a playlist via
// playlistItems.list (1 unit/page). Used as the backfill path when the RSS
// feed's ~15-item window isn't enough, e.g. a channel's first sync.
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

// ListPlaylistItemRefs lists every item in a playlist with its playlist-item
// ID (needed for AddPlaylistItem/RemovePlaylistItem bookkeeping), paginating
// 50 at a time (1 unit/page).
func (c *Client) ListPlaylistItemRefs(ctx context.Context, playlistID string) ([]PlaylistItemRef, error) {
	var refs []PlaylistItemRef
	pageToken := ""

	for {
		call := c.yt.PlaylistItems.List([]string{"snippet"}).PlaylistId(playlistID).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list playlist items for %s: %w", playlistID, err)
		}
		c.Quota.Spend(CostListPage)

		for _, item := range resp.Items {
			if item.Snippet == nil || item.Snippet.ResourceId == nil {
				continue
			}
			refs = append(refs, PlaylistItemRef{
				PlaylistItemID: item.Id,
				VideoID:        item.Snippet.ResourceId.VideoId,
				Title:          item.Snippet.Title,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return refs, nil
}

// AddPlaylistItem appends videoID to playlistID (playlistItems.insert, 50
// units) and returns the new playlist-item ID.
func (c *Client) AddPlaylistItem(ctx context.Context, playlistID, videoID string) (string, error) {
	item := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: videoID},
		},
	}
	resp, err := c.yt.PlaylistItems.Insert([]string{"snippet"}, item).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("add video %s to playlist %s: %w", videoID, playlistID, err)
	}
	c.Quota.Spend(CostWrite)
	return resp.Id, nil
}

// RemovePlaylistItem removes a playlist entry by its playlist-item ID
// (playlistItems.delete, 50 units).
func (c *Client) RemovePlaylistItem(ctx context.Context, playlistItemID string) error {
	if err := c.yt.PlaylistItems.Delete(playlistItemID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("remove playlist item %s: %w", playlistItemID, err)
	}
	c.Quota.Spend(CostWrite)
	return nil
}
