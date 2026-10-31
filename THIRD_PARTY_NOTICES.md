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

## golang.org/x/mobile

The Android bridge uses `golang.org/x/mobile`, licensed under the BSD 3-Clause License.

Upstream license: <https://github.com/golang/mobile/blob/master/LICENSE>

## Gradle Wrapper and JUnit

The Android project includes the Gradle Wrapper under Apache-2.0 and uses JUnit 4.13.2 for tests under Eclipse Public License 1.0. These tools do not change the license of VidOtter's original code.

## Go executable dependencies

`vidotterdev licenses` collects full upstream license artifacts and a versioned dependency report for every desktop target using pinned go-licenses v2.0.1. Formal packaging includes them in `THIRD_PARTY_LICENSES/` alongside the Go runtime license; a collection failure stops packaging. FFmpeg, Whisper, Llama and LLVM OpenMP remain separately licensed sidecars with their own full license and source/build notices.

Go 维护入口按桌面目标平台生成完整许可及版本清单，并随包放入 THIRD_PARTY_LICENSES；收集失败停止打包。正式 Windows 包同时要求 FFmpeg、Whisper、Llama、LLVM OpenMP 的许可与来源文件齐全。
