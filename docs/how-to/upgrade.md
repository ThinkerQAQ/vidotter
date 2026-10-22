# Upgrade, move or uninstall

## Upgrade a desktop installation

1. Finish or pause active jobs. Stop the old Bridge before replacing executables, especially on Windows where running files are locked.
2. Back up the installation's `Data/` and any media outside it that you want to keep.
3. Extract the new archive to a separate folder. Replace program directories (`bin/`, `extension/`, packaged `tools/`) as a matching release. Keep your existing `Data/`; do not replace it with empty data from another installation.
4. From the resulting installation root run the new `vidotterdev install` command for your OS, as in [first run](../tutorial/first-run-on-new-machine.md).
5. Reload the unpacked extension in the browser's extensions page. If you changed its directory, load the new `extension/` instead of continuing to use the old copy.
6. Open VidOtter, compare extension/Bridge versions in **Settings → About**, check tools and output directory, then try one download.

The installer writes Native Messaging registrations. It does not migrate an arbitrary old data directory, rename legacy configuration, copy model files, or upgrade the loaded browser extension for you.

## Move the installation

Move the complete installation including `Data/`, then re-run installation from the new root. Check absolute paths in settings: custom output directories, executable overrides and model paths can still point to the old location. Re-select paths as needed. Media stored outside the installation is not moved automatically.

## Uninstall

Run `.\bin\vidotterdev.exe uninstall` (Windows) or `./bin/vidotterdev uninstall` (Linux/macOS) from the installation root, then remove the browser extension. This unregisters Native Messaging; Windows also removes the VidOtter URL protocol registration. It leaves downloads and `Data/` in place. Delete the installation directory manually only after saving any files you need.
