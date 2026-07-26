package classifier

import (
	"testing"

	"github.com/ali5ter/unspool/internal/store"
)

func TestScore_Clean(t *testing.T) {
	v := store.Video{Title: "A quiet afternoon fixing a bicycle", Description: "Notes from today's repair."}
	score, reasons := Score(v, 0, 400)
	if score != 0 {
		t.Errorf("score = %v, want 0 (reasons: %v)", score, reasons)
	}
}

func TestScore_ClickbaitTitle(t *testing.T) {
	v := store.Video{Title: "You won't believe what happened next!!!"}
	score, reasons := Score(v, 0, 400)
	if score == 0 {
		t.Errorf("expected a nonzero score for clickbait title, got 0")
	}
	if !contains(reasons, "clickbait title") {
		t.Errorf("reasons = %v, want to contain \"clickbait title\"", reasons)
	}
}

func TestScore_AllCapsHeavy(t *testing.T) {
	v := store.Video{Title: "THIS CHANGES EVERYTHING FOREVER"}
	score, reasons := Score(v, 0, 400)
	if score == 0 {
		t.Errorf("expected a nonzero score for all-caps title, got 0")
	}
	if !contains(reasons, "all-caps title") {
		t.Errorf("reasons = %v, want to contain \"all-caps title\"", reasons)
	}
}

func TestScore_ShortAcronymNotAllCaps(t *testing.T) {
	v := store.Video{Title: "NASA JPL update"}
	_, reasons := Score(v, 0, 400)
	if contains(reasons, "all-caps title") {
		t.Errorf("a short acronym title shouldn't trip the all-caps signal, reasons = %v", reasons)
	}
}

func TestScore_EmojiStuffing(t *testing.T) {
	v := store.Video{Title: "Big news 🔥🔥🔥 you need to see this 🚀"}
	score, reasons := Score(v, 0, 400)
	if score == 0 {
		t.Errorf("expected a nonzero score for emoji stuffing, got 0")
	}
	if !contains(reasons, "emoji-stuffed title") {
		t.Errorf("reasons = %v, want to contain \"emoji-stuffed title\"", reasons)
	}
}

func TestScore_BoilerplateDescription(t *testing.T) {
	v := store.Video{Title: "Weekly update", Description: "Like and subscribe for more!"}
	_, reasons := Score(v, 0, 400)
	if !contains(reasons, "templated description") {
		t.Errorf("reasons = %v, want to contain \"templated description\"", reasons)
	}
}

func TestScore_HighCadence(t *testing.T) {
	v := store.Video{Title: "Daily video"}
	score, reasons := Score(v, highCadenceThreshold, 400)
	if score == 0 {
		t.Errorf("expected a nonzero score for high upload cadence, got 0")
	}
	if !contains(reasons, "high upload cadence") {
		t.Errorf("reasons = %v, want to contain \"high upload cadence\"", reasons)
	}
}

func TestScore_NewChannelHighOutput(t *testing.T) {
	v := store.Video{Title: "Daily video"}
	_, reasons := Score(v, 2, 3)
	if !contains(reasons, "new channel, high output") {
		t.Errorf("reasons = %v, want to contain \"new channel, high output\"", reasons)
	}
}

func TestScore_ClampedToOne(t *testing.T) {
	v := store.Video{
		Title:       "YOU WON'T BELIEVE THIS SHOCKING NEWS 🔥🔥🔥🚀 TOP 10",
		Description: "Like and subscribe for more! Turn on notifications!",
	}
	score, _ := Score(v, highCadenceThreshold, 3)
	if score > 1 {
		t.Errorf("score = %v, want clamped to <= 1", score)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
