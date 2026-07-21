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

// ErrMissingDependency is returned when mpv isn't on PATH.
type ErrMissingDependency struct {
	Bin string
}

func (e *ErrMissingDependency) Error() string {
	return fmt.Sprintf("%s not found on PATH — see README for install instructions", e.Bin)
}

// Handle is a launched, detached mpv process that hasn't been waited on
// yet. The caller decides when and how to wait for its eventual exit (see
// Wait) — Play itself no longer guesses at this by waiting a fixed grace
// period before reporting success, which turned out not to be safely
// possible: confirmed live against a real account's Liked videos, mpv/
// yt-dlp failures ("Video unavailable", extraction errors) surfaced
// anywhere from ~2.2s to 30+ seconds after launch depending on the reason,
// with no fixed cutoff able to safely tell a slow failure from a video
// that's genuinely still starting up.
type Handle struct {
	cmd *exec.Cmd
	out *bytes.Buffer
}

// Process returns the OS process handle, e.g. to kill it early (see Stop).
func (h *Handle) Process() *os.Process {
	return h.cmd.Process
}

// Wait blocks until mpv exits, returning a diagnostic error built from its
// captured output if it exited non-zero (confirmed live: every observed
// failure did) — a clean (zero) exit is treated as normal, whether that's
// the video finishing or the user closing the window, since nothing mpv
// itself flagged as wrong. The caller decides what a late failure still
// means by the time it arrives — the user may have already stopped this
// video or started another.
func (h *Handle) Wait() error {
	waitErr := h.cmd.Wait()
	if waitErr == nil {
		return nil
	}
	msg := strings.TrimSpace(h.out.String())
	if msg == "" {
		msg = waitErr.Error()
	}
	return fmt.Errorf("%s", msg)
}

// Play launches mpv on the given video (mpv shells out to yt-dlp as its
// stream backend automatically) and records the launch in the watch log.
// Detached (fire-and-forget) or blocking is controlled by cfg.PlaybackDetached.
//
// Returns a Handle when detached, so a caller can kill it early (Stop) or
// wait for its eventual exit (Handle.Wait) — mpv opens its own native
// window, which on macOS often doesn't take focus when launched from a
// background process, leaving it unreachable by mouse or keyboard. Without
// a way to kill it from here, that's a stuck, unstoppable video with only
// "quit the whole terminal session" as a way out.
func Play(cfg *config.Config, st *store.Store, v store.Video, channel string, audioOnly bool) (*Handle, error) {
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
	return &Handle{cmd: cmd, out: &out}, nil
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
