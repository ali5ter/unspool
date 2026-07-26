package thumbnail

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Render shells out to chafa to draw imgPath, sized to fit within cols
// columns and rows rows, and returns its raw output ready to embed as a
// single element of a Lip Gloss vertical join.
//
// chafa is only asked for rows-1 rows: its symbol-mode output always ends
// with a short trailing reset/cleanup sequence on its own line beyond the
// requested row count (confirmed empirically, not documented), and the
// returned block is only ever padded — never truncated, since trimming a
// line could cut into live escape-sequence bytes or drop that trailing
// reset (bleeding terminal color/attribute state into whatever renders
// after it). The result is padded up to rows-1 embedded newlines so it
// reliably occupies no more than the caller's reserved row budget.
//
// mode is one of "auto" or "chafa" (symbol/truecolor art — see below for
// why these two are identical) or "halfblock" (force the plainest, most
// portable half-block glyph rendering). Any other value is treated as
// "auto"/"chafa".
//
// "auto" deliberately never lets chafa choose a real bitmap protocol
// (kitty/iTerm2/sixel), even though chafa itself can auto-detect and use
// one: those protocols draw an image spanning multiple terminal rows via a
// single escape sequence, but Bubble Tea's renderer has no concept of
// that — it tracks cursor position purely by counting the newlines it
// wrote, so its next repaint (which can fire within milliseconds, e.g. the
// busy spinner or the periodic header-logo sweep) starts overwriting from
// a row it thinks is correct but isn't, erasing the image almost as soon
// as it's drawn. Confirmed live, both via VHS capture and directly in a
// real iTerm2 window: the fetch/render/cache pipeline succeeded every
// time, but no image ever stayed visible. This is a known, acknowledged
// limitation of the framework, not a bug in this package — see
// https://github.com/charmbracelet/bubbletea/issues/163, where Bubble
// Tea's own maintainer calls native terminal images "functionally
// impossible to handle in TUIs" and recommends half-block instead, which
// is exactly what chafa's "symbols" format (used here) provides.
func Render(ctx context.Context, imgPath string, cols, rows int, mode string) (string, error) {
	if rows < 2 {
		rows = 2
	}
	size := strconv.Itoa(cols) + "x" + strconv.Itoa(rows-1)

	var args []string
	switch mode {
	case "halfblock":
		args = []string{"-f", "symbols", "--symbols=half", "-c", "full", "-s", size, imgPath}
	default: // "auto", "chafa", or anything unrecognized
		args = []string{"-f", "symbols", "-c", "full", "-s", size, imgPath}
	}

	out, err := exec.CommandContext(ctx, "chafa", args...).Output()
	if err != nil {
		return "", fmt.Errorf("render thumbnail: %w", err)
	}

	return padLines(string(out), rows-1), nil
}

// padLines appends trailing newlines to s until it spans at least n
// newlines. It never truncates.
func padLines(s string, n int) string {
	have := strings.Count(s, "\n")
	if have >= n {
		return s
	}
	return s + strings.Repeat("\n", n-have)
}
