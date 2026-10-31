package vidotter

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const remoteBrowserVersion = "v0.3.0"
const remoteBrowserReleaseURL = "https://github.com/ThinkerQAQ/remote-browser-mcp/releases/download/" + remoteBrowserVersion
const browserArchiveLimit = 24 << 20
const browserExtractLimit = 32 << 20

func browserComponentDir() (string, error) {
	root, err := bridgeDataPath("components")
	return filepath.Join(root, "browser-automation", remoteBrowserVersion), err
}
func browserComponentInstalled() bool {
	dir, err := browserComponentDir()
	if err != nil {
		return false
	}
	for _, name := range []string{"package.json", "package-lock.json", "scripts/proxy.mjs", "LICENSE", "THIRD_PARTY_NOTICES.md", "node_modules/chrome-devtools-mcp/package.json"} {
		info, e := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if e != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}
func browserReleaseAsset(goos string) (string, error) {
	switch goos {
	case "windows":
		return "remote-browser-mcp-" + remoteBrowserVersion + "-windows.zip", nil
	case "linux":
		return "remote-browser-mcp-" + remoteBrowserVersion + "-linux.tar.gz", nil
	case "darwin":
		return "remote-browser-mcp-" + remoteBrowserVersion + "-macos.tar.gz", nil
	default:
		return "", fmt.Errorf("browser automation unavailable on %s", goos)
	}
}
func verifyBrowserRelease(ctx context.Context, client *http.Client, baseURL, asset string) ([]byte, error) {
	sums, err := fetchLimited(ctx, client, baseURL+"/SHA256SUMS", 1<<20)
	if err != nil {
		return nil, errors.New("cannot fetch browser automation SHA256SUMS")
	}
	want, err := checksumForAsset(sums, asset)
	if err != nil {
		return nil, errors.New("browser automation checksum entry not found")
	}
	raw, err := fetchLimited(ctx, client, baseURL+"/"+asset, browserArchiveLimit)
	if err != nil {
		return nil, errors.New("cannot fetch browser automation release")
	}
	got := sha256.Sum256(raw)
	if hex.EncodeToString(got[:]) != want {
		return nil, errors.New("browser automation SHA-256 mismatch")
	}
	return raw, nil
}
func extractBrowserArchive(archive []byte, asset, target string) error {
	prefix := "remote-browser-mcp-" + remoteBrowserVersion + "/"
	total := int64(0)
	extract := func(name string, mode os.FileMode, content io.Reader, size int64) error {
		if size < 0 || size > 4<<20 {
			return errors.New("browser archive entry exceeds size limit")
		}
		if strings.Contains("/"+name+"/", "/../") || strings.Contains("/"+name+"/", "/./") {
			return errors.New("unsafe archive traversal")
		}
		clean := path.Clean(name)
		if !strings.HasPrefix(clean, prefix) || strings.Contains(clean, "\\") || strings.HasPrefix(name, "/") {
			return errors.New("unsafe browser archive path")
		}
		relative := strings.TrimPrefix(clean, prefix)
		if relative == "" || relative == "." {
			return nil
		}
		if relative == ".." || strings.HasPrefix(relative, "../") {
			return errors.New("browser archive path escapes root")
		}
		if !mode.IsRegular() {
			return errors.New("browser archive contains unsupported entry")
		}
		total += size
		if total > browserExtractLimit {
			return errors.New("browser archive exceeds extraction limit")
		}
		dest := filepath.Join(target, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(file, content, size)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return err
		}
		for _, file := range zr.File {
			if file.FileInfo().IsDir() {
				continue
			}
			reader, err := file.Open()
			if err != nil {
				return err
			}
			err = extract(file.Name, file.Mode(), reader, int64(file.UncompressedSize64))
			_ = reader.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue // PAX metadata is not an extractable filesystem entry.
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("browser archive contains non-regular entry type=%d", header.Typeflag)
		}
		if err := extract(header.Name, 0600, tr, header.Size); err != nil {
			return err
		}
	}
}
func findBrowserNode() (string, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return "", errors.New("Node.js 22+ is required for optional browser automation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--version").Output()
	if err != nil {
		return "", errors.New("cannot execute Node.js")
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(output)), "v")
	major, _ := strconv.Atoi(strings.SplitN(version, ".", 2)[0])
	if major < 22 {
		return "", errors.New("Node.js 22+ is required")
	}
	return node, nil
}
func installBrowserAutomation(ctx context.Context, proxy string) error {
	if browserComponentInstalled() {
		return nil
	}
	runtimeNode, err := ensureBrowserNodeRuntime(ctx, proxy)
	if err != nil {
		return err
	}
	asset, err := browserReleaseAsset(runtime.GOOS)
	if err != nil {
		return err
	}
	client, err := browserComponentHTTPClient(proxy)
	if err != nil {
		return err
	}
	archive, err := verifyBrowserRelease(ctx, client, remoteBrowserReleaseURL, asset)
	if err != nil {
		return err
	}
	target, err := browserComponentDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(target), ".browser-automation-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := extractBrowserArchive(archive, asset, staging); err != nil {
		return err
	}
	for _, name := range []string{"package.json", "package-lock.json", "scripts/proxy.mjs", "LICENSE", "THIRD_PARTY_NOTICES.md"} {
		if _, err := os.Stat(filepath.Join(staging, filepath.FromSlash(name))); err != nil {
			return errors.New("browser release is incomplete")
		}
	}
	// A fixed policy is generated by VidOtter, rather than trusting the upstream
	// configuration, the current user's independent RBT config, or a public file.
	config := "[browser]\nname = \"edge\"\nendpoint = \"http://127.0.0.1:9222\"\n\n[security]\nallow_existing_pages = false\nallow_javascript = false\n"
	if err := os.WriteFile(filepath.Join(staging, "config.toml"), []byte(config), 0600); err != nil {
		return err
	}
	installCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	if runtimeNode.NPMScript {
		cmd = exec.CommandContext(installCtx, runtimeNode.Node, runtimeNode.NPM, "ci", "--omit=dev", "--ignore-scripts", "--no-audit", "--no-fund")
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(installCtx, "cmd.exe", "/d", "/s", "/c", runtimeNode.NPM, "ci", "--omit=dev", "--ignore-scripts", "--no-audit", "--no-fund")
	} else {
		cmd = exec.CommandContext(installCtx, runtimeNode.NPM, "ci", "--omit=dev", "--ignore-scripts", "--no-audit", "--no-fund")
	}
	cmd.Dir = staging
	cmd.Env = append(os.Environ(), "NPM_CONFIG_IGNORE_SCRIPTS=true", "NPM_CONFIG_AUDIT=false", "NPM_CONFIG_FUND=false", "PATH="+filepath.Dir(runtimeNode.Node)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = output
		return errors.New("npm dependency installation failed; verify network access and Node.js")
	}
	if _, err := os.Stat(filepath.Join(staging, "node_modules", "chrome-devtools-mcp", "package.json")); err != nil {
		return errors.New("browser automation dependency missing after install")
	}
	if _, err := os.Stat(target); err == nil {
		// The verified and fully built version replaces only its own component
		// directory; never replace any user profile, downloads or existing Data.
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return os.Rename(staging, target)
}

// Optional dependency bootstrap honors the operating system's HTTPS_PROXY
// environment only when VidOtter has no explicit component proxy configured.
// This does not change the media downloader's direct-only networking policy.
func browserComponentHTTPClient(proxy string) (*http.Client, error) {
	if strings.TrimSpace(proxy) != "" {
		return componentHTTPClient(proxy)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// DefaultTransport includes ProxyFromEnvironment; do not expose this
	// transport through the browser or AI tool interfaces.
	return &http.Client{Transport: transport, Timeout: 5 * time.Minute}, nil
}
