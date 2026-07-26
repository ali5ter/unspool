package thumbnail

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// thumbnailBaseURL is YouTube's stable (if undocumented) per-video
// thumbnail base — a var, not a const, so tests can point it at an
// httptest.Server instead of the real network.
var thumbnailBaseURL = "https://i.ytimg.com/vi/"

// candidates are the per-video thumbnail URLs to try, most-preferred
// first. Not every video has an mqdefault — some very old or restricted
// ones only have the smaller default-quality image.
func candidates(videoID string) []string {
	base := thumbnailBaseURL + videoID + "/"
	return []string{base + "mqdefault.jpg", base + "hqdefault.jpg", base + "default.jpg"}
}

// Fetch returns the local path to videoID's cached thumbnail under cacheDir,
// downloading it first if not already cached. Cached files are never
// re-validated — a thumbnail doesn't change for a given video ID. The
// caller's ctx bounds the whole operation (matching internal/classifier's
// exec.CommandContext convention — the caller owns the deadline, not the
// callee) since this is a network call with no business hanging the TUI.
func Fetch(ctx context.Context, videoID, cacheDir string) (string, error) {
	dest := filepath.Join(cacheDir, videoID+".jpg")
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create thumbnail cache dir: %w", err)
	}

	var lastErr error
	for _, url := range candidates(videoID) {
		if err := download(ctx, url, dest); err != nil {
			lastErr = err
			continue
		}
		return dest, nil
	}
	return "", fmt.Errorf("fetch thumbnail for %s: %w", videoID, lastErr)
}

// download GETs url and writes it atomically to dest (temp file + rename),
// matching internal/store's write convention so an interrupted download
// can never leave a corrupt partial image on disk.
func download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: unexpected status %s", url, resp.Status)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}
