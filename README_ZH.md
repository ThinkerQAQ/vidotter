# VidOtter

[English](README.md) · [中文文档](docs/zh-CN/index.md) · [下载发行包](https://github.com/ThinkerQAQ/vidotter/releases)

VidOtter 通过 Chrome/Edge 扩展发现网页媒体，由本机 Go Bridge 下载。支持 HLS、MP4，以及桌面端对部分网页和分离音视频来源的解析。可选工具提供原站字幕、本地语音识别和字幕翻译。Android 是面向媒体直链的独立客户端。

## 安装与首次下载

1. 从 [Releases](https://github.com/ThinkerQAQ/vidotter/releases) 下载适合系统和 CPU 架构的发行包，完整解压到固定目录。
2. 在解压目录运行安装命令：Windows PowerShell 使用 `.\bin\vidotterdev.exe install`；Linux/macOS 使用 `./bin/vidotterdev install`。Linux/macOS 需另外安装 FFmpeg 并加入 PATH，用于合并和封装。
3. 打开 `chrome://extensions` 或 `edge://extensions`，开启“开发人员模式”，选择“加载解压缩的扩展”，加载 `extension/`。
4. 播放视频，打开 VidOtter，确认 Bridge 连接状态，在“资源嗅探”中选择目标资源。首次下载选择“不下载字幕”。
5. 到“下载任务”查看任务，等待完成后检查输出文件。

浏览器通过 Native Messaging 按需启动 Bridge。使用发行包不需要安装 Go 或 Node.js。扩展 ID、安装结果和连接检查见[首次使用教程](docs/zh-CN/tutorial/first-run-on-new-machine.md)。

## 支持范围

| 功能 | 桌面端 | Android |
| --- | --- | --- |
| 检测网页媒体 | Chrome/Edge 扩展 | 需要支持扩展的浏览器；也可粘贴或分享直链 |
| MP4/HLS 直链下载 | Go 下载器 | 复用 Go 下载器 |
| 网页解析与分离音视频 | 可选 yt-dlp、FFmpeg，取决于来源 | 暂不普遍支持 |
| 原站字幕 | 可选 yt-dlp | 未集成 |
| 本地识别与翻译 | Windows 包附带可执行程序，模型另行下载 | 未集成 |
| 完整工作区 | 扩展中继 + 本机 Web UI | 原生 App |

检测到资源不代表一定能下载；工具不绕过 DRM。Linux/macOS 发行包不附带 FFmpeg 或本地识别、翻译 sidecar。

## 界面

![VidOtter 检测界面](docs/images/first-run-detect-zh.png)

上图是 v1.3.0 尚未发现媒体时的中文检测页，用于说明界面入口；连接状态与下载结果需按教程在自己的机器上确认。

Popup、Side Panel 和 Web UI 共用任务与设置。从扩展点击“打开工作区”可使用完整页面。设置与运行数据位于 `bin/` 同级的 `Data/`，升级时保留该目录。

## 文档

- [快速入门](docs/zh-CN/quick-start.md)、[首次下载](docs/zh-CN/tutorial/first-download.md)
- [工具与模型](docs/zh-CN/how-to/configure-tools.md)、[字幕](docs/zh-CN/how-to/subtitles.md)、[任务与断点续传](docs/zh-CN/how-to/manage-downloads.md)
- [升级与卸载](docs/zh-CN/how-to/upgrade.md)、[故障排查](docs/zh-CN/how-to/troubleshoot.md)、[Android](docs/zh-CN/how-to/android.md)
- [配置与接口参考](docs/zh-CN/reference/index.md)、[架构](docs/zh-CN/architecture/index.md)

## 开发

需要 Go 1.24 或更高版本；浏览器测试需要 Node.js。在仓库根目录运行：

```sh
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

产物位于 `.build/out/`，构建不会自动安装。部署与打包见[开发教程](docs/zh-CN/tutorial/development.md)及[工程维护](docs/zh-CN/architecture/maintenance.md)。Agent 约定见 [AGENTS.md](AGENTS.md)。

项目代码和文档采用 [Apache-2.0](LICENSE)；第三方组件采用各自的许可，见 [NOTICE](NOTICE) 和 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
