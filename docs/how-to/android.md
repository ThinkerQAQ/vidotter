# Use the Android app

Android is an independent native client, not a remote view of the desktop Bridge. The current build targets Android 8.0/API 26 or later and arm64. See the release's assets for APK availability; a desktop ZIP is not an Android package.

1. Install an APK built from this project on a compatible device.
2. Paste a direct MP4 or HLS URL in the app, or share a text link using Android's share menu. Sharing prefills the link; confirm it to start the download.
3. Keep the app's foreground download notification available and inspect its result. The app currently does not provide a desktop-style task list.
4. On Android 10 or later, completed media is published through MediaStore under `Downloads/VidOtter`. Older supported systems use the app-specific directory.

An ordinary webpage URL is not a media direct link. A compatible browser extension can detect the media and hand it off via `vidotter://download?...`; Android browsers do not use desktop Native Messaging. Desktop cookies, tools and task lists are not automatically shared with the app.

Android uses the Go download engine and Android MediaExtractor/MediaMuxer. Unsupported HLS formats or timestamps can fail muxing; the work directory is retained. The app does not currently integrate yt-dlp, local subtitle models, a split-track handoff protocol or the FFmpeg JNI muxer. In particular, desktop YouTube capabilities do not imply Android support.

Build requirements and commands: [Android developer notes](../../mobile/android/README.md).
