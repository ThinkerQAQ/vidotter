# 架构

先看[组件与数据](../concepts/system.md)，再看[原则](principles.md)。桌面主流程为浏览器检测 → Bridge 认证任务 → Worker → 输出；Android 使用独立原生路径。

| 范围 | 源码入口 |
| --- | --- |
| Native Host 与 HTTP 控制 | `bridge/bridge.go` |
| 任务、快照与恢复 | `bridge/bridge_jobs.go`、`bridge/bridge_job_store.go` |
| 配置与目录定位 | `bridge/bridge_config.go`、`bridge/runtime_layout.go` |
| 下载、封装、字幕 | `bridge/main.go`、`bridge/muxer.go`、`bridge/subtitle.go` |
| 工具与模型 | `bridge/tool_registry.go`、`bridge/components.go`、模型目录文件 |
| 浏览器检测与会话 | `extension/background.js`、`extension/core/`、`extension/content/` |
| 共用界面与工作区 | `extension/popup/`、`extension/console-relay.js`、`bridge/web_console.go` |
| Android 集成 | `bridge/mobile.go`、`mobile/android/` |
| 工程编排 | `tools/vidotterdev/` |

协议见 [API 参考](../reference/bridge-api.md)，首次构建见[开发教程](../tutorial/development.md)，打包与工具链见[工程维护](maintenance.md)。
