# VidOtter

[English](README.md) · [下载安装](https://github.com/ThinkerQAQ/vidotter/releases/tag/v1.6.3) · [问题反馈](https://github.com/ThinkerQAQ/vidotter/issues)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

在浏览器里看视频时，我们有时需要把有权保存的视频下载到本地。但网页中的播放地址可能临时生成，下载 HLS 时又需要处理分片和合并，手动操作比较麻烦。

VidOtter 通过浏览器扩展发现当前页面的媒体，再交给本地下载服务（Bridge）处理。主要提供以下功能：

- 发现 MP4、HLS 等媒体，创建下载任务，查看进度并打开下载文件。
- 在弹窗、侧边栏和全屏工作区中管理同一份任务、配置及历史记录。
- 按需使用字幕、语音识别、翻译和 ChatGPT MCP。

桌面端支持 Windows、macOS、Linux，使用 Chrome、Chromium 或 Edge。Android 是独立客户端，使用方式见后文。

![VidOtter 资源嗅探界面](assets/first-run-detect-zh.png)

截图来自 v1.3.0，用于说明资源嗅探入口。

## 2. Quick Start

现有的场景是这样的：浏览器中有一个可以播放的 MP4，我们希望将它下载到本地，并确认文件能正常打开。

下面以这个场景完成第一次使用：

1. 准备运行环境。
2. 安装 VidOtter。
3. 加载浏览器扩展。
4. 播放视频并下载。
5. 检查下载结果。

### 2.1. 准备运行环境

需要一台 Windows、macOS 或 Linux 电脑，以及 Chrome、Chromium 或 Edge 浏览器。

使用发行包不需要安装 Go、Node.js，也不需要 OpenAI 账号或 API Key。首次下载普通 MP4，不需要配置字幕、模型或其他下载工具。HLS 合并、分离音视频处理需要 FFmpeg：Windows 发行包已附带，Linux/macOS 需要自行安装。

### 2.2. 安装 VidOtter

第一步，从 [v1.6.3 Release](https://github.com/ThinkerQAQ/vidotter/releases/tag/v1.6.3) 下载与系统匹配的安装器：

- Windows x64：运行 `VidOtter-Setup-v1.6.3-windows-amd64.exe`。
- macOS Intel：打开 `VidOtter-Setup-v1.6.3-macos-amd64.dmg`，运行其中的安装器。
- Linux x64：下载 `VidOtter-Setup-v1.6.3-linux-amd64.run`，在下载目录执行：

```bash
chmod +x VidOtter-Setup-v1.6.3-linux-amd64.run
./VidOtter-Setup-v1.6.3-linux-amd64.run
```

按安装器提示完成安装。安装器会注册本地下载服务，完成时打开扩展激活说明和安装目录。浏览器会按需启动 Bridge，不需要一直开着终端。

**如果使用 Apple Silicon、Linux ARM64，或者希望手动安装：** 在同一 Release 下载对应的 ZIP/TAR.GZ，完整解压到长期使用的目录，保持 `bin/` 和 `extension/` 在同一个根目录。在解压根目录打开终端，执行：

Windows PowerShell：

```powershell
.\bin\vidotterdev.exe install
```

Linux/macOS：

```bash
./bin/vidotterdev install
```

输出中应包含 `Native Messaging installed:` 和 `Host manifest:`，表示本地服务已注册。移动手动安装目录后，需要在新位置重新执行此命令。

### 2.3. 加载浏览器扩展

第二步，让浏览器连接到刚安装的本地服务：

1. Edge 打开 `edge://extensions`，Chrome 打开 `chrome://extensions`。
2. 开启**开发者模式**，点击**加载解压缩的扩展**。
3. 选择 VidOtter 安装目录下的 `extension/` 文件夹，这个文件夹中应直接包含 `manifest.json`。
4. 打开 VidOtter，在**设置**中检查浏览器与本地服务的连接状态。

扩展目前通过本地文件安装，尚未上架扩展商店。这一步需要用户在浏览器中操作。

如果提示找不到 Native Messaging Host，先复制扩展管理页显示的 32 位扩展 ID，再在安装根目录重新注册：

```powershell
# Windows：将占位符替换为实际扩展 ID
.\bin\vidotterdev.exe install --extension-id YOUR_32_CHARACTER_ID
```

Linux/macOS 使用 `./bin/vidotterdev install --extension-id YOUR_32_CHARACTER_ID`。注册后，在扩展管理页重新加载 VidOtter。

### 2.4. 播放视频并下载

第三步，打开 [MDN 的 CC0 示例视频](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4)并开始播放。

保持在这个标签页，打开 VidOtter：

1. 在**资源嗅探**中找到 MP4。
2. 字幕选择**不下载字幕**。
3. 点击**下载**。
4. 切换到**下载任务**，查看刚创建的任务。

如果换成 HLS，选择完整播放清单，不要选择单独的媒体分片。

### 2.5. 检查下载结果

第四步，等待任务变为**已完成**，打开下载文件，确认视频可以播放。默认下载目录是安装根目录下的 `Data/downloads/`。

至此，浏览器发现资源、本地服务创建任务、下载文件这条链路就跑通了。

需要更大的界面时，从扩展点击**打开工作区**，进入 `http://127.0.0.1:17891/console/`。工作区通过扩展中继执行操作，请从扩展打开。

### 2.6. 常见问题

- **扩展连接不上本地服务：** 检查扩展 ID，重新执行上面的 `install`，然后重新加载扩展。如果仍失败，查看 `Data/logs/`，检查 `127.0.0.1:17891` 是否被其他进程占用。
- **发现不到视频：** 回到原标签页播放或拖动进度条，让页面重新请求媒体。部分网站没有可下载的 MP4/HLS，或使用 DRM。
- **任务报 403 或地址过期：** 回到原页面重新播放，获取新的媒体请求，再按任务提示恢复或重新下载。
- **HLS 合并失败：** 检查 FFmpeg 是否可用。字幕问题先切回“不下载字幕”，确认普通下载是否正常。
- **升级后还是旧版本，或提示不支持的 Tool 操作：** 停止旧 Bridge，更新程序文件，在扩展管理页点击**重新加载**，再到**设置 → 关于**核对版本。不要通过删除 `Data/` 解决。

## 3. Configuration

第一次使用没有必填配置。需要调整下载目录、界面或工具时，打开 **VidOtter → 设置**，修改后保存即可。

常用配置如下：

- `download.output_dir`：下载目录，默认是 `Data/downloads/`。
- `download.concurrent`：下载并发，默认 `12`，范围 `1–64`。
- `download.quality`：默认自动选择，也可指定 `best` 或来源支持的高度，例如 `720`。
- `proxy.enabled`、`proxy.url`：默认关闭代理；支持 HTTP 代理，不支持代理凭据和 SOCKS。
- `interface.language`：界面语言，默认 `zh-CN`，可改为 `en`。
- `interface.open_mode`：默认 `popup`，可改为 `sidePanel`。

配置保存在安装根目录的 `Data/config.toml`。设置页会校验并保存完整配置；如需手动修改，先备份、停止 Bridge，修改后重启。不要用几行配置片段覆盖整个文件。

**升级与卸载：** 先暂停任务并备份 `Data/`。图形安装用户通过 **VidOtter Maintenance** 移除旧程序，再运行新安装器；当前尚未发布在线更新源。手动安装用户替换同一版本的 `bin/`、`extension/` 和随包 `tools/`，保留 `Data/`，重新注册并加载扩展。程序卸载会保留数据和下载文件；手动删除安装目录前需自行备份。

手动安装的注销命令为 Windows `.\bin\vidotterdev.exe uninstall`，Linux/macOS `./bin/vidotterdev uninstall`，之后在浏览器移除扩展。

## 4. Optional Features

以下功能根据实际场景选择，不是首次下载的前置条件。

### 4.1. Case 1：下载更多格式或网页中的视频

MP4/HLS 可以使用内置下载器。需要解析其他受支持的网页时，在设置页安装 yt-dlp；需要合并、封装音视频时，配置 FFmpeg。DASH 和分离音视频的支持取决于来源及工具。

### 4.2. Case 2：生成字幕或翻译

根据字幕来源选择处理方式：

- 使用原站字幕：需要 yt-dlp，且来源提供可获取的字幕。
- 本地语音识别：需要 FFmpeg、whisper.cpp 和语音模型。
- 字幕翻译：需要 llama.cpp 和翻译模型；使用语音识别结果时，还要指定原声语言。

在设置页安装或配置工具及模型，再创建带字幕选项的任务。任务完成后，检查字幕文件、内容和时间轴。

Windows v1.6.3 已附带 FFmpeg Slim、whisper.cpp、llama.cpp 可执行文件，模型需另行下载；Linux/macOS 按需提供兼容工具。

### 4.3. Case 3：通过 ChatGPT 管理下载

桌面程序提供 STDIO MCP，启动参数为 `--mcp-stdio`，复用本地 Bridge。

连接 ChatGPT 的步骤如下：

1. 在 OpenAI Platform 创建自己的 Tunnel。
2. 打开**设置 → AI 集成 → ChatGPT MCP**，按需安装官方 Tunnel Client，填写 Tunnel ID，点击**配置并启动 Tunnel**。
3. 如果没有可复用的凭据，按本机终端提示输入具有 Tunnels Read + Use 权限的 Runtime API Key。
4. 在 ChatGPT 创建私有自定义 MCP 插件，连接方式选 Tunnel，填写同一个 Tunnel ID。
5. 调用 `vidotter_status` 检查连接。提交下载后，使用返回的任务 ID 查询结果。

Tunnel Ready 只表示本机连接就绪；收到 ChatGPT 中的实际工具响应，才完成客户端验收。凭据在本机终端处理，不经浏览器扩展传输。

### 4.4. Case 4：需要浏览器操作

在设置中按需安装 [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) 组件。它作为独立发行包使用，只操作受限后台标签页；自动点击需要本机显式开启。

如果 ChatGPT 已连接 Remote Browser Tool，复用现有连接操作页面，VidOtter 负责发现媒体和下载，无需再建一个 Tunnel。

### 4.5. Case 5：在 Android 使用

Android 是独立原生客户端，支持 Android 8.0 / API 26 及以上，通过粘贴或分享 MP4/HLS 直链下载。它不接入桌面任务、Native Messaging、yt-dlp 或 ChatGPT Tunnel。

当前桌面 Release 不包含 APK。源码构建需要 JDK 17、Android SDK 35 / NDK、gomobile 和 Gradle Wrapper，准备好环境后执行：

```bash
go -C tools/vidotterdev run . android-bind
go -C tools/vidotterdev run . android-build
```

## 5. Development

开发环境需要 Git、Go **1.25+**、Node.js（CI 使用 Node 22）。使用发行包的用户不需要安装这些工具。

### 5.1. 打开项目并验证

```bash
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` 执行两个 Go 模块的测试、扩展 JS 语法检查与脚本测试，成功后输出 `Go and browser checks passed`。`build` 将当前系统的 Bridge 编译到 `.build/out/`，不执行安装。

跨平台编译示例：

```bash
go -C tools/vidotterdev run . build --os windows --arch amd64
```

### 5.2. 安装开发版本

在源码仓库执行，替换为独立的开发安装目录：

```bash
go -C tools/vidotterdev run . deploy --target /absolute/path/to/vidotter-dev
```

然后到目标系统的部署目录执行 `bin/vidotterdev install`，Windows 使用 `bin\vidotterdev.exe install`。扩展加载步骤同 Quick Start。

### 5.3. 打包与 CI

统一使用 `tools/vidotterdev/` 构建、验证和打包。正式打包命令为 `go -C tools/vidotterdev run . package`，要求准备已校验的侧车工具、许可证及固定版本的 FFmpeg 源码包。`--allow-missing-tools` 仅用于不完整的开发包。

[CI](.github/workflows/ci.yml) 在 Windows、macOS 和 Linux 执行测试、构建及 README 链接检查；真实浏览器下载、图形安装器操作及 Android 设备需要另外验收。协作约定见 [AGENTS.md](AGENTS.md)。

## 6. License

原创源码采用 [Apache-2.0](LICENSE)，第三方组件遵循各自许可证，详见 [NOTICE](NOTICE) 和[第三方声明](THIRD_PARTY_NOTICES.md)。Windows 的 FFmpeg Slim 使用 LGPL-2.1-or-later，对应源码随同一 Release 提供；Qt Installer Framework 使用 GPL-3.0 与 Qt Company Exception。模型权重和下载的媒体不在项目源码许可范围内。

请只下载有权保存的媒体。VidOtter 不绕过 DRM、付费墙或访问控制，不与内容平台存在从属关系。当前图形安装器未完成 Windows 代码签名和 Apple 公证，ARM64 平台使用压缩包安装。

`Data/` 中可能包含任务来源、断点数据和本地认证信息，不要上传到公开仓库。浏览器会话由扩展处理，Bridge 仅在本机回环地址监听；私有 MCP 只连接可信客户端。
