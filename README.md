# VidOtter

[简体中文](README_ZH.md)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

Saving a video you are allowed to download can involve finding a temporary playback URL, downloading HLS segments, and merging them into a file.

VidOtter handles this through a browser extension and a local download service called the Bridge. It provides:

- MP4/HLS detection, download jobs, progress tracking, and access to downloaded files.
- A popup, side panel, and full-screen workspace sharing the same jobs, settings, and history.
- Optional subtitles, speech recognition, translation, and ChatGPT MCP.

The desktop application runs on Windows, macOS, and Linux with Chrome, Chromium, or Edge. Android is a separate client, described below.

![VidOtter Detect interface](assets/first-run-detect-zh.png)

Interface example; the installed version's layout may differ.

## 2. Quick Start

Suppose an MP4 plays in your browser and you want to save it locally, then check that the downloaded file plays.

Complete the workflow in this order:

1. Prepare the environment.
2. Install VidOtter.
3. Load the browser extension.
4. Play the video and download it.
5. Check the downloaded file.

### 2.1. Prepare the environment

Use a Windows, macOS, or Linux computer with Chrome, Chromium, or Edge.

Release users do not need Go, Node.js, an OpenAI account, or an API key. A first plain MP4 download needs no subtitle tools or models. HLS muxing and separate audio/video processing require FFmpeg. Check its availability in Settings and install or configure it if missing.

### 2.2. Install VidOtter

#### 2.2.1. Use the GUI installer

Open the [latest Release](https://github.com/ThinkerQAQ/vidotter/releases/latest). In Assets, choose an installer matching your operating system and CPU architecture. Available targets are listed in that Release; if no matching installer exists, use the archive method below.

- Windows: run the matching `.exe` installer.
- macOS: open the matching `.dmg` and launch the installer inside.
- Linux: download the matching `.run` installer. For a consistent command, rename the downloaded file to `VidOtter-Setup.run` in your file manager, then run these commands in the download directory:

```bash
chmod +x VidOtter-Setup.run
./VidOtter-Setup.run
```

Choose an installation directory, complete the installer, and keep a note of that directory. The installer registers the local service and opens the extension activation instructions and installation folder. The browser launches the Bridge on demand; you do not need to keep a terminal open.

**The GUI installer registers the local Bridge automatically. After a successful installation, continue directly to [2.3](#23-install-the-browser-extension); do not run the manual registration commands below.**

If your system displays an unrecognized-publisher warning, check the download source and that Release's signing/notarization information before proceeding. Complete installation before loading the extension.

#### 2.2.2. Install an archive manually

**Only archive-installation users follow this subsection.** If no matching installer exists, or you prefer a manual installation, choose an archive from the same Release's Assets. Its full name includes a version; identify the platform using the suffix:

- Windows x64: `windows-amd64.zip`.
- macOS Intel: `macos-amd64.tar.gz`.
- macOS Apple Silicon: `macos-arm64.tar.gz`.
- Linux x64: `linux-amd64.tar.gz`.
- Linux ARM64: `linux-arm64.tar.gz`.

These are filename suffixes, not complete filenames. Choose only an asset actually provided by that Release and matching your device.

To identify your architecture, check System type in Windows Settings → System → About, or Chip/Processor in About This Mac. On Linux, run `uname -m`: `x86_64` means x64 and `aarch64` means ARM64.

Extract the complete archive and find the folder directly containing `bin/` and `extension/`. This is the installation root; keep it at a permanent location. Open a terminal, change to that directory, then register the service. Replace the sample directory below with your actual extracted path.

Windows PowerShell:

```powershell
Set-Location 'C:\path\to\VidOtter'
.\bin\vidotterdev.exe install
```

Linux/macOS:

```bash
cd '/path/to/VidOtter'
./bin/vidotterdev install
```

The output should include `Native Messaging installed:` and `Host manifest:`, confirming service registration. If you move a manual installation, run this command again from its new location.

### 2.3. Install the browser extension

Next, connect the browser to the installed service:

1. Open `edge://extensions` in Edge or `chrome://extensions` in Chrome.
2. Enable **Developer mode** and select **Load unpacked**.
3. Select the installed VidOtter `extension/` folder, which directly contains `manifest.json`.
4. Open VidOtter from the browser toolbar's extensions menu; optionally pin it to the toolbar.
5. Go to **Settings → Connection & runtime** and confirm **Browser and download service connected**. If it cannot connect, follow the registration steps below and resolve the connection before downloading.

**Registration repair only if the connection fails:** if the Native Messaging host cannot be found, copy the 32-character extension ID from the extensions page and register it again from the installation root:

```powershell
# Windows: replace the placeholder with the actual extension ID
.\bin\vidotterdev.exe install --extension-id YOUR_32_CHARACTER_ID
```

On Linux/macOS, use `./bin/vidotterdev install --extension-id YOUR_32_CHARACTER_ID`. Reload VidOtter on the extensions page afterward.

### 2.4. Play the video and download it

Open the [MDN CC0 sample video](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4) and start playback.

Stay on that tab and open VidOtter:

1. Find the MP4 in **Detect**.
2. Select **No subtitles**.
3. Click **Download**.
4. Open **Downloads** and find the new job.

For HLS, select the complete playlist rather than an individual media segment.

### 2.5. Check the downloaded file

Wait for **Completed**, expand **Downloaded file** in the job, and click **Open** beside the file. Confirm that it plays. You can also open it directly in the default output directory, `Data/downloads/` under the installation root.

This verifies the complete path from browser detection through local job creation to a downloaded file.

For a larger interface, select **Open workspace** in the extension to open `http://127.0.0.1:17891/console/`. Workspace actions use the extension relay, so open it through the extension.

### 2.6. Troubleshooting

- **The extension cannot connect:** check the extension ID, repeat `install` above, and reload the extension. If it still fails, inspect `Data/logs/` and check whether another process occupies `127.0.0.1:17891`.
- **No video is detected:** play or seek in the original tab to trigger a fresh media request. Some websites expose no downloadable MP4/HLS or use DRM.
- **HTTP 403 or an expired URL:** replay the original page to refresh the media request, then follow the job's instructions to resume or download again.
- **HLS muxing fails:** check FFmpeg availability. For subtitle failures, select No subtitles first and verify the basic download.
- **An old version remains, or an action reports unsupported Tool operation:** stop the old Bridge, update the program files, reload the extension, and compare the version in **Settings → About**. Keep `Data/` intact.

If these checks do not resolve the problem, [open an Issue](https://github.com/ThinkerQAQ/vidotter/issues) with the VidOtter version, operating system, reproduction steps, and sanitized error information. Do not attach the complete `Data/` directory.

## 3. Configuration

No manual configuration is required for the first download. To change the output directory, interface, or tools, open **VidOtter → Settings** and save your changes.

Common settings:

- `download.output_dir`: output directory; defaults to `Data/downloads/`.
- `download.concurrent`: download concurrency; defaults to `12`, valid range `1–64`.
- `download.quality`: automatic by default; also accepts `best` or a source-supported height such as `720`.
- `proxy.enabled`, `proxy.url`: proxy disabled by default; HTTP proxies are supported, proxy credentials and SOCKS are not.
- `interface.language`: interface language; defaults to `zh-CN`, also accepts `en`.
- `interface.open_mode`: defaults to `popup`, also accepts `sidePanel`.

Settings are saved in `Data/config.toml` under the installation root. The settings page validates and writes a complete file. For manual edits, back it up, stop the Bridge, edit, and restart. Do not replace the file with a partial snippet.

**Upgrade and uninstall:** pause jobs and back up `Data/`. GUI users follow the matching Release's upgrade instructions using **VidOtter Maintenance**. If the installed version has no online update feed, remove the old program through Maintenance, then run the new installer. Manual users replace matching `bin/`, `extension/`, and packaged `tools/` directories, retain `Data/`, register again, and reload the extension. Program uninstall preserves data and downloads; back them up before manually deleting the installation folder.

To unregister a manual installation, run `.\bin\vidotterdev.exe uninstall` on Windows or `./bin/vidotterdev uninstall` on Linux/macOS, then remove the browser extension.

## 4. Optional Features

Choose these features for the relevant scenario. They are not first-download prerequisites.

### 4.1. Case 1: More formats or webpage downloads

Use the built-in downloader for MP4/HLS. Install yt-dlp through Settings when you need to resolve other supported webpages; configure FFmpeg for muxing and merging. DASH and separate audio/video support depend on the source and available tools.

### 4.2. Case 2: Subtitles or translation

Choose the workflow that matches the subtitle source:

- Site captions: requires yt-dlp and captions available from the source.
- Local speech recognition: requires FFmpeg, whisper.cpp, and a speech model.
- Subtitle translation: requires llama.cpp and a translation model; specify the spoken language when using speech recognition output.

Configure the required tools under **Settings → External components**. For local recognition or translation, select and download a model, or set the path to an existing one. Once the tools and models are ready, return to **Detect**, select a subtitle workflow, and create the download job. Check the resulting subtitle files, text, and timestamps.

Whether FFmpeg Slim, whisper.cpp, and llama.cpp are bundled depends on the Release. Check its notes and the status in Settings; install or configure missing tools as needed and download models separately.

### 4.3. Case 3: Manage downloads through ChatGPT

The desktop executable provides STDIO MCP through `--mcp-stdio`, reusing the local Bridge.

Complete Quick Start and confirm that the extension and Bridge are connected, then connect ChatGPT:

1. Create your Tunnel on OpenAI Platform.
2. Open **Settings → AI integrations → ChatGPT MCP**. Install the official Tunnel Client if needed, enter the Tunnel ID, and choose **Configure and start Tunnel**.
3. If no reusable credential is available, enter a Runtime API Key with Tunnels Read + Use permissions when prompted in the local terminal.
4. Create a private custom MCP plugin in ChatGPT, choose Tunnel as the connection type, and enter the same Tunnel ID.
5. Call `vidotter_status` to verify the connection. After submitting a download, use its returned job ID to check the result.

Tunnel Ready confirms the local connection; a real tool response in ChatGPT verifies the client connection. Credentials are handled in the local terminal, not sent through the extension.

### 4.4. Case 4: Browser interaction

Install the optional [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) component in Settings. VidOtter uses its independent release package and limits it to controlled background tabs. Interactive clicking requires explicit local opt-in.

If ChatGPT already has a Remote Browser Tool connection, reuse it for page actions and use VidOtter for media discovery and downloads. No second Tunnel is needed.

### 4.5. Case 5: Android

Android is an independent native client for Android 8.0 / API 26 and later. It downloads pasted/shared direct MP4/HLS URLs. It does not connect to desktop jobs, Native Messaging, yt-dlp, or ChatGPT Tunnel.

First check the [latest Release](https://github.com/ThinkerQAQ/vidotter/releases/latest) for an APK matching your device. If none is provided, build it from source:

1. Follow [5.1](#51-open-and-verify-the-project) to clone the repository and prepare Go. Run the following commands from the repository root.
2. Prepare the JDK, Android SDK Platform, NDK, and Platform-Tools required by the [Android build configuration](mobile/android/app/build.gradle.kts) and [build tool environment checks](tools/vidotterdev/internal/maint/mobile_media.go). Set `JAVA_HOME` and `ANDROID_SDK_ROOT` to their directories. To select a specific NDK version, set `ANDROID_NDK_HOME` to that version's directory.
3. Install both Go mobile tools at the version pinned in `bridge/go.mod`, then build the APK:

Linux/macOS:

```bash
vidotter_mobile_version="$(go -C bridge list -m -f '{{.Version}}' golang.org/x/mobile)"
go install "golang.org/x/mobile/cmd/gomobile@$vidotter_mobile_version"
go install "golang.org/x/mobile/cmd/gobind@$vidotter_mobile_version"
go -C tools/vidotterdev run . android-build
```

Windows PowerShell:

```powershell
$vidotterMobileVersion = go -C bridge list -m -f '{{.Version}}' golang.org/x/mobile
go install "golang.org/x/mobile/cmd/gomobile@$vidotterMobileVersion"
go install "golang.org/x/mobile/cmd/gobind@$vidotterMobileVersion"
go -C tools/vidotterdev run . android-build
```

`android-build` generates the Go binding and runs the bundled Gradle Wrapper. The resulting `.build/artifacts/android/vidotter-android-arm64-debug.apk` is for ARM64 devices. Transfer it to your device, permit installation from that source when prompted, open VidOtter, paste the sample MP4 URL above, and download it. Check the job and the downloaded file. This workflow requires separate Android-device acceptance.

## 5. Development

Development requires Git, Go, and Node.js. Use a Go toolchain satisfying [`bridge/go.mod`](bridge/go.mod) and [`tools/vidotterdev/go.mod`](tools/vidotterdev/go.mod), and the Node.js version in the [CI configuration](.github/workflows/ci.yml). Release users do not need these tools.

### 5.1. Open and verify the project

```bash
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` runs tests for both Go modules, extension JavaScript syntax checks, and script tests. Success prints `Go and browser checks passed`. `build` compiles the current platform's Bridge into `.build/out/` without installing it.

For example, cross-compile for Windows:

```bash
go -C tools/vidotterdev run . build --os windows --arch amd64
```

### 5.2. Install a development version

Run from the source repository root. These commands deploy into an isolated `.build/dev-install/` directory, then change to that directory and register the service.

Linux/macOS:

```bash
vidotter_dev_dir="$PWD/.build/dev-install"
go -C tools/vidotterdev run . deploy --target "$vidotter_dev_dir"
cd "$vidotter_dev_dir"
./bin/vidotterdev install
```

Windows PowerShell:

```powershell
$vidotterDevDir = Join-Path (Get-Location).Path '.build\dev-install'
go -C tools/vidotterdev run . deploy --target $vidotterDevDir
Set-Location $vidotterDevDir
.\bin\vidotterdev.exe install
```

Follow [2.3](#23-install-the-browser-extension) to load this deployment's `extension/`, then complete 2.4–2.5 to download and check a file. Change back to the original source repository root before further source work or packaging.

### 5.3. Packaging and CI

Use `tools/vidotterdev/` for builds, verification, and packaging. Formal packaging uses `go -C tools/vidotterdev run . package` and requires verified sidecar tools, license materials, and the pinned FFmpeg source archive. `--allow-missing-tools` is for incomplete development bundles only.

[CI](.github/workflows/ci.yml) runs tests, builds, and README link checks on Windows, macOS, and Linux. Real browser downloads, GUI installer interaction, and Android devices require separate acceptance testing. See [AGENTS.md](AGENTS.md) for contribution rules.

## 6. License

Original source uses [Apache-2.0](LICENSE); third-party components retain their own licenses. See [NOTICE](NOTICE) and [third-party notices](THIRD_PARTY_NOTICES.md). Windows FFmpeg Slim uses LGPL-2.1-or-later, with matching source supplied in the same Release. Qt Installer Framework uses GPL-3.0 with the Qt Company Exception. Model weights and downloaded media are not covered by the source license.

Download only media you are allowed to save. VidOtter does not bypass DRM, paywalls, or access controls and is not affiliated with content platforms. Refer to the matching Release for installer platform support, code-signing, and notarization status.

`Data/` may contain source information, checkpoints, and local authentication details; keep it out of public repositories. The extension handles browser sessions, the Bridge listens on loopback, and private MCP access should be granted only to trusted clients.
