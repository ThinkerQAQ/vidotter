package vidotter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPartialExportCurlDenial(t *testing.T) {
	if _, err := curlExecutable(); err != nil {
		t.Skip(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "<Error><Code>Example</Code></Error>", 403)
	}))
	defer srv.Close()
	dir := t.TempDir()
	a := &app{workDir: dir}
	d, err := newGoDownloader(a, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	f := filepath.Join(dir, "seg000000.ts")
	err = d.downloadSegmentWithCurl(segment{url: srv.URL, path: f})
	if err == nil || retryableDownloadError(err) || mediaFailureReason(err) != "media_access_denied" {
		t.Fatalf("wrong error classification: %v", err)
	}
	for _, suffix := range []string{"", ".part", ".curl.tmp"} {
		if _, err := os.Stat(f + suffix); !os.IsNotExist(err) {
			t.Fatal("unexpected file", suffix)
		}
	}
}

func TestRetryDiscardsLegacyErrorPart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			t.Error("resumed an XML document")
		}
		w.Write([]byte("whole segment"))
	}))
	defer srv.Close()
	d, err := newGoDownloader(&app{}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	f := filepath.Join(t.TempDir(), "seg000000.ts")
	if err = os.WriteFile(f+".part", []byte("<Error><Code>Example</Code></Error>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = d.downloadSegmentOnce(segment{url: srv.URL, path: f}); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, f, []byte("whole segment"))
}

func TestPartialExportFromFailedJob(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip(err)
	}
	out := t.TempDir()
	id := "abc123abc123"
	dir := filepath.Join(out, ".vidotter-work", "job_"+id)
	segdir := filepath.Join(dir, "segments")
	if err = os.MkdirAll(segdir, 0700); err != nil {
		t.Fatal(err)
	}
	a := &app{localPlaylist: filepath.Join(dir, "local.m3u8"), workDir: dir, segmentDir: segdir}
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\na.ts\n#EXTINF:1,\nb.ts\n#EXTINF:1,\nc.ts\n#EXT-X-ENDLIST\n"
	segments, err := a.prepareMediaPlaylist(manifest, "https://example.invalid/a.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if err = saveHLSResumePlan(a, segments, nil, ""); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command(ff, "-loglevel", "error", "-f", "lavfi", "-i", "color=size=64x64:rate=25", "-t", "1", "-c:v", "mpeg2video", "-f", "mpegts", segments[0].path).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	data, err := os.ReadFile(segments[0].path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(segments[2].path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(segments[1].path+".part", []byte("error response"), 0600); err != nil {
		t.Fatal(err)
	}
	pl, err := partialTSPlaylist(dir)
	if err != nil || strings.Contains(pl, segments[1].name) || !strings.Contains(pl, "#EXT-X-DISCONTINUITY") {
		t.Fatalf("unexpected playlist %q: %v", pl, err)
	}
	j := &bridgeJob{ID: id, Status: "failed", Title: "sample"}
	s := &bridgeServer{jobs: map[string]*bridgeJob{id: j}, config: bridgeConfig{OutputDir: out, FFmpegPath: ff}}
	if err = s.exportPartial(id); err != nil {
		t.Fatal(err)
	}
	if j.Status != "failed" || len(j.OutputPaths) != 1 {
		t.Fatal("unexpected job state")
	}
	if b, err := exec.Command(ff, "-v", "error", "-i", j.OutputPaths[0], "-f", "null", "-").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
}
