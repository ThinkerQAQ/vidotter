package maint

import "testing"

func TestPublicReleaseTargetsRemainDesktopOnly(t *testing.T) {
    got := targets()
    if len(got) != 5 {
        t.Fatalf("public release targets = %d; want 5 desktop targets", len(got))
    }
    want := map[string]bool{
        "windows/amd64": true,
        "linux/amd64":   true,
        "linux/arm64":   true,
        "darwin/amd64":  true,
        "darwin/arm64":  true,
    }
    for _, target := range got {
        key := target.OS + "/" + target.Arch
        if !want[key] {
            t.Fatalf("unexpected public release target: %s", key)
        }
        delete(want, key)
    }
    if len(want) != 0 {
        t.Fatalf("missing public release targets: %v", want)
    }
}

func TestReleaseScriptFilterExcludesWindowsAutomation(t *testing.T) {
    for _, path := range []string{
        "smoke-offline-installer.ps1",
        "test-native-mcp.PS1",
        "legacy.cmd",
        "hidden.vbs",
    } {
        if releaseScriptFilter(path, false) {
            t.Errorf("release must not ship obsolete Windows script %s", path)
        }
    }
    for _, path := range []string{
        "setup.sh",
        "run.py",
        "installer-gui-smoke.qs",
    } {
        if !releaseScriptFilter(path, false) {
            t.Errorf("unexpectedly excluded %s", path)
        }
    }
    if !releaseScriptFilter("scripts", true) {
        t.Fatal("script directory must remain included")
    }
}
