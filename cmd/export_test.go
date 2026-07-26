package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func testVideos() []pipelineVideo {
	return []pipelineVideo{
		{VideoID: "abc123", Title: "Plain Title", Channel: "Some Channel", Published: "2026-07-01T10:00:00Z", Duration: 754, Seen: true},
		{VideoID: "def456", Title: "A | Pipe, and a comma", Channel: "Weird | Channel", Published: "2026-07-02T11:30:00Z", Duration: 42, Seen: false},
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCSV(&buf, testVideos()); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}

	want := "video_id,title,channel,published_at,duration_seconds,seen\n" +
		"abc123,Plain Title,Some Channel,2026-07-01T10:00:00Z,754,true\n" +
		"def456,\"A | Pipe, and a comma\",Weird | Channel,2026-07-02T11:30:00Z,42,false\n"
	if got := buf.String(); got != want {
		t.Fatalf("writeCSV output mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := writeMarkdown(&buf, testVideos()); err != nil {
		t.Fatalf("writeMarkdown: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "| Title | Channel | Published | Duration (s) | Seen |\n| --- | --- | --- | --- | --- |\n") {
		t.Fatalf("writeMarkdown header mismatch:\n%s", out)
	}
	if !strings.Contains(out, "A \\| Pipe, and a comma") {
		t.Fatalf("writeMarkdown did not escape a pipe character in a title:\n%s", out)
	}
	if !strings.Contains(out, "Weird \\| Channel") {
		t.Fatalf("writeMarkdown did not escape a pipe character in a channel name:\n%s", out)
	}
}
