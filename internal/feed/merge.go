package feed

import "github.com/ali5ter/unspool/internal/store"

// mergeVideos combines cached videos with freshly fetched ones, deduplicating
// by video ID and preferring existing entries (which may already carry
// duration/provenance/description data) while refreshing mutable fields like
// title. Returns the merged set plus the IDs that have never had
// FetchVideoDetails run for them, for batching. Videos cached before the
// preview pane shipped predate DetailsFetched, so this also backfills their
// descriptions — a one-time cost the next time each channel syncs.
func mergeVideos(existing, fresh []store.Video) ([]store.Video, []string) {
	byID := make(map[string]store.Video, len(existing)+len(fresh))
	order := make([]string, 0, len(existing)+len(fresh))

	for _, v := range existing {
		byID[v.VideoID] = v
		order = append(order, v.VideoID)
	}
	for _, v := range fresh {
		if prior, ok := byID[v.VideoID]; ok {
			prior.Title = v.Title
			byID[v.VideoID] = prior
			continue
		}
		byID[v.VideoID] = v
		order = append(order, v.VideoID)
	}

	merged := make([]store.Video, 0, len(order))
	var needDetails []string
	for _, id := range order {
		v := byID[id]
		merged = append(merged, v)
		if !v.DetailsFetched {
			needDetails = append(needDetails, id)
		}
	}
	return merged, needDetails
}
