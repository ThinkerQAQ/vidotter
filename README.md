# VidOtter

**English** · [简体中文](README_ZH.md) · [Releases](https://github.com/ThinkerQAQ/vidotter/releases) · [Issues](https://github.com/ThinkerQAQ/vidotter/issues)

## Introduction

VidOtter detects media playing in **Chrome or Edge** and downloads it through a **local Go Bridge**. It is for people who want to save video they are authorized to access without copying temporary media URLs or setting up a separate downloader for every website.

- **Everyday use:** detect MP4/HLS media in the current tab, choose a resource, follow the download job, and open the resulting file. DASH and separated audio/video may need additional tools and are not universally supported.
- **One workspace:** Popup, Side Panel, and the loopback Web UI share the same downloads, preferences, and task history.
- **Optional:** webpage resolution via yt-dlp; captions, local speech recognition, and subtitle translation; a private MCP connection for ChatGPT; restricted browser automation. None is needed for a first MP4 download.
- **Separate Android client:** paste/share supported direct media URLs; it is not a remote desktop Bridge or a replacement for the desktop extension.

VidOtter does **not** bypass DRM, paywalls, or access controls, and detecting a resource does not guarantee a successful download. The optional [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) integration is a separately maintained dependency, not part of VidOtter's core.

![VidOtter browser interface (older v1.3.0 capture)](assets/first-run-detect-zh.png)

*Older Chinese Detect screen for orientation; layouts and labels may differ in newer releases.*

## Quick Start

### 1. Prerequisites

| Platform | Official v1.6.2 archive | First-download requirements |
| --- | --- | --- |
| Windows | x64 ZIP | Edge or Chrome; no Go, Node.js, or administrator account required |
| Linux | x64 or ARM64 TAR.GZ | Chrome/Chromium or Edge; install system FFmpeg for HLS muxing or separated tracks |
| macOS | Intel or Apple Silicon TAR.GZ | Chrome/Chromium or Edge; install system FFmpeg for HLS muxing or separated tracks |

**No account, API key, speech model, or Tunnel is required** for ordinary browser downloads. The Windows release includes a limited FFmpeg Slim sidecar for VidOtter's media workflows; Linux/macOS archives do not include FFmpeg. For a simple, directly playable MP4, optional AI/media tools are unnecessary.

### 2. Download and register the Bridge

Download the archive matching your **OS and CPU** from [GitHub Releases](https://github.com/ThinkerQAQ/vidotter/releases/latest). Fully extract it to a **permanent** directory. Keep its `bin/` and `extension/` directories together. The current published release is **v1.6.2**; experimental Qt GUI installer builds are **not** published releases.

Open a terminal **in the extracted archive root**, then register Native Messaging:

**Windows — PowerShell**

```powershell
.\bin\vidotterdev.exe install
```

**Linux / macOS — Terminal**

```bash
./bin/vidotterdev install
```

**Check:** the command must print `Native Messaging installed:` and `Host manifest:`; the latter points under `Data/native-host/`. This registration is per user and does not keep a terminal or background server permanently running. The browser launches the Bridge when needed.

If you move the installation directory later, run `install` again from the **new** directory.

### 3. Activate the browser extension (required)

1. In Edge open `edge://extensions`; in Chrome open `chrome://extensions`.
2. Turn on **Developer mode** and click **Load unpacked**.
3. Select the **`extension/` directory**, containing `manifest.json`, in the extracted VidOtter folder. Do **not** select the ZIP or the project root.
4. Pin VidOtter to the toolbar and open it. In **Settings**, confirm that the Bridge/environment is healthy.

The extension is distributed locally, **not through a browser store**. Browser confirmation cannot be skipped. If the extension ID differs from the registered one, find its 32-character ID on the extensions page and run, from the same extracted root:

```powershell
# Windows; replace with the actual ID from Edge/Chrome
.\bin\vidotterdev.exe install --extension-id YOUR_32_CHARACTER_ID
```

On Linux/macOS use `./bin/vidotterdev install --extension-id YOUR_32_CHARACTER_ID`. After updating extension files, **Reload** VidOtter in the browser's extensions page; replacing Bridge binaries does not reload the browser's background worker.

### 4. Complete one download

1. Open a permitted video and **start playback**. For a public MP4 test, use the [MDN CC0 flower video](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4).
2. Open VidOtter **on the same tab**. Under **Detect / 资源嗅探**, choose a complete MP4 or HLS playlist rather than an individual media segment.
3. Choose **No subtitles** and click **Download** once.
4. Open **Downloads / 下载任务**, find the newly created job, and wait for **Completed / 已完成**. Use the output-file action to open it, or check the default `Data/downloads/` folder under your installation.

**Done means all three checks pass:** the extension can reach the Bridge, a download job exists, and its completed **media file can be opened**. Merely installing, seeing a green status, or detecting a URL is **not** a full download test. Actual browser playback acceptance must be performed on your machine.

Use **Open workspace** in the extension to open the full-screen view at `http://127.0.0.1:17891/console/`. It works through the extension relay; opening that loopback address alone does not authorize privileged actions.

### 5. Troubleshooting

| Symptom | What to check |
| --- | --- |
| **Native Messaging host not found** / unhealthy environment | Re-run `vidotterdev install` from the correct extracted root. Check the extension ID and press **Reload** on `edge://extensions` / `chrome://extensions`. |
| Extension interface changed, but actions say **unsupported Tool operation** | Reload the unpacked extension to restart its Manifest V3 background worker; then reopen Settings. Do not delete `Data/`. |
| Bridge never becomes ready | Check whether `127.0.0.1:17891` is already occupied; inspect `Data/logs/`. Identify the process before stopping it. |
| **No media detected** | Play or seek in the original tab. Some sites expose no downloadable MP4/HLS, or use DRM. Detection is not universal. |
| Job fails, HTTP 403, or media expired | Read the job error; return to the permitted page and trigger a **fresh** media request. Some tasks can resume the same ID; others require a new download. |
| HLS cannot merge, or subtitle processing fails | Check FFmpeg availability; local captions/translation also need optional executable(s) and model files. Select **No subtitles** first. |
| Upgrade seems unchanged | Stop the old Bridge, update **all** program files together, re-register the Native Host, reload the unpacked extension, then compare versions in **Settings → About**. |

**Upgrade / uninstall:** pause jobs, back up `Data/`, replace `bin/`, `extension/` and bundled `tools/` with files from one matching new release, **keep the original `Data/`**, then run `install` again and reload the extension. To unregister: run `.\bin\vidotterdev.exe uninstall` (Windows) or `./bin/vidotterdev uninstall` (Linux/macOS), then remove the extension from the browser. Uninstall **does not delete** `Data/` or downloaded media; remove the installation directory only after backing up what you need.

## Configuration

The Bridge creates `Data/config.toml` under the installed directory; **no fields need manual editing for a first download**. Prefer **VidOtter → Settings**: the UI validates inputs and writes a complete configuration file.

| Setting | Default | When to change |
| --- | --- | --- |
| `download.output_dir` | `Data/downloads/` | Choose a writable output directory |
| `download.concurrent` | `12` | Downloader concurrency, valid range 1–64 |
| `download.quality` | Automatic (`""`) | `best` or a supported resolution such as `720` |
| `proxy.enabled`, `proxy.url` | `false`, empty | Explicit HTTP proxy when required; proxy credentials and SOCKS are not supported |
| `interface.language` | `zh-CN` | `en` or `zh-CN`; this does not translate video or subtitles |
| `interface.open_mode` | `popup` | `popup` or `sidePanel` |

Other tool paths, sniffer switches, logging, and model selection are available in Settings. **Do not replace `config.toml` with a partial snippet**: a malformed or incomplete file can cause defaults to be used. For a manual change, back it up, stop Bridge, edit, restart, and verify with a new job.

`Data/bridge.json` can contain a local auth token; `Data/jobs.json` and `Data/work/` may contain sensitive source URLs or checkpoints. Never publish or share `Data/` verbatim. The Bridge HTTP listener must stay on loopback, not a public interface.

## Optional Features

These are **not** first-run prerequisites:

- **More formats and webpages:** enable **yt-dlp** through its tool card to resolve supported pages; FFmpeg handles muxing/merging. Windows v1.6.2 contains FFmpeg Slim, whisper.cpp and llama.cpp executables; **models are not bundled**. Linux/macOS users supply compatible local executables as needed.
- **Subtitles:** choose site captions (needs yt-dlp and a supported source), local transcription (FFmpeg + whisper.cpp + downloaded model), or translation (llama.cpp + separately licensed model). For translation from ASR, explicitly set the spoken language. Check subtitle files and timestamps, not just the video job.
- **Private MCP / ChatGPT:** the desktop executable exposes **13 MCP tools** via `--mcp-stdio`; the installed Bridge still handles ordinary downloads. A private ChatGPT Tunnel is **optional** and requires a user-owned Tunnel ID, official [tunnel-client](https://github.com/openai/tunnel-client), and a restricted runtime credential (Tunnels Read + Use). With the official client installed, run `powershell -NoProfile -File scripts/Setup-VidOtter-MCP.ps1` on Windows or `bash scripts/setup-mcp.sh` on Linux/macOS from the archive root. The helper configures a user-level login-time service, but **does not install the client or create the ChatGPT plugin**. Create a private custom MCP plugin using Connection: Tunnel and the same ID; verify `vidotter_status` from ChatGPT. Never put a runtime key in the README.
- **Browser automation:** optional [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) integration through VidOtter Settings. It operates in isolated background tabs; interactive clicks require explicit opt-in. Do not expose arbitrary JavaScript, private tabs, browser cookies, or a second public Tunnel. Windows public-MP4 smoke coverage is **not** proof of universal website support.
- **Android (source-built, separate):** supports pasted/shared direct MP4/HLS URLs on Android API 26+ (Android 8.0). It does **not** use desktop Native Messaging, desktop jobs, yt-dlp, or ChatGPT Tunnel. There is no Android APK in the current desktop Release. Building it requires JDK 17, Android SDK 35/NDK, gomobile, and the Gradle Wrapper: from a configured source checkout run `go -C tools/vidotterdev run . android-bind` followed by `go -C tools/vidotterdev run . android-build`. Device-level testing is separate.

## Development

Clone source only if you want to build or contribute; users of a published archive do not need developer tools. Current source requirements are **Go 1.25+** (the Bridge module specifies `go 1.25.0`), **Node.js** for extension tests (CI uses Node 22), and Git.

```bash
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` executes both Go modules' tests, JavaScript syntax checks, and extension test scripts. `build` writes the current OS/architecture Bridge to `.build/out/`; **it does not register Native Messaging or create a distributable release**. Cross-compile, for example:

```bash
go -C tools/vidotterdev run . build --os windows --arch amd64
```

For an isolated development installation, use `go -C tools/vidotterdev run . deploy --target /absolute/path/to/vidotter-dev` from source, then register Native Messaging **on the target OS** using its `bin/vidotterdev`. Do not overwrite a running Windows Bridge or an existing user's `Data/`.

Formal release packaging uses `go -C tools/vidotterdev run . package`; it requires matching Windows FFmpeg/whisper/llama sidecars, license/source notices, and the pinned FFmpeg source archive. `--allow-missing-tools` produces an **incomplete development bundle only** and must not be published as a complete Release. The Go maintainer also supports `licenses`, `tool-install`, `ffmpeg-native`, and Android build commands. CI builds and tests on **Windows, macOS and Linux**; this does not replace real browser downloads or Android-device validation.

Basic flow: **browser extension → Native Messaging → authenticated local Bridge → download worker → `Data/`**. Optional MCP connects to the same Bridge through stdio; it is not a second downloader. Contribution guidance: [AGENTS.md](AGENTS.md).

## License

VidOtter's **original source** is [Apache-2.0](LICENSE); [NOTICE](NOTICE) and [third-party notices](THIRD_PARTY_NOTICES.md) document other obligations. These terms **do not license downloaded media, user content, third-party websites, or separately distributed models**.

For example, the Windows distribution ships an independent **FFmpeg Slim** binary using FFmpeg under **LGPL-2.1-or-later**. The corresponding source for v1.6.2 is provided as [FFmpeg-n8.1.2-source.tar.gz](https://github.com/ThinkerQAQ/vidotter/releases/download/v1.6.2/FFmpeg-n8.1.2-source.tar.gz), with accompanying license/source files in that release. Observe the *matching release's* component notices; other models and runtimes have their own terms.

Use VidOtter only for media you are permitted to download. The project is not endorsed by source websites, provides no DRM bypass, and does not determine copyright ownership for you. Task history may contain signed URLs; browser session cookies remain in the extension and must not be put in public logs or repositories. Private Tunnel access can permit AI-side tool execution; grant it only to trusted clients and never expose the loopback Bridge port. GUI installer candidates and unreleased features in development PRs are **not** assumed available in the published v1.6.2 archives.
