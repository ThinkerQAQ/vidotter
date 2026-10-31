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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const bundledNodeVersion = "v24.22.2"
const nodeReleaseBaseURL = "https://nodejs.org/dist/" + bundledNodeVersion

type browserNodeRuntime struct {
	Node      string
	NPM       string
	NPMScript bool
}

func browserNodeRoot() (string, error) {
	root, err := bridgeDataPath("components")
	return filepath.Join(root, "node-runtime", bundledNodeVersion), err
}
func nodeRuntimeLocations() (string, string, error) {
	root, err := browserNodeRoot()
	if err != nil {
		return "", "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "node.exe"), filepath.Join(root, "node_modules", "npm", "bin", "npm-cli.js"), nil
	}
	return filepath.Join(root, "bin", "node"), filepath.Join(root, "lib", "node_modules", "npm", "bin", "npm-cli.js"), nil
}
func installedBrowserNode() (browserNodeRuntime, bool) {
	node, npm, err := nodeRuntimeLocations()
	if err != nil {
		return browserNodeRuntime{}, false
	}
	for _, name := range []string{node, npm} {
		info, e := os.Stat(name)
		if e != nil || !info.Mode().IsRegular() {
			return browserNodeRuntime{}, false
		}
	}
	return browserNodeRuntime{Node: node, NPM: npm, NPMScript: true}, true
}
func resolveBrowserNode() (string, error) {
	if node, err := findBrowserNode(); err == nil {
		return node, nil
	}
	if bundled, ok := installedBrowserNode(); ok {
		return bundled.Node, nil
	}
	return "", errors.New("Node.js runtime is missing; install browser automation in VidOtter settings")
}
func nodeReleaseAsset(goos, goarch string) (string, error) {
	architecture := map[string]string{"amd64": "x64", "arm64": "arm64"}[goarch]
	if architecture == "" {
		return "", errors.New("unsupported Node.js CPU architecture")
	}
	switch goos {
	case "windows":
		return "node-" + bundledNodeVersion + "-win-" + architecture + ".zip", nil
	case "linux":
		return "node-" + bundledNodeVersion + "-linux-" + architecture + ".tar.gz", nil
	case "darwin":
		return "node-" + bundledNodeVersion + "-darwin-" + architecture + ".tar.gz", nil
	default:
		return "", errors.New("Node.js bootstrap unavailable on this platform")
	}
}
func extractMinimalNode(archive []byte, asset, target string) error {
	prefix := strings.TrimSuffix(asset, ".zip")
	prefix = strings.TrimSuffix(prefix, ".tar.gz") + "/"
	maxExtract := int64(400 << 20)
	total := int64(0)
	put := func(name string, size int64, mode os.FileMode, content io.Reader) error {
		if size < 0 || size > 180<<20 {
			return errors.New("Node.js archive entry exceeds size limit")
		}
		if strings.Contains("/"+name+"/", "/../") || strings.Contains("/"+name+"/", "/./") {
			return errors.New("unsafe archive traversal")
		}
		clean := path.Clean(name)
		if !strings.HasPrefix(clean, prefix) || strings.Contains(clean, "\\") || strings.HasPrefix(name, "/") {
			return errors.New("unsafe Node.js archive path")
		}
		relative := strings.TrimPrefix(clean, prefix)
		wanted := relative == "LICENSE" || relative == "README.md"
		if runtime.GOOS == "windows" || strings.Contains(asset, "-win-") {
			wanted = wanted || relative == "node.exe" || strings.HasPrefix(relative, "node_modules/npm/")
		} else {
			wanted = wanted || relative == "bin/node" || strings.HasPrefix(relative, "lib/node_modules/npm/")
		}
		if !wanted {
			return nil
		}
		if !mode.IsRegular() {
			return errors.New("Node.js archive contains unsupported entry")
		}
		total += size
		if total > maxExtract {
			return errors.New("Node.js archive exceeds safe extracted size")
		}
		filename := filepath.Join(target, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			return err
		}
		permissions := os.FileMode(0600)
		if relative == "bin/node" || relative == "node.exe" {
			permissions = 0700
		}
		file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permissions)
		if err != nil {
			return err
		}
		_, err = io.CopyN(file, content, size)
		closeErr := file.Close()
		if err != nil {
			return err
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
			err = put(file.Name, int64(file.UncompressedSize64), file.Mode(), reader)
			_ = reader.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeDir || h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return fmt.Errorf("unexpected Node.js archive entry type")
		}
		if err := put(h.Name, h.Size, 0600, tr); err != nil {
			return err
		}
	}
}
func ensureBrowserNodeRuntime(ctx context.Context, proxy string) (browserNodeRuntime, error) {
	systemNode, err := findBrowserNode()
	if err == nil {
		if npm, err := exec.LookPath("npm"); err == nil {
			return browserNodeRuntime{Node: systemNode, NPM: npm}, nil
		}
	}
	if local, ok := installedBrowserNode(); ok {
		return local, nil
	}
	asset, err := nodeReleaseAsset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return browserNodeRuntime{}, err
	}
	client, err := browserComponentHTTPClient(proxy)
	if err != nil {
		return browserNodeRuntime{}, err
	}
	sums, err := fetchLimited(ctx, client, nodeReleaseBaseURL+"/SHASUMS256.txt", 1<<20)
	if err != nil {
		return browserNodeRuntime{}, errors.New("cannot fetch official Node.js checksums")
	}
	want, err := checksumForAsset(sums, asset)
	if err != nil {
		return browserNodeRuntime{}, errors.New("official Node.js checksum unavailable")
	}
	raw, err := fetchLimited(ctx, client, nodeReleaseBaseURL+"/"+asset, 140<<20)
	if err != nil {
		return browserNodeRuntime{}, errors.New("cannot fetch official Node.js runtime")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != want {
		return browserNodeRuntime{}, errors.New("Node.js SHA-256 mismatch")
	}
	target, err := browserNodeRoot()
	if err != nil {
		return browserNodeRuntime{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return browserNodeRuntime{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".node-verified-")
	if err != nil {
		return browserNodeRuntime{}, err
	}
	defer os.RemoveAll(stage)
	if err := extractMinimalNode(raw, asset, stage); err != nil {
		return browserNodeRuntime{}, err
	}
	var node, npm string
	if runtime.GOOS == "windows" {
		node = filepath.Join(stage, "node.exe")
		npm = filepath.Join(stage, "node_modules", "npm", "bin", "npm-cli.js")
	} else {
		node = filepath.Join(stage, "bin", "node")
		npm = filepath.Join(stage, "lib", "node_modules", "npm", "bin", "npm-cli.js")
	}
	for _, item := range []string{node, npm, filepath.Join(stage, "LICENSE")} {
		if _, err := os.Stat(item); err != nil {
			return browserNodeRuntime{}, errors.New("verified Node.js bundle is incomplete")
		}
	}
	if _, err := os.Stat(target); err == nil {
		return browserNodeRuntime{}, errors.New("Node.js runtime directory already exists but is incomplete")
	}
	if err := os.Rename(stage, target); err != nil {
		return browserNodeRuntime{}, err
	}
	installed, ok := installedBrowserNode()
	if !ok {
		return browserNodeRuntime{}, errors.New("Node.js runtime could not be verified after installation")
	}
	return installed, nil
}
