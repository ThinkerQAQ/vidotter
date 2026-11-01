package vidotter

// Install the official OpenAI tunnel-client as an explicitly selected optional
// component. Never execute a downloaded script: download a single official
// platform release, verify its GitHub-published SHA-256 digest, and extract only
// the expected executables into the current user's private bin directory.

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	tunnelClientReleaseAPI = "https://api.github.com/repos/openai/tunnel-client/releases/latest"
	tunnelClientMaxArchive = 120 << 20
	tunnelClientMaxFile    = 80 << 20
)

var tunnelClientInstallMu sync.Mutex

type tunnelClientRelease struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
		URL    string `json:"browser_download_url"`
	} `json:"assets"`
}

func tunnelClientPlatform(goos, goarch string) (string, string, error) {
	switch goos {
	case "windows", "linux", "darwin":
	default:
		return "", "", errors.New("unsupported tunnel-client operating system")
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", errors.New("unsupported tunnel-client architecture")
	}
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("%s-%s", goos, goarch), ext, nil
}

func tunnelClientTarget(goos, home string) string {
	name := "tunnel-client"
	if goos == "windows" {
		name += ".exe"
	}
	return filepath.Join(home, ".local", "bin", name)
}

func tunnelReleaseHTTPClient() *http.Client {
	return &http.Client{Timeout: 95 * time.Second}
}

func readVerifiedTunnelArchive(ctx context.Context, client *http.Client, goos, goarch string) (*zip.Reader, error) {
	platform, _, err := tunnelClientPlatform(goos, goarch)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, tunnelClientReleaseAPI, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "VidOtter-optional-Tunnel-installer")
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GitHub release lookup failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub release lookup HTTP %d", response.StatusCode)
	}
	var release tunnelClientRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("invalid GitHub release data: %w", err)
	}
	if !strings.HasPrefix(release.Tag, "v") || strings.ContainsAny(release.Tag, "/\\") {
		return nil, errors.New("invalid official Tunnel release tag")
	}
	expectedName := "tunnel-client-" + release.Tag + "-" + platform + ".zip"
	var asset struct {
		URL, Digest string
		Size        int64
	}
	for _, item := range release.Assets {
		if item.Name == expectedName {
			asset.URL, asset.Digest, asset.Size = item.URL, item.Digest, item.Size
			break
		}
	}
	if asset.URL == "" {
		return nil, fmt.Errorf("no official tunnel-client binary for %s", platform)
	}
	if asset.URL != "https://github.com/openai/tunnel-client/releases/download/"+release.Tag+"/"+expectedName {
		return nil, errors.New("release download URL does not match the official GitHub repository")
	}
	if !strings.HasPrefix(asset.Digest, "sha256:") || len(asset.Digest) != 71 {
		return nil, errors.New("official release has no SHA-256 digest; refusing unverified install")
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(asset.Digest, "sha256:"))
	if err != nil {
		return nil, errors.New("invalid official SHA-256 digest")
	}
	if asset.Size <= 0 || asset.Size > tunnelClientMaxArchive {
		return nil, errors.New("unexpected Tunnel archive size")
	}
	archiveRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return nil, err
	}
	archiveResponse, err := client.Do(archiveRequest)
	if err != nil {
		return nil, fmt.Errorf("Tunnel download failed: %w", err)
	}
	defer archiveResponse.Body.Close()
	if archiveResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tunnel download HTTP %d", archiveResponse.StatusCode)
	}
	// 30 MB typical; bounded to prevent a manipulated upstream response from
	// consuming unbounded memory. No executable is run during extraction.
	body, err := io.ReadAll(io.LimitReader(archiveResponse.Body, tunnelClientMaxArchive+1))
	if err != nil || len(body) > tunnelClientMaxArchive {
		return nil, errors.New("Tunnel archive could not be safely downloaded")
	}
	hash := sha256.Sum256(body)
	if !equalDigest(hash[:], digest) {
		return nil, errors.New("Tunnel archive SHA-256 mismatch")
	}
	return zip.NewReader(bytes.NewReader(body), int64(len(body)))
}

func equalDigest(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}

func extractTunnelBinaries(zipArchive *zip.Reader, dest, goos string) (err error) {
	binaryName := "tunnel-client"
	sidecar := "cloudflared"
	if goos == "windows" {
		binaryName += ".exe"
		sidecar += ".exe"
	}
	names := []string{binaryName, sidecar}
	selected := make([]*zip.File, 0, len(names))
	// Validate the *complete* payload before writing either file.
	for _, filename := range names {
		var file *zip.File
		for _, f := range zipArchive.File {
			if f.Name == filename {
				file = f
				break
			}
		}
		if file == nil || file.FileInfo().IsDir() || file.FileInfo().Mode()&os.ModeSymlink != 0 ||
			file.UncompressedSize64 > tunnelClientMaxFile {
			return fmt.Errorf("verified Tunnel archive lacks safe %s binary", filename)
		}
		if _, statErr := os.Stat(filepath.Join(dest, filename)); statErr == nil {
			return fmt.Errorf("tunnel executable already exists: %s (refusing overwrite)", filename)
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		selected = append(selected, file)
	}
	var installed []string
	defer func() {
		if err != nil {
			for _, path := range installed {
				os.Remove(path)
			}
		}
	}()
	for index, file := range selected {
		target := filepath.Join(dest, names[index])
		reader, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		temp, createErr := os.CreateTemp(dest, ".vidotter-tunnel-*")
		if createErr != nil {
			reader.Close()
			return createErr
		}
		tempName := temp.Name()
		length, copyErr := io.Copy(temp, io.LimitReader(reader, tunnelClientMaxFile+1))
		closeErr, readCloseErr := temp.Close(), reader.Close()
		if copyErr != nil || closeErr != nil || readCloseErr != nil || length > tunnelClientMaxFile {
			os.Remove(tempName)
			return fmt.Errorf("unable to extract verified %s", names[index])
		}
		if err := os.Chmod(tempName, 0o755); err != nil {
			os.Remove(tempName)
			return err
		}
		if err := os.Rename(tempName, target); err != nil {
			os.Remove(tempName)
			return err
		}
		installed = append(installed, target)
	}
	return nil
}

func installTunnelClient(ctx context.Context, client *http.Client, goos, goarch, home string) (string, error) {
	tunnelClientInstallMu.Lock()
	defer tunnelClientInstallMu.Unlock()
	if existing := findTunnelClient(goos, home); existing != "" {
		return existing, nil
	}
	if _, _, err := tunnelClientPlatform(goos, goarch); err != nil {
		return "", err
	}
	archive, err := readVerifiedTunnelArchive(ctx, client, goos, goarch)
	if err != nil {
		return "", err
	}
	path := tunnelClientTarget(goos, home)
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	if err := extractTunnelBinaries(archive, parent, goos); err != nil {
		return "", err
	}
	return path, nil
}

func (s *bridgeServer) handleMCPClientInstall(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost || !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "user home unavailable"})
		return
	}
	path, err := installTunnelClient(r.Context(), tunnelReleaseHTTPClient(), runtime.GOOS, runtime.GOARCH, home)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	_ = path // Never expose local absolute paths in browser responses.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Official Tunnel Client installed and verified; no remote service started"})
}
