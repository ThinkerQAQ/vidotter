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
