# ChatGPT and MCP integration

[中文](../zh-CN/how-to/mcp.md) · [Bridge API](../reference/bridge-api.md)

VidOtter exposes **13 optional MCP tools** through the same desktop Go executable. The browser extension and MCP share the existing authenticated loopback Bridge and download queue; MCP does not ship a second downloader.

| MCP tool | Operation |
| --- | --- |
| `vidotter_status` | Check whether the Bridge is online and required tools are installed (does not launch the Bridge). |
| `vidotter_download` | Download direct MP4/M3U8 natively; for webpage URLs first use matching browser-captured HLS/MP4. `strategy=page` explicitly opts into yt-dlp resolution. |
| `vidotter_list_jobs` | List recent download states. |
| `vidotter_get_job` | Inspect one task and its progress. |
| `vidotter_control_job` | Pause, resume, or retry without providing browser credentials. |
| `vidotter_read_subtitle` | Read a subtitle already produced for a completed job, up to 256 KiB. |

| `vidotter_list_browser_media` | List extension-captured media using short-lived opaque IDs (never raw signed URLs or cookies). |
| `vidotter_download_browser_media` | Ask the extension to submit the selected ID using a freshly scoped browser session. |
| `vidotter_browser_status` | Check optional Browser Automation installation and permission states. |
| `vidotter_browser_open` | Open a public URL in a new background tab via optional Remote Browser Tool. |
| `vidotter_browser_snapshot` | Read the owned background page accessibility snapshot. |
| `vidotter_browser_click` | Click a control on an owned tab **only after local opt-in**. |
| `vidotter_browser_close` | Close only a component-owned background tab. |

For a webpage URL, the default strategy is **browser-first**: open the page in an Edge/Chrome background tab, start the normal player, then call `vidotter_download(url=...)`. VidOtter matches the **exact page URL** against the extension's short-lived capture index. Matching HLS/MP4 uses the existing native download queue with the browser's request context, captured only at submission time. It never silently retries an uncaptured webpage through yt-dlp. If no matching media was detected, play/reload the page and retry; choose `strategy=page` only when you explicitly want yt-dlp to extract a supported public webpage. MP4/M3U8 direct links work without a browser capture. `strategy=browser` explicitly requires a browser match.

When the Bridge restarts or its in-memory index expires, the extension republishes **only recent media metadata**, on reconnect and on demand, without exposing signed URLs, cookies, or request headers to MCP. Matching checks the full page identity (including query parameters) inside the Bridge, even though MCP listings redact the query. HLS is preferred to MP4 when both are captured; DASH-only or separated-track pages still need an explicitly chosen compatible page resolver.

Download submission returns a `task_id` immediately. Poll `vidotter_get_job` until completed/failed; the submission itself does **not** wait for video transfer.

## Setup helpers (optional)

After installing the official tunnel-client, Windows users can run `powershell -NoProfile -File scripts/Setup-VidOtter-MCP.ps1` and Linux/macOS users can run `bash scripts/setup-mcp.sh` from the extracted release directory. The setup asks for your Tunnel ID and restricted runtime key, saves them under private user storage (Windows DPAPI; Unix user-readable-only file), runs doctor, and configures an OS login-time tunnel service when supported. Re-running preserves the existing private profile; use only one active daemon per Tunnel. On Windows the current user startup shortcut starts the tunnel at login; Unix uses systemd user services or LaunchAgent. The helpers do **not** download tunnel-client or register a ChatGPT connector on your behalf.

Modern MCP 2026-07-28 clients can discover tools with a self-contained `server/discover` request, while older clients retain the legacy `initialize` protocol. Both are supported over the same stdio connection.

## Optional browser discovery and automation

**No second plugin and no second Tunnel:** the optional [Browser Automation component](browser-automation.md) installs the complete independently maintained Remote Browser MCP v0.3.0 release as a local stdio dependency. It does not copy or fork its code. VidOtter exposes only a restrictive subset of browser tools; arbitrary JavaScript, existing user tabs, raw network requests and cookie access are not exposed to AI. Clicking is disabled until the user explicitly enables it with a local confirmation.

The extension publishes only nonsecret media metadata to Bridge's ten-minute in-memory index, and responds to MCP download requests over a persistent Native Messaging channel. The extension itself fetches cookies only upon actually submitting a download, and the Bridge validates MCP-triggered destinations as public URLs.

### Validated Windows download sequence

When Browser Automation is enabled in **Settings → Components**, use the single VidOtter MCP connection:

```text
vidotter_browser_status
vidotter_browser_open(url="https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4")
vidotter_list_browser_media
vidotter_download_browser_media(resource_id=<opaque ID from matching MP4>, quality="best")
vidotter_get_job(id=<task ID returned by the download>)
vidotter_browser_close(page_id=<page ID returned by open>)
```

Each ID is ephemeral; read it from the corresponding response, never hard-code it. For a resource already detected in a browser tab, skip `browser_open` and use the media-list/download tools. This direct public MP4 example was confirmed on Windows; it is not a claim of universal HLS/DASH or website compatibility. See [the acceptance results](browser-automation.md#verified-single-mcp-workflow-windows).

## Local stdio

The installed binary already supports the new argument:

```sh
# Windows: bin/vidotter-windows-amd64.exe --mcp-stdio
# Linux:   bin/vidotter-linux-amd64 --mcp-stdio
# macOS:   bin/vidotter-macos-arm64 --mcp-stdio
```

Your MCP client starts the binary with `--mcp-stdio`. The CLI handles repeated sequential initialize requests without restarting the main Bridge. Only MCP protocol frames go to stdout.

## Optional ChatGPT Tunnel

1. Install the **official** [OpenAI tunnel-client](https://github.com/openai/tunnel-client) and select a runtime API key restricted to **Tunnels Read + Use**. Never commit or paste your key into a README.
2. Create a **private, per-user** Tunnel at [OpenAI Platform — Tunnels](https://platform.openai.com/settings/organization/tunnels). Runtime API keys are managed at [API Keys](https://platform.openai.com/settings/organization/api-keys).
3. From a terminal on the **same OS as the running VidOtter Bridge**, create your private profile (replace `tunnel_...` and the executable path):

   ```sh
   tunnel-client init --sample sample_mcp_stdio_local --profile vidotter \
     --tunnel-id tunnel_... \
     --mcp-command "/absolute/path/to/bin/vidotter-linux-amd64 --mcp-stdio" \
     --health-listen-addr 127.0.0.1:0
   ```

4. Supply `CONTROL_PLANE_API_KEY` through a secure local environment or credential store, then run `tunnel-client doctor --profile vidotter` and `tunnel-client run --profile vidotter`. The daemon must remain running.
5. In ChatGPT, choose **Plugins → Add custom MCP server → Tunnel** (wording may vary), select the Tunnel and explicitly authorize it. Refresh tool discovery and call `vidotter_status` twice. Local `doctor`, Tunnel health and ChatGPT end-to-end tool calls are separate checks.

The profile and runtime key belong in private OS storage, **not** this public Git repository. A Tunnel lets ChatGPT invoke local tools; it does **not** upload the downloaded video file automatically. When subtitles are requested through MCP, their text is sent to the AI service.

## Limitations and security

- MCP requires public HTTP(S) destinations, rejects local/private addresses and nonstandard ports at submission, and does not accept Cookie/authorization headers. Webpage URLs use browser-captured HLS/MP4 by default; optional yt-dlp is used only with explicit `strategy=page`.
- This submission-time validation is **not** a complete protection from DNS rebinding, redirects, or source-specific yt-dlp behavior. Do not share the private Tunnel with untrusted clients.
- Browser-login-dependent downloads and session refresh remain in the **Chrome/Edge extension**, not MCP. `needs-session` jobs cannot be resumed using AI tools.
- MCP never exposes the Bridge token, local absolute paths, raw source URLs or arbitrary filesystem reads in ordinary job responses. It does not offer deletion or modification of global settings.
- `vidotter_read_subtitle` can return personal text to the connected AI service; call it only on the user's request. It accepts only registered subtitle files belonging to completed jobs under the configured output directory.
- The local Bridge HTTP API must remain on loopback; never expose port `17891` to the internet. Tunnel only the restricted MCP stdio server.

See [Secure MCP Tunnels](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) for access and account requirements.
