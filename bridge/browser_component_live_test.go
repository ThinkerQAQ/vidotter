package vidotter

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt-in integration smoke: requires public GitHub/npm access. The Go test
// executable has a disposable runtime directory; this never touches the
// user's installed VidOtter Data or their independent Remote Browser MCP.
func TestLiveOptionalBrowserComponentInstall(t *testing.T) {
	if os.Getenv("VIDOTTER_BROWSER_COMPONENT_LIVE") != "1" {
		t.Skip("set VIDOTTER_BROWSER_COMPONENT_LIVE=1 for live upstream install smoke")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := installBrowserAutomation(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if !browserComponentInstalled() {
		t.Fatal("component not installed")
	}
	dir, err := browserComponentDir()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	value := string(body)
	if !strings.Contains(value, "allow_existing_pages = false") || !strings.Contains(value, "allow_javascript = false") {
		t.Fatal("browser permissions not restricted")
	}
	t.Logf("Verified optional Remote Browser MCP %s install with npm integrity checks", remoteBrowserVersion)
}

func TestLiveOptionalBrowserMCPReadOnly(t *testing.T) {
	if os.Getenv("VIDOTTER_BROWSER_COMPONENT_LIVE") != "1" {
		t.Skip("explicit live opt-in")
	}
	if err := installBrowserAutomation(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	p := &browserAutomationProxy{}
	defer p.close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	output, err := p.call(ctx, browserAutomationCall{Name: "browser_status"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"existing_tabs_protected":true`) || !strings.Contains(output, `"arbitrary_javascript_enabled":false`) {
		t.Fatalf("component permissions not enforced: %s", output)
	}
	t.Logf("MCP stdio protected browser status: %s", output)
}

// Only runs with explicit test opt-in: creates a NEW background page and closes it.
// It never selects, enumerates, or modifies existing user tabs.
func TestLiveOptionalBrowserOwnedPage(t *testing.T) {
	if os.Getenv("VIDOTTER_BROWSER_PAGE_LIVE") != "1" {
		t.Skip("explicit background page integration opt-in required")
	}
	if err := installBrowserAutomation(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	p := &browserAutomationProxy{}
	defer p.close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := p.call(ctx, browserAutomationCall{Name: "browser_open", URL: "https://example.com/"})
	if err != nil {
		t.Fatalf("open public background test page: %v", err)
	}
	t.Logf("background page response: %s", out)
	// The upstream response is text; identify the newly created page with a
	// strict structured page ID parser only when a page ID was returned.
	match := regexp.MustCompile(`(?m)^\s*(\d+): .*\[selected\]`).FindStringSubmatch(out)
	if len(match) != 2 {
		t.Fatalf("upstream did not identify the component-owned selected page: %s", out)
	}
	id, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.call(ctx, browserAutomationCall{Name: "browser_snapshot", PageID: id})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("owned page snapshot has %d bytes", len(snapshot))
	if _, err := p.call(ctx, browserAutomationCall{Name: "browser_close", PageID: id}); err != nil {
		t.Fatal(err)
	}
}
