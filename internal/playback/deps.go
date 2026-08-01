package playback

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Missing returns the names of any playback dependency (mpv, yt-dlp) not
// found on PATH, or nil if both are present — the raw check behind
// CheckDependencies, exposed separately so cmd.runTUI can fold it into one
// combined warning alongside internal/thumbnail's chafa check instead of
// printing each dependency's own formatted error independently (issue #8).
func Missing() []string {
	var missing []string
	if _, err := exec.LookPath("mpv"); err != nil {
		missing = append(missing, "mpv")
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		missing = append(missing, "yt-dlp")
	}
	return missing
}

// CheckDependencies reports whether mpv and yt-dlp are on PATH, returning an
// actionable error listing per-platform install hints for whatever is
// missing. It never blocks startup — callers decide whether a missing
// dependency should be fatal or just a status-bar warning.
func CheckDependencies() error {
	missing := Missing()
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing playback dependencies: %v\n\n%s", missing, installHint())
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "Install with Homebrew:\n  brew install mpv yt-dlp"
	case "linux":
		return "Install with your package manager, e.g.:\n  sudo apt install mpv yt-dlp\n  # or: sudo pacman -S mpv yt-dlp"
	default:
		return "See https://mpv.io and https://github.com/yt-dlp/yt-dlp for install instructions"
	}
}
