# From detection to a finished file

1. **Detect:** the extension observes page requests and media elements. Start playback before checking the resource list. A segment URL is not a complete video.
2. **Resolve:** the Bridge selects a direct resource or uses yt-dlp for supported webpage/track sources. Resolution can require browser authorization or a playlist choice.
3. **Download:** a worker fetches files or segments and records supported checkpoints under `Data/work/`.
4. **Mux:** FFmpeg joins or repackages desktop media where needed. Android uses its platform muxer and has a narrower format range.
5. **Subtitles:** the selected mode fetches site captions or runs local ASR; a target language adds translation.
6. **Output:** inspect the completed task's files, not just the progress percentage. Media may finish downloading while subtitle processing is still running.

Detection, task submission and completion are separate results. An expired URL can fail after successful detection; a missing model can fail a subtitle stage after media has downloaded.

Pausing retains supported work. Resuming is conditional on checkpoints and current source access. Deleting a task record and deleting output files are separate choices. See [task management](../how-to/manage-downloads.md).
