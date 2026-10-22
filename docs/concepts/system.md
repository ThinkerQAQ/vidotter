# Components and data

VidOtter has two runtime paths: a desktop extension/Bridge pair, and a native Android app. Installing the desktop Bridge does not add its tools to Android.

| Component | Responsibility | Data it owns |
| --- | --- | --- |
| Browser extension | Observe page media, collect scoped session context, submit tasks | Temporary tab detections and browser session access |
| Go Bridge | Authenticate local requests, manage jobs/settings, launch workers | `Data/config.toml`, jobs and runtime state |
| Worker | Resolve media, download, mux and process subtitles | Work checkpoints and output files |
| FFmpeg / yt-dlp / whisper.cpp / llama.cpp | Optional capabilities required by the selected workflow | Tool binaries and separately selected models |
| Android app | Accept a media URL, download with Go, mux with Android APIs | App state and exported media |

## Desktop UI

Popup, Side Panel and the full-page Web UI reuse the extension UI modules. Closing a popup is not the same as pausing a Bridge job. The Web UI loads at `http://127.0.0.1:17891/console/`; authenticated operations pass through the extension relay. A page that renders successfully may still lack a working relay.

Native Messaging uses host `com.vidotter.bridge`. Its generated JSON manifest tells the browser which executable and extension IDs are allowed. The HTTP token is delivered to the extension; it is not a user setting or a value to paste into the Web UI.

## Persistent state

For an installed executable under `bin/`, the data directory is its parent's `Data/`. Outside `bin/`, development binaries use a `Data/` directory beside the executable. The shell's current directory does not choose the data directory.

Jobs retain URLs and output metadata, but cookies and the recognized sensitive authorization headers are removed from snapshots. After a Bridge restart a protected task may need the original page session again. See [task recovery](../how-to/manage-downloads.md) and [configuration](../reference/configuration.md).
