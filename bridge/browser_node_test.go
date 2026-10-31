package vidotter

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNodeReleaseAssetFixedVersion(t *testing.T) {
	for _, test := range []struct{ platform, arch, suffix string }{
		{"windows", "amd64", "win-x64.zip"},
		{"windows", "arm64", "win-arm64.zip"},
		{"linux", "amd64", "linux-x64.tar.gz"},
		{"linux", "arm64", "linux-arm64.tar.gz"},
		{"darwin", "arm64", "darwin-arm64.tar.gz"},
		{"darwin", "amd64", "darwin-x64.tar.gz"},
	} {
		name, err := nodeReleaseAsset(test.platform, test.arch)
		if err != nil || name != "node-"+bundledNodeVersion+"-"+test.suffix {
			t.Fatalf("%+v got=%s err=%v", test, name, err)
		}
	}
	if _, err := nodeReleaseAsset("android", "arm64"); err == nil {
		t.Fatal("Android must not install Node.js runtime")
	}
}
func testNodeArchive(t *testing.T, asset string, files map[string]string) []byte {
	t.Helper()
	if !strings.HasSuffix(asset, ".zip") {
		return testBrowserArchive(t, files)
	}
	var raw bytes.Buffer
	writer := zip.NewWriter(&raw)
	for filename, content := range files {
		file, err := writer.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func TestNodeArchiveRejectsUnsafePaths(t *testing.T) {
	asset, err := nodeReleaseAsset(runtime.GOOS, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	prefix := strings.TrimSuffix(strings.TrimSuffix(asset, ".zip"), ".tar.gz") + "/"
	files := []string{"bin/node", "lib/node_modules/npm/bin/npm-cli.js", "LICENSE"}
	if runtime.GOOS == "windows" {
		files = []string{"node.exe", "node_modules/npm/bin/npm-cli.js", "LICENSE"}
	}
	archiveEntries := make(map[string]string, len(files))
	for _, name := range files {
		archiveEntries[prefix+name] = "test fixture"
	}
	root := t.TempDir()
	if err := extractMinimalNode(testNodeArchive(t, asset, archiveEntries), asset, root); err != nil {
		t.Fatal(err)
	}
	for _, entry := range files {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry))); err != nil {
			t.Fatal(err)
		}
	}
	bad := testNodeArchive(t, asset, map[string]string{prefix + "../outside/" + files[0]: "evil"})
	if err := extractMinimalNode(bad, asset, t.TempDir()); err == nil {
		t.Fatal("Node archive traversal accepted")
	}
}

func TestLiveOfficialNodeBootstrap(t *testing.T) {
	if os.Getenv("VIDOTTER_NODE_BOOTSTRAP_LIVE") != "1" {
		t.Skip("set VIDOTTER_NODE_BOOTSTRAP_LIVE=1 for official Node.js bootstrap")
	}
	previous := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir()) // Force the no-system-Node fresh-user path.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	runtime, err := ensureBrowserNodeRuntime(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !runtime.NPMScript {
		t.Fatal("expected privately bootstrapped Node and npm")
	}
	version, err := exec.Command(runtime.Node, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	npmVersion, err := exec.Command(runtime.Node, runtime.NPM, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("bootstrap verified Node %s, npm %s; original PATH length %d", strings.TrimSpace(string(version)), strings.TrimSpace(string(npmVersion)), len(previous))
}
