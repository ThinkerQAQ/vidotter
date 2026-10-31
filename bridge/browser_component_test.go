package vidotter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserAssetAndNoMobileDependency(t *testing.T) {
	for _, test := range []struct{ os, asset string }{
		{"windows", "remote-browser-mcp-v0.3.0-windows.zip"},
		{"linux", "remote-browser-mcp-v0.3.0-linux.tar.gz"},
		{"darwin", "remote-browser-mcp-v0.3.0-macos.tar.gz"},
	} {
		got, err := browserReleaseAsset(test.os)
		if err != nil || got != test.asset {
			t.Fatalf("%s: %q %v", test.os, got, err)
		}
	}
	if _, err := browserReleaseAsset("android"); err == nil {
		t.Fatal("Android should not install desktop browser automation")
	}
}

func testBrowserArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tarWriter := tar.NewWriter(gz)
	for name, content := range entries {
		head := &tar.Header{Name: name, Mode: 0600, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tarWriter.WriteHeader(head); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}
func TestBrowserArchiveSafeExtraction(t *testing.T) {
	base := "remote-browser-mcp-v0.3.0/"
	good := testBrowserArchive(t, map[string]string{
		base + "package.json":      "{}",
		base + "LICENSE":           "Apache 2.0",
		base + "scripts/proxy.mjs": "console.log('ok');",
	})
	root := t.TempDir()
	if err := extractBrowserArchive(good, "sample.tar.gz", root); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(root, "scripts", "proxy.mjs")); err != nil || string(body) != "console.log('ok');" {
		t.Fatalf("incorrect output: %q %v", body, err)
	}
	for _, bad := range []string{
		"remote-browser-mcp-v0.3.0/../../outside",
		"../remote-browser-mcp-v0.3.0/package.json",
		"/remote-browser-mcp-v0.3.0/package.json",
	} {
		b := testBrowserArchive(t, map[string]string{bad: "danger"})
		if err := extractBrowserArchive(b, "sample.tar.gz", t.TempDir()); err == nil {
			t.Fatalf("allowed traversal %q", bad)
		}
	}
}
func TestBrowserReleaseChecksum(t *testing.T) {
	const asset = "remote-browser-mcp-v0.3.0-linux.tar.gz"
	contents := []byte("verified archive")
	sum := sha256.Sum256(contents)
	valid := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
		case "/" + asset:
			if !valid {
				_, _ = w.Write([]byte("tampered"))
			} else {
				_, _ = w.Write(contents)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	got, err := verifyBrowserRelease(context.Background(), server.Client(), server.URL, asset)
	if err != nil || !bytes.Equal(got, contents) {
		t.Fatalf("valid release rejected: %v", err)
	}
	valid = false
	if _, err := verifyBrowserRelease(context.Background(), server.Client(), server.URL, asset); err == nil {
		t.Fatal("tampered release accepted")
	}
}
