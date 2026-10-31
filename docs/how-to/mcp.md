# ChatGPT and MCP integration

[中文](../zh-CN/how-to/mcp.md) · [Bridge API](../reference/bridge-api.md)

VidOtter exposes **six optional MCP tools** through the same desktop Go executable. The browser extension and MCP share the existing authenticated loopback Bridge and download queue; MCP does not ship a second downloader.

| MCP tool | Operation |
| --- | --- |
| `vidotter_status` | Check whether the Bridge is online and required tools are installed (does not launch the Bridge). |
| `vidotter_download` | Enqueue an authorized public HTTP(S) media URL or supported webpage, optionally with subtitles. |
| `vidotter_list_jobs` | List recent download states. |
| `vidotter_get_job` | Inspect one task and its progress. |
| `vidotter_control_job` | Pause, resume, or retry without providing browser credentials. |
| `vidotter_read_subtitle` | Read a subtitle already produced for a completed job, up to 256 KiB. |

Download submission returns a `task_id` immediately. Poll `vidotter_get_job` until completed/failed; the submission itself does **not** wait for video transfer.

## Setup helpers (optional)

After installing the official tunnel-client, Windows users can run `powershell -NoProfile -File scripts/Setup-VidOtter-MCP.ps1` and Linux/macOS users can run `bash scripts/setup-mcp.sh` from the extracted release directory. The setup asks for your Tunnel ID and restricted runtime key, saves them under private user storage (Windows DPAPI; Unix user-readable-only file), runs doctor, and configures an OS login-time tunnel service when supported. Re-running preserves the existing private profile; use only one active daemon per Tunnel. On Windows the current user startup shortcut starts the tunnel at login; Unix uses systemd user services or LaunchAgent. The helpers do **not** download tunnel-client or register a ChatGPT connector on your behalf.

Modern MCP 2026-07-28 clients can discover tools with a self-contained `server/discover` request, while older clients retain the legacy `initialize` protocol. Both are supported over the same stdio connection.

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

- MCP requires direct public HTTP(S) destinations, rejects local/private addresses and nonstandard ports at submission, and does not accept Cookie/authorization headers. Public webpage resolution may require the optional yt-dlp component.
- This submission-time validation is **not** a complete protection from DNS rebinding, redirects, or source-specific yt-dlp behavior. Use only for user-authorized media and do not share the private Tunnel with untrusted clients.
- Browser-login-dependent downloads and session refresh remain in the **Chrome/Edge extension**, not MCP. `needs-session` jobs cannot be resumed using AI tools.
- MCP never exposes the Bridge token, local absolute paths, raw source URLs or arbitrary filesystem reads in ordinary job responses. It does not offer deletion or modification of global settings.
- `vidotter_read_subtitle` can return personal text to the connected AI service; call it only on the user's request. It accepts only registered subtitle files belonging to completed jobs under the configured output directory.
- The local Bridge HTTP API must remain on loopback; never expose port `17891` to the internet. Tunnel only the restricted MCP stdio server.

See [Secure MCP Tunnels](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) for access and account requirements.
