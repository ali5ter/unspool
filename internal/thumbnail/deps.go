// Package thumbnail fetches and renders video thumbnails for the preview
// pane. Rendering is deliberately not reimplemented in Go: chafa already
// auto-detects kitty/iTerm2/sixel terminal support and has a first-class
// ANSI-symbol/half-block fallback when none is available, so this package's
// job is only to get the image bytes onto disk and shell out to it.
package thumbnail

import (
	"fmt"
	"os/exec"
	"runtime"
)

// CheckDependency reports whether chafa is on PATH, returning an actionable
// error with per-platform install hints if not. It never blocks startup —
// callers decide whether a missing dependency is fatal or just a
// status-bar/stderr warning, mirroring internal/playback's CheckDependencies.
func CheckDependency() error {
	if _, err := exec.LookPath("chafa"); err != nil {
		return fmt.Errorf("missing thumbnail dependency: chafa\n\n%s", installHint())
	}
	return nil
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "Install with Homebrew:\n  brew install chafa"
	case "linux":
		return "Install with your package manager, e.g.:\n  sudo apt install chafa\n  # or: sudo pacman -S chafa"
	default:
		return "See https://hpjansson.org/chafa/ for install instructions"
	}
}
