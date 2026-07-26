package thumbnail

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPadLines(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want int // resulting newline count
	}{
		{"no newlines here", 3, 3},
		{"one\ntwo", 5, 5},
		{"a\nb\nc\nd", 2, 3}, // already more than n — never truncated
	}
	for _, c := range cases {
		out := padLines(c.in, c.n)
		if got := strings.Count(out, "\n"); got != c.want {
			t.Errorf("padLines(%q, %d): newline count = %d, want %d", c.in, c.n, got, c.want)
		}
		if !strings.HasPrefix(out, c.in) {
			t.Errorf("padLines(%q, %d) = %q, want it to start with the original content unmodified", c.in, c.n, out)
		}
	}
}

func writeFixtureImage(t *testing.T, dir string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 18))
	for y := 0; y < 18; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 8), uint8(y * 14), 128, 255})
		}
	}
	path := filepath.Join(dir, "fixture.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture image: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode fixture image: %v", err)
	}
	return path
}

func TestRenderModes(t *testing.T) {
	if _, err := exec.LookPath("chafa"); err != nil {
		t.Skip("chafa not on PATH")
	}

	path := writeFixtureImage(t, t.TempDir())
	const rows = 8

	for _, mode := range []string{"auto", "chafa", "halfblock"} {
		t.Run(mode, func(t *testing.T) {
			out, err := Render(context.Background(), path, 20, rows, mode)
			if err != nil {
				t.Fatalf("Render(%q): %v", mode, err)
			}
			if out == "" {
				t.Fatalf("Render(%q) returned empty output", mode)
			}
			if got := strings.Count(out, "\n"); got < rows-1 {
				t.Fatalf("Render(%q): newline count = %d, want at least %d (rows-1)", mode, got, rows-1)
			}
		})
	}
}
