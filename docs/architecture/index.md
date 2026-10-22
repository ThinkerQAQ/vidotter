# Architecture

Start with [components and data](../concepts/system.md), then [principles](principles.md). The desktop flow is browser detection → authenticated Bridge task → worker → output. Android follows its own native path.

| Area | Source entry points |
| --- | --- |
| Native host and HTTP control | `bridge/bridge.go` |
| Jobs, snapshots and recovery | `bridge/bridge_jobs.go`, `bridge/bridge_job_store.go` |
| Config and layout | `bridge/bridge_config.go`, `bridge/runtime_layout.go` |
| Download, mux, subtitles | `bridge/main.go`, `bridge/muxer.go`, `bridge/subtitle.go` |
| Tools and models | `bridge/tool_registry.go`, `bridge/components.go`, model catalog files |
| Browser detection and session | `extension/background.js`, `extension/core/`, `extension/content/` |
| Shared UI and workspace | `extension/popup/`, `extension/console-relay.js`, `bridge/web_console.go` |
| Android integration | `bridge/mobile.go`, `mobile/android/` |
| Engineering orchestration | `tools/vidotterdev/` |

Use [API reference](../reference/bridge-api.md) for protocols, [development tutorial](../tutorial/development.md) for a first build and [maintenance](maintenance.md) for packaging/toolchains.
