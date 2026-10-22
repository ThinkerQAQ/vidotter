# llama.cpp Windows server

VidOtter keeps the Windows x64 CPU server as a separate sidecar under
`dist/tools/llama/`. The standard release package copies that directory
without embedding it into the Bridge executable.

Provision the pinned official server with:

```powershell
go -C tools/vidotterdev run . tool-install llama
```

The Go maintainer downloads the official llama.cpp Windows CPU archive, verifies it against the
pinned SHA-256 recorded in the maintainer, and copies `llama-server.exe` with its required
CPU runtime libraries. Translation model files are deliberately excluded:
TranslateGemma uses the Gemma license, which the user must review separately.

These commands prepare Windows release sidecars under `dist/tools/`, not user model weights or Linux/macOS executables. Run them from the repository root. Optional flags such as `--archive /path/to/verified.zip` must precede the positional tool name (Go flag parsing stops at the first positional argument). Runtime setup: [tools and models](../../docs/how-to/configure-tools.md).
