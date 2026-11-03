package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"encoding/json"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSaveRejectsInvalidCredentials(t *testing.T) {
	root := t.TempDir()
	if e := saveNative(root, "not-tunnel", "key"); e == nil {
		t.Fatal("invalid tunnel accepted")
	}
	if e := saveNative(root, "tunnel_test", "bad\nkey"); e == nil {
		t.Fatal("multiline key accepted")
	}
	if e := saveNative(root, "tunnel_test", ""); e == nil {
		t.Fatal("missing key accepted")
	}
}
func TestNativeConfigDoesNotExposeSecretOrRestart(t *testing.T) {
	root := t.TempDir()
	// Save is independent of Node, PowerShell, Bash and any desktop.
	if e := saveNative(root, "tunnel_TEST", "synthetic-not-a-real-key"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"config.toml", "runtime/tunnel.yaml", "runtime/setup.json"} {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(b), "synthetic-not-a-real-key") {
			t.Fatalf("secret leaked into %s", name)
		}
	}
	state, e := readSetup(root)
	if e != nil {
		t.Fatal(e)
	}
	if state.Manager != "go-client" || state.ID != "tunnel_TEST" {
		t.Fatalf("unexpected state: %#v", state)
	}
	raw, e := os.ReadFile(filepath.Join(root, "runtime", "tunnel.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	var decoded map[string]any
	if e = json.Unmarshal(raw, &decoded); e != nil {
		t.Fatal(e)
	}
	if decoded["config_version"].(float64) != 1 {
		t.Fatal("wrong tunnel version")
	}
	if e = saveNative(root, "tunnel_CHANGED", ""); e != nil {
		t.Fatal(e)
	}
	state, _ = readSetup(root)
	if state.ID != "tunnel_CHANGED" {
		t.Fatal("saved config not reused")
	}
	if e = saveNative(root, "tunnel_CHANGED", ""); e != nil {
		t.Fatal(e)
	}
}

func TestBrowserSettingsSurviveConnectionChanges(t *testing.T) {
	root := t.TempDir()
	original := `[browser]
name = "chrome"
endpoint = "http://127.0.0.1:9222"
user_data_dir = 'C:\Users\Example\Browser Profile'
[security]
allow_existing_pages = true
allow_javascript = false
[custom]
label = "preserve"
`
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tunnel_FIRST", "tunnel_SECOND"} {
		if err := saveNative(root, id, "synthetic-key"); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(root, "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if err := toml.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		browser := config["browser"].(map[string]any)
		if browser["name"] != "chrome" || browser["user_data_dir"] != `C:\Users\Example\Browser Profile` {
			t.Fatal("browser settings changed")
		}
		if config["security"].(map[string]any)["allow_existing_pages"] != true || config["custom"].(map[string]any)["label"] != "preserve" {
			t.Fatal("unrelated settings changed")
		}
	}
}

func TestLegacyBrowserSetupRequiresExplicitMigration(t *testing.T) {
	root := t.TempDir()
	original := "[setup]\ntunnel_id = \"tunnel_OLD\"\n"
	file := filepath.Join(root, "config.toml")
	if err := os.WriteFile(file, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveNative(root, "tunnel_NEW", "synthetic-key"); err == nil {
		t.Fatal("silently migrated a legacy runtime")
	}
	raw, _ := os.ReadFile(file)
	if string(raw) != original {
		t.Fatal("modified legacy config on rejection")
	}
	if _, err := os.Stat(filepath.Join(root, "runtime")); !os.IsNotExist(err) {
		t.Fatal("created runtime on rejection")
	}
}
