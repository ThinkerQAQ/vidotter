# Architecture principles

| Principle | Implementation consequence |
| --- | --- |
| Reuse mature components | Keep FFmpeg, yt-dlp, whisper.cpp and llama.cpp behind tool/pipeline contracts rather than reimplementing their formats or inference runtimes |
| One desktop task owner | Bridge owns jobs, settings and workers; UI surfaces call shared modules |
| Browser-scoped authorization | Extension acquires page/media context; credentials are removed from job snapshots, with fresh-session recovery after restart |
| Loopback workspace | Web UI uses shared assets and an allowlisted extension relay; no token is embedded in HTTP assets |
| Replaceable processing | Download, mux, ASR and translation have separate interfaces and availability checks |
| Separate mobile runtime | Android reuses Go download code with native platform integration; desktop tool availability does not imply mobile support |
| One engineering entry point | `tools/vidotterdev/` owns build, verify, package and deploy orchestration; upstream Gradle/FFmpeg toolchains remain in use |

When changing a contract, update both implementations and the corresponding reference/tutorial. Keep [AGENTS.md](../../AGENTS.md) focused on engineering instructions; put user procedures in the docs. Verify code changes and inspect the affected UI flow before describing it as working.
