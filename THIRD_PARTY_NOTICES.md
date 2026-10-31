# Third-party notices

The Apache-2.0 license in `LICENSE` covers VidOtter's original source code and documentation. It does not replace the licenses of the components listed below.

## FFmpeg

Desktop release packages may include a separate FFmpeg Slim executable built from FFmpeg 8.1.2. The current build disables GPL and nonfree components and reports `LGPL-2.1-or-later`.

Each release that bundles FFmpeg must also include:

- the LGPL 2.1 license text;
- the exact corresponding FFmpeg source archive;
- the build configuration and any VidOtter-specific changes;
- a source and license notice next to the download.

FFmpeg source and license information: <https://ffmpeg.org/legal.html>

## yt-dlp

yt-dlp is an optional external tool. VidOtter does not bundle it in standard releases; the user can request an authenticated download from the official yt-dlp release page. The upstream source is published under the Unlicense, while some packaged executables include components under additional licenses, including GPL-3.0-or-later for PyInstaller-based executables.

Upstream project and licensing information: <https://github.com/yt-dlp/yt-dlp>

## whisper.cpp

Windows desktop release packages include the separate CPU build of `whisper-cli` and its runtime libraries from whisper.cpp. The client is licensed under the MIT License. Speech-recognition model files are not bundled by VidOtter and retain the terms published by their respective distributors.

Upstream project and licensing information: <https://github.com/ggml-org/whisper.cpp>

## llama.cpp

Windows desktop release packages include the separate CPU build of `llama-server` and its runtime libraries from llama.cpp. llama.cpp is licensed under the MIT License; the bundled LLVM OpenMP runtime retains its upstream license. Translation model files are not bundled by VidOtter and retain their own terms, including the Gemma license where applicable.

Upstream project and licensing information: <https://github.com/ggml-org/llama.cpp>

## Model Context Protocol Go SDK

The optional AI integration uses `github.com/modelcontextprotocol/go-sdk` under Apache-2.0. Its transitive Go dependencies are included in the generated per-platform `THIRD_PARTY_LICENSES/` directory of formal desktop releases.

Upstream: <https://github.com/modelcontextprotocol/go-sdk>

## Optional Remote Browser MCP and Node.js runtime

VidOtter can install the separately maintained [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) **v0.3.0** from its verified GitHub Release. This optional external component remains Apache-2.0, and its own LICENSE and THIRD_PARTY_NOTICES are preserved. It is installed on demand into the user's local Data directory and is not embedded in the VidOtter source repository or desktop Release.

The optional setup installs npm dependencies from the pinned upstream package-lock using integrity checks and with npm lifecycle scripts disabled. The upstream project's own dependency notices apply. When no compatible Node.js is installed, VidOtter may bootstrap the official [Node.js](https://nodejs.org/en/about/previous-releases) **v24.22.2 LTS** binary from nodejs.org, SHA-256 verified against the corresponding official SHASUMS256.txt. Node.js is MIT-licensed with bundled third-party dependency notices; its LICENSE is preserved locally. The official npm package and its license files are retained.

Neither dependency grants rights to download third-party audiovisual content, bypass DRM, nor control websites without the user's authorization.

## golang.org/x/mobile

The Android bridge uses `golang.org/x/mobile`, licensed under the BSD 3-Clause License.

Upstream license: <https://github.com/golang/mobile/blob/master/LICENSE>

## Gradle Wrapper and JUnit

The Android project includes the Gradle Wrapper under Apache-2.0 and uses JUnit 4.13.2 for tests under Eclipse Public License 1.0. These tools do not change the license of VidOtter's original code.

## Go executable dependencies

`vidotterdev licenses` collects full upstream license artifacts and a versioned dependency report for every desktop target using pinned go-licenses v2.0.1. Formal packaging includes them in `THIRD_PARTY_LICENSES/` alongside the Go runtime license; a collection failure stops packaging. FFmpeg, Whisper, Llama and LLVM OpenMP remain separately licensed sidecars with their own full license and source/build notices.

Go 维护入口按桌面目标平台生成完整许可及版本清单，并随包放入 THIRD_PARTY_LICENSES；收集失败停止打包。正式 Windows 包同时要求 FFmpeg、Whisper、Llama、LLVM OpenMP 的许可与来源文件齐全。
