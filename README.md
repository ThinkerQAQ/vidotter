# VidOtter

[简体中文](README_ZH.md) · [Documentation](docs/index.md) · [Releases](https://github.com/ThinkerQAQ/vidotter/releases)

VidOtter detects media in Chrome/Edge and downloads it with a local Go Bridge. It supports HLS, MP4 and desktop resolution of supported webpage or split-track sources. Optional tools provide site subtitles, local speech recognition and subtitle translation. Android is a separate client for direct media links.

## Install and download

**Installer status:** Cross-platform GUI installer packaging is under development, but no verified installer assets have been published. The archive instructions below are the supported installation path. [Build/status](docs/how-to/build-installers.md).



**FFmpeg license notice (Windows package):** The Windows download includes a separate **FFmpeg Slim** executable using FFmpeg code under **LGPL-2.1-or-later**; it is not licensed as VidOtter's original Apache-2.0 code. For the current [v1.6.2 release](https://github.com/ThinkerQAQ/vidotter/releases/tag/v1.6.2), the [exact corresponding FFmpeg source archive](https://github.com/ThinkerQAQ/vidotter/releases/download/v1.6.2/FFmpeg-n8.1.2-source.tar.gz) is supplied beside the binaries. The Windows archive includes `COPYING.LGPLv2.1` and `ffmpeg-slim-SOURCE.txt` for license and build/source details. Check the matching assets and notices for each version you download. See [third-party notices](THIRD_PARTY_NOTICES.md) and [FFmpeg's license guidance](https://ffmpeg.org/legal.html).

1. Download the archive for your system and CPU from [Releases](https://github.com/ThinkerQAQ/vidotter/releases). Extract the whole archive to a permanent folder.
2. In that folder run `.\bin\vidotterdev.exe install` in Windows PowerShell, or `./bin/vidotterdev install` on Linux/macOS. Linux/macOS need a separate FFmpeg installation in PATH for merging and muxing.
3. **Required browser extension activation (no store listing needed):** Open `edge://extensions` or `chrome://extensions`, enable **Developer mode**, click **Load unpacked**, and select the installed `extension/` directory containing `manifest.json`. The new GUI installer opens the browser extensions page, the correct directory, and an offline guide (`docs/extension-install.html`) automatically when you finish; you can reopen it from **VidOtter → Install Browser Extension** in the Windows Start Menu.
4. Play a video, open VidOtter, check the Bridge status, and select the intended resource in **Detect**. For the first download choose **No subtitles**.
5. Open **Downloads**, wait for **Completed**, then check the output file.

The browser starts the Bridge through Native Messaging. A packaged release needs no Go or Node.js installation. See [first-run setup](docs/tutorial/first-run-on-new-machine.md) for extension IDs and connection checks.

## Capabilities

| Feature | Desktop | Android |
| --- | --- | --- |
| Detect media in a browser tab | Chrome/Edge extension | Requires a compatible browser extension; otherwise paste/share a direct URL |
| Direct MP4/HLS download | Go downloader | Shared Go downloader |
| Webpage resolution and split audio/video | Optional yt-dlp and FFmpeg, source dependent | Not generally supported |
| Site subtitles | Optional yt-dlp | Not integrated |
| Local ASR and translation | Windows release includes executables; models downloaded separately | Not integrated |
| Full-page workspace | Extension relay + loopback Web UI | Native app |

Detection does not guarantee a source can be downloaded. DRM is not bypassed. Linux/macOS releases do not bundle FFmpeg or local ASR/translation sidecars.

## Interface

![VidOtter Detect screen](docs/images/first-run-detect-zh.png)

Chinese Detect screen from v1.3.0, shown before media detection. The screenshot illustrates navigation; follow the tutorial to check your own connection and output.

Popup, Side Panel and Web UI share tasks and settings. Use **Open workspace** in the extension for the full-page UI. Settings and runtime data live in `Data/` beside `bin/`; preserve that directory during upgrades.

## Optional AI / MCP integration

The updated extension provides **Settings → AI integrations → ChatGPT MCP** with an inline Tunnel ID input and **Configure and start Tunnel** action. A restricted Runtime API Key with Tunnels Read + Use is also required, entered in the local terminal and never sent via extension messages. The button does not redirect to another website; use the separate [VidOtter setup documentation](docs/how-to/mcp.md) link for official client installation and detailed guidance.


The desktop Bridge includes 13 optional MCP tools for submitting public media downloads, checking tasks, controlling retries, and reading completed subtitles. MCP reuses the existing download engine and requires no separate business service. [Setup and security](docs/how-to/mcp.md). The browser extension can now share short-lived opaque media IDs with MCP, and an opt-in Remote Browser Tool component can open isolated background tabs through the same VidOtter Tunnel. Browser session cookies remain exclusively in the extension. [Browser automation setup](docs/how-to/browser-automation.md).

The optional Browser Automation component now has a **Windows-verified single-MCP path**: open an isolated background tab, discover a public MP4 with the extension, submit by opaque ID, poll to completion, and close the tab. The smoke test used only public MDN CC0 media; HLS/DASH transfer and browser autoplay are not part of this acceptance. See [tested workflow](docs/how-to/browser-automation.md#verified-single-mcp-workflow-windows).

## Documentation

- [Quick start](docs/quick-start.md) and [first download](docs/tutorial/first-download.md)
- [Tools and models](docs/how-to/configure-tools.md), [subtitles](docs/how-to/subtitles.md), [tasks and resume](docs/how-to/manage-downloads.md)
- [Upgrade or uninstall](docs/how-to/upgrade.md), [troubleshooting](docs/how-to/troubleshoot.md), [Android](docs/how-to/android.md)
- [Configuration and API reference](docs/reference/index.md), [architecture](docs/architecture/index.md)

## Legal Use and Third-Party Services

VidOtter is an independent media management tool, not affiliated with or endorsed by video websites. It does not validate the copyright or license status of media, and does not filter downloads based on copyright classification. Users are responsible for complying with applicable laws, source-site terms, and third-party rights when downloading, storing, or distributing content. VidOtter does not bypass DRM or access-control mechanisms.

The [Apache-2.0 License](LICENSE) covers VidOtter's original code, not third-party media or packaged dependencies. See [third-party notices](THIRD_PARTY_NOTICES.md) for separate component terms.

**Local privacy:** Download task history may retain source or page URLs, including URL query parameters that act as access tokens, in the local `Data/` directory. Cookies and standard authorization headers are excluded from persisted jobs, but do not upload or share task-history files, logs or backups without reviewing and removing sensitive information.

## Development

Requires Go 1.24 or later and Node.js for browser tests. From the repository root:

```sh
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

Build outputs go to `.build/out/`. This does not install the application. Follow the [development tutorial](docs/tutorial/development.md) and [maintenance guide](docs/architecture/maintenance.md) for deployment and release packaging. Agent instructions are in [AGENTS.md](AGENTS.md).

VidOtter code and documentation use [Apache-2.0](LICENSE). Third-party components have separate terms: [NOTICE](NOTICE), [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

**Uninstall (Windows installer):** New installer candidates provide **Start → VidOtter → Uninstall VidOtter**, or launch VidOtter-Maintenance.exe from the installed directory and choose **Remove all components**. The Qt maintenance tool removes managed program files and Native Messaging registration while preserving local Data/. Older installer candidates may not have the Start Menu shortcut. VidOtter-Maintenance.exe.new is not the ordinary uninstaller.

**Simplified ChatGPT setup:** Create your Tunnel ID once, then use VidOtter Settings to install the verified official client when needed, configure the local runtime and copy the ID into ChatGPT Plugins → Add custom MCP server → Tunnel. No plugin ZIP or per-tool schema upload is needed. See docs/how-to/mcp.md.

**Reload an unpacked extension after upgrading:** If the MCP install button reports an unsupported tool action, Edge/Chrome is probably still running the previous MV3 service worker while showing a new popup. Use the new **Reload VidOtter extension** prompt, or reload VidOtter in `edge://extensions` / `chrome://extensions` and reopen Settings. Do not delete local Data/ or recreate your Tunnel.
