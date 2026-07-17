// Package playback launches mpv (which uses yt-dlp as its stream backend)
// to play a video, and records the launch to the watch log.
package playback

import (
	"fmt"
	"os/exec"
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

// Play launches mpv on the given video (mpv shells out to yt-dlp as its
// stream backend automatically) and records the launch in the watch log.
// Detached (fire-and-forget) or blocking is controlled by cfg.PlaybackDetached.
func Play(cfg *config.Config, st *store.Store, v store.Video, channel string, audioOnly bool) error {
	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		return &ErrMissingDependency{Bin: "mpv"}
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
		return fmt.Errorf("record watch log: %w", entryErr)
	}

	if cfg.PlaybackDetached {
		return cmd.Start()
	}
	return cmd.Run()
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
