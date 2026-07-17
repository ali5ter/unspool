package cmd

import (
	"context"
	"fmt"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/feed"
)

func runSync(cfg *config.Config) error {
	result, err := feed.Sync(context.Background(), cfg)
	if err != nil {
		return err
	}
	fmt.Printf("synced %d videos across subscriptions (quota: %d/%d units)\n",
		len(result.Items), result.QuotaSpent, result.QuotaBudget)
	if len(result.SkippedChannels) > 0 {
		fmt.Printf("skipped %d channel(s) this run: %v\n", len(result.SkippedChannels), result.SkippedChannels)
	}
	return nil
}
