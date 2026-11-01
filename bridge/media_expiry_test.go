package vidotter

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMediaExpiryIsHostIndependentAndConservative(t *testing.T) {
	now := time.Unix(1793500590, 0)
	tests := []struct {
		label    string
		url      string
		expiring bool
		known    bool
	}{
		{"expired seconds", "https://media.test/playlist.m3u8?token=secret&expires=1793500557", true, true},
		{"expired ms", "https://another.test/m.m3u8?EXP=1793500557000", true, true},
		{"future seconds", "https://media.test/playlist.m3u8?expires=1793500700", false, true},
		{"within margin", "https://media.test/m.m3u8?expiry=1793500605", true, true},
		{"unrecognized token", "https://media.test/playlist.m3u8?token=abc", false, false},
		{"unrecognized expiry", "https://media.test/playlist.m3u8?exp=garbage", false, false},
		{"ambiguous numeric duration", "https://media.test/playlist.m3u8?exp=300", false, false},
		{"unrelated integer", "https://media.test/playlist.m3u8?item=1793500557", false, false},
		{"ISO time", "https://media.test/playlist.m3u8?expiration=2026-11-01T02%3A35%3A57Z", true, true},
		{"invalid URL", "not a url", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			_, known := mediaURLExpiry(tt.url)
			if known != tt.known || mediaURLExpiring(tt.url, now) != tt.expiring {
				t.Fatalf("expiry mismatch: %q => known %t, expiring %t", tt.url, known, mediaURLExpiring(tt.url, now))
			}
		})
	}
}

func TestExpiredPlaylistDoesNotIssueRequestOrLeakURL(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	source := fmt.Sprintf("%s/master.m3u8?token=one-off-secret&expires=1793500557", server.URL)
	app := &app{workDir: t.TempDir()}
	_, err := app.fetchPlaylist(source, "playlist.m3u8")
	if err == nil || mediaFailureReason(err) != "media_expired" || count != 0 {
		t.Fatalf("expired playlist dispatched: %d, %v", count, err)
	}
	if strings.Contains(err.Error(), "one-off-secret") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("signed URL exposed in error: %v", err)
	}
}

func TestLivePlaylist403ReturnsStructuredReasonWithoutResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("sensitive-key=never-print-this"))
	}))
	defer server.Close()
	app := &app{workDir: t.TempDir()}
	_, err := app.fetchPlaylist(server.URL+"/master.m3u8", "playlist.m3u8")
	if err == nil || mediaFailureReason(err) != "media_access_denied" {
		t.Fatalf("expected access denied: %v", err)
	}
	if strings.Contains(err.Error(), "never-print-this") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("HTTP 403 response leaked: %v", err)
	}
	if mediaFailureReason(fmt.Errorf("task failed: %w", err)) != "media_access_denied" {
		t.Fatal("typed reason was lost through wrapped error")
	}
}

func TestSignedMediaJobStoreDoesNotPersistCredentialsOrURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	now := time.Now()
	const id = "123456789abc"
	const media = "https://stream.test/path-is-secret/video.m3u8?token=private-query&expires=1793500557"
	const page = "https://page.test/watch?session=private-page"
	const errorText = "fetch " + media + ": HTTP 403; " + page
	jobs := map[string]*bridgeJob{id: {
		ID: id, Status: "running", Phase: "resolving",
		SourceURL: media, PageURL: page, Error: errorText,
		CreatedAt: now, UpdatedAt: now,
		task: bridgeTask{
			URL: media, PageURL: page, Referer: page,
			MediaHeaders: map[string]string{
				"X-Private-Token": "private-header",
				"Referer":         page,
			},
		},
	}}
	if err := saveBridgeJobs(path, jobs); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-query", "private-page", "path-is-secret", "private-header"} {
		if bytes.Contains(content, []byte(secret)) {
			t.Fatalf("jobs.json leaked %s", secret)
		}
	}
	if jobs[id].task.URL != media {
		t.Fatal("persistence altered the live task's URL")
	}
	restored, err := loadBridgeJobs(path)
	if err != nil {
		t.Fatal(err)
	}
	job := restored[id]
	if job.Status != "needs-session" || !job.RequiresFreshSession || job.task.URL != "" {
		t.Fatalf("signed task resumed without fresh browser: %+v", job)
	}
}

func TestSafeErrorRedactsSignedURL(t *testing.T) {
	msg := "request failed (URL=\"https://media.test/hidden/secret.m3u8?token=private-value\")"
	safe := safeJobError(msg)
	if strings.Contains(safe, "private-value") || strings.Contains(safe, "/hidden/") {
		t.Fatal("safeJobError did not redact signed URL")
	}
	if !strings.Contains(safe, "request failed") {
		t.Fatal("diagnostic context was lost")
	}
	if nonDurableMediaURL("https://media.test/file.mp4?download=1") != true {
		t.Fatal("unrecognized queries may contain signatures")
	}
	if nonDurableMediaURL("https://media.test/file.mp4") {
		t.Fatal("plain public URL should remain resumable")
	}
	if mediaFailureReason(errors.New("HTTP 403")) != "" {
		t.Fatal("error classification may not guess based on message text")
	}
}
