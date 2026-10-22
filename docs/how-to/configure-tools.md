# Configure download tools and models

First complete a [download without subtitles](../tutorial/first-download.md). Then open **Settings → Download capabilities** and inspect each tool's status before enabling the feature that uses it.

| Tool | Used for | Release delivery |
| --- | --- | --- |
| Go downloader | Direct media and HLS fetching | Built into Bridge |
| FFmpeg | Muxing, track merging, audio preparation for ASR | Windows includes Slim; Linux/macOS use an external installation |
| yt-dlp | Supported webpage resolution and site subtitles | Optional download initiated in Settings |
| whisper.cpp | Local speech recognition | Windows includes CPU CLI and libraries; model is separate |
| llama.cpp | Local subtitle translation | Windows includes CPU server and libraries; model is separate |

## Set output and network options

1. Choose a writable output directory. The default is `Data/downloads/` under the installation.
2. Set download concurrency from 1 to 64 (default 12). This controls downloader concurrency, not a guarantee of that many simultaneous jobs.
3. Leave quality automatic initially, or choose `best` or a height such as `720`. Available qualities depend on the source; a requested height may fall back to another available variant.
4. Enable a proxy only when needed, enter its actual address, save, then test one task. Browser connectivity does not prove the Bridge or component downloader can use the same route.

## Locate executable files

Explicit paths in Settings take precedence. If one points to a missing file, correct or clear it; the Bridge does not silently ignore a broken override. Automatic discovery checks tools near the executable/installation, downloaded tools under `Data/tools/`, then PATH. whisper.cpp and llama.cpp keep their runtime libraries in their respective subdirectories.

On Linux/macOS, install FFmpeg using your system's package manager, or select an existing executable. If PATH changed after the Bridge started, restart it and refresh tool status. FFmpeg Slim is for the application's media workflows; it is not a full replacement for a general FFmpeg distribution.

## Install optional components

For yt-dlp, open its tool card and use the install action. Installation downloads an upstream release asset and verifies its checksum before activation. On failure, read the card error and check network/proxy settings; changing the media URL will not fix a component-download failure.

For whisper.cpp, choose a model in its tool details and download it. English-only models contain `.en` in the ID; choose a multilingual model for other languages. Model size and selected state are shown in the UI. The executable alone is insufficient.

For llama.cpp, select the translation model, review the displayed license and confirm acceptance before downloading. The current catalog contains a TranslateGemma 4B GGUF model of about 2.49 GB; allow additional memory for inference. Weight size is not a RAM requirement or a speed estimate. Model downloads are separate from program releases.

Verify that the selected executable and model are available, then follow [subtitles and translation](subtitles.md). Linux/macOS users must supply compatible local ASR/translation executables themselves if using those features.

Manual path changes are stored in [Data/config.toml](../reference/configuration.md). UI saves apply to subsequent work; editing TOML directly requires a Bridge restart.
