package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/feed"
)

// pipelineVideo is the --json output shape: a flattened, jq-friendly view of
// a feed item.
type pipelineVideo struct {
	VideoID   string `json:"video_id"`
	Title     string `json:"title"`
	Channel   string `json:"channel"`
	Published string `json:"published_at"`
	Duration  int    `json:"duration_seconds"`
	Seen      bool   `json:"seen"`
}

func runPipeline(cfg *config.Config) error {
	result, err := feed.Sync(context.Background(), cfg)
	if err != nil {
		return err
	}

	out := make([]pipelineVideo, 0, len(result.Items))
	for _, it := range result.Items {
		out = append(out, pipelineVideo{
			VideoID:   it.Video.VideoID,
			Title:     it.Video.Title,
			Channel:   it.Channel,
			Published: it.Video.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
			Duration:  it.Video.DurationSeconds,
			Seen:      it.State.Seen,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode feed as JSON: %w", err)
	}
	return nil
}
