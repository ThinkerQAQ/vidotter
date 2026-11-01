# Manage downloads and recover interrupted jobs

Open **Downloads** in Popup, Side Panel or the workspace. These surfaces show the same Bridge jobs. Check status, phase, error and output files before taking an action.

| Status | Meaning | Next step |
| --- | --- | --- |
| `waiting_for_browser` | Page has not emitted playable media yet | Complete the required browser interaction and play media; the same job resumes automatically (20-minute limit) |
| `queued` | Submitted, awaiting worker activity | Wait and inspect runtime health if it stalls |
| `running` | Downloading or processing | Inspect phase; pause if needed |
| `paused` | Stopped with recoverable task state | Resume while the source remains available |
| `needs-session` | Restored task requires browser authorization again | Reopen the original page and use reauthorize/resume |
| `failed` | Worker or processing stage failed | Fix the reported cause, then retry |
| `completed` | Worker finished | Open/reveal outputs and check playback/subtitles |

## Pause, resume and retry

Pause stops an active worker. Resume starts a paused task; Retry restarts a failed task. The downloader reuses supported checkpoints, but recovery is not guaranteed for every source, expired link or external tool.

Bridge snapshots strip cookies and recognized sensitive authorization headers. On restart, interrupted public tasks return paused; tasks that require those credentials can become `needs-session`. Keep or reopen the original page, sign in if needed, play the media, then use **Reauthorize and resume**. If the source URL itself expired, detect the current resource again rather than repeatedly retrying an old URL.

Do not edit `jobs.json` while Bridge is running or remove `Data/work/` when you want to keep checkpoints.

## Output and deletion

Use the task's open/reveal action or the configured download directory. A playlist can have multiple output files; check the individual file list. A failed subtitle stage may leave a usable media file; inspect the output before downloading it again.

Deleting the task record keeps output files unless file deletion is explicitly selected. **Clear** removes completed and failed records; it is not a disk-cleanup command. Back up media you want to keep before choosing an action that deletes files.

See [upgrade](upgrade.md) for preserving jobs/settings when replacing program files, and [troubleshooting](troubleshoot.md) for persistent failures.
