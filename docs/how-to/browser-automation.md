# Optional Browser Automation

[中文](../zh-CN/how-to/browser-automation.md) · [MCP tools](mcp.md) · [Security / licenses](../../THIRD_PARTY_NOTICES.md)

VidOtter is a **single product and a single MCP connection**. Browser Automation is an optional component that uses the complete, separately maintained [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) implementation as an **external stdio MCP subprocess**. Its source is not copied, vendored, forked, or merged into VidOtter.

## Media discovery flow

1. The user opens and plays media in Edge/Chrome (manually, or in a dedicated background tab created with the optional browser component).
2. The VidOtter extension's existing `chrome.webRequest` listener captures playable HLS/DASH/MP4 requests.
3. The extension sends **only metadata** to the authenticated local Bridge: a tab-scoped resource key, media kind, page title, and page URL. Bridge removes URL query strings/fragments and stores an opaque ID **in memory for 10 minutes**. It never puts captured URLs, cookies, or browser headers in this index.
4. `vidotter_list_browser_media` returns these opaque IDs and sanitized page identity. Match the page and episode **before downloading**.
5. `vidotter_download_browser_media` asks the extension, through a persistent **Native Messaging** connection, to submit the selected resource. The extension looks up the resource in its existing `chrome.storage.session` cache and retrieves cookies **on demand**, using the unchanged task construction and Bridge download queue. The MCP response contains only a task ID; poll `vidotter_get_job`.

If no network media appears, opening HTML alone is not sufficient. VidOtter never ships site-specific extractors and does not bypass DRM or access controls. Media licensing and lawful use are the user's responsibility; see the README legal notice.

## Verified single-MCP workflow (Windows)

On October 31, 2026, a Windows installation completed a **real end-to-end smoke test using only VidOtter MCP**, without a separately connected Remote Browser Tool plugin:

1. `vidotter_status` reported Bridge `1.5.0` ready; `vidotter_browser_status` confirmed the optional `v0.3.0` component was installed and enabled.
2. `vidotter_browser_open` created a dedicated background tab for the public [MDN CC0 flower.mp4](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4) test video. A separate documentation-page test also passed `vidotter_browser_snapshot` and `vidotter_browser_close`.
3. `vidotter_list_browser_media` returned `extension_connected=true`, a matching MP4 entry and its short-lived opaque `resource_id`.
4. `vidotter_download_browser_media` submitted that ID with `quality=best` and returned a queued task ID. `vidotter_get_job` then reported **completed / 100% / 1,128,375 bytes / one output file**.
5. `vidotter_browser_close` closed the test tab. Existing tabs were not selected or changed.

**What this proves:** browser open, page snapshot, owned-tab close, extension media detection, local job submission, and completed direct MP4 transfer all work together on Windows. Browser autoplay/click, HLS/DASH full-transfer paths, and automated multi-platform browser acceptance **were not covered**. Unit tests and builds run on Windows, Linux, and macOS; this does not establish real browser acceptance on all three.

A newly installed user only needs the VidOtter extension, Bridge, local browser-debugging consent and the optional Browser Automation component. The component is opt-in, and interactive clicking remains off by default. If the browser is already showing the selected media, use `vidotter_list_browser_media` directly; opening a fresh tab or clicking Play is **not required** when the resource is already captured.

### Additional browser-first HLS acceptance (Windows, October 31, 2026)

After the v1.5.1 browser-first MCP change, an independent test used the public [hls.js demo](https://hlsjs.video-dev.org/demo/) as a webpage input: the Edge extension captured multiple HLS resources, `vidotter_download(url="https://hlsjs.video-dev.org/demo/", quality="480")` returned `route=browser`, and job `77ce068babe5` reached `completed`/100% with one media file. No yt-dlp webpage extractor was used. A second public HTML video test on W3Schools also completed with `route=browser` and one MP4 file. These are smoke tests, not a claim that all HLS sites or codecs work.

## When Remote Browser Tool is already connected

If ChatGPT already has a working Remote Browser Tool MCP connection, **reuse that existing browser tool** for opening background tabs and inspecting or playing media. Then use `vidotter_list_browser_media` and `vidotter_download_browser_media` from VidOtter. The VidOtter extension captures media independently of how the tab was opened. **Do not enable or call VidOtter's nested Browser Automation component in this mode**: starting a second CDP client may cause another Edge/Chrome authorization prompt. This two-MCP workflow does not require an additional browser component installation, does not copy Remote Browser Tool source, and preserves the existing browser session.

For users without a separate Remote Browser Tool connection, the optional standalone component described below provides one-product installation and one VidOtter MCP connection; browser authorization may still be required by Edge/Chrome.

## Optional one-product setup

1. Install VidOtter normally; load its browser extension. Ordinary downloads work without Node.js or the optional component.
2. In the extension's **Settings → Components** screen choose **Browser Automation → Install and enable**. VidOtter downloads the **pinned Remote Browser MCP v0.3.0** archive and verifies the upstream SHA-256 checksum. It then uses an existing Node.js 22+ runtime, or bootstraps the official **Node.js v24.22.2 LTS** runtime from nodejs.org with its own verified SHA-256; npm dependencies are installed using the release's locked dependency versions and with npm lifecycle scripts disabled. Everything stays in the local `Data/components/` directory and does not require administrative installation.
3. Enable Edge/Chrome's **local CDP on 127.0.0.1:9222**, approving the browser's own security prompt when shown. Browser automation cannot silently grant itself permission or bypass browser restrictions.
4. In ChatGPT connect **only VidOtter MCP** through the user's private VidOtter Tunnel. No second Tunnel or ChatGPT plugin is required. Agent tools include `vidotter_browser_open`, `vidotter_browser_snapshot`, `vidotter_browser_close`, and the browser media discovery/download tools.
5. If automated playback clicking is truly necessary, separately enable **Allow automatic clicks** in the local Components panel. This displays an explicit confirmation and is **off by default**.

The upstream browser component's independent configuration, browser profile, login data, Tunnel, and API keys are never reused. VidOtter supplies its own restrictive config: `allow_existing_pages=false`, `allow_javascript=false`, loopback-only CDP at `9222`. The upstream tool whitelist is narrowed again by the VidOtter adapter.

## Permission matrix

| Browser operation | Default |
| --- | --- |
| Read component health | Allowed when installed |
| Create a new background tab for a public URL | Allowed when enabled |
| Snapshot a component-owned tab | Allowed when enabled |
| Close a component-owned tab | Allowed when enabled |
| Click on a component-owned tab | **Separate local opt-in** |
| Control existing user tabs | **Denied** |
| Run arbitrary JavaScript / read browser cookies or storage / upload files | **Denied** |
| Use network/cookie APIs from AI tools | **Denied**; media network access stays inside the extension |
| Log raw signed media URLs, request headers, or cookies | **Denied** |

Browser clicking can have side effects even on an owned tab; this cannot be made safe just from accessibility labels. Do not use click tools for purchases, login, submitting forms, or account changes. Browser access and media use must comply with applicable permissions and laws.

## Architecture

```text
ChatGPT
  |
  +-- VidOtter MCP (one private Tunnel)
        |
        +-- Go Bridge ---- existing download engine
        |      ^
        |      | local Native Messaging and authenticated loopback HTTP
        |      |
        +-- VidOtter Extension ---- captured media in session storage
        |
        +-- Browser Automation adapter (optional; bounded tool allowlist)
               |
               +-- Remote Browser MCP v0.3.0 (separate release, local stdio)
                      |
                      +-- Chrome DevTools MCP / Edge or Chrome
```

The optional browser component remains maintained and versioned independently. Updating or removing it must not modify VidOtter's downloads, user `Data/config.toml`, or other app profiles.

## Troubleshooting

- **No media resources:** Confirm that the updated extension is loaded and the video actually started playing. A site may provide only HTML, or block media; no resource is invented.
- **`extension_connected=false`:** Restart/reload the extension to reconnect Native Messaging. Check that the local Bridge is active.
- **Browser component missing:** Install it from Components; it is not required for manual playback.
- **Browser CDP unavailable:** Enable local remote debugging in a supported Edge/Chrome instance; grant browser consent. Component installation does not imply CDP authorization.
- **Browser click refused:** Enable interactions explicitly in the local Components panel. The AI cannot change this setting.
- **Browser automation request times out:** The adapter does not automatically restart or retry the CDP child. Failed sessions have a 30-second reconnect cooldown to avoid repeated browser authorization prompts. Check the existing Remote Browser Tool connection and browser consent before retrying manually.
- **`Unsupported URL` for webpage:** Do not submit the webpage to yt-dlp as if it were an m3u8; capture the actual media request in the extension first.
