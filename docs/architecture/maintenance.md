# Go maintenance and development tools

VidOtter uses one native Go maintainer for builds, checks, release packaging, installation and platform-tool integration: `tools/vidotterdev/`. The application does not require a separate development gateway or project SDK.

## Responsibility boundaries

| Directory | Responsibility |
| --- | --- |
| `bridge/` | Desktop downloader, authenticated loopback API, Native Messaging host and Android Go bindings |
| `extension/` | Manifest V3 browser integration, Popup, Side Panel and local Web UI relay |
| `mobile/android/` | Kotlin app and upstream Gradle Wrapper |
| `tools/vidotterdev/` | Build, verify, package, deploy, component management, Android and FFmpeg orchestration |
| `docs/`, `docs/zh-CN/` | English docs and aligned Chinese translations |
| `.build/out/`, `.build/artifacts/`, `.build/cache/` | Ignored, reproducible build outputs and caches |
| Installed `bin/`, `extension/`, `tools/`, `Data/` | Program files and persistent user data (never reset `Data/` during deploy) |

`bridge/` remains a separate Go module because its import and gomobile paths are already part of the product's toolchain.

## Native commands

Run at the checkout root:

```sh
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
go -C tools/vidotterdev run . build --os windows --arch amd64
go -C tools/vidotterdev run . package
go -C tools/vidotterdev run . components --root /path/to/installed/vidotter
go -C tools/vidotterdev run . tool-install whisper
go -C tools/vidotterdev run . tool-install llama
go -C tools/vidotterdev run . android-bind
go -C tools/vidotterdev run . android-build
go -C tools/vidotterdev run . ffmpeg-native --source /path/to/FFmpeg
go -C tools/vidotterdev run . ffmpeg-android --source /path/to/FFmpeg
go -C tools/vidotterdev run . icons
```

`verify` runs tests for both Go modules, JavaScript syntax checks and browser-extension test scripts. It does **not** establish real browser, Android or Windows installation acceptance.

Default build output: `.build/out/`. Release archives: `.build/artifacts/release/v<version>/`. Android artifacts: `.build/artifacts/android/`. Generated assets and caches are ignored by Git.

## Language servers for maintainers and agents

- **Go:** use `gopls serve` through an LSP client, with `bridge/` and `tools/vidotterdev/` as separate Go modules. Use symbols/definitions/references and diagnostics for targeted changes.
- **Browser code:** use installed editor language-server support where available. Otherwise use `rg`, source inspection, `node --check`, and browser tests.
- **Markdown:** `marksman server` provides LSP navigation and link references. Review prose and tables directly; a heading outline is insufficient for full-document review.

The language servers must be launched by an LSP client; CLI shells and plain text searches are valid fallbacks. No custom tool gateway is required.

## Release and deployment

`package` builds Windows amd64, Linux amd64/arm64 and macOS amd64/arm64 archives, checksums and `components.json`. The complete Windows release requires appropriately licensed FFmpeg, whisper.cpp and llama.cpp sidecars under `dist/tools/`; missing components fail packaging. `--allow-missing-tools` is only for deliberately incomplete development bundles, not published releases.

For Windows deployment from WSL, stop the Bridge before running:

```sh
go -C tools/vidotterdev run . deploy --os windows --arch amd64 --target /mnt/c/software/Coding/vidotter
```

Register Native Messaging with `bin/vidotterdev.exe install` on Windows or `./bin/vidotterdev install` on Linux/macOS. Uninstall with the corresponding `uninstall` command. Install/deploy leave `Data/`, existing downloads and saved configuration intact.

Android uses the official Gradle Wrapper, gomobile, SDK/NDK and JDK. FFmpeg uses upstream configure/make, Android NDK and MSYS2 as needed; no custom Shell, CMD or PowerShell orchestration replaces these toolchains. Verify the required SDK/toolchain before claiming a platform build succeeded.

## Flags and prerequisites

Base builds need Go 1.24+; `verify` additionally needs Node.js. Start with the [development tutorial](../tutorial/development.md).

| Flag | Purpose |
| --- | --- |
| `--root` | Explicit repository/install root; otherwise search upward from working directory or executable for `extension/manifest.json` |
| `--os` / `--arch` | Build/deploy target; use `darwin` for macOS |
| `--output` | Output file or directory for commands supporting it |
| `--target` | Required explicit installation destination for deploy |
| `--extension-id` | 32-character a–p extension ID for install |
| `--source` | FFmpeg source tree, or corresponding source archive for package |
| `--archive` | Local pinned tool archive for tool-install; SHA-256 is still verified |

Go flag parsing stops at the first positional argument. Use `tool-install --archive /path/to/archive.zip whisper`, not options after `whisper`. `build` generates only Bridge; `deploy` also builds the maintainer and copies extension/docs, but does not provision media tools or register the native host.

Formal packaging also needs `.build/cache/source/FFmpeg-n8.1.2-source.tar.gz`, or `package --source /path/to/archive`, matching the pinned SHA-256. Only Windows archives copy `dist/tools/`; Linux/macOS do not bundle FFmpeg, Whisper or Llama. See [release notes](../../build/release/README.md) for component requirements. `package` does not build Android or create a GitHub Release.

## Dependency licenses

Run `go -C tools/vidotterdev run . licenses` to collect target-specific full Go notices through pinned upstream go-licenses v2.0.1. `package` runs this automatically. Formal Windows packaging also rejects missing sidecar license/source notices. CI runs the existing verify/build commands on Windows, macOS and Linux; browser and Android acceptance remain separate.
