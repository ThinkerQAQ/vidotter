package vidotter

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tunnelRoundTripper func(*http.Request) (*http.Response, error)

func (f tunnelRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func tunnelArchiveFixture(t *testing.T, platform string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	suffix := ""
	if strings.HasPrefix(platform, "windows-") {
		suffix = ".exe"
	}
	for _, name := range []string{"tunnel-client" + suffix, "cloudflared" + suffix} {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, "verified executable "+name); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func tunnelFixtureClient(t *testing.T, platform string, badDigest bool) *http.Client {
	t.Helper()
	body := tunnelArchiveFixture(t, platform)
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if badDigest {
		digest = strings.Repeat("0", 64)
	}
	assetName := "tunnel-client-v0.0.19-" + platform + ".zip"
	assetURL := "https://github.com/openai/tunnel-client/releases/download/v0.0.19/" + assetName
	release := map[string]any{
		"tag_name": "v0.0.19",
		"assets": []map[string]any{{
			"name": assetName, "size": len(body), "digest": "sha256:" + digest,
			"browser_download_url": assetURL,
		}},
	}
	metadata, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: tunnelRoundTripper(func(r *http.Request) (*http.Response, error) {
		var raw []byte
		switch r.URL.String() {
		case tunnelClientReleaseAPI:
			raw = metadata
		case assetURL:
			raw = body
		default:
			return nil, fmt.Errorf("unexpected URL %s", r.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(raw)),
			Header: make(http.Header)}, nil
	})}
}

func TestTunnelClientPlatform(t *testing.T) {
	for _, tc := range []struct{ os, arch, suffix string }{
		{"windows", "amd64", ".exe"}, {"windows", "arm64", ".exe"},
		{"linux", "amd64", ""}, {"linux", "arm64", ""},
		{"darwin", "amd64", ""}, {"darwin", "arm64", ""},
	} {
		p, ext, err := tunnelClientPlatform(tc.os, tc.arch)
		if err != nil || ext != tc.suffix || p != tc.os+"-"+tc.arch {
			t.Fatalf("%s/%s: %v %q %q", tc.os, tc.arch, err, p, ext)
		}
	}
	if _, _, err := tunnelClientPlatform("freebsd", "amd64"); err == nil {
		t.Fatal("unknown OS accepted")
	}
	if _, _, err := tunnelClientPlatform("linux", "386"); err == nil {
		t.Fatal("unknown architecture accepted")
	}
}
func TestTunnelClientVerifiedInstallAndIdempotency(t *testing.T) {
	for _, tc := range []struct{ os, arch string }{{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"}} {
		t.Run(tc.os+"/"+tc.arch, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			home := t.TempDir()
			client := tunnelFixtureClient(t, tc.os+"-"+tc.arch, false)
			got, err := installTunnelClient(context.Background(), client, tc.os, tc.arch, home)
			if err != nil {
				t.Fatal(err)
			}
			if got != tunnelClientTarget(tc.os, home) {
				t.Fatalf("wrong target: %q", got)
			}
			if _, err := os.Stat(got); err != nil {
				t.Fatal(err)
			}
			suffix := ""
			if tc.os == "windows" {
				suffix = ".exe"
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(got), "cloudflared"+suffix)); err != nil {
				t.Fatal(err)
			}
			// Running installer again should preserve the already installed binary.
			again, err := installTunnelClient(context.Background(), client, tc.os, tc.arch, home)
			if err != nil || again != got {
				t.Fatalf("duplicate install: %v %s", err, again)
			}
		})
	}
}
func TestTunnelClientRejectsChecksumMismatch(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	_, err := installTunnelClient(context.Background(), tunnelFixtureClient(t, "linux-amd64", true), "linux", "amd64", home)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("checksum mismatch not rejected: %v", err)
	}
	if _, err := os.Stat(tunnelClientTarget("linux", home)); !os.IsNotExist(err) {
		t.Fatal("bad archive wrote an executable")
	}
}
func TestTunnelInstallerRequiresBridgeToken(t *testing.T) {
	server := &bridgeServer{state: bridgeState{Token: "test-secret"}}
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		req := httptest.NewRequest(method, "/v1/mcp/client/install", nil)
		writer := httptest.NewRecorder()
		server.handleMCPClientInstall(writer, req)
		if writer.Code != http.StatusUnauthorized {
			t.Fatalf("%s code %d", method, writer.Code)
		}
	}
}
