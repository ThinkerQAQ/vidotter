package vidotter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPToolProtocolAndRedaction(t *testing.T) {
	const jobID = "123456789abc"
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-VidOtter-Token") != "fake-private-token" {
			t.Errorf("missing Bridge auth")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/environment":
			_ = json.NewEncoder(w).Encode(map[string]any{"environment": map[string]any{
				"version": "1.4.1", "platform": "linux", "executable": "/private/path",
				"ffmpeg": map[string]any{"available": true}, "ytDlp": map[string]any{"available": true},
			}})
		case "/v1/tasks":
			if r.Method != http.MethodPost {
				t.Errorf("bad method")
			}
			var task bridgeTask
			if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
				t.Fatal(err)
			}
			if task.URL != "https://8.8.8.8/movie.mp4" || task.Subtitles.Mode != "site" {
				t.Errorf("bad task: %+v", task)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"taskId": jobID})
		case "/v1/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []bridgeJob{{
				ID: jobID, Status: "completed", Progress: 100, Title: "Test",
				SourceURL:   "https://public.example/video?token=SECRET-HIDDEN",
				PageURL:     "https://public.example/private?auth=SECRET-HIDDEN",
				OutputPaths: []string{"/home/private/SECRET-HIDDEN/video.mp4", "/home/private/SECRET-HIDDEN/subs.srt"},
			}}})
		case "/v1/jobs/" + jobID + "/subtitle":
			_ = json.NewEncoder(w).Encode(mcpSubtitleResult{Name: "subs.srt", Text: "sample subtitles"})
		default:
			t.Errorf("unexpected Bridge route %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer bridge.Close()
	client := &mcpBridgeClient{
		httpClient: bridge.Client(),
		connect: func(bool) (bridgeState, error) {
			return bridgeState{BaseURL: bridge.URL, Token: "fake-private-token"}, nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := newVidOtterMCPServer(client).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	if init := clientSession.InitializeResult(); init == nil ||
		!strings.Contains(init.Instructions, "already-connected Remote Browser Tool") ||
		!strings.Contains(init.Instructions, "second CDP client") {
		t.Fatal("MCP instructions must prioritize the already-connected browser plugin")
	}
	list, err := clientSession.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 13 {
		t.Fatalf("want 13 tools, got %d, %v", len(list.Tools), err)
	}
	call := func(name string, args any) *mcp.CallToolResult {
		t.Helper()
		out, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || out.IsError {
			t.Fatalf("%s failed: %v %v", name, err, out)
		}
		return out
	}
	status := call("vidotter_status", map[string]any{})
	b, _ := json.Marshal(status.StructuredContent)
	if !strings.Contains(string(b), "true") || strings.Contains(string(b), "private") {
		t.Fatalf("invalid status response: %s", b)
	}
	result := call("vidotter_download", mcpDownloadInput{URL: "https://8.8.8.8/movie.mp4", SubtitleMode: "site"})
	b, _ = json.Marshal(result.StructuredContent)
	if !strings.Contains(string(b), jobID) {
		t.Fatalf("missing id: %s", b)
	}
	jobs := call("vidotter_list_jobs", map[string]any{})
	b, _ = json.Marshal(jobs.StructuredContent)
	if strings.Contains(string(b), "SECRET-HIDDEN") || strings.Contains(string(b), "/home/private") {
		t.Fatalf("private info leaked: %s", b)
	}
	call("vidotter_get_job", mcpJobInput{ID: jobID})
	sub := call("vidotter_read_subtitle", mcpSubtitleInput{ID: jobID})
	b, _ = json.Marshal(sub.StructuredContent)
	if !strings.Contains(string(b), "sample subtitles") {
		t.Fatalf("missing subtitle: %s", b)
	}
}

func TestMCPRejectsPrivateDownloadTargets(t *testing.T) {
	ctx := context.Background()
	for _, url := range []string{
		"file:///etc/passwd", "http://localhost/a", "http://127.0.0.1/video",
		"http://169.254.169.254/latest/meta-data", "http://10.0.0.4/a",
		"https://example.com:8000/watch", "https://user:pass@example.com/watch",
		"https://internal.local/watch", "https://example.com/watch#fragment",
		"http://192.168.1.1/video.mp4", "http://100.100.100.100/a",
	} {
		if err := validateMCPMediaURL(ctx, url); err == nil {
			t.Errorf("unsafe URL accepted: %s", url)
		}
	}
	if err := validateMCPMediaURL(ctx, "https://8.8.8.8/movie.mp4"); err != nil {
		t.Fatal(err)
	}
}

func TestMCPSubtitleAccessScopedToCompletedJob(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "captions.srt")
	if err := os.WriteFile(inside, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	id := "123456789abc"
	server := &bridgeServer{
		state:  bridgeState{Token: "private-token"},
		config: bridgeConfig{OutputDir: dir},
		jobs:   map[string]*bridgeJob{id: {ID: id, Status: "completed", OutputPaths: []string{inside}}},
	}
	read := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-VidOtter-Token", "private-token")
		w := httptest.NewRecorder()
		server.handleJobs(w, r)
		return w
	}
	resp := read("/v1/jobs/" + id + "/subtitle")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Hello") {
		t.Fatalf("%d %s", resp.Code, resp.Body.String())
	}
	// Symlink escaping the downloads directory must not be readable.
	outside := filepath.Join(t.TempDir(), "secret.srt")
	if err := os.WriteFile(outside, []byte("PRIVATE-DATA"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "escape.srt")
	if err := os.Symlink(outside, link); err == nil {
		server.jobs[id].OutputPaths = []string{link}
		resp = read("/v1/jobs/" + id + "/subtitle")
		if resp.Code == http.StatusOK || strings.Contains(resp.Body.String(), "PRIVATE-DATA") {
			t.Fatalf("escaped root: %s", resp.Body.String())
		}
	}
	server.jobs[id].Status = "running"
	server.jobs[id].OutputPaths = []string{inside}
	resp = read("/v1/jobs/" + id + "/subtitle")
	if resp.Code == http.StatusOK {
		t.Fatal("active task subtitle exposed")
	}
}
