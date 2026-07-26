package classifier

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunShellCommand_BareScriptPathReceivesArgs is a regression test for a
// real bug: a bare script path (the common case for classifier.
// inspect_command/transcript_command) never received extraArgs at all,
// because sh -c command -- args only forwards args into command's own
// execution scope if command's text explicitly references "$1" — a plain
// path never does.
func TestRunShellCommand_BareScriptPathReceivesArgs(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "echo-arg.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"got:$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runShellCommand(context.Background(), script, nil, "https://example.com/video")
	if err != nil {
		t.Fatalf("runShellCommand: %v", err)
	}
	got := strings.TrimSpace(string(out))
	want := "got:https://example.com/video"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunShellCommand_InlineCommandStillWorks(t *testing.T) {
	out, err := runShellCommand(context.Background(), `printf 'got:%s'`, nil, "hello")
	if err != nil {
		t.Fatalf("runShellCommand: %v", err)
	}
	got := string(out)
	want := "got:hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunShellCommand_Stdin(t *testing.T) {
	out, err := runShellCommand(context.Background(), "cat", strings.NewReader("piped text"))
	if err != nil {
		t.Fatalf("runShellCommand: %v", err)
	}
	if string(out) != "piped text" {
		t.Errorf("got %q, want %q", string(out), "piped text")
	}
}
