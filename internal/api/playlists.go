package api

import (
	"context"
	"fmt"

	"google.golang.org/api/youtube/v3"

	"github.com/ali5ter/unspool/internal/store"
)

// ListPlaylists fetches every playlist the authenticated user owns via
// playlists.list(mine=true), paginating 50 at a time (1 unit/page).
func (c *Client) ListPlaylists(ctx context.Context) ([]store.Playlist, error) {
	var playlists []store.Playlist
	pageToken := ""

	for {
		call := c.yt.Playlists.List([]string{"snippet", "contentDetails"}).Mine(true).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list playlists: %w", err)
		}
		c.Quota.Spend(CostListPage)

		for _, item := range resp.Items {
			if item.Snippet == nil {
				continue
			}
			itemCount := 0
			if item.ContentDetails != nil {
				itemCount = int(item.ContentDetails.ItemCount)
			}
			playlists = append(playlists, store.Playlist{
				PlaylistID: item.Id,
				Title:      item.Snippet.Title,
				ItemCount:  itemCount,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return playlists, nil
}

// CreatePlaylist creates a new private playlist with the given title
// (playlists.insert, 50 units) and returns its ID.
func (c *Client) CreatePlaylist(ctx context.Context, title string) (string, error) {
	playlist := &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{Title: title},
		Status:  &youtube.PlaylistStatus{PrivacyStatus: "private"},
	}
	resp, err := c.yt.Playlists.Insert([]string{"snippet", "status"}, playlist).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create playlist %q: %w", title, err)
	}
	c.Quota.Spend(CostWrite)
	return resp.Id, nil
}
