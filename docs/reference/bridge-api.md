# Bridge HTTP API

This internal desktop protocol serves the extension and Go workers. Base URL: `http://127.0.0.1:17891`. JSON operations use `Content-Type: application/json`. Privileged requests need `X-VidOtter-Token`, delivered to the extension through Native Messaging. Even ping needs authentication; a bare request returning 401 does not mean Bridge is absent.

Web UI calls traverse the extension's action allowlist. The HTTP page does not obtain the token. Worker handoff uses a separate one-use token under `/v1/worker/{token}`, not a browser integration entry point.

## Routes

| Method / path | Result or action |
| --- | --- |
| `GET /v1/ping` | Authenticated health: `{ok, pid}` |
| `GET /v1/environment` | Bridge platform/version and FFmpeg/yt-dlp health |
| `GET /v1/config` | Full configuration |
| `PUT /v1/config` | Validate and save full config |
| `GET /v1/preferences` | Interface and detection preferences |
| `PUT /v1/preferences` | Partial preference update |
| `GET /v1/tools` | Tool registry and model snapshots |
| `POST /v1/tools/yt-dlp/install` | Install yt-dlp |
| `POST /v1/tools/whisper/models/install` | Install/select ASR model |
| `POST /v1/tools/llama/models/install` | Install/select translation model; license acceptance required |
| `POST /v1/playlist/probe` | Inspect supported playlist |
| `POST /v1/tasks` | Submit; `202` with `{ok, taskId}` |
| `GET /v1/jobs` | Job snapshots |
| `GET /v1/jobs/{id}/subtitle?index=0` | Read registered subtitle of a completed job within output directory (max 256 KiB) |
| `POST /v1/jobs/clear` | Remove completed/failed records |
| `POST /v1/jobs/{id}/pause` | Pause active worker |
| `POST /v1/jobs/{id}/resume` | Resume with refreshed session if required |
| `POST /v1/jobs/{id}/retry` | Retry failed task |
| `POST /v1/jobs/{id}/delete` | Delete record; optional `deleteFiles` removes outputs |
| `POST /v1/jobs/{id}/open` | Open output |
| `POST /v1/jobs/{id}/reveal` | Reveal output in file manager |
| `POST /v1/open-output` | Open configured output directory |
| `GET /v1/logs?limit=100` | Recent events; at most 1000 |
| `DELETE /v1/logs` | Clear known runtime log files |
| `POST /v1/restart` | Restart when supported and no active jobs |
| `GET /console/` | Redirect to shared UI assets |

## Task body

Illustrative public direct-media task. `example.invalid` is a placeholder, not a working download:

```json
{
  "url": "https://example.invalid/video.mp4",
  "title": "Example video",
  "quality": "best",
  "playlist": "single",
  "resolvePage": false,
  "subtitles": { "mode": "none" }
}
```

`bridgeTask` also carries page context (`referer`, `origin`, `userAgent`, `pageUrl`, tab/frame IDs) and separate page/media headers and cookies. Acquire these through the existing extension flow; do not embed credentials in examples or logs.

`subtitles.mode` accepts `none`, `site`, `site-or-asr`, `asr`. Other fields are `languages`, `includeAutomatic`, `format` (`best`, `srt`, `vtt`, `ass`), `asrLanguage`, `targetLanguage`, `bilingual`. ASR plus translation requires explicit speech language; bilingual output requires a target language.

Full configuration PUT uses JSON camelCase fields such as `outputDir` and `ffmpegPath`, not TOML sections. Read current config and preserve the full object. Use preferences PUT for partial interface/detection changes.

## Responses and internal reports

Handlers commonly return `{ok:false,error}` with 400 for invalid input/state, 401 for authentication failure and 404 for missing routes/tasks. Some method errors have no JSON body. Restart can return 409 for active jobs or 503 when restart is unavailable.

Submission is not completion. Poll jobs and inspect status, phase and files; see [task states](../how-to/manage-downloads.md).

Worker-only `progress`, `output`, `files`, `state` actions under `/v1/jobs/{id}/` report execution results. They are not user controls. Field definitions are in `bridge/bridge.go`, `bridge/bridge_jobs.go`, `bridge/subtitle.go` and `bridge/bridge_preferences.go`. The relay contract is in `extension/background.js` and `extension/console-relay.js`; its actions differ from HTTP routes.
