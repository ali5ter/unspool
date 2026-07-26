package thumbnail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCandidates(t *testing.T) {
	got := candidates("abc123")
	want := []string{
		thumbnailBaseURL + "abc123/mqdefault.jpg",
		thumbnailBaseURL + "abc123/hqdefault.jpg",
		thumbnailBaseURL + "abc123/default.jpg",
	}
	if len(got) != len(want) {
		t.Fatalf("candidates() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidates()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFetchCachesOnDisk(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer srv.Close()

	orig := thumbnailBaseURL
	thumbnailBaseURL = srv.URL + "/vi/"
	defer func() { thumbnailBaseURL = orig }()

	dir := t.TempDir()
	ctx := context.Background()

	path, err := Fetch(ctx, "vid1", dir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests after first Fetch = %d, want 1", requests)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached file: %v", err)
	}
	if string(data) != "fake-jpeg-bytes" {
		t.Fatalf("cached content = %q, want %q", data, "fake-jpeg-bytes")
	}

	if _, err := Fetch(ctx, "vid1", dir); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests after second (cached) Fetch = %d, want still 1 (no re-download)", requests)
	}
}

func TestFetchFallsBackThroughCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Base(r.URL.Path) == "hqdefault.jpg" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hq-bytes"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := thumbnailBaseURL
	thumbnailBaseURL = srv.URL + "/vi/"
	defer func() { thumbnailBaseURL = orig }()

	dir := t.TempDir()
	path, err := Fetch(context.Background(), "vid2", dir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached file: %v", err)
	}
	if string(data) != "hq-bytes" {
		t.Fatalf("cached content = %q, want fallback %q", data, "hq-bytes")
	}
}

func TestFetchAllCandidatesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := thumbnailBaseURL
	thumbnailBaseURL = srv.URL + "/vi/"
	defer func() { thumbnailBaseURL = orig }()

	if _, err := Fetch(context.Background(), "vid3", t.TempDir()); err == nil {
		t.Fatal("Fetch: want an error when every candidate URL 404s, got nil")
	}
}
