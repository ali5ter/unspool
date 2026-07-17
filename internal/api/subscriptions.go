package api

import (
	"context"
	"fmt"
	"time"

	"github.com/ali5ter/unspool/internal/store"
)

// ResolveSubscriptions fetches every channel the authenticated user is
// subscribed to via subscriptions.list(mine=true), paginating 50 at a time
// (1 unit/page — PRD §5.1), and derives each channel's Shorts-free uploads
// playlist ID.
func (c *Client) ResolveSubscriptions(ctx context.Context) ([]store.Subscription, error) {
	var subs []store.Subscription
	pageToken := ""

	for {
		call := c.yt.Subscriptions.List([]string{"snippet"}).Mine(true).MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list subscriptions: %w", err)
		}
		c.Quota.Spend(CostListPage)

		for _, item := range resp.Items {
			if item.Snippet == nil || item.Snippet.ResourceId == nil {
				continue
			}
			channelID := item.Snippet.ResourceId.ChannelId
			subs = append(subs, store.Subscription{
				ChannelID:           channelID,
				Title:               item.Snippet.Title,
				UploadsLFPlaylistID: UploadsLongFormPlaylistID(channelID),
				LastSeen:            time.Now(),
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return subs, nil
}

// UploadsLongFormPlaylistID derives a channel's Shorts-free uploads playlist
// ID by taking the 22-character suffix of its "UC…" channel ID and
// prefixing "UULF" (PRD §5.1). This convention is undocumented by Google but
// has been stable for years.
func UploadsLongFormPlaylistID(channelID string) string {
	if len(channelID) < 2 || channelID[:2] != "UC" {
		return ""
	}
	return "UULF" + channelID[2:]
}
