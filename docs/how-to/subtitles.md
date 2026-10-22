# Download subtitles, transcribe and translate

Subtitles are chosen per download on **Detect**. Interface language is a separate preference under **Settings → Appearance & interaction**.

## Choose the subtitle source

| Detect option | Behavior | Dependencies |
| --- | --- | --- |
| No subtitles | Download media only | Tools needed by the media source |
| Site Chinese / Site English | Request matching site subtitles, including automatic captions | yt-dlp and a supported source |
| All site subtitles | Request available site languages | yt-dlp and a supported source |
| Site first, local recognition if unavailable | Try site captions, then transcribe when none are found | yt-dlp; FFmpeg, whisper.cpp and a model for fallback |
| Always use local speech recognition | Transcribe the downloaded media | FFmpeg, whisper.cpp and a model |

A site-only selection does not silently turn on ASR. Not every direct media URL exposes site captions; the source page must be resolvable for site subtitle retrieval.

## Generate local subtitles

1. In [tool settings](configure-tools.md), confirm FFmpeg, whisper.cpp and the selected model are available.
2. Return to **Detect**, select local recognition or the site-first fallback.
3. Select the video's spoken language, or automatic detection if no translation is requested.
4. Leave output language unchanged for transcription only. Submit the download.
5. In **Downloads**, inspect the audio/recognition/subtitle stages and the generated subtitle file alongside the media.

## Translate or produce bilingual SRT

1. Confirm llama.cpp and its selected translation model are available.
2. Choose a subtitle source, then choose an **Output language** such as Simplified Chinese or English.
3. For any ASR mode, explicitly select **Video speech**. Automatic speech-language detection is rejected when translation is requested.
4. Choose translated-only or bilingual output. Bilingual output keeps the original text and adds the translation on the following line.
5. Wait for model loading and translation to finish; media download completion alone does not mean subtitles are finished.

Generated translation/bilingual subtitles use SRT. Site originals can use the source's subtitle format. Check several timestamps and lines before relying on recognition or translation quality.

If a stage fails, read its error first: missing executable, missing model, model download, source captions and translation are different failures. Use [troubleshooting](troubleshoot.md); avoid resubmitting the same download repeatedly.
