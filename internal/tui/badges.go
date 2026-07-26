package tui

import "charm.land/bubbles/v2/list"

// verdictBadgeText is shown when a cached inspect verdict (tier 2) came
// back LikelyAI — it has to actually state the verdict, not just that an
// inspection happened. An earlier version read "🤖 AI (inspected)", which
// only says a check occurred, not what it found — genuinely ambiguous
// (reported directly: "how does this tell me it's AI generated?"). Kept
// deliberately close to the inspect dialog's own wording ("likely
// AI-generated") for consistency, and still advisory per PRD §5.2 — never
// "AI", always "likely AI".
const verdictBadgeText = "  🤖 likely AI (inspected)"

// aiBadgeFor returns the AI-slop badge text for a video (a leading
// "  "-prefixed fragment, or "" if unflagged), combining two independent
// signals per PRD §5.2's layered design:
//   - a cached inspect verdict (`i`, tier 2) with LikelyAI true — a human
//     explicitly checked this video and got a high-confidence advisory
//     result, so it's badged consistently wherever it appears from then on,
//     not just recalled if `i` happens to be pressed on it again.
//   - the cheap metadata heuristic score (tier "secondary, advisory") —
//     only meaningful where heuristicScore is actually available (the Feed
//     tab, computed during sync); callers elsewhere pass 0, which can never
//     cross a threshold > 0.
//
// Advisory only in both cases — never asserts a video "is AI" as fact.
func (m Model) aiBadgeFor(videoID string, heuristicScore float64) string {
	if v, ok := m.verdicts[videoID]; ok && v.LikelyAI {
		return verdictBadgeText
	}
	threshold := m.cfg.Filters.AIScoreThreshold
	if threshold > 0 && heuristicScore >= threshold {
		return "  🤖 slop?"
	}
	return ""
}

// patchInspectedBadge updates videoID's badge to reflect a just-cached
// "likely AI" verdict in whichever list it was selected from for
// inspection — without this, a freshly-inspected video's badge wouldn't
// appear until that list's next reload (a full resync, or switching away
// from and back to the tab), which would read as "nothing happened" right
// after the one moment a user is actually watching for it.
func (m *Model) patchInspectedBadge(videoID string) {
	if m.searchResultsActive {
		patchSearchResultBadge(&m.searchResultsList, videoID, verdictBadgeText)
		return
	}
	switch m.activeTab {
	case tabFeed:
		patchFeedBadge(&m.feedList, videoID, verdictBadgeText)
	case tabQueue:
		patchQueueBadge(&m.queueList, videoID, verdictBadgeText)
	case tabPlaylists:
		patchPlaylistItemBadge(&m.playlistItemsList, videoID, verdictBadgeText)
	case tabLiked:
		patchLikedBadge(&m.likedList, videoID, verdictBadgeText)
	case tabRecommended:
		patchRecommendedBadge(&m.recommendedList, videoID, verdictBadgeText)
	}
}

func patchFeedBadge(l *list.Model, videoID, badge string) {
	for i, it := range l.Items() {
		row, ok := it.(feedItem)
		if !ok || row.Video.VideoID != videoID {
			continue
		}
		row.aiBadge = badge
		l.SetItem(i, row)
		return
	}
}

func patchQueueBadge(l *list.Model, videoID, badge string) {
	for i, it := range l.Items() {
		row, ok := it.(queueRow)
		if !ok || row.videoID != videoID {
			continue
		}
		row.aiBadge = badge
		l.SetItem(i, row)
		return
	}
}

func patchPlaylistItemBadge(l *list.Model, videoID, badge string) {
	for i, it := range l.Items() {
		row, ok := it.(playlistItemRow)
		if !ok || row.ref.VideoID != videoID {
			continue
		}
		row.aiBadge = badge
		l.SetItem(i, row)
		return
	}
}

func patchLikedBadge(l *list.Model, videoID, badge string) {
	for i, it := range l.Items() {
		row, ok := it.(likedRow)
		if !ok || row.video.VideoID != videoID {
			continue
		}
		row.aiBadge = badge
		l.SetItem(i, row)
		return
	}
}

func patchRecommendedBadge(l *list.Model, videoID, badge string) {
	for i, it := range l.Items() {
		row, ok := it.(recommendedRow)
		if !ok || row.item.Video.VideoID != videoID {
			continue
		}
		row.aiBadge = badge
		l.SetItem(i, row)
		return
	}
}

func patchSearchResultBadge(l *list.Model, videoID, badge string) {
	for i, it := range l.Items() {
		row, ok := it.(searchResultRow)
		if !ok || row.result.Video.VideoID != videoID {
			continue
		}
		row.aiBadge = badge
		l.SetItem(i, row)
		return
	}
}
