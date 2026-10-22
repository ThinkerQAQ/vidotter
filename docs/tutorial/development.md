# Build and check a source checkout

This tutorial verifies a checkout and produces a Bridge executable. It does not register a browser host or replace an installed application.

## Prerequisites

- Git and Go 1.24 or later, as required by both Go modules.
- Node.js for JavaScript syntax checks and extension tests. There is no npm install step for the plain-JavaScript extension.
- Network access for Go dependencies on a cold module cache.
- Desktop acceptance additionally needs Chrome/Edge and FFmpeg for media merging.

```sh
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go version
node --version
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` runs tests in both Go modules, checks extension JavaScript syntax and executes extension tests. A passing result does not establish a connected browser, a successful media download or an Android build.

`build` defaults to the host OS/architecture and writes `.build/out/vidotter-<os>-<arch>[.exe]`. A macOS host uses the Go OS name `darwin` here. Release archives use `macos` in their names.

## Cross-compile

```sh
go -C tools/vidotterdev run . build --os windows --arch amd64
```

This compiles Bridge only. It does not supply third-party tools, browser registrations or an installer binary. Go build artifacts are not a complete release archive.

## Try a development installation

Use a dedicated target so development does not replace your normal installation:

```sh
go -C tools/vidotterdev run . deploy --target /absolute/path/to/vidotter-dev
```

The command builds/copies program files and a maintainer binary. It does not populate packaged media tools or perform native registration. On the target OS, run the installed maintainer's `install` command from that target root, then load its `extension/` in Chrome/Edge. On Windows use a Windows executable, not Linux's WSL build.

For WSL-to-Windows deployment use `--os windows --arch amd64` and an explicit `/mnt/c/...` target. Stop that target's Bridge before replacement. Existing `Data/` is preserved. Separate installed roots have separate data, but browser registration for the same host name points to the most recently registered root.

Reload the browser extension after JS/HTML/CSS/manifest/locale changes. Open Settings → About to compare extension and Bridge versions. Run a permitted media download and inspect its output for browser acceptance. Restore the normal installation's registration when done if you used a separate root.

For packaging and native dependencies see [maintenance](../architecture/maintenance.md). For Android see [developer notes](../../mobile/android/README.md).
