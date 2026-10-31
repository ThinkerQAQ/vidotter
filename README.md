# VidOtter

[简体中文](README_ZH.md) · [Documentation](docs/index.md) · [Releases](https://github.com/ThinkerQAQ/vidotter/releases)

VidOtter detects media in Chrome/Edge and downloads it with a local Go Bridge. It supports HLS, MP4 and desktop resolution of supported webpage or split-track sources. Optional tools provide site subtitles, local speech recognition and subtitle translation. Android is a separate client for direct media links.

## Install and download

**FFmpeg license notice (Windows package):** The Windows download includes a separate **FFmpeg Slim** executable using FFmpeg code under **LGPL-2.1-or-later**; it is not licensed as VidOtter's original Apache-2.0 code. For the current [v1.4.1 release](https://github.com/ThinkerQAQ/vidotter/releases/tag/v1.4.1), the [exact corresponding FFmpeg source archive](https://github.com/ThinkerQAQ/vidotter/releases/download/v1.4.1/FFmpeg-n8.1.2-source.tar.gz) is supplied beside the binaries. The Windows archive includes `COPYING.LGPLv2.1` and `ffmpeg-slim-SOURCE.txt` for license and build/source details. Check the matching assets and notices for each version you download. See [third-party notices](THIRD_PARTY_NOTICES.md) and [FFmpeg's license guidance](https://ffmpeg.org/legal.html).

1. Download the archive for your system and CPU from [Releases](https://github.com/ThinkerQAQ/vidotter/releases). Extract the whole archive to a permanent folder.
2. In that folder run `.\bin\vidotterdev.exe install` in Windows PowerShell, or `./bin/vidotterdev install` on Linux/macOS. Linux/macOS need a separate FFmpeg installation in PATH for merging and muxing.
3. Open `chrome://extensions` or `edge://extensions`, enable **Developer mode**, choose **Load unpacked**, and select `extension/`.
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

The desktop Bridge includes six optional MCP tools for submitting authorized public downloads, checking tasks, controlling retries, and reading completed subtitles. MCP reuses the existing download engine and requires no separate business service. [Setup and security](docs/how-to/mcp.md). Browser session cookies remain exclusively in the extension.

## Documentation

- [Quick start](docs/quick-start.md) and [first download](docs/tutorial/first-download.md)
- [Tools and models](docs/how-to/configure-tools.md), [subtitles](docs/how-to/subtitles.md), [tasks and resume](docs/how-to/manage-downloads.md)
- [Upgrade or uninstall](docs/how-to/upgrade.md), [troubleshooting](docs/how-to/troubleshoot.md), [Android](docs/how-to/android.md)
- [Configuration and API reference](docs/reference/index.md), [architecture](docs/architecture/index.md)

## Legal Use and Third-Party Services

VidOtter is an independent media management tool, not affiliated with or endorsed by video websites. Use it only for content you are legally entitled to download, store or process, and comply with applicable law and source-site terms. A detectable media URL does not establish permission to download or redistribute the content. VidOtter does not bypass DRM or access-control mechanisms.

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
