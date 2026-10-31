package vidotter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const browserMediaTTL = 10 * time.Minute
const browserCommandWait = 13 * time.Second

type browserMediaAnnouncement struct {
	ResourceKey string
	TabID       int
	Kind        string
	PageTitle   string
	PageURL     string
}
type browserMediaItem struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	PageTitle   string    `json:"page_title"`
	PageURL     string    `json:"page_url"`
	DetectedAt  time.Time `json:"detected_at"`
	TabID       int       `json:"-"`
	ResourceKey string    `json:"-"`
	PageHash    string    `json:"-"`
}
type browserMediaList struct {
	ExtensionConnected bool               `json:"extension_connected"`
	Resources          []browserMediaItem `json:"resources"`
}
type browserDownloadRequest struct {
	ResourceID string `json:"resource_id"`
	Quality    string `json:"quality,omitempty"`
}
type browserDownloadCommand struct {
	Type        string    `json:"type"`
	CommandID   string    `json:"commandId"`
	TabID       int       `json:"tabId"`
	ResourceKey string    `json:"resourceKey"`
	PageHash    string    `json:"pageHash"`
	Quality     string    `json:"quality,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt"`
}
type browserDownloadReply struct {
	CommandID string `json:"commandId"`
	TaskID    string `json:"taskId"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}
type browserMediaHub struct {
	mu        sync.Mutex
	resources map[string]browserMediaItem
	next      chan browserDownloadCommand
	awaiting  map[string]chan browserDownloadReply
	lastPoll  time.Time
}

func newBrowserMediaHub() *browserMediaHub {
	return &browserMediaHub{
		resources: make(map[string]browserMediaItem),
		next:      make(chan browserDownloadCommand, 8),
		awaiting:  make(map[string]chan browserDownloadReply),
	}
}
func (s *bridgeServer) mediaHub() *browserMediaHub {
	s.componentMu.Lock()
	defer s.componentMu.Unlock()
	if s.browserMedia == nil {
		s.browserMedia = newBrowserMediaHub()
	}
	return s.browserMedia
}
func redactedBrowserPage(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" {
		return "", errors.New("invalid media page")
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath(), nil
}
func allowedMediaKind(kind string) bool {
	switch kind {
	case "hls", "dash", "mp4", "media-track", "resolved-media-page":
		return true
	}
	return false
}
func (h *browserMediaHub) put(in browserMediaAnnouncement) (browserMediaItem, error) {
	if in.TabID < 0 || len(in.ResourceKey) == 0 || len(in.ResourceKey) > 128 || !allowedMediaKind(in.Kind) {
		return browserMediaItem{}, errors.New("invalid browser resource")
	}
	if len(in.PageURL) > 4096 {
		return browserMediaItem{}, errors.New("browser page URL too long")
	}
	page, err := redactedBrowserPage(in.PageURL)
	fullPageDigest := sha256.Sum256([]byte(in.PageURL))
	pageHash := hex.EncodeToString(fullPageDigest[:])
	if err != nil {
		return browserMediaItem{}, err
	}
	now := time.Now()
	title := strings.TrimSpace(in.PageTitle)
	if len([]rune(title)) > 90 {
		title = string([]rune(title)[:90])
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.prune(now)
	for key, item := range h.resources {
		if item.TabID == in.TabID && item.ResourceKey == in.ResourceKey {
			item.DetectedAt = now
			item.Kind, item.PageTitle, item.PageURL = in.Kind, title, page
			item.PageHash = pageHash
			h.resources[key] = item
			return item, nil
		}
	}
	id, err := randomToken()
	if err != nil {
		return browserMediaItem{}, err
	}
	item := browserMediaItem{ID: id, Kind: in.Kind, TabID: in.TabID, ResourceKey: in.ResourceKey, PageHash: pageHash, PageTitle: title, PageURL: page, DetectedAt: now}
	h.resources[id] = item
	if len(h.resources) > 120 {
		var oldest string
		for k, v := range h.resources {
			if oldest == "" || v.DetectedAt.Before(h.resources[oldest].DetectedAt) {
				oldest = k
			}
		}
		delete(h.resources, oldest)
	}
	return item, nil
}
func (h *browserMediaHub) prune(now time.Time) {
	for id, item := range h.resources {
		if now.Sub(item.DetectedAt) > browserMediaTTL {
			delete(h.resources, id)
		}
	}
}
func (h *browserMediaHub) list() browserMediaList {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.prune(time.Now())
	items := make([]browserMediaItem, 0, len(h.resources))
	for _, item := range h.resources {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DetectedAt.After(items[j].DetectedAt) })
	return browserMediaList{ExtensionConnected: time.Since(h.lastPoll) < 25*time.Second, Resources: items}
}
func (h *browserMediaHub) poll(ctx context.Context) (*browserDownloadCommand, error) {
	h.mu.Lock()
	h.lastPoll = time.Now()
	h.mu.Unlock()
	timer := time.NewTimer(9 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, nil
		case command := <-h.next:
			h.mu.Lock()
			_, exists := h.awaiting[command.CommandID]
			h.mu.Unlock()
			if !exists || time.Now().After(command.ExpiresAt) {
				continue
			}
			return &command, nil
		}
	}
}
func (h *browserMediaHub) download(ctx context.Context, in browserDownloadRequest) (string, error) {
	h.mu.Lock()
	h.prune(time.Now())
	item, exists := h.resources[in.ResourceID]
	online := time.Since(h.lastPoll) < 25*time.Second
	h.mu.Unlock()
	if !exists {
		return "", errors.New("browser media expired or not found; play the video again")
	}
	if !online {
		return "", errors.New("browser extension not connected; open Edge/Chrome and enable VidOtter")
	}
	switch in.Quality {
	case "", "best", "1080", "720", "480":
	default:
		return "", errors.New("unsupported quality")
	}
	id, err := randomToken()
	if err != nil {
		return "", err
	}
	reply := make(chan browserDownloadReply, 1)
	h.mu.Lock()
	h.awaiting[id] = reply
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.awaiting, id); h.mu.Unlock() }()
	command := browserDownloadCommand{Type: "browser_media_download", CommandID: id, TabID: item.TabID, ResourceKey: item.ResourceKey, PageHash: item.PageHash, Quality: in.Quality, ExpiresAt: time.Now().Add(browserCommandWait)}
	select {
	case h.next <- command:
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "", errors.New("browser command queue full")
	}
	timer := time.NewTimer(browserCommandWait)
	defer timer.Stop()
	select {
	case r := <-reply:
		if !r.OK || !mcpJobID.MatchString(r.TaskID) {
			return "", errors.New("browser extension did not accept the download; check the tab and permissions")
		}
		return r.TaskID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		return "", errors.New("browser extension did not respond before timeout")
	}
}
func (h *browserMediaHub) complete(r browserDownloadReply) bool {
	h.mu.Lock()
	ch, ok := h.awaiting[r.CommandID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- r:
		return true
	default:
		return false
	}
}
func (s *bridgeServer) handleBrowserMedia(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false})
		return
	}
	h := s.mediaHub()
	switch {
	case r.URL.Path == "/v1/browser/media" && r.Method == http.MethodPost:
		var in browserMediaAnnouncement
		if decodeBridgeJSON(w, r, &in) != nil {
			return
		}
		item, err := h.put(in)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": item.ID})
	case r.URL.Path == "/v1/browser/media" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.list())
	case strings.HasPrefix(r.URL.Path, "/v1/browser/media/") && r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/download"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/browser/media/"), "/download")
		if len(id) != 64 || strings.Contains(id, "/") {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false})
			return
		}
		var in struct{ Quality string }
		if decodeBridgeJSON(w, r, &in) != nil {
			return
		}
		job, err := h.download(r.Context(), browserDownloadRequest{ResourceID: id, Quality: in.Quality})
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "taskId": job})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false})
	}
}
func (s *bridgeServer) handleBrowserCommands(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false})
		return
	}
	h := s.mediaHub()
	switch {
	case r.URL.Path == "/v1/browser/commands/next" && r.Method == http.MethodGet:
		command, err := h.poll(r.Context())
		if err != nil {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "command": command})
	case r.URL.Path == "/v1/browser/commands/result" && r.Method == http.MethodPost:
		var in browserDownloadReply
		if decodeBridgeJSON(w, r, &in) != nil {
			return
		}
		if !h.complete(in) {
			writeJSON(w, http.StatusGone, map[string]any{"ok": false})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false})
	}
}
