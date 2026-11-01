# Troubleshoot a first installation

This guide follows the order that errors appear during [first run](../tutorial/first-run-on-new-machine.md). Do not change the download engine, models or proxy settings before confirming that the browser can talk to the Bridge.

## Native Messaging host not found

The browser extension connects through the registered native host `com.vidotter.bridge`, **not** by finding an arbitrary `vidotter` process.

1. Open the **extracted package root** and run `.\bin\vidotterdev.exe install` in Windows PowerShell or `./bin/vidotterdev install` on Linux/macOS.
2. Confirm that the installer prints the executable path and the host manifest path under `Data/native-host/`.
3. In `chrome://extensions` or `edge://extensions`, inspect VidOtter's extension ID. If it differs from `hfjpenjlamneepmigemmmiibebpokmik`, re-run `vidotterdev install --extension-id ACTUAL_EXTENSION_ID`.
4. Click **Reload** on the unpacked extension, then open it again. If the package was moved, re-run install from its **new** root.

## Bridge indicator remains Checking environment

Start by clicking the extension icon rather than opening the Web UI URL directly. The extension starts the Bridge on demand. Open **Settings → Connection & runtime** and refresh the health display.

Once the Bridge is started, you can check the local loopback endpoint at `http://127.0.0.1:17891/` from the **same computer**. A network response alone does not establish that the extension has Native Messaging permission or an authenticated relay. Do not expose this listener on a public address.

If there is a stale or incompatible process, stop it before reinstalling; do not delete `Data/`, which contains existing tasks and settings. Diagnostic logs are stored in `Data/logs/`.

The Bridge requires local port `127.0.0.1:17891`. If the port is already owned by another process, Native Messaging may be registered correctly but the Bridge cannot start. In PowerShell, run `Get-NetTCPConnection -LocalPort 17891 -State Listen` and inspect `OwningProcess`. Stop a process **only after identifying it**, then reopen the extension. When the Bridge successfully starts, `Data/logs/bridge.log` should contain `event=ready`.


## Detect shows No media found

- Return to the original webpage tab and **play** the permitted video; some websites do not request media until playback or seeking.
- Make sure you are looking at the same tab in which the media is playing.
- Look for full MP4, `.m3u8` or `.mpd` requests instead of individual segments.
- Pages with DRM, inaccessible cross-origin requests or proprietary delivery are not guaranteed to work.

An empty result is not the same as a failed download; no task has been submitted yet. Follow [First download](../tutorial/first-download.md) to select a resource.

## Task appears but fails

Open **Downloads** and inspect the failed stage and error. Check whether FFmpeg is available, whether the media URL/session has expired, whether a proxy is required, and whether the output directory is writable. The default **No subtitles** path does not need any local model; ASR requires whisper.cpp and a model, and translation requires llama.cpp and a model.

Do not press Download repeatedly for a timed-out task. Check whether a job already exists, then use the supported Retry/Resume action. Review logs in `Data/logs/` without posting credentials or signed media URLs.

## Temporary media expired or HTTP 403

For a browser-captured HLS/MP4 resource, a link may have a short-lived URL signature. VidOtter v1.6.2 recognizes explicit expiry timestamps where possible and avoids starting downloads with known-expired links. Unknown signature formats are **not** assumed to be expired.

If a browser-origin download fails while resolving an HLS playlist or probing an MP4 with HTTP 403, it may change to **Waiting for browser** instead of immediately failing. Return to the intended playback page, confirm you are authorized to access it, and play normally so the extension captures a **new media request**. The existing task resumes automatically with the **same task ID**; do not resubmit the expired URL or repeatedly retry the old resource.

HTTP 403 can also mean unavailable access rights, region restrictions or other server policy; VidOtter does not bypass these restrictions. If segment downloads have already started, VidOtter does **not** automatically mix a new media source into the existing checkpoint. The task may remain failed and require a separate authorized download.

Never share the entire `Data/` folder, `Data/work/`, raw browser requests or signed media URLs when reporting problems. Logs and screenshots should be reviewed and redacted before sharing.

## Workspace opens but cannot perform actions

The Web UI at `http://127.0.0.1:17891/console/` uses the browser extension as a relay for privileged requests. Open it with **Open workspace** in the extension. The page itself does not hold the Bridge token. Check extension permissions and connection state before treating a static page as a functioning downloader.


## A restored task needs browser authorization

Reopen the original page, sign in if needed, play the media and choose Reauthorize and resume from Downloads. Cookies and recognized sensitive authorization headers are not persisted across Bridge restarts. If the URL expired, detect the current resource. Do not edit `jobs.json` or copy tokens to recover a task. See [task management](manage-downloads.md).

## Subtitle executable exists but processing fails

The executable and model are separate dependencies. Check that the model is downloaded and selected; English-only Whisper models are not multilingual models. Translation also needs llama.cpp and its model. ASR followed by translation requires an explicit spoken language, not auto-detection. Follow [subtitle setup](subtitles.md).

## Manual TOML edits appear to reset settings

Stop Bridge and check TOML syntax and field completeness. Invalid configuration causes fallback to defaults without overwriting the bad file. Keep a backup, correct it using the [complete example](../reference/configuration.md), then restart. Do not erase all of `Data/`.

## Upgrade still shows an old version

Settings → About shows extension and Bridge versions separately. Reload the extension from the right directory, run the new root's installer to update registration, and ensure the old Bridge stopped. Replacing Bridge does not reload browser HTML, JS or locales. See [upgrade](upgrade.md).

## Report a reproducible issue

Include OS/architecture, browser, extension/Bridge versions, failed phase, reproduction steps and a redacted error. Add the relevant log job ID/events if useful; do not upload the entire `Data/` directory. Job snapshots can contain source URLs, and `bridge.json` contains authentication information. Identify whether installation, detection, download, muxing or subtitles failed.
