package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Verdict is the JSON shape both classifier tiers must emit on stdout.
// Score is mainly filled in by the tier-1 transcript classifier;
// LikelyAI/Reasoning/SuspectedTools mainly by the tier-2 inspect hook — but
// either tier may set either field, so both are always accepted. Advisory
// only, per PRD §5.2 — never treated as fact by any caller.
type Verdict struct {
	Score          *float64 `json:"score,omitempty"`
	LikelyAI       bool     `json:"likely_ai"`
	Reasoning      string   `json:"reasoning,omitempty"`
	SuspectedTools []string `json:"suspected_tools,omitempty"`
}

// RunInspect runs the tier-2 on-demand classifier hook (PRD §5.2.4): the
// configured command receives videoURL as $1 and must print a Verdict as
// JSON on stdout.
func RunInspect(ctx context.Context, command, videoURL string) (Verdict, error) {
	out, err := runShellCommand(ctx, command, nil, videoURL)
	if err != nil {
		return Verdict{}, err
	}
	return parseVerdict(out)
}

// RunTranscriptClassifier runs the tier-1 batchable classifier hook (PRD
// §5.2.4): the configured command receives the transcript text on stdin
// and must print a Verdict as JSON on stdout.
func RunTranscriptClassifier(ctx context.Context, command, transcript string) (Verdict, error) {
	out, err := runShellCommand(ctx, command, strings.NewReader(transcript))
	if err != nil {
		return Verdict{}, err
	}
	return parseVerdict(out)
}

func parseVerdict(out []byte) (Verdict, error) {
	var v Verdict
	if err := json.Unmarshal(bytes.TrimSpace(out), &v); err != nil {
		return Verdict{}, fmt.Errorf("classifier output wasn't valid JSON: %w", err)
	}
	return v, nil
}
