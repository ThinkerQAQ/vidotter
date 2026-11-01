# ChatGPT and MCP integration

[中文](../zh-CN/how-to/mcp.md) · [Bridge API](../reference/bridge-api.md)

**MCP installation and activation:** Every desktop VidOtter release already contains the
MCP server inside its standard Go Bridge binary; there is no second server
package. The Chrome/Edge extension starts its local Bridge through Native
Messaging only when needed. Local MCP clients launch the same binary using
`--mcp-stdio` on demand. Nothing needs to run permanently for local downloads.

**Cloud ChatGPT connection is separate and opt-in.** Only users choosing
ChatGPT integration need their own private Tunnel ID, restricted API key and
the official tunnel-client. The provided setup helper can register the Tunnel
at user login, so normal use does not require manually starting it each time.
This service is never started or configured as a side effect of the regular
VidOtter installer. Never turn on remote access without explicit user choice.

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

For a webpage URL, the default strategy is **browser-first**: open the page in an Edge/Chrome background tab, start the normal player, then call `vidotter_download(url=...)`. VidOtter matches the **exact page URL** against the extension's short-lived capture index. Matching HLS/MP4 uses the existing native download queue with the browser's request context, captured only at submission time. It never silently retries an uncaptured webpage through yt-dlp. If no media is captured yet, the same call returns `status=waiting_for_browser` and a persistent `task_id`. Finish any required manual browser interaction and start playback; the extension will automatically resume the **same task ID** when it detects HLS/MP4. No second download request is needed. A one-time local browser notification is attempted while the extension is connected, and MCP returns `requires_user_action` with instructions. There is no automated verification or security challenge solving. After 20 minutes without matching media, the task fails with a timeout; submit a new task if needed. Choose `strategy=page` only when you explicitly want yt-dlp to extract a supported public webpage. MP4/M3U8 direct links work without a browser capture. `strategy=browser` explicitly requires a browser match.

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

### Configure and start ChatGPT Tunnel inside VidOtter Settings

1. Copy your Tunnel ID from [OpenAI Platform Tunnels](https://platform.openai.com/settings/organization/tunnels) and create a Restricted Runtime API Key with Tunnels Read + Use at [API Keys](https://platform.openai.com/settings/organization/api-keys). The ID alone does not authenticate the daemon.
2. Install official [tunnel-client](https://github.com/openai/tunnel-client/releases/latest) on the same host as VidOtter and place it on PATH or in the user .local/bin directory.
3. Open VidOtter Settings → AI integrations → ChatGPT MCP. Enter the Tunnel ID in the inline field and click Configure and start Tunnel. This action does not open an external browser page. Only the separate VidOtter documentation link opens the setup guide.
4. The authenticated local Bridge validates the ID and launches a fixed local setup script. Enter the Runtime API Key inside the system terminal, not into the extension. The helper initializes the profile, runs doctor, and arranges per-user login-time startup (Windows Startup shortcut, macOS LaunchAgent or Linux systemd user unit).
5. Add the same Tunnel as a custom MCP plugin in ChatGPT. Verify client health/ready and perform a real tool call.

If the Tunnel Client is absent, VidOtter reports the missing dependency in Settings without redirecting to other sites. An existing profile for a different Tunnel ID is not silently replaced. Local configuration presence is never proof of a healthy connection. See the [official Secure MCP Tunnel guide](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels).

## Streamlined ChatGPT onboarding (development branch)

**No generic plugin ZIP is required.** ChatGPT can create a private custom MCP plugin with Connection: Tunnel and discover VidOtter's 13 tools automatically. A ZIP cannot replace the user's private Tunnel association or consent. Secure MCP Tunnel is not eligible for public plugin-directory distribution.

Create a Tunnel ID once in OpenAI Platform. In VidOtter Settings → AI integrations → ChatGPT MCP, install the official tunnel-client when missing (the app downloads the matching official GitHub Release archive and verifies its published SHA-256 digest), enter the ID, and select Configure and start Tunnel. The local setup script reuses a valid credential already stored for this VidOtter user or explicitly provided in CONTROL_PLANE_API_KEY. Only first-time users without credentials are asked for the restricted Runtime API Key in a local terminal; other applications' secrets are never harvested.

For an existing profile, VidOtter exposes Start, Stop, and Restart actions. The official tunnel-client health check with a successful control-plane poll is required before the daemon is shown Ready; configuration alone never proves remote connectivity. Click Copy Tunnel ID and Open ChatGPT Plugins, then in ChatGPT choose Plugins → + → Add custom MCP server → Connection: Tunnel and select/paste the ID. Configure application authentication when applicable, review the warning and create the private plugin. Validate by calling vidotter_status from ChatGPT. The installer never creates or authorizes plugins silently.
