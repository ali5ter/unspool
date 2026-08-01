package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/playback"
	"github.com/ali5ter/unspool/internal/thumbnail"
	"github.com/ali5ter/unspool/internal/tui"
)

func runTUI(cfg *config.Config) error {
	printDependencyWarnings(cfg)
	p := tea.NewProgram(tui.New(cfg))
	_, err := p.Run()
	return err
}

// printDependencyWarnings checks playback (mpv, yt-dlp) and, unless
// thumbnails are disabled, thumbnail (chafa) dependencies, printing one
// tidy warning naming everything missing plus a single fix — neither
// check is fatal, since every feature they gate degrades gracefully
// (playback, and the preview-pane thumbnail) rather than unspool itself
// failing to start. Previously each dependency printed its own
// independent multi-line "warning: ...\n\nInstall with Homebrew:\n  ..."
// block, which read as two unrelated failures instead of one command to
// run — see issue #8. The Homebrew tap's formula (.goreleaser.yml) now
// also declares all three as install-time dependencies, so this path
// should only fire for `go install`/build-from-source installs.
func printDependencyWarnings(cfg *config.Config) {
	missing := playback.Missing()
	if cfg.Thumbnails != "off" {
		if err := thumbnail.CheckDependency(); err != nil {
			missing = append(missing, "chafa")
		}
	}
	if len(missing) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "warning: missing dependencies (%s) — playback and/or preview-pane thumbnails are disabled until installed\n", strings.Join(missing, ", "))
	fmt.Fprintln(os.Stderr, "  fix:", installHint(missing))
}

// installHint returns a one-line, per-platform fix for the given missing
// dependency names, pointing first at scripts/install-deps.sh (this
// repo's codified installer, mirroring scripts/setup-gcp.sh) and falling
// back to the manual command it wraps.
func installHint(missing []string) string {
	switch runtime.GOOS {
	case "darwin":
		return "./scripts/install-deps.sh, or: brew install " + strings.Join(missing, " ")
	case "linux":
		return "./scripts/install-deps.sh, or your package manager, e.g.: sudo apt install " + strings.Join(missing, " ")
	default:
		return "see https://mpv.io, https://github.com/yt-dlp/yt-dlp, and https://hpjansson.org/chafa/"
	}
}
