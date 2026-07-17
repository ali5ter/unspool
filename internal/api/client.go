// Package api wraps the YouTube Data API v3 and its quota-free RSS
// companion, implementing the Shorts-free subscription feed and staying
// disciplined about quota spend.
package api

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Per-call quota costs. videos.rate, playlists.insert, and
// playlistItems.insert/delete/update all cost 50 units; search.list costs
// 100; list calls (subscriptions, playlistItems, playlists, videos) cost 1
// unit per page.
const (
	CostListPage = 1
	CostWrite    = 50
	CostSearch   = 100
)

// Client wraps the YouTube Data API v3 service with in-process quota
// tracking. Quota is never persisted across runs — it resets with the
// process, matching the "never poll on a timer" design invariant.
type Client struct {
	yt    *youtube.Service
	Quota *Quota
}

// NewClient builds a Client from an already-authenticated HTTP client (see
// internal/auth.Client).
func NewClient(ctx context.Context, hc *http.Client) (*Client, error) {
	svc, err := youtube.NewService(ctx, option.WithHTTPClient(hc))
	if err != nil {
		return nil, fmt.Errorf("create YouTube API client: %w", err)
	}
	return &Client{yt: svc, Quota: NewQuota()}, nil
}
