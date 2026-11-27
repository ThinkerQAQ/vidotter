package vidotter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var partialMediaPath = regexp.MustCompile("^segments/seg[0-9]+\\.ts$")

// Build a file-only playlist. Skip a missing entry and its duration together.
func partialTSPlaylist(dir string) (string, error) {
	local := filepath.Join(dir, "local.m3u8")
	plan, ok := loadHLSResumePlan(&app{workDir: dir, localPlaylist: local})
	if !ok {
		return "", errors.New("未找到可导出的 HLS 检查点")
	}
	if len(plan.AudioSegments) > 0 || plan.AudioInput != "" {
		return "", errors.New("暂不支持音视频分轨的缺片导出")
	}
	sizes := map[string]int64{}
	for _, s := range plan.Segments {
		key := "segments/" + s.Name
		if s.Encrypted || !partialMediaPath.MatchString(key) {
			return "", errors.New("缺片导出仅支持未加密的 TS 分片")
		}
		sizes[key] = s.RangeLength
	}
	raw, err := os.ReadFile(local)
	if err != nil {
		return "", err
	}
	var body strings.Builder
	var target int
	duration := ""
	gap := false
	for _, line := range splitLines(string(raw)) {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			target, err = strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"))
			if err != nil {
				return "", err
			}
		case strings.HasPrefix(line, "#EXTINF:"):
			duration, _, _ = strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			n, err := strconv.ParseFloat(duration, 64)
			if err != nil || !(n > 0 && n < 86400) {
				return "", errors.New("无效的分片时长")
			}
		case line == "#EXT-X-DISCONTINUITY":
			gap = true
		case strings.HasPrefix(line, "#") || line == "":
			continue
		default:
			size, ok := sizes[line]
			if !ok || duration == "" {
				return "", errors.New("播放清单与检查点不匹配")
			}
			if segmentFileComplete(segment{path: filepath.Join(dir, line), rangeLength: size}) {
				if gap && body.Len() > 0 {
					body.WriteString("#EXT-X-DISCONTINUITY\n")
				}
				gap = false
				fmt.Fprintf(&body, "#EXTINF:%s,\n%s\n", duration, line)
			} else {
				gap = true
			}
			duration = ""
		}
	}
	if body.Len() == 0 || target <= 0 {
		return "", errors.New("没有可导出的完整分片")
	}
	return fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n%s#EXT-X-ENDLIST\n", target, body.String()), nil
}

func (s *bridgeServer) exportPartial(id string) error {
	s.mu.Lock()
	job := s.jobs[id]
	if job == nil || job.Status != "failed" || job.Exporting {
		s.mu.Unlock()
		return errors.New("任务不可导出或正在导出")
	}
	job.Exporting = true
	cfg, title := s.config, job.Title
	s.mu.Unlock()
	defer func() { s.mu.Lock(); job.Exporting = false; s.mu.Unlock() }()
	dir := filepath.Join(cfg.OutputDir, ".vidotter-work", "job_"+id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		root, e := bridgeDataPath("work")
		if e != nil {
			return e
		}
		dir = filepath.Join(root, "job_"+id)
	}
	playlist, err := partialTSPlaylist(dir)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "export-*.m3u8")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(playlist)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	ffmpeg, err := findTool(cfg.FFmpegPath, "ffmpeg")
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(cfg.OutputDir, ".vidotter-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	tmp := filepath.Join(stage, "export.mp4")
	if err = (ffmpegMuxer{path: ffmpeg}).Mux(muxRequest{WorkDir: dir, Video: filepath.Base(f.Name()), Output: tmp}); err != nil {
		return err
	}
	out := uniqueOutput(cfg.OutputDir, strings.TrimSuffix(mp4OutputName(title), ".mp4")+"-"+id+"-partial.mp4")
	if err = os.Rename(tmp, out); err != nil {
		return err
	}
	return s.addJobOutput(id, out)
}
