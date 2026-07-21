// Package playback launches mpv (which uses yt-dlp as its stream backend)
// to play a video, and records the launch to the watch log.
package playback

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/store"
)

// startupGrace is how long Play waits, after launching mpv detached,
// before reporting success. mpv is a real GUI app with its own startup
// handshake (window creation, GPU context, yt-dlp stream resolution) — an
// exit inside this window is treated as a launch failure, not legitimate
// playback ending early.
const startupGrace = 2 * time.Second

// ErrMissingDependency is returned when mpv isn't on PATH.
type ErrMissingDependency struct {
	Bin string
}

func (e *ErrMissingDependency) Error() string {
	return fmt.Sprintf("%s not found on PATH — see README for install instructions", e.Bin)
}

// Play launches mpv on the given video (mpv shells out to yt-dlp as its
// stream backend automatically) and records the launch in the watch log.
// Detached (fire-and-forget) or blocking is controlled by cfg.PlaybackDetached.
//
// Returns the spawned process when detached, so a caller can kill it later —
// mpv opens its own native window, which on macOS often doesn't take focus
// when launched from a background process, leaving it unreachable by mouse
// or keyboard. Without a way to kill it from here, that's a stuck,
// unstoppable video with only "quit the whole terminal session" as a way out.
func Play(cfg *config.Config, st *store.Store, v store.Video, channel string, audioOnly bool) (*os.Process, error) {
	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		return nil, &ErrMissingDependency{Bin: "mpv"}
	}

	args := buildArgs(cfg, audioOnly)
	url := "https://www.youtube.com/watch?v=" + v.VideoID
	cmd := exec.Command(mpvPath, append(args, url)...)

	if entryErr := st.AppendWatchLog(store.WatchLogEntry{
		VideoID:   v.VideoID,
		Title:     v.Title,
		Channel:   channel,
		StartedAt: time.Now(),
	}); entryErr != nil {
		return nil, fmt.Errorf("record watch log: %w", entryErr)
	}

	if !cfg.PlaybackDetached {
		return nil, cmd.Run()
	}

	// mpv logs its actual errors (bad video ID, extraction failure, missing
	// codec) to stdout, not stderr — confirmed directly: a deliberately
	// invalid video ID produced nothing on stderr and the real message
	// ("[ytdl_hook] ERROR: ... Video unavailable") on stdout. Capture both
	// into the same buffer so whichever stream carries it is caught.
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// mpv is launched detached (fire-and-forget, for UI responsiveness) —
	// but cmd.Start() only reports fork/exec failures, not anything mpv
	// itself goes wrong on afterward (a deleted/region-locked video, a
	// yt-dlp extraction failure, a missing codec). Without this, that
	// whole class of failure launches "successfully" at the OS level and
	// then dies silently a moment later: the status bar says "playing…"
	// forever, with no window, no audio, and no indication anything went
	// wrong. Waiting briefly for an early exit catches it. The Wait() call
	// is required regardless, detached or not — without it, mpv becomes a
	// zombie process under unspool once it does exit.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case waitErr := <-done:
		msg := strings.TrimSpace(out.String())
		if msg == "" && waitErr != nil {
			msg = waitErr.Error()
		}
		if msg == "" {
			msg = "mpv exited immediately with no output"
		}
		return nil, fmt.Errorf("%s", msg)
	case <-time.After(startupGrace):
		// Still running past the grace window — report success. The
		// Wait() goroutine above keeps running to reap the process
		// whenever it does eventually exit (naturally or via Stop);
		// nothing left to do with that result now.
		return cmd.Process, nil
	}
}

// Stop kills a process returned by Play. Safe to call with nil (no-op).
func Stop(p *os.Process) error {
	if p == nil {
		return nil
	}
	return p.Kill()
}

func buildArgs(cfg *config.Config, audioOnly bool) []string {
	var args []string
	if cfg.MaxResolution > 0 {
		args = append(args, fmt.Sprintf("--ytdl-format=bestvideo[height<=?%d]+bestaudio/best", cfg.MaxResolution))
	}
	if audioOnly || cfg.AudioOnlyDefault {
		args = append(args, "--no-video")
	}
	if cfg.CookiesFromBrowser != "" {
		args = append(args, "--ytdl-raw-options=cookies-from-browser="+cfg.CookiesFromBrowser)
	}
	return args
}
