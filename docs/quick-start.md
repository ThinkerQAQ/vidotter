# Quick start

Use a packaged desktop release for this path. A source checkout requires the separate [development tutorial](tutorial/development.md).

1. Download the matching system/CPU archive from [Releases](https://github.com/ThinkerQAQ/vidotter/releases) and fully extract it to a permanent directory.
2. In that directory run `.\bin\vidotterdev.exe install` in Windows PowerShell, or `./bin/vidotterdev install` on Linux/macOS. Linux/macOS also need FFmpeg in PATH for muxing/merging.
3. Open `chrome://extensions` or `edge://extensions`, enable Developer mode, choose Load unpacked and select `extension/`.
4. Open VidOtter and check its Bridge connection in Settings. The browser starts Bridge on demand; no terminal needs to remain open.
5. Play a video, choose its complete MP4/HLS resource in Detect, leave subtitles at No subtitles and submit once. In Downloads wait for Completed and inspect the actual output.

If installation or connection is unclear, follow [first run](tutorial/first-run-on-new-machine.md). If resource selection is unclear, follow [first download](tutorial/first-download.md).

Next: [tools and models](how-to/configure-tools.md) · [subtitles](how-to/subtitles.md) · [workspace](how-to/use-web-ui.md) · [task recovery](how-to/manage-downloads.md) · [troubleshooting](how-to/troubleshoot.md).

> Future one-click GUI installers are being prepared for all three desktop platforms. Until they appear as tested release assets, use the archive steps above. [Installer build/status](how-to/build-installers.md).
