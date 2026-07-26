package api

import (
	"context"
	"fmt"
	"time"
)

// SearchResult is one hit from search.list.
type SearchResult struct {
	VideoID      string
	Title        string
	ChannelID    string
	ChannelTitle string
	PublishedAt  time.Time
}

// Search runs a genuine web-wide video search via search.list (PRD §5.5) —
// deliberately a single page (no pagination): CostSearch is spent once per
// call, not once per page like the list endpoints, so paginating here would
// silently under-count spend against the "100/day" ration the TUI tracks.
func (c *Client) Search(ctx context.Context, query string, maxResults int64) ([]SearchResult, error) {
	call := c.yt.Search.List([]string{"snippet"}).Q(query).Type("video").MaxResults(maxResults)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("search videos: %w", err)
	}
	c.Quota.Spend(CostSearch)

	results := make([]SearchResult, 0, len(resp.Items))
	for _, item := range resp.Items {
		if item.Id == nil || item.Snippet == nil {
			continue
		}
		pub, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		results = append(results, SearchResult{
			VideoID:      item.Id.VideoId,
			Title:        item.Snippet.Title,
			ChannelID:    item.Snippet.ChannelId,
			ChannelTitle: item.Snippet.ChannelTitle,
			PublishedAt:  pub,
		})
	}
	return results, nil
}
