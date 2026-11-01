package vidotter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newBrowserWaitTestServer(t *testing.T) *bridgeServer {
	t.Helper()
	return &bridgeServer{
		jobs:         make(map[string]*bridgeJob),
		pending:      make(map[string]pendingBridgeTask),
		jobStorePath: filepath.Join(t.TempDir(), "jobs.json"),
		browserWake:  make(chan struct{}, 1),
	}
}

func TestBrowserWaitPersistsOnlyPageHashAndRestores(t *testing.T) {
	s := newBrowserWaitTestServer(t)
	const page = "https://example.org/watch?token=private-query-token#fragment"
	subtitles := SubtitleRequest{Mode: "site", Languages: []string{"en"}}
	id, err := s.createBrowserWait(page, "", "480", subtitles)
	if err != nil || !mcpJobID.MatchString(id) {
		t.Fatalf("create waiting task: %q, %v", id, err)
	}
	data, err := os.ReadFile(s.jobStorePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("private-query-token")) || bytes.Contains(data, []byte("#fragment")) {
		t.Fatal("raw page URL leaked to durable job store")
	}
	if !bytes.Contains(data, []byte(browserPageDigest(page))) {
		t.Fatal("exact-page hash not persisted")
	}
	second, err := s.createBrowserWait(page, "", "480", subtitles)
	if err != nil || second != id {
		t.Fatalf("duplicate request created a second task: %q %q %v", id, second, err)
	}
	different, err := s.createBrowserWait(page, "", "720", subtitles)
	if err != nil || different == id {
		t.Fatalf("different quality must remain independent: %q, %v", different, err)
	}
	restored, err := loadBridgeJobs(s.jobStorePath)
	if err != nil {
		t.Fatal(err)
	}
	job := restored[id]
	if job == nil || job.Status != "waiting_for_browser" ||
		job.BrowserPageHash != browserPageDigest(page) ||
		job.task.Quality != "480" || job.task.Subtitles.Mode != "site" {
		t.Fatalf("waiting task not restored correctly: %+v", job)
	}
	exposed := summarizeMCPJob(*job)
	if !exposed.RequiresUserAction || exposed.ActionMessage == "" {
		t.Fatalf("user action missing from MCP summary: %+v", exposed)
	}
}

func TestBrowserWaitClaimSameIDAndNoDuplicate(t *testing.T) {
	s := newBrowserWaitTestServer(t)
	const page = "https://example.org/watch?token=alpha"
	id, err := s.createBrowserWait(page, "Video title", "720", SubtitleRequest{Mode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	task := bridgeTask{
		ResumeJobID: id, ViaMCP: true,
		PageURL: page, URL: "https://cdn.example.org/video.m3u8?sig=secret",
		Title: "Actual video", Quality: "1080",
		Subtitles:    SubtitleRequest{Mode: "asr", ASRLanguage: "en"},
		MediaCookies: []browserCookie{{Name: "session", Value: "secret-cookie"}},
	}
	invalid := task
	invalid.PageURL = "https://example.org/watch?token=beta"
	if _, _, err := s.claimBrowserWait(invalid); err == nil {
		t.Fatal("mismatched page query accepted for waiting task")
	}
	invalid = task
	invalid.ViaMCP = false
	if _, _, err := s.claimBrowserWait(invalid); err == nil {
		t.Fatal("unscoped continuation accepted")
	}

	var wg sync.WaitGroup
	results := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, started, e := s.claimBrowserWait(task)
			if e != nil || got != id {
				t.Errorf("claim result: %q %v", got, e)
			}
			results <- started
		}()
	}
	wg.Wait()
	close(results)
	started := 0
	for result := range results {
		if result {
			started++
		}
	}
	if started != 1 {
		t.Fatalf("duplicate worker launch claims: %d, want 1", started)
	}
	s.mu.Lock()
	job := s.jobs[id]
	if job.Status != "queued" || job.task.Quality != "720" || job.task.Subtitles.Mode != "none" ||
		len(job.task.MediaCookies) != 1 || job.task.ResumeJobID != "" || job.Title != "Actual video" {
		t.Errorf("normal task not resumed correctly: %+v", job)
	}
	s.mu.Unlock()
	saved, err := os.ReadFile(s.jobStorePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved, []byte("secret-cookie")) {
		t.Fatal("browser credentials persisted during resumed submission")
	}
}

func TestBrowserWaitAutomaticallySelectsHLSAndUsesOriginalID(t *testing.T) {
	s := newBrowserWaitTestServer(t)
	const page = "https://example.org/watch?episode=12"
	h := s.mediaHub()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	result := make(chan error, 1)
	idCh := make(chan string, 1)
	go func() {
		for {
			command, e := h.poll(ctx)
			if e != nil {
				result <- e
				return
			}
			if command == nil {
				result <- errors.New("no browser media command")
				return
			}
			if command.Type == "browser_wait_notice" {
				continue
			}
			expected := <-idCh
			if command.Type != "browser_media_download" || command.ResourceKey != "hls" ||
				command.ResumeJobID != expected {
				result <- errors.New("waiting task did not select HLS and preserve ID")
				return
			}
			claimed, shouldStart, e := s.claimBrowserWait(bridgeTask{
				ResumeJobID: expected, ViaMCP: true, PageURL: page,
				URL: "https://cdn.example.org/video.m3u8", Title: "Movie",
			})
			if e != nil || !shouldStart || claimed != expected {
				result <- errors.New("failed to claim original job ID")
				return
			}
			h.complete(browserDownloadReply{CommandID: command.CommandID, OK: true, TaskID: claimed})
			result <- nil
			return
		}
	}()
	for i := 0; i < 60; i++ {
		h.mu.Lock()
		online := !h.lastPoll.IsZero()
		h.mu.Unlock()
		if online {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	id, err := s.createBrowserWait(page, "", "best", SubtitleRequest{Mode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	idCh <- id
	_, _ = h.put(browserMediaAnnouncement{TabID: 7, ResourceKey: "mp4", Kind: "mp4", PageURL: page})
	_, _ = h.put(browserMediaAnnouncement{TabID: 7, ResourceKey: "hls", Kind: "hls", PageURL: page})
	s.processBrowserWaits()
	select {
	case e := <-result:
		if e != nil {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal("waiting task did not resume")
	}
}

func TestBrowserWaitExpiresWithoutDownload(t *testing.T) {
	s := newBrowserWaitTestServer(t)
	id, err := s.createBrowserWait("https://example.org/watch", "", "", SubtitleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.jobs[id].CreatedAt = time.Now().Add(-browserWaitLimit - time.Minute)
	s.mu.Unlock()
	s.processBrowserWaits()
	s.mu.Lock()
	job := *s.jobs[id]
	s.mu.Unlock()
	if job.Status != "failed" || job.Phase != "waiting-for-media" || job.worker != nil {
		t.Fatalf("expired wait should fail without spawning worker: %+v", job)
	}
	summary := summarizeMCPJob(job)
	if !summary.RequiresUserAction || summary.ActionMessage == "" {
		t.Fatalf("expired wait lost MCP guidance: %+v", summary)
	}
}

func TestBrowserWaitNoticeUsesExistingNativeCommandQueue(t *testing.T) {
	h := newBrowserMediaHub()
	h.mu.Lock()
	h.lastPoll = time.Now()
	h.mu.Unlock()
	const page = "https://example.org/watch?private=yes"
	const id = "123456789abc"
	h.notifyWaiting(browserPageDigest(page), id)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd, err := h.poll(ctx)
	if err != nil || cmd == nil || cmd.Type != "browser_wait_notice" ||
		cmd.ResumeJobID != id || cmd.PageHash != browserPageDigest(page) {
		t.Fatalf("incorrect notification command: %+v, %v", cmd, err)
	}
	if cmd.ResourceKey != "" || cmd.TabID != 0 {
		t.Fatal("notification includes browser resource identity")
	}
}
