package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type persistedSetup struct {
	Autostart   *bool    `json:"autostart_enabled,omitempty"`
	AutoConnect *bool    `json:"tunnel_auto_connect,omitempty"`
	Revision    string   `json:"revision,omitempty"`
	ID          string   `json:"id"`
	Tunnel      string   `json:"tunnel"`
	TunnelArgs  []string `json:"tunnelArgs"`
	Port        int      `json:"port"`
	Manager     string   `json:"manager"`
}
type persistedCredentials struct {
	APIKey    string `json:"apiKey,omitempty"`
	Protected string `json:"protectedKey,omitempty"`
}

func readSetup(root string) (persistedSetup, error) {
	var data persistedSetup
	b, e := os.ReadFile(filepath.Join(root, "runtime", "setup.json"))
	if e != nil {
		return data, e
	}
	e = json.Unmarshal(b, &data)
	return data, e
}

func reservePort(requested int) (int, error) {
	for _, n := range []int{requested, 0} {
		if n <= 0 && requested <= 0 {
			continue
		}
		l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", n))
		if e == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
func shellArg(s string) string {
	s = strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + s + "'"
}
func tunnelCommand(s string) string {
	if os.PathSeparator == '\\' {
		s = strings.ReplaceAll(s, "\\", "/")
	}
	return shellArg(s)
}
func jsonMarshal(v interface{}) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
func makeTunnel(root, id string, port int) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := tunnelCommand(exe) + " --mcp-stdio"
	return jsonMarshal(map[string]any{
		"config_version": 1,
		"control_plane":  map[string]any{"base_url": "https://api.openai.com", "tunnel_id": id, "api_key": "env:OPENAI_API_KEY"},
		"health":         map[string]any{"listen_addr": fmt.Sprintf("127.0.0.1:%d", port)},
		"admin_ui":       map[string]any{"open_browser": false},
		"log":            map[string]any{"level": "info", "format": "json"},
		"mcp":            map[string]any{"commands": []any{map[string]any{"channel": "main", "command": cmd}}, "stdio_send_initialized_notification": true},
	})
}

func writePrivate(path string, content []byte) error {
	dir := filepath.Dir(path)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("private directory must be real")
	}
	// Never restrict the install directory; only private runtime/ is user-only.
	if filepath.Base(path) != "config.toml" {
		if e = protectPrivate(dir, true); e != nil {
			return e
		}
	}
	var existing os.FileInfo
	if info, e := os.Lstat(path); e == nil {
		existing = info
		if !info.Mode().IsRegular() {
			return errors.New("refusing non-regular private config")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	var nonce [8]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return e
	}
	tmp := path + "." + hex.EncodeToString(nonce[:]) + ".tmp"
	f, e := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer os.Remove(tmp)
	if _, e = f.Write(content); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = protectPrivate(tmp, false); e != nil {
		return e
	}
	// sudo service management must not take ownership away from the runtime user.
	if e = preserveExistingOwner(tmp, existing); e != nil {
		return e
	}
	if e = os.Rename(tmp, path); e != nil {
		return e
	}
	return protectPrivate(path, false)
}
func savedKey(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "runtime", "credentials.json"))
	var saved persistedCredentials
	if err != nil || json.Unmarshal(b, &saved) != nil {
		return "", errors.New("valid saved runtime API key required")
	}
	key, err := decodeCredentials(saved)
	if err != nil || key == "" || strings.ContainsAny(key, "\r\n\x00") || strings.Trim(key, "*") == "" {
		return "", errors.New("valid saved runtime API key required")
	}
	return key, nil
}

func validateSavedConnection(root string) error {
	setup, err := readSetup(root)
	if err != nil || !tunnelIDPattern.MatchString(setup.ID) {
		return errors.New("save a valid Tunnel ID first")
	}
	_, err = savedKey(root)
	return err
}

func saveNative(root, id, key string) error {
	if !tunnelIDPattern.MatchString(id) {
		return errors.New("invalid Tunnel ID")
	}
	if strings.ContainsAny(key, "\r\n\x00") || (key != "" && strings.Trim(key, "*") == "") {
		return errors.New("API key must be a single line")
	}
	runtimeDir := filepath.Join(root, "runtime")
	setup, err := readSetup(root)
	if err != nil && !os.IsNotExist(err) {
		return errors.New("invalid existing setup")
	}
	if setup.Tunnel != "" && setup.Tunnel != tunnel(root) {
		return errors.New("configured Tunnel executable does not match this installation")
	}
	previousKey, _ := savedKey(root)
	if key == "" {
		if previousKey == "" {
			return errors.New("runtime API key required for new connection")
		}
		key = previousKey
	}
	if info, e := os.Lstat(filepath.Join(root, "config.toml")); e == nil {
		if !info.Mode().IsRegular() {
			return errors.New("refusing existing non-regular config")
		}

	} else if !os.IsNotExist(e) {
		return e
	}
	// Preserve the configured health port while the daemon is active.
	port := setup.Port
	if port == 0 {
		port, err = reservePort(18833)
		if err != nil {
			return err
		}
	}
	// Merge connection fields only; never reset browser/profile/security settings.
	config := map[string]any{}
	configPath := filepath.Join(root, "config.toml")
	if raw, err := os.ReadFile(configPath); err == nil {
		if toml.Unmarshal(raw, &config) != nil {
			return errors.New("invalid existing config.toml")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if config["setup"] != nil {
		return errors.New("unsupported setup configuration")
	}
	// Reusing identical credentials must not rewrite files or mark the service stale.
	if setup.Revision != "" && setup.ID == id && previousKey == key {
		return nil
	}
	setup.Revision = sessionToken()
	config["tunnel"] = map[string]any{"id": id, "api_key_env": "OPENAI_API_KEY", "health_port": port}
	encoded, err := toml.Marshal(config)
	if err != nil {
		return errors.New("cannot encode configuration")
	}

	payload, e := makeTunnel(root, id, port)
	if e != nil {
		return e
	}
	setup.ID = id
	setup.Tunnel = tunnel(root)
	setup.Port = port
	if setup.Manager == "" {
		setup.Manager = "go-client"
	}
	protected, e := encodeCredentials(key)
	if e != nil {
		return e
	}
	credentials, e := jsonMarshal(protected)
	if e != nil {
		return e
	}
	record, e := jsonMarshal(setup)
	if e != nil {
		return e
	}
	// Keep existing files under user-only ACLs. Config writes are never logged.
	if e = writePrivate(filepath.Join(runtimeDir, "credentials.json"), credentials); e != nil {
		return e
	}
	if e = writePrivate(filepath.Join(runtimeDir, "tunnel.yaml"), payload); e != nil {
		return e
	}
	if e = writePrivate(filepath.Join(root, "config.toml"), encoded); e != nil {
		return e
	}
	return writePrivate(filepath.Join(runtimeDir, "setup.json"), record)
}
