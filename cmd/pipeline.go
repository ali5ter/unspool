package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/feed"
)

// pipelineVideo is the --json/--export output shape: a flattened,
// jq-friendly view of a feed item.
type pipelineVideo struct {
	VideoID   string `json:"video_id"`
	Title     string `json:"title"`
	Channel   string `json:"channel"`
	Published string `json:"published_at"`
	Duration  int    `json:"duration_seconds"`
	Seen      bool   `json:"seen"`
}

// loadFeedResult picks the live-sync or local-cache-only read path
// depending on --offline.
func loadFeedResult(cfg *config.Config) (*feed.Result, error) {
	if flagOffline {
		return feed.LoadCached(cfg)
	}
	return feed.Sync(context.Background(), cfg)
}

// toPipelineVideos flattens feed items into the pipeline/export output shape.
func toPipelineVideos(items []feed.Item) []pipelineVideo {
	out := make([]pipelineVideo, 0, len(items))
	for _, it := range items {
		out = append(out, pipelineVideo{
			VideoID:   it.Video.VideoID,
			Title:     it.Video.Title,
			Channel:   it.Channel,
			Published: it.Video.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
			Duration:  it.Video.DurationSeconds,
			Seen:      it.State.Seen,
		})
	}
	return out
}

// writeJSON encodes videos as an indented JSON array.
func writeJSON(w io.Writer, videos []pipelineVideo) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(videos); err != nil {
		return fmt.Errorf("encode feed as JSON: %w", err)
	}
	return nil
}

func runPipeline(cfg *config.Config) error {
	result, err := loadFeedResult(cfg)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, toPipelineVideos(result.Items))
}
