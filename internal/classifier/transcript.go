package classifier

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// FetchTranscript pulls a video's auto-generated captions via yt-dlp (PRD
// §5.2.4 tier 1: "pull the transcript ... it's just text") and returns them
// as plain text, stripped of VTT cue timing and markup.
func FetchTranscript(ctx context.Context, videoID string) (string, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("yt-dlp not found on PATH — see README for install instructions")
	}

	tmpDir, err := os.MkdirTemp("", "unspool-transcript-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	outTemplate := filepath.Join(tmpDir, "sub")
	url := "https://www.youtube.com/watch?v=" + videoID
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--skip-download", "--write-auto-sub", "--sub-lang", "en", "--sub-format", "vtt",
		"-o", outTemplate, url)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}

	matches, err := filepath.Glob(outTemplate + "*.vtt")
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no auto-generated captions available for this video")
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "", err
	}
	return stripVTT(string(data)), nil
}

var vttTagRe = regexp.MustCompile(`<[^>]*>`)
var vttCueTimingRe = regexp.MustCompile(`^\d\d:\d\d:\d\d\.\d\d\d\s*-->\s*\d\d:\d\d:\d\d\.\d\d\d`)

// stripVTT reduces a WebVTT auto-subtitle file to plain text: drops the
// header, cue-timing lines, and inline markup tags, and dedupes
// consecutive repeated lines (yt-dlp's auto-subs repeat the current line
// across several cues as a side effect of how the captions are timed).
func stripVTT(vtt string) string {
	var lines []string
	last := ""
	for _, raw := range strings.Split(vtt, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "", line == "WEBVTT", strings.HasPrefix(line, "Kind:"), strings.HasPrefix(line, "Language:"):
			continue
		case vttCueTimingRe.MatchString(line):
			continue
		}
		line = vttTagRe.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" || line == last {
			continue
		}
		lines = append(lines, line)
		last = line
	}
	return strings.Join(lines, " ")
}
