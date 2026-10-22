# VidOtter

[简体中文](README_ZH.md) · [Documentation](docs/index.md) · [Releases](https://github.com/ThinkerQAQ/vidotter/releases)

VidOtter detects media in Chrome/Edge and downloads it with a local Go Bridge. It supports HLS, MP4 and desktop resolution of supported webpage or split-track sources. Optional tools provide site subtitles, local speech recognition and subtitle translation. Android is a separate client for direct media links.

## Install and download

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

## Documentation

- [Quick start](docs/quick-start.md) and [first download](docs/tutorial/first-download.md)
- [Tools and models](docs/how-to/configure-tools.md), [subtitles](docs/how-to/subtitles.md), [tasks and resume](docs/how-to/manage-downloads.md)
- [Upgrade or uninstall](docs/how-to/upgrade.md), [troubleshooting](docs/how-to/troubleshoot.md), [Android](docs/how-to/android.md)
- [Configuration and API reference](docs/reference/index.md), [architecture](docs/architecture/index.md)

## Development

Requires Go 1.24 or later and Node.js for browser tests. From the repository root:

```sh
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

Build outputs go to `.build/out/`. This does not install the application. Follow the [development tutorial](docs/tutorial/development.md) and [maintenance guide](docs/architecture/maintenance.md) for deployment and release packaging. Agent instructions are in [AGENTS.md](AGENTS.md).

VidOtter code and documentation use [Apache-2.0](LICENSE). Third-party components have separate terms: [NOTICE](NOTICE), [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
