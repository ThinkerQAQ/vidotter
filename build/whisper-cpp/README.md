# whisper.cpp Windows client

VidOtter keeps the Windows x64 CPU client as a separate sidecar under
`dist/tools/whisper/`. The standard release package copies that directory
without embedding it into the Bridge executable.

Provision the pinned official client with:

```powershell
go -C tools/vidotterdev run . tool-install whisper
```

The Go maintainer downloads `whisper-bin-x64.zip` from the official whisper.cpp
release, verifies the pinned SHA-256 recorded in the maintainer, and
copies only the CLI and its CPU runtime libraries. Model files are deliberately
excluded because they are large and users may choose different multilingual
models.

These commands prepare Windows release sidecars under `dist/tools/`, not user model weights or Linux/macOS executables. Run them from the repository root. Optional flags such as `--archive /path/to/verified.zip` must precede the positional tool name (Go flag parsing stops at the first positional argument). Runtime setup: [tools and models](../../docs/how-to/configure-tools.md).
