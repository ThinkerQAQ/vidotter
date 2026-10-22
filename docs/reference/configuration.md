# Desktop configuration reference

The Bridge reads `Data/config.toml` beside the installed `bin/` directory. Prefer UI settings: they are validated and persisted. Direct file edits require a Bridge restart. Existing workers can retain their task configuration snapshot; verify changes with a new task.

## Fields and defaults

| Field | Default | Values or effect |
| --- | --- | --- |
| `download.output_dir` | Installation `Data/downloads/` | Non-empty output path; choose a writable absolute path |
| `download.concurrent` | `12` | Integer 1–64, downloader concurrency |
| `download.quality` | `""` | Default selection, `best`, or positive height such as `720` / `720p` |
| `proxy.enabled` | `false` | Enable explicit HTTP proxy |
| `proxy.url` | `""` | HTTP proxy URL, normalized with host/port |
| `proxy.host`, `proxy.port` | `""`, `0` | Host only and integer port 1–65535 when configured |
| `tools.ffmpeg`, `tools.yt_dlp` | `""` | Executable overrides; blank enables discovery |
| `tools.whisper`, `tools.llama` | `""` | whisper-cli / llama-server executable paths |
| `tools.whisper_model`, `tools.translation_model` | `""` | Local model file paths |
| `interface.open_mode` | `popup` | `popup` or `sidePanel` |
| `interface.language` | `zh-CN` | `zh-CN` or `en` |
| `interface.show_logs` | `true` | Show Logs navigation; separate from file logging |
| `sniffer.hls`, `sniffer.dash`, `sniffer.mp4` | All `true` | At least one must remain enabled |
| `logging.enabled` | `true` | Write structured event logs |
| `logging.max_size_mb` | `10` | Integer 1–100 per active component log |

The proxy supports HTTP, with CONNECT for HTTPS destinations. SOCKS and URLs containing proxy credentials are not supported. Enter host/port together; the host field excludes scheme and port. Correct conflicting URL and host/port fields through the UI.

## Complete example

Replace the output path. For Windows use a TOML literal string such as `'C:\Users\YourName\Downloads\VidOtter'`; single quotes preserve backslashes.

```toml
[download]
output_dir = '/home/yourname/Downloads/VidOtter'
concurrent = 12
quality = ""

[proxy]
enabled = false
url = ""
host = ""
port = 0

[tools]
ffmpeg = ""
yt_dlp = ""
whisper = ""
whisper_model = ""
llama = ""
translation_model = ""

[interface]
open_mode = "popup"
language = "zh-CN"
show_logs = true

[sniffer]
hls = true
dash = true
mp4 = true

[logging]
enabled = true
max_size_mb = 10
```

Do not overwrite a working config with a partial snippet. The loader decodes and validates the whole file; invalid TOML or values cause fallback to defaults without overwriting that file. Missing fields can invalidate a hand-written partial file. Back up the original before editing.

## Runtime files

| Under `Data/` | Purpose |
| --- | --- |
| `config.toml` | Editable settings |
| `bridge.json` | Runtime PID, endpoint and token; do not share |
| `jobs.json` | Task snapshots including URLs and output metadata; not a sanitized export |
| `native-host/com.vidotter.bridge.json` | Generated browser registration |
| `models/`, `tools/` | Downloaded weights and optional components |
| `work/` | Checkpoints and processing files |
| `logs/` | Structured Bridge/Worker events and rotations |
| `downloads/` | Default output directory |

The structured event sink accepts fixed fields such as job ID, phase, counts and elapsed time, excluding URLs, cookies, tokens, titles, paths, subtitles and raw errors. Other runtime files and manually captured console output may contain sensitive information. Logs rotate to `.1`; the UI reads a bounded recent tail rather than all history.

Definitions: `bridge/bridge_config.go`, `bridge/runtime_layout.go`, `bridge/bridge_job_store.go`. See [tools](../how-to/configure-tools.md) and [upgrades](../how-to/upgrade.md).
