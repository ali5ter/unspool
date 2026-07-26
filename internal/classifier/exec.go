package classifier

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// runShellCommand runs command through "sh -c", appending extraArgs to
// command's own argv via "$@" forwarding — the standard EDITOR/PAGER Unix
// convention (e.g. `sh -c "$EDITOR \"$@\"" -- "$file"`), not by relying on
// command's text to reference "$1" itself.
//
// That distinction is real, not pedantic: an earlier version did `sh -c
// command -- extraArgs...`, which only delivers extraArgs into command's
// OWN execution scope if command's text explicitly contains "$1" — a bare
// script path (the overwhelmingly common case for classifier.
// inspect_command/transcript_command, e.g. "/path/to/my-script.sh") never
// does, so extraArgs silently never reached the invoked script at all.
// Confirmed live, not just in theory: a real inspect_command pointed at a
// script failed its own "$1 unset" usage check on every call from
// unspool, despite running perfectly when invoked by hand with the same
// URL as a plain argument. "$@" forwarding fixes this for a bare path (it
// just receives extraArgs as normal argv, no shell trivia required) and
// still works for a multi-word inline command.
//
// If stdin is non-nil it's piped to the command (used by the tier-1
// transcript_command, which "receives transcript text on stdin"). Mirrors
// internal/playback's dual-stream-capture idiom.
func runShellCommand(ctx context.Context, command string, stdin io.Reader, extraArgs ...string) ([]byte, error) {
	wrapped := command + ` "$@"`
	args := append([]string{"-c", wrapped, "--"}, extraArgs...)
	cmd := exec.CommandContext(ctx, "sh", args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return out.Bytes(), nil
}
