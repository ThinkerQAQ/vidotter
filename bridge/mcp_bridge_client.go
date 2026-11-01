package vidotter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const mcpResponseLimit = 1 << 20

var mcpJobID = regexp.MustCompile(`^[a-f0-9]{12}$`)

type mcpBridgeClient struct {
	connect    func(start bool) (bridgeState, error)
	httpClient *http.Client
}

func newMCPBridgeClient() *mcpBridgeClient {
	return &mcpBridgeClient{
		connect: func(start bool) (bridgeState, error) {
			if start {
				return ensureBridge()
			}
			state, err := readBridgeState()
			if err != nil {
				return bridgeState{}, errors.New("Bridge not running")
			}
			if err = pingBridge(state); err != nil {
				return bridgeState{}, errors.New("Bridge not running")
			}
			return state, nil
		},
		httpClient: &http.Client{Timeout: 22 * time.Second},
	}
}

func (c *mcpBridgeClient) request(ctx context.Context, start bool, method, path string, body any, target any) error {
	state, err := c.connect(start)
	if err != nil {
		return errors.New("VidOtter Bridge unavailable")
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return errors.New("invalid request")
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, state.BaseURL+path, reader)
	if err != nil {
		return errors.New("cannot create Bridge request")
	}
	request.Header.Set("X-VidOtter-Token", state.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(request)
	if err != nil {
		return errors.New("Bridge request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Never relay Bridge messages: download errors and URLs may contain secrets.
		return fmt.Errorf("Bridge returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, mcpResponseLimit)).Decode(target); err != nil {
		return errors.New("invalid Bridge response")
	}
	return nil
}

type mcpStatus struct {
	Ready           bool   `json:"ready"`
	Version         string `json:"version,omitempty"`
	Platform        string `json:"platform,omitempty"`
	FFmpegAvailable bool   `json:"ffmpeg_available"`
	YTDLPAvailable  bool   `json:"yt_dlp_available"`
}

func (c *mcpBridgeClient) status(ctx context.Context) (mcpStatus, error) {
	var resp struct {
		Environment bridgeEnvironment `json:"environment"`
	}
	if err := c.request(ctx, false, http.MethodGet, "/v1/environment", nil, &resp); err != nil {
		return mcpStatus{Ready: false}, nil
	}
	return mcpStatus{Ready: true, Version: resp.Environment.Version, Platform: resp.Environment.Platform,
		FFmpegAvailable: resp.Environment.FFmpeg.Available, YTDLPAvailable: resp.Environment.YTDLP.Available}, nil
}

type mcpJobSummary struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Status               string `json:"status"`
	Phase                string `json:"phase,omitempty"`
	Progress             int    `json:"progress"`
	DownloadedBytes      int64  `json:"downloaded_bytes,omitempty"`
	TotalBytes           int64  `json:"total_bytes,omitempty"`
	RequiresFreshSession bool   `json:"requires_fresh_browser_session,omitempty"`
	OutputFiles          int    `json:"output_files"`
	SubtitleFiles        int    `json:"subtitle_files"`
	RequiresUserAction   bool   `json:"requires_user_action,omitempty"`
	ActionMessage        string `json:"action_message,omitempty"`
}

func summarizeMCPJob(j bridgeJob) mcpJobSummary {
	s := mcpJobSummary{ID: j.ID, Title: j.Title, Status: j.Status, Phase: j.Phase, Progress: j.Progress,
		DownloadedBytes: j.DownloadedBytes, TotalBytes: j.TotalBytes,
		RequiresFreshSession: j.RequiresFreshSession, OutputFiles: len(j.OutputPaths)}
	if j.Status == "waiting_for_browser" {
		s.RequiresUserAction = true
		s.ActionMessage = "请在浏览器中完成必要操作并播放视频；媒体出现后会自动继续下载。"
	} else if j.Status == "failed" && j.Phase == "waiting-for-media" {
		s.RequiresUserAction = true
		s.ActionMessage = "等待媒体已超时，请重新打开页面并提交下载。"
	}
	for _, path := range j.OutputPaths {
		if isSubtitleFile(path) {
			s.SubtitleFiles++
		}
	}
	return s
}

func isSubtitleFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".srt", ".vtt", ".ass":
		return true
	default:
		return false
	}
}

func (c *mcpBridgeClient) list(ctx context.Context) ([]mcpJobSummary, error) {
	var resp struct {
		Jobs []bridgeJob `json:"jobs"`
	}
	if err := c.request(ctx, false, http.MethodGet, "/v1/jobs", nil, &resp); err != nil {
		return nil, err
	}
	summaries := make([]mcpJobSummary, 0, len(resp.Jobs))
	for _, job := range resp.Jobs {
		summaries = append(summaries, summarizeMCPJob(job))
	}
	return summaries, nil
}

func (c *mcpBridgeClient) job(ctx context.Context, id string) (mcpJobSummary, error) {
	if !mcpJobID.MatchString(id) {
		return mcpJobSummary{}, errors.New("invalid task ID")
	}
	jobs, err := c.list(ctx)
	if err != nil {
		return mcpJobSummary{}, err
	}
	for _, job := range jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return mcpJobSummary{}, errors.New("task not found among recent jobs")
}

type mcpDownloadInput struct {
	Strategy         string   `json:"strategy,omitempty" jsonschema:"auto (default browser-first), browser (require capture), or page (explicit yt-dlp webpage extraction)"`
	URL              string   `json:"url" jsonschema:"Public HTTP/HTTPS video or media URL; no private network addresses"`
	Title            string   `json:"title,omitempty" jsonschema:"Optional file title"`
	Quality          string   `json:"quality,omitempty" jsonschema:"best, 1080, 720, 480; empty uses configured preference"`
	SubtitleMode     string   `json:"subtitle_mode,omitempty" jsonschema:"none, site, site-or-asr, or asr"`
	SubtitleLanguage string   `json:"subtitle_language,omitempty" jsonschema:"ASR source language, e.g. en or zh"`
	SiteLanguages    []string `json:"site_languages,omitempty" jsonschema:"Requested site subtitle language codes, e.g. en or zh; empty means all"`
	TargetLanguage   string   `json:"target_language,omitempty" jsonschema:"Optional translation language"`
}

type mcpDownloadResult struct {
	Route              string `json:"route,omitempty"`
	TaskID             string `json:"task_id"`
	Status             string `json:"status"`
	RequiresUserAction bool   `json:"requires_user_action,omitempty"`
	ActionMessage      string `json:"action_message,omitempty"`
}

func (c *mcpBridgeClient) download(ctx context.Context, in mcpDownloadInput) (mcpDownloadResult, error) {
	if err := validateMCPMediaURL(ctx, in.URL); err != nil {
		return mcpDownloadResult{}, err
	}
	if len(in.Title) > 200 {
		return mcpDownloadResult{}, errors.New("title too long")
	}
	if strings.ContainsAny(in.Title, "\r\n") {
		return mcpDownloadResult{}, errors.New("invalid title")
	}
	switch in.Quality {
	case "", "best", "1080", "720", "480":
	default:
		return mcpDownloadResult{}, errors.New("unsupported quality")
	}
	mode := in.SubtitleMode
	if mode == "" {
		mode = "none"
	}
	switch mode {
	case "none", "site", "site-or-asr", "asr":
	default:
		return mcpDownloadResult{}, errors.New("unsupported subtitle mode")
	}
	if (in.TargetLanguage != "" || in.SubtitleLanguage != "") && mode == "none" {
		return mcpDownloadResult{}, errors.New("subtitle mode required")
	}
	if len(in.SiteLanguages) > 10 {
		return mcpDownloadResult{}, errors.New("too many subtitle languages")
	}
	for _, language := range in.SiteLanguages {
		if len(language) > 16 || !mcpLanguage(language) || language == "" {
			return mcpDownloadResult{}, errors.New("invalid site subtitle language")
		}
	}
	if len(in.SubtitleLanguage) > 16 || len(in.TargetLanguage) > 16 ||
		!mcpLanguage(in.SubtitleLanguage) || !mcpLanguage(in.TargetLanguage) {
		return mcpDownloadResult{}, errors.New("invalid subtitle language")
	}
	switch in.Strategy {
	case "", "auto", "browser", "page":
	default:
		return mcpDownloadResult{}, errors.New("unsupported download strategy")
	}
	parsed, _ := url.Parse(in.URL)
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	resolvePage := ext != ".mp4" && ext != ".m3u8"
	subtitles := SubtitleRequest{Mode: mode, Languages: in.SiteLanguages, ASRLanguage: in.SubtitleLanguage, TargetLanguage: in.TargetLanguage}
	if resolvePage && in.Strategy != "page" {
		var captured struct {
			Matched            bool   `json:"matched"`
			ExtensionConnected bool   `json:"extensionConnected"`
			TaskID             string `json:"taskId"`
			Status             string `json:"status"`
		}
		err := c.request(ctx, true, http.MethodPost, "/v1/browser/media/by-page/download",
			struct {
				PageURL   string          `json:"pageUrl"`
				Title     string          `json:"title"`
				Quality   string          `json:"quality"`
				Subtitles SubtitleRequest `json:"subtitles"`
			}{PageURL: in.URL, Title: in.Title, Quality: in.Quality, Subtitles: subtitles}, &captured)
		if err != nil {
			return mcpDownloadResult{}, errors.New("browser media submission failed; ensure VidOtter extension is connected and retry")
		}
		if captured.Matched {
			if !mcpJobID.MatchString(captured.TaskID) {
				return mcpDownloadResult{}, errors.New("Bridge returned invalid browser task ID")
			}
			return mcpDownloadResult{TaskID: captured.TaskID, Status: "queued", Route: "browser"}, nil
		}
		if !mcpJobID.MatchString(captured.TaskID) || captured.Status != "waiting_for_browser" {
			return mcpDownloadResult{}, errors.New("Bridge returned invalid waiting task")
		}
		message := "请在 Edge/Chrome 中打开该页面、完成必要操作并播放视频；检测到媒体后会自动继续。"
		if !captured.ExtensionConnected {
			message = "请先在 Edge/Chrome 中启用 VidOtter 扩展，再打开页面并播放视频；连接恢复后自动继续。"
		}
		return mcpDownloadResult{TaskID: captured.TaskID, Status: "waiting_for_browser", Route: "browser",
			RequiresUserAction: true, ActionMessage: message}, nil
	}
	task := bridgeTask{
		URL: in.URL, Title: in.Title, Quality: in.Quality, Playlist: "single", ResolvePage: resolvePage,
		Subtitles: subtitles,
	}
	var resp struct {
		TaskID string `json:"taskId"`
	}
	if err := c.request(ctx, true, http.MethodPost, "/v1/tasks", task, &resp); err != nil {
		return mcpDownloadResult{}, err
	}
	if !mcpJobID.MatchString(resp.TaskID) {
		return mcpDownloadResult{}, errors.New("Bridge returned invalid task ID")
	}
	route := "native"
	if resolvePage {
		route = "page"
	}
	return mcpDownloadResult{TaskID: resp.TaskID, Status: "queued", Route: route}, nil
}

var mcpLanguageRE = regexp.MustCompile(`^[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)*$`)

func mcpLanguage(s string) bool { return s == "" || mcpLanguageRE.MatchString(s) }

func validateMCPMediaURL(ctx context.Context, value string) error {
	if len(value) > 4096 {
		return errors.New("URL too long")
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" || u.Fragment != "" {
		return errors.New("only public HTTP/HTTPS URLs without credentials are supported")
	}
	hostname := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if hostname == "localhost" || !strings.Contains(hostname, ".") {
		return errors.New("local hostnames are forbidden")
	}
	for _, suffix := range []string{".local", ".localhost", ".internal", ".test", ".invalid", ".onion", ".home.arpa"} {
		if strings.HasSuffix(hostname, suffix) {
			return errors.New("private hostnames are forbidden")
		}
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return errors.New("only standard HTTP/HTTPS ports are permitted")
	}
	var ips []net.IP
	if ip := net.ParseIP(hostname); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolveCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		addresses, err := net.DefaultResolver.LookupIPAddr(resolveCtx, hostname)
		if err != nil || len(addresses) == 0 {
			return errors.New("cannot verify public destination")
		}
		for _, address := range addresses {
			ips = append(ips, address.IP)
		}
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return errors.New("invalid destination address")
		}
		addr = addr.Unmap()
		if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() ||
			(addr.Is4() && (netip.MustParsePrefix("100.64.0.0/10").Contains(addr) ||
				netip.MustParsePrefix("198.18.0.0/15").Contains(addr) ||
				netip.MustParsePrefix("192.0.0.0/24").Contains(addr))) {
			return errors.New("non-public destinations are forbidden")
		}
	}
	return nil
}

func (c *mcpBridgeClient) control(ctx context.Context, id, action string) (mcpJobSummary, error) {
	job, err := c.job(ctx, id)
	if err != nil {
		return mcpJobSummary{}, err
	}
	switch action {
	case "pause":
		if job.Status != "running" && job.Status != "queued" {
			return mcpJobSummary{}, errors.New("task not active")
		}
	case "resume":
		if job.Status != "paused" {
			return mcpJobSummary{}, errors.New("task not paused, or requires browser reauthorization")
		}
	case "retry":
		if job.Status != "failed" {
			return mcpJobSummary{}, errors.New("task not failed")
		}
	default:
		return mcpJobSummary{}, errors.New("unsupported action")
	}
	if job.RequiresFreshSession {
		return mcpJobSummary{}, errors.New("requires a fresh browser session; use extension")
	}
	var result any
	if err := c.request(ctx, false, http.MethodPost, "/v1/jobs/"+id+"/"+action, nil, &result); err != nil {
		return mcpJobSummary{}, err
	}
	return c.job(ctx, id)
}

type mcpSubtitleResult struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

func (c *mcpBridgeClient) subtitle(ctx context.Context, id string, index int) (mcpSubtitleResult, error) {
	if !mcpJobID.MatchString(id) {
		return mcpSubtitleResult{}, errors.New("invalid task ID")
	}
	if index < 0 || index > 100 {
		return mcpSubtitleResult{}, errors.New("invalid subtitle index")
	}
	var resp mcpSubtitleResult
	if err := c.request(ctx, false, http.MethodGet, "/v1/jobs/"+id+"/subtitle?index="+strconv.Itoa(index), nil, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *mcpBridgeClient) browserMedia(ctx context.Context) (browserMediaList, error) {
	var result browserMediaList
	err := c.request(ctx, false, http.MethodGet, "/v1/browser/media?refresh=1", nil, &result)
	return result, err
}
func (c *mcpBridgeClient) downloadBrowserMedia(ctx context.Context, id, quality string) (mcpDownloadResult, error) {
	if len(id) != 64 || !regexp.MustCompile("^[0-9a-f]{64}$").MatchString(id) {
		return mcpDownloadResult{}, errors.New("invalid resource ID")
	}
	var reply struct{ TaskID string }
	err := c.request(ctx, false, http.MethodPost, "/v1/browser/media/"+id+"/download", struct{ Quality string }{Quality: quality}, &reply)
	if err != nil {
		return mcpDownloadResult{}, err
	}
	if !mcpJobID.MatchString(reply.TaskID) {
		return mcpDownloadResult{}, errors.New("invalid Bridge task ID")
	}
	return mcpDownloadResult{TaskID: reply.TaskID, Status: "queued"}, nil
}

type mcpBrowserState struct {
	Installed   bool
	Enabled     bool
	Interactive bool
	Version     string
}
type mcpBrowserActionResult struct{ Text string }

func (c *mcpBridgeClient) browserState(ctx context.Context) (mcpBrowserState, error) {
	var status mcpBrowserState
	err := c.request(ctx, false, http.MethodGet, "/v1/browser/automation", nil, &status)
	return status, err
}
func (c *mcpBridgeClient) browserAction(ctx context.Context, in browserAutomationCall) (mcpBrowserActionResult, error) {
	// Browser subprocess cold starts can take longer than ordinary Bridge calls.
	// Do not let the normal 22-second HTTP timeout cancel browser operations.
	browserClient := *c
	if c.httpClient != nil {
		longClient := *c.httpClient
		longClient.Timeout = browserAutomationTimeout + 12*time.Second
		browserClient.httpClient = &longClient
	}
	var result mcpBrowserActionResult
	err := browserClient.request(ctx, false, http.MethodPost, "/v1/browser/automation/call", in, &result)
	return result, err
}
