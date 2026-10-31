package vidotter

import (
	"context"
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
