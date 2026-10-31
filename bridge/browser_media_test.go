package vidotter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserMediaDoesNotExposeURLOrCredentials(t *testing.T) {
	h := newBrowserMediaHub()
	in := browserMediaAnnouncement{TabID: 17, ResourceKey: "local-extension-id", Kind: "hls", PageTitle: "Public playback", PageURL: "https://video.example/watch/46?token=PRIVATE-TOKEN#secret"}
	item, err := h.put(in)
	if err != nil {
		t.Fatal(err)
	}
	if item.PageURL != "https://video.example/watch/46" {
		t.Fatalf("page query not stripped: %s", item.PageURL)
	}
	got := h.list()
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "PRIVATE-TOKEN") || strings.Contains(string(b), "local-extension-id") || strings.Contains(string(b), "TabID") {
		t.Fatalf("private data leaked: %s", b)
	}
	if got.ExtensionConnected || len(got.Resources) != 1 || len(got.Resources[0].ID) != 64 {
		t.Fatalf("invalid resource listing: %+v", got)
	}
	if len(item.PageHash) != 64 || item.PageHash == "" {
		t.Fatal("captured page identity is not preserved for tab navigation safety")
	}
	if strings.Contains(string(b), item.PageHash) {
		t.Fatal("private tab identity hash leaked into MCP listing")
	}
	repeat, err := h.put(in)
	if err != nil || item.ID != repeat.ID {
		t.Fatal("duplicate detection should update resource")
	}
	if _, err := h.put(browserMediaAnnouncement{TabID: 1, ResourceKey: "x", Kind: "hls", PageURL: "file:///private"}); err == nil {
		t.Fatal("file URL allowed")
	}
}

func TestBrowserMediaCommandFlowAndTimeout(t *testing.T) {
	h := newBrowserMediaHub()
	item, err := h.put(browserMediaAnnouncement{TabID: 9, ResourceKey: "tab-resource", Kind: "hls", PageURL: "https://example.org/play", PageTitle: "Video"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	poll := make(chan browserDownloadCommand, 1)
	go func() {
		command, e := h.poll(ctx)
		if e == nil && command != nil {
			poll <- *command
		}
	}()
	for i := 0; i < 10; i++ {
		h.mu.Lock()
		online := !h.lastPoll.IsZero()
		h.mu.Unlock()
		if online {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	result := make(chan string, 1)
	go func() {
		id, e := h.download(ctx, browserDownloadRequest{ResourceID: item.ID, Quality: "best"})
		if e != nil {
			result <- "error: " + e.Error()
			return
		}
		result <- id
	}()
	select {
	case command := <-poll:
		if command.ResourceKey != "tab-resource" || command.TabID != 9 || command.Type != "browser_media_download" {
			t.Fatal(command)
		}
		if !h.complete(browserDownloadReply{CommandID: command.CommandID, OK: true, TaskID: "123456789abc"}) {
			t.Fatal("lost completion")
		}
	case <-ctx.Done():
		t.Fatal("poll did not get request")
	}
	select {
	case got := <-result:
		if got != "123456789abc" {
			t.Fatalf("result: %s", got)
		}
	case <-ctx.Done():
		t.Fatal("download did not return")
	}
	if h.complete(browserDownloadReply{CommandID: "fake", OK: true}) {
		t.Fatal("unknown command accepted")
	}
}

func TestBrowserMediaHTTPAuthorizationAndMetadata(t *testing.T) {
	s := &bridgeServer{state: bridgeState{Token: "test-token"}}
	r := httptest.NewRequest(http.MethodGet, "/v1/browser/media", nil)
	w := httptest.NewRecorder()
	s.handleBrowserMedia(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated access code=%d", w.Code)
	}
	r = httptest.NewRequest(http.MethodPost, "/v1/browser/media", strings.NewReader(`{"resourceKey":"x","tabId":1,"kind":"hls","pageTitle":"Movie","pageUrl":"https://example.com/play?key=private"}`))
	r.Header.Set("X-VidOtter-Token", "test-token")
	w = httptest.NewRecorder()
	s.handleBrowserMedia(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("announce code=%d body=%s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest(http.MethodGet, "/v1/browser/media", nil)
	r.Header.Set("X-VidOtter-Token", "test-token")
	w = httptest.NewRecorder()
	s.handleBrowserMedia(w, r)
	if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "resourceKey") {
		t.Fatalf("leak: %s", w.Body.String())
	}
}

func TestBrowserFirstMCPUsesHLSWithoutYTDLP(t *testing.T) {
	const pageURL = "https://8.8.8.8/watch?token=private-a"
	const jobID = "123456789abc"
	s := &bridgeServer{state: bridgeState{Token: "test-token"}}
	h := s.mediaHub()
	// Exact page identity matters even when the MCP listing strips query data.
	_, _ = h.put(browserMediaAnnouncement{TabID: 12, ResourceKey: "wrong-page", Kind: "hls", PageURL: "https://8.8.8.8/watch?token=private-b"})
	_, _ = h.put(browserMediaAnnouncement{TabID: 15, ResourceKey: "mp4-item", Kind: "mp4", PageURL: pageURL})
	_, _ = h.put(browserMediaAnnouncement{TabID: 15, ResourceKey: "hls-item", Kind: "hls", PageURL: pageURL})
	if _, matched, _ := h.matchPage("https://8.8.8.8/watch?token=private-c"); matched {
		t.Fatal("incorrectly matched another page's query")
	}
	handler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tasks" {
			t.Error("browser-first requests must not submit a yt-dlp task")
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		s.handleBrowserMedia(w, r)
	}))
	defer handler.Close()
	client := &mcpBridgeClient{
		httpClient: handler.Client(),
		connect:    func(bool) (bridgeState, error) { return bridgeState{BaseURL: handler.URL, Token: "test-token"}, nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		command, err := h.poll(ctx)
		if err != nil || command == nil {
			t.Errorf("native command missing: %v", err)
			return
		}
		if command.Type != "browser_media_download" || command.TabID != 15 || command.ResourceKey != "hls-item" {
			t.Errorf("did not select captured HLS: %+v", command)
		}
		if command.Subtitles == nil || command.Subtitles.Mode != "site" || command.Subtitles.Languages[0] != "en" {
			t.Errorf("subtitle options lost: %+v", command.Subtitles)
		}
		if !h.complete(browserDownloadReply{CommandID: command.CommandID, OK: true, TaskID: jobID}) {
			t.Error("failed native acknowledgement")
		}
	}()
	for i := 0; i < 50; i++ {
		h.mu.Lock()
		ready := !h.lastPoll.IsZero()
		h.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	got, err := client.download(ctx, mcpDownloadInput{URL: pageURL, Quality: "best", SubtitleMode: "site", SiteLanguages: []string{"en"}})
	if err != nil || got.TaskID != jobID || got.Route != "browser" {
		t.Fatalf("browser download: %+v %v", got, err)
	}
	<-done
}

func TestBrowserFirstMCPRequestsTargetedRefresh(t *testing.T) {
	const pageURL = "https://8.8.8.8/new-video"
	s := &bridgeServer{state: bridgeState{Token: "test-token"}}
	h := s.mediaHub()
	handler := httptest.NewServer(http.HandlerFunc(s.handleBrowserMedia))
	defer handler.Close()
	client := &mcpBridgeClient{httpClient: handler.Client(),
		connect: func(bool) (bridgeState, error) { return bridgeState{BaseURL: handler.URL, Token: "test-token"}, nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		command, err := h.poll(ctx)
		if err != nil || command == nil {
			t.Errorf("missing refresh: %v", err)
			return
		}
		digest := sha256.Sum256([]byte(pageURL))
		if command.Type != "browser_media_refresh" || command.PageHash != hex.EncodeToString(digest[:]) {
			t.Errorf("refresh was not scoped to requested page: %+v", command)
			return
		}
		_, _ = h.put(browserMediaAnnouncement{TabID: 4, ResourceKey: "fresh-hls", Kind: "hls", PageURL: pageURL})
		h.complete(browserDownloadReply{CommandID: command.CommandID, OK: true})
		command, err = h.poll(ctx)
		if err != nil || command == nil {
			t.Errorf("missing download after refresh: %v", err)
			return
		}
		if command.Type != "browser_media_download" || command.ResourceKey != "fresh-hls" {
			t.Errorf("bad download command %+v", command)
		}
		h.complete(browserDownloadReply{CommandID: command.CommandID, OK: true, TaskID: "123456789abc"})
	}()
	for i := 0; i < 50; i++ {
		h.mu.Lock()
		online := !h.lastPoll.IsZero()
		h.mu.Unlock()
		if online {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	got, err := client.download(ctx, mcpDownloadInput{URL: pageURL})
	if err != nil || got.Route != "browser" {
		t.Fatalf("download after refresh: %+v %v", got, err)
	}
	<-done
}

func TestBrowserFirstMissingCaptureNeverFallsBackToYTDLP(t *testing.T) {
	s := &bridgeServer{state: bridgeState{Token: "test-token"}}
	handler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tasks" {
			t.Error("unexpected yt-dlp fallback")
			http.Error(w, "invalid", 500)
			return
		}
		s.handleBrowserMedia(w, r)
	}))
	defer handler.Close()
	c := &mcpBridgeClient{httpClient: handler.Client(),
		connect: func(bool) (bridgeState, error) { return bridgeState{BaseURL: handler.URL, Token: "test-token"}, nil },
	}
	got, err := c.download(context.Background(), mcpDownloadInput{URL: "https://8.8.8.8/watch"})
	if err == nil || got.TaskID != "" || !strings.Contains(err.Error(), "browser capture required") {
		t.Fatalf("silent webpage fallback: %+v %v", got, err)
	}
}

func TestExplicitYTDLPPageStrategy(t *testing.T) {
	handler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		var task bridgeTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			t.Errorf("decode: %v", err)
		}
		if !task.ResolvePage {
			t.Error("explicit webpage strategy must use page resolver")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"taskId": "123456789abc"})
	}))
	defer handler.Close()
	c := &mcpBridgeClient{httpClient: handler.Client(),
		connect: func(bool) (bridgeState, error) { return bridgeState{BaseURL: handler.URL, Token: "test-token"}, nil },
	}
	got, err := c.download(context.Background(), mcpDownloadInput{URL: "https://8.8.8.8/watch", Strategy: "page"})
	if err != nil || got.Route != "page" {
		t.Fatalf("page strategy: %+v %v", got, err)
	}
}
