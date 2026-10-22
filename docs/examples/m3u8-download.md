# Example: download a finite HLS video

Use a video page you control or are authorized to save. This scenario assumes a finite, non-DRM HLS video and working FFmpeg. It does not promise recording of an endless live stream.

## Pick the playlist

1. Open the page and start playback, then open VidOtter's Detect view.
2. Look for `.m3u8`. A master playlist lists quality variants; a media playlist lists segments. Either differs from an individual `.ts` or `.m4s` segment.
3. Select the intended video, excluding ads. If multiple qualities appear, check the URL/title or select the desired height in download settings.
4. Leave subtitles off for this example and submit once. If a webpage playlist dialog appears, select only the current video unless you want all entries. A webpage playlist (multiple videos) is different from an HLS manifest (segments/variants).

## Check the result

Open Downloads and observe downloading followed by any muxing. When completed, open the MP4 and check both audio and video, seek near the end and compare duration with the source. A playable first segment alone does not establish that the complete video was saved.

If requests expire, reopen/play the source and refresh detection/session before recovery. Missing FFmpeg is a tool issue; a 403 can be an expired or missing session; a segment failure is not fixed by selecting subtitles. See [troubleshooting](../how-to/troubleshoot.md) and [resume](../how-to/manage-downloads.md).
