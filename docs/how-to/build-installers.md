# Unified desktop installers (Qt Installer Framework)

This is the **build/release workflow**, not an installer download. Do not advertise
one-click installers until each platform has an actual release asset and a verified
first-install/update/uninstall smoke test.

## Why Qt Installer Framework?

The same offline installer definition drives Windows, macOS, and Linux. The
user runs the platform's installer and chooses an install directory; the
installer extracts VidOtter and calls the **existing Go** `vidotterdev install`
to register Native Messaging. On uninstall it calls `vidotterdev uninstall`.
There is no second Bridge implementation, persistent service, or Qt runtime
requirement for the **installed VidOtter application**.

| Host used to build | Intended output | Installation |
| --- | --- | --- |
| Windows amd64 | `VidOtter-Setup-vVERSION-windows-amd64.exe` | Open .exe |
| macOS amd64/arm64 | `VidOtter-Setup-vVERSION-macos-ARCH.dmg` | Open disk image and installer |
| Linux amd64/arm64 | `VidOtter-Setup-vVERSION-linux-ARCH.run` | Make executable and run (graphical session) |

These are platform-specific **artifacts**, even though the source configuration
and user-facing wizard are shared. macOS distribution may need Developer ID signing
and notarization; Windows may prompt for an unsigned download. Linux requires a
desktop graphical session and compatible Qt platform dependencies.

## Build an offline installer

1. For a current official toolchain, install **Qt Installer Framework 4.11.0**'s build tools on the build machine,
   using Qt Online Installer's **Qt → Build tools** category. Verify
   `binarycreator` is on PATH. This is a **build-time-only** dependency.
   Review license obligations for shipped Qt IFW binaries before public release.
2. Generate the matching **complete** release using
   `go -C tools/vidotterdev run . package`; do not publish development bundles
   built with `--allow-missing-tools`.
3. Select the matching release staging directory, for example on Linux amd64:

```bash
go -C tools/vidotterdev run . installer \
  --stage .build/out/release-stage/vidotter-v1.6.2-linux-amd64 \
  --os linux --arch amd64 \
  --output .build/artifacts/installers/VidOtter-Setup-v1.6.2-linux-amd64.run
```

On Windows, use `--os windows --arch amd64` and an `.exe` output.
On macOS, use `--os darwin --arch amd64` or `--arch arm64` and a `.dmg`
output. Use `--binarycreator /absolute/path/to/binarycreator` if it is not
on PATH. The build machine must be running the target OS; this command does
not cross-build a Qt IFW executable. The release stage must contain
`bin/vidotterdev[.exe]`, the matching Bridge binary, and `extension/`.

The installer packages a **local copy** of the release program files; it
**excludes Data/**. Qt IFW's package uses `RemoveTargetDir=false`, and the
Go host's uninstall command leaves local settings, tasks, models, and downloads
intact. Keep the install location unchanged for upgrades. Do not claim
upgrade safety until testing replacement of a running Bridge and replay of
the host registration on each platform.

## Browser extension — still requires user consent

No browser store is used. After installing the local Bridge, open
`edge://extensions` or `chrome://extensions`, enable Developer mode and
choose **Load unpacked**, selecting the new install directory's `extension/`
folder. Restart or reload the extension if the connection status remains
unhealthy. The installer cannot silently add an unmanaged Chrome/Edge
extension on arbitrary consumer machines.

## Automated candidate workflow

[Installer CI workflow](../../.github/workflows/installer-smoke.yml) pulls the existing
GitHub Release archive, checks SHA-256, then builds offline installer candidates
on Windows, macOS Intel, and Linux amd64. It uses pinned Qt IFW **4.7** via
`aqtinstall` for reproducible open-source CI; Qt IFW 4.11 is available through
the official Qt Online Installer and needs separate release validation.
Candidates are temporary workflow artifacts, **not published Releases**.
Native CI additionally exercises isolated command-line install/uninstall smoke tests,
including explicit target-directory registration and retained `Data/`.

On 2026-11-01, native CI built **Windows x64, macOS Intel, and Linux x64**
installers successfully from the complete v1.6.2 release. Linux amd64 also
passed command-line install/uninstall and local Data preservation.
Windows x64, macOS Intel and Linux x64 all passed native command-line
install/uninstall tests with Data preservation in GitHub Actions
[run #41284068572](https://github.com/ThinkerQAQ/vidotter/actions/runs/41284068572).
Native GUI and browser extension onboarding are **not yet verified**.

**Offline update limitation:** Trying to run a newer Qt IFW offline installer
against an existing installation directory returns `TargetDirectoryInUse`.
Do not instruct users to overwrite the old installation this way. Until a
verified update/migration path exists, use the documented archive upgrade
procedure. For existing Qt IFW installations, removing components via the
Maintenance Tool preserves Data, but it is **not** a verified one-click upgrade.

## Release acceptance checklist

- Verify `binarycreator` produces a self-contained installer and the
  platform launches its actual GUI (not merely that Go tests pass).
- Install into a path with spaces; load the extension and validate Bridge
  auto-start, status, and a public, authorized MP4 download.
- Upgrade after preserving a sentinel in `Data/config.toml` and a test job.
  Ensure active downloads are stopped safely before attempting replacement.
- Uninstall and verify Native Messaging is removed but `Data/` persists.
- Test Windows, macOS Intel/Apple Silicon, and Linux amd64/arm64 separately.
- Recheck all third-party license files, **Qt IFW GPLv3 with exception**
  redistribution/source obligations, and installer redistribution terms.
- Sign/notarize platform deliverables where applicable, verify checksums,
  then upload the installer assets to GitHub Releases alongside archives.

The old archive + manual `vidotterdev install` method remains the supported
fallback until each installer is published and verified.

See [manual first run](../tutorial/first-run-on-new-machine.md) and
[upgrade](upgrade.md).

## One-click update: tested mechanism and publishing requirement

Qt IFW's **Maintenance Tool** is the updater. Do **not** launch a new offline
setup on top of an existing install: that still yields `TargetDirectoryInUse`.
The correct update path is to publish an IFW update repository generated from
the verified new release. The installed Maintenance Tool checks it, downloads
the changed components, updates in place, and re-runs Native Messaging
registration without replacing `Data/`.

The new CI packaging command is:

```bash
go -C tools/vidotterdev run . update-repository \
  --stage .build/out/release-stage/vidotter-v1.6.2-linux-amd64 \
  --os linux --arch amd64 --repogen /path/to/qtifw/repogen \
  --output .build/artifacts/update-repos/linux-amd64
```

For a **published HTTPS repository** containing `Updates.xml` and all referenced
7z archives, build the installer with `--update-url https://.../updates/linux/amd64`.
This creates an update-capable Maintenance Tool. A local `file://` repository
is permitted for isolated acceptance tests. Without `--update-url`, the
installer remains fully offline and `Maintenance Tool --at file:///... update`
can still be used for controlled migration.

**Verified locally on Linux:** full v1.6.2 installation → synthetic 1.6.3
IFW update repository → `check-updates` → `update` → component version 1.6.3,
with original config and Native Messaging intact. Version 1.6.3 here is a
**test fixture, not a publicly released binary**. A second run with a bundled
updatable installer and an embedded local repository URL completed without
passing `--at` manually. Automatic updates for end users require a hosted,
reviewed and reachable repository, signed releases and native GUI acceptance;
none of those is currently advertised as shipped.

The MCP server is bundled as part of the same Bridge binary in every release.
Configuring ChatGPT Tunnel is optional, user-authorized, and not performed by
the default installer; see [MCP](mcp.md).

### Publish the feed (release-maintainer action)

The [manual GitHub Pages workflow](../../.github/workflows/publish-update-feeds.yml)
may be dispatched **only from protected `main`** after the new version is a
completed, non-prerelease GitHub Release. It verifies each platform archive
against the Release SHA256SUMS, generates IFW update metadata per platform,
then deploys a complete static update site. It does not publish synthetic CI
versions. First enable GitHub Pages for this repository using GitHub Actions
as its source, then verify the **actual returned Pages URL**. The owner's main
site uses a custom domain, so do not assume a GitHub Pages URL or accidentally
replace existing blog pages.

Before building installers that embed an update URL, confirm on the real HTTPS
site that `updates/windows/amd64/Updates.xml`, `updates/linux/amd64/Updates.xml`
and `updates/macos/amd64/Updates.xml` are accessible, plus all referenced
archives. The build script then accepts `--update-base-url VERIFIED_BASE_URL`.
CI defaults to standalone offline installers until the hosted feed is verified.

With an update-enabled installer, Windows adds an **Update VidOtter** Start Menu
shortcut; Linux adds a **VidOtter Updater** desktop entry. On macOS, open the
installed **VidOtter-Maintenance.app**. In each case use the Maintenance Tool to
check and install an available release. Finish/pause downloads and close the
running Bridge first; never forcibly terminate active media jobs during update.
**Legacy archive adoption:** an existing unpacked installation can be adopted by
selecting its current directory as the Qt IFW installation target, after
finishing/stopping all tasks and backing up `Data/`. The wizard warns that the
directory is nonempty and requires explicit approval. The installer overwrites
managed `bin/`, `extension/`, `docs/` and `scripts/`, but **never packages or
overwrites `Data/`**. Tested on Linux with a preexisting sentinel config;
Windows/macOS regression cases are also included in CI. This is not silent
auto-detection of old installations, and choosing a different target does not
automatically migrate the old Data directory.
