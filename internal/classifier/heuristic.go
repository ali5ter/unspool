// Package classifier implements unspool's AI-slop signal layer (PRD §5.2):
// a cheap metadata heuristic (this file), and the model-agnostic LLM
// shell-out hooks (exec.go, inspect.go, transcript.go). Every signal here is
// advisory — none of it is ever surfaced as a fact.
package classifier

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/ali5ter/unspool/internal/store"
)

// highCadenceThreshold is how many same-channel uploads within a 7-day
// window of a video's own publish date counts as "suspiciously high
// cadence" (PRD §5.2.2).
const highCadenceThreshold = 4

// newChannelAgeDays is how young a channel's earliest cached upload has to
// be for "brand-new channel + high output" to contribute its own signal,
// distinct from (and additive to) plain high cadence.
const newChannelAgeDays = 14

var clickbaitPhrases = []string{
	"you won't believe", "won't believe what", "gone wrong", "gone sexual",
	"in this video", "before you", "this is why", "the truth about",
	"they don't want you to know", "must watch", "shocking", "insane",
	"you need to see", "watch until the end",
}

var clickbaitRe = regexp.MustCompile(`\b(top \d+|number \d+)\b`)

var boilerplateDescPhrases = []string{
	"subscribe for more", "like and subscribe", "don't forget to subscribe",
	"turn on notifications", "smash that like button",
}

// Score returns a 0–1 "likely AI-slop" heuristic score for v plus the
// signals that contributed, from cheap metadata already fetched during
// sync. recentChannelUploads is the count of the same channel's videos
// published within 7 days of v's own PublishedAt (see feed.uploadCadence);
// channelAgeDays is how many days old the channel's earliest cached upload
// is. Purely advisory — see the package doc comment.
func Score(v store.Video, recentChannelUploads, channelAgeDays int) (float64, []string) {
	var score float64
	var reasons []string

	if hasClickbaitTitle(v.Title) {
		score += 0.25
		reasons = append(reasons, "clickbait title")
	}
	if isAllCapsHeavy(v.Title) {
		score += 0.15
		reasons = append(reasons, "all-caps title")
	}
	if hasEmojiStuffing(v.Title) {
		score += 0.15
		reasons = append(reasons, "emoji-stuffed title")
	}
	if hasBoilerplateDescription(v.Description) {
		score += 0.15
		reasons = append(reasons, "templated description")
	}
	if recentChannelUploads >= highCadenceThreshold {
		score += 0.25
		reasons = append(reasons, "high upload cadence")
	}
	if channelAgeDays >= 0 && channelAgeDays <= newChannelAgeDays && recentChannelUploads >= 2 {
		score += 0.2
		reasons = append(reasons, "new channel, high output")
	}

	if score > 1 {
		score = 1
	}
	return score, reasons
}

func hasClickbaitTitle(title string) bool {
	lower := strings.ToLower(title)
	for _, phrase := range clickbaitPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return clickbaitRe.MatchString(lower)
}

// isAllCapsHeavy reports whether more than half of a title's letters are
// uppercase — a common clickbait/slop signal, but only meaningful once
// there's enough letters to judge (short acronym-only titles shouldn't
// trip this).
func isAllCapsHeavy(title string) bool {
	var upper, letters int
	for _, r := range title {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.IsUpper(r) {
			upper++
		}
	}
	if letters < 12 {
		return false
	}
	return float64(upper)/float64(letters) > 0.7
}

// hasEmojiStuffing reports whether a title contains several emoji —
// templated AI-slop titles lean heavily on emoji as visual filler.
func hasEmojiStuffing(title string) bool {
	count := 0
	for _, r := range title {
		if isEmojiRune(r) {
			count++
		}
	}
	return count >= 3
}

func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF: // symbols, pictographs, emoticons, supplemental
		return true
	case r >= 0x2600 && r <= 0x27BF: // misc symbols, dingbats
		return true
	}
	return false
}

func hasBoilerplateDescription(desc string) bool {
	lower := strings.ToLower(desc)
	for _, phrase := range boilerplateDescPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}
