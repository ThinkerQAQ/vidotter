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
	s.jobs[id].BrowserWaitSince = s.jobs[id].CreatedAt
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

func TestTerminalJobSendsOneBrowserNotification(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		t.Run(status, func(t *testing.T) {
			s := newBrowserWaitTestServer(t)
			const id = "123456789abc"
			s.jobs[id] = &bridgeJob{ID: id, Status: "running", Title: "Sensitive movie title",
				SourceURL: "https://example.org/private?token=secret",
				CreatedAt: time.Now(), UpdatedAt: time.Now()}
			h := s.mediaHub()
			h.mu.Lock()
			h.lastPoll = time.Now()
			h.mu.Unlock()
			if err := s.setJobStatus(id, status, ""); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			command, err := h.poll(ctx)
			if err != nil || command == nil || command.Type != "browser_job_result_notice" ||
				command.JobStatus != status || command.ResumeJobID != id {
				t.Fatalf("bad terminal command: %+v / %v", command, err)
			}
			if command.PageHash != "" || command.ResourceKey != "" || command.TabID != 0 ||
				command.Quality != "" {
				t.Fatalf("terminal command leaks media identity: %+v", command)
			}
			if err := s.setJobStatus(id, status, ""); err != nil {
				t.Fatal(err)
			}
			if len(h.next) != 0 {
				t.Fatal("repeated status change produced duplicate notification")
			}
		})
	}
}

// A refreshed bridge index must retain the capture timestamp originally
// recorded by webRequest. Syncing metadata cannot revive expired media.
func TestBrowserMediaIndexKeepsRealCaptureAndRejectsExpiry(t *testing.T) {
	h := newBrowserMediaHub()
	const page = "https://page.test/watch?episode=1"
	now := time.Now()
	stale := browserMediaAnnouncement{TabID: 12, Kind: "hls", ResourceKey: "stale",
		PageURL: page, CapturedAt: now.Add(-5 * time.Minute).UnixMilli()}
	old, err := h.put(stale)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.put(stale); err != nil {
		t.Fatal(err)
	}
	media, matched, _ := h.matchHashAfter(browserPageDigest(page), now.Add(-time.Minute))
	if matched || media.ID != "" {
		t.Fatal("replayed browser index falsely matched as newly captured")
	}
	expired := browserMediaAnnouncement{TabID: 12, Kind: "hls", ResourceKey: "expired",
		PageURL: page, CapturedAt: now.UnixMilli(), ExpiresAt: now.Add(10 * time.Second).UnixMilli()}
	if _, err := h.put(expired); err != nil {
		t.Fatal(err)
	}
	fresh := browserMediaAnnouncement{TabID: 12, Kind: "mp4", ResourceKey: "fresh",
		PageURL: page, CapturedAt: now.UnixMilli(), ExpiresAt: now.Add(5 * time.Minute).UnixMilli()}
	result, err := h.put(fresh)
	if err != nil {
		t.Fatal(err)
	}
	media, matched, _ = h.matchHash(browserPageDigest(page))
	if !matched || media.ID != old.ID {
		// The old unknown-expiry HLS remains eligible unless a newer-capture
		// boundary has been declared by a recovering job.
		t.Fatalf("unexpected HLS selection: %v %+v", matched, media)
	}
	media, matched, _ = h.matchHashAfter(browserPageDigest(page), now.Add(-time.Minute))
	if !matched || media.ID != result.ID {
		t.Fatalf("did not choose new valid media: %v %+v", matched, media)
	}
}

func TestBrowserAccessFailureWaitsOnlyForNewCaptureAndKeepsJobID(t *testing.T) {
	s := newBrowserWaitTestServer(t)
	const page = "https://page.test/watch?episode=2"
	const id = "123456789abc"
	now := time.Now()
	h := s.mediaHub()
	h.mu.Lock()
	h.lastPoll = now
	h.mu.Unlock()
	old, _ := h.put(browserMediaAnnouncement{TabID: 2, Kind: "hls", ResourceKey: "old",
		PageURL: page, CapturedAt: now.Add(-time.Second).UnixMilli()})
	s.jobs[id] = &bridgeJob{
		ID: id, Title: "A title", Status: "running", Phase: "resolving",
		SourceTabID: 2, PageURL: page, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
		task: bridgeTask{URL: "https://media.test/old.m3u8?token=expired", PageURL: page,
			Quality: "720", Subtitles: SubtitleRequest{Mode: "none"}},
	}
	if err := s.setJobStatusReason(id, "failed", "HTTP 403", "media_access_denied"); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	job := *s.jobs[id]
	s.mu.Unlock()
	if job.Status != "waiting_for_browser" || job.WaitReason != "media_access_denied" ||
		job.BrowserWaitSince.IsZero() || job.BrowserWaitAfter.IsZero() {
		t.Fatalf("403 did not enter safe waiting state: %+v", job)
	}
	if browserWaitStart(&job).Before(now) {
		t.Fatal("recovery window incorrectly started at task creation")
	}
	if media, matched, _ := h.matchHashAfter(browserPageDigest(page), job.BrowserWaitAfter); matched || media.ID == old.ID {
		t.Fatal("old capture was reused after HTTP 403")
	}
	_, err := h.put(browserMediaAnnouncement{TabID: 2, Kind: "hls", ResourceKey: "new",
		PageURL: page, CapturedAt: time.Now().Add(time.Second).UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	// The new timestamp is clamped to now, still later than the 403.
	_, matched, _ := h.matchHashAfter(browserPageDigest(page), job.BrowserWaitAfter)
	if !matched {
		t.Fatal("freshly captured media did not match")
	}
	_, shouldStart, err := s.claimBrowserWait(bridgeTask{
		ResumeJobID: id, ViaMCP: true, PageURL: page, URL: "https://media.test/new.m3u8",
		Quality: "best", Title: "same video",
	})
	if err != nil || !shouldStart {
		t.Fatalf("original task not claimed: %v %t", err, shouldStart)
	}
	s.mu.Lock()
	after := *s.jobs[id]
	s.mu.Unlock()
	if after.ID != id || after.Status != "queued" || after.task.Quality != "720" ||
		after.WaitReason != "" || after.BrowserWaitAfter != (time.Time{}) {
		t.Fatalf("job identity/settings changed: %+v", after)
	}
}

func TestAccessDenialDuringSegmentsDoesNotAutorecover(t *testing.T) {
	s := newBrowserWaitTestServer(t)
	const id = "123456789abc"
	s.jobs[id] = &bridgeJob{ID: id, Status: "running", Phase: "downloading",
		SourceTabID: 4, PageURL: "https://page.test/watch",
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.setJobStatusReason(id, "failed", "HTTP 403", "media_access_denied"); err != nil {
		t.Fatal(err)
	}
	if s.jobs[id].Status != "failed" {
		t.Fatal("unsafe mixing of previously downloaded HLS segments was permitted")
	}
}
