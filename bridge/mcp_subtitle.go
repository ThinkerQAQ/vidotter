package vidotter

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"
)

const maxMCPSubtitleBytes = 256 << 10

// A narrow read interface is preferable to exposing arbitrary file paths to AI.
// It only serves subtitle outputs already registered against completed jobs.
func (s *bridgeServer) handleJobSubtitle(response http.ResponseWriter, request *http.Request, id string) {
	if request.Method != http.MethodGet || !mcpJobID.MatchString(id) {
		writeJSON(response, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid task"})
		return
	}
	index := 0
	if raw := request.URL.Query().Get("index"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 100 {
			writeJSON(response, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid index"})
			return
		}
		index = value
	}
	s.mu.Lock()
	job := s.jobs[id]
	outputDir := s.config.OutputDir
	var paths []string
	if job != nil && job.Status == "completed" {
		paths = append(paths, job.OutputPaths...)
	}
	s.mu.Unlock()
	if paths == nil {
		writeJSON(response, http.StatusNotFound, map[string]any{"ok": false, "error": "completed task not found"})
		return
	}
	subtitles := make([]string, 0)
	for _, path := range paths {
		if isSubtitleFile(path) {
			subtitles = append(subtitles, path)
		}
	}
	if index >= len(subtitles) {
		writeJSON(response, http.StatusNotFound, map[string]any{"ok": false, "error": "subtitle not found"})
		return
	}
	path, err := validatedOutputFile(outputDir, subtitles[index])
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid subtitle output"})
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeJSON(response, http.StatusNotFound, map[string]any{"ok": false, "error": "subtitle missing"})
		return
	}
	defer f.Close()
	contents, err := io.ReadAll(io.LimitReader(f, maxMCPSubtitleBytes+1))
	if err != nil || len(contents) > maxMCPSubtitleBytes || !utf8.Valid(contents) {
		writeJSON(response, http.StatusRequestEntityTooLarge, map[string]any{"ok": false, "error": "unsupported subtitle size or encoding"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"name": filepath.Base(path), "text": string(contents)})
}
