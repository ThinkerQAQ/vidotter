package vidotter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"sort"
	"time"
)

// A browser wait is an ordinary Bridge job without a worker yet. The extension
// remains the only owner of captured URLs and browser credentials.
const browserWaitLimit = 20 * time.Minute

func browserPageDigest(pageURL string) string {
	digest := sha256.Sum256([]byte(pageURL))
	return hex.EncodeToString(digest[:])
}

func (s *bridgeServer) createBrowserWait(pageURL, title, quality string, subtitles SubtitleRequest) (string, error) {
	page, err := redactedBrowserPage(pageURL)
	if err != nil {
		return "", err
	}
	pageHash := browserPageDigest(pageURL)
	now := time.Now()
	s.mu.Lock()
	if s.jobs == nil {
		s.jobs = make(map[string]*bridgeJob)
	}
	for _, existing := range s.jobs {
		if existing.Status == "waiting_for_browser" && existing.BrowserPageHash == pageHash &&
			existing.task.Quality == quality && reflect.DeepEqual(existing.task.Subtitles, subtitles) &&
			now.Before(browserWaitStart(existing).Add(browserWaitLimit)) {
			id := existing.ID
			s.mu.Unlock()
			return id, nil
		}
	}
	token, err := randomToken()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	id := token[:12]
	if title == "" {
		title = page
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	s.jobs[id] = &bridgeJob{
		ID: id, Title: title, SourceURL: page, PageURL: page,
		BrowserPageHash: pageHash, Status: "waiting_for_browser", Phase: "waiting-for-media",
		Detail:    "请在浏览器中完成必要操作并播放视频，检测到媒体后自动继续",
		CreatedAt: now, UpdatedAt: now, BrowserWaitSince: now,
		task: bridgeTask{Quality: quality, Subtitles: subtitles},
	}
	err = s.persistJobsLocked()
	if err != nil {
		delete(s.jobs, id)
	}
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	appendRuntimeEvent("INFO", "bridge", "browser_media_waiting", "jobId", id)
	s.mediaHub().notifyWaiting(pageHash, id)
	s.wakeBrowserWaits()
	return id, nil
}

// activateBrowserWait reserves the same ID before starting the normal worker.
// The browser must supply fresh request context, with exactly the original
// page identity. No URL, Cookie or authorization header is copied from MCP.
func (s *bridgeServer) claimBrowserWait(task bridgeTask) (string, bool, error) {
	if !task.ViaMCP || !mcpJobID.MatchString(task.ResumeJobID) || task.ResolvePage ||
		task.PageURL == "" || task.URL == "" {
		return "", false, errors.New("invalid browser wait continuation")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[task.ResumeJobID]
	if job == nil || job.BrowserPageHash == "" ||
		job.BrowserPageHash != browserPageDigest(task.PageURL) {
		return "", false, errors.New("browser page does not match waiting task")
	}
	if job.Status == "queued" || job.Status == "running" || job.Status == "completed" {
		return job.ID, false, nil
	}
	if job.Status != "waiting_for_browser" || time.Now().After(browserWaitStart(job).Add(browserWaitLimit)) {
		return "", false, errors.New("browser wait is no longer active")
	}
	// The original MCP request controls quality and subtitles. A media event
	// cannot replace those settings by supplying a different payload.
	task.Quality = job.task.Quality
	task.Subtitles = job.task.Subtitles
	task.ResumeJobID = ""
	job.task = task
	job.Title = task.Title
	job.SourceURL = task.URL
	job.PageURL = task.PageURL
	job.SourceTabID = task.SourceTabID
	job.SourceFrameID = task.SourceFrameID
	job.CookieStoreID = task.CookieStoreID
	job.Status = "queued"
	job.WaitReason = ""
	job.BrowserWaitAfter = time.Time{}
	job.BrowserWaitSince = time.Time{}
	job.Phase = "resolving"
	job.Detail = "已检测到媒体，准备下载"
	job.UpdatedAt = time.Now()
	// This is the same persistence path used for every ordinary task. It
	// removes cookies/authorization headers before saving.
	_ = s.persistJobsLocked()
	return job.ID, true, nil
}

func (s *bridgeServer) activateBrowserWait(task bridgeTask) (string, error) {
	id, shouldStart, err := s.claimBrowserWait(task)
	if err != nil || !shouldStart {
		return id, err
	}
	if err := s.startJob(id); err != nil {
		return "", err
	}
	return id, nil
}

func browserWaitStart(job *bridgeJob) time.Time {
	if !job.BrowserWaitSince.IsZero() {
		return job.BrowserWaitSince
	}
	return job.CreatedAt
}

func (s *bridgeServer) wakeBrowserWaits() {
	if s.browserWake == nil {
		return
	}
	select {
	case s.browserWake <- struct{}{}:
	default:
	}
}

// One Bridge-owned monitor checks expiration and reacts to media events.
// It does not poll browser pages, reauthorize sessions or start another CDP
// client. A short debounce lets HLS win over early MP4 network requests.
func (s *bridgeServer) monitorBrowserWaits(done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	s.processBrowserWaits()
	for {
		select {
		case <-done:
			return
		case <-s.browserWake:
			timer := time.NewTimer(750 * time.Millisecond)
			select {
			case <-timer.C:
			case <-done:
				timer.Stop()
				return
			}
		case <-ticker.C:
		}
		s.processBrowserWaits()
	}
}

func (s *bridgeServer) processBrowserWaits() {
	type pending struct {
		id        string
		hash      string
		quality   string
		subtitles SubtitleRequest
		created   time.Time
		after     time.Time
	}
	now := time.Now()
	items := []pending{}
	s.mu.Lock()
	for _, job := range s.jobs {
		if job.Status != "waiting_for_browser" {
			continue
		}
		if now.After(browserWaitStart(job).Add(browserWaitLimit)) {
			job.Status = "failed"
			job.Phase = "waiting-for-media"
			job.Detail = "等待媒体超时"
			job.Error = "未在等待期限内检测到可下载媒体；请重新打开页面并提交任务"
			job.UpdatedAt = now
			_ = s.persistJobsLocked()
			continue
		}
		items = append(items, pending{id: job.ID, hash: job.BrowserPageHash,
			quality: job.task.Quality, subtitles: job.task.Subtitles,
			created: browserWaitStart(job), after: job.BrowserWaitAfter})
	}
	s.mu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].created.Before(items[j].created) })
	for _, item := range items {
		if item.hash == "" {
			continue
		}
		s.mu.Lock()
		active := s.jobs[item.id] != nil && s.jobs[item.id].Status == "waiting_for_browser"
		s.mu.Unlock()
		if !active {
			continue
		}
		media, matched, online := s.mediaHub().matchHashAfter(item.hash, item.after)
		if !matched || !online {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), browserCommandWait+time.Second)
		_, _ = s.mediaHub().download(ctx, browserDownloadRequest{
			ResourceID: media.ID, Quality: item.quality, Subtitles: &item.subtitles, ResumeJobID: item.id,
		})
		cancel()
	}
}
