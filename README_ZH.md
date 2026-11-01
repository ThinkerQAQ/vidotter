# VidOtter

[English](README.md)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

网页视频的播放地址可能临时生成，HLS 下载还需要处理分片和合并。VidOtter 通过浏览器扩展发现媒体，交给本地下载服务（Bridge）处理，提供以下功能：

- 发现 MP4、HLS 等媒体，创建下载任务，查看进度并打开下载文件。
- 在弹窗、侧边栏和全屏工作区中管理同一份任务、配置及历史记录。
- 按需使用字幕、语音识别、翻译和 ChatGPT MCP。

![VidOtter 资源嗅探界面](assets/first-run-detect-zh.png)

界面示意，实际布局以所安装版本为准。

## 2. Quick Start

下面以下载一段 MP4 为例，从安装到打开文件，完成第一次使用。

### 2.1. 准备运行环境

需要一台 Windows、macOS 或 Linux 电脑，以及 Chrome、Chromium 或 Edge 浏览器。

普通 MP4 下载无需开发工具、账号或模型配置。

### 2.2. 安装 VidOtter

#### 2.2.1. 使用图形安装器

打开[最新 Release](https://github.com/ThinkerQAQ/vidotter/releases/latest)，在 Assets 中选择与操作系统和 CPU 架构匹配的安装器。没有对应安装器时，使用 [2.2.2](#222-使用压缩包手动安装) 的压缩包方式。

- Windows：运行对应的 `.exe` 安装器。
- macOS：打开对应的 `.dmg`，运行其中的安装器。
- Linux：下载对应的 `.run` 安装器。为方便执行，可先在文件管理器中将下载的文件重命名为 `VidOtter-Setup.run`，再在下载目录执行：

```bash
chmod +x VidOtter-Setup.run
./VidOtter-Setup.run
```

按提示完成安装并记下安装目录。安装器会自动注册 Bridge，并打开扩展激活说明；完成后直接进入 [2.3](#23-安装浏览器扩展)。Bridge 由浏览器按需启动。

若系统提示未知发行者，先核对下载来源和该 Release 的签名、公证说明。

#### 2.2.2. 使用压缩包手动安装

手动安装时，在同一 Release 的 Assets 中按以下文件名后缀选择压缩包：

- Windows x64：`windows-amd64.zip`。
- macOS Intel：`macos-amd64.tar.gz`。
- macOS Apple Silicon：`macos-arm64.tar.gz`。
- Linux x64：`linux-amd64.tar.gz`。
- Linux ARM64：`linux-arm64.tar.gz`。

如果不确定架构，Windows 查看“设置 → 系统 → 系统信息”的系统类型；macOS 查看“关于本机”的芯片或处理器；Linux 执行 `uname -m`，`x86_64` 对应 x64，`aarch64` 对应 ARM64。

完整解压到固定位置，将直接包含 `bin/` 和 `extension/` 的文件夹作为安装根目录。打开终端，将下面的示例路径替换为实际路径后执行：

Windows PowerShell：

```powershell
Set-Location 'C:\path\to\VidOtter'
.\bin\vidotterdev.exe install
```

Linux/macOS：

```bash
cd '/path/to/VidOtter'
./bin/vidotterdev install
```

看到 `Native Messaging installed:` 和 `Host manifest:` 表示注册完成。以后移动安装目录，需要在新位置重新注册。

### 2.3. 安装浏览器扩展

1. Edge 打开 `edge://extensions`，Chrome 打开 `chrome://extensions`。
2. 开启**开发者模式**，点击**加载解压缩的扩展**。
3. 选择 VidOtter 安装目录下的 `extension/` 文件夹，这个文件夹中应直接包含 `manifest.json`。
4. 在浏览器工具栏的扩展菜单中打开 VidOtter，也可以将它固定到工具栏。
5. 进入**设置 → 连接与运行时**，确认显示**浏览器与下载服务连接正常**后，再继续下载。

若提示找不到 Native Messaging Host，复制扩展管理页的 32 位扩展 ID，在安装根目录重新注册：

```powershell
# Windows：将占位符替换为实际扩展 ID
.\bin\vidotterdev.exe install --extension-id YOUR_32_CHARACTER_ID
```

Linux/macOS 使用 `./bin/vidotterdev install --extension-id YOUR_32_CHARACTER_ID`。注册后，在扩展管理页重新加载 VidOtter。

### 2.4. 播放视频并下载

连接正常后，打开 [MDN 的 CC0 示例视频](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4)并开始播放。

保持在这个标签页，打开 VidOtter：

1. 在**资源嗅探**中找到 MP4。
2. 字幕选择**不下载字幕**。
3. 点击**下载**。
4. 切换到**下载任务**，查看刚创建的任务。

如果换成 HLS，选择完整播放清单，不要选择单独的媒体分片。

### 2.5. 检查下载结果

等待任务变为**已完成**，在任务中展开**下载文件**，点击文件旁的**打开**，确认视频可以播放。也可以直接进入默认下载目录：安装根目录下的 `Data/downloads/`。

需要全屏界面时，在扩展中点击**打开工作区**。

### 2.6. 常见问题

- **扩展连接不上本地服务：** 按 [2.3](#23-安装浏览器扩展) 核对扩展 ID 并修复注册。仍失败时，查看 `Data/logs/`，检查端口 `127.0.0.1:17891` 是否被占用。
- **发现不到视频：** 回到原标签页播放或拖动进度条，让页面重新请求媒体。部分网站没有可下载的 MP4/HLS，或使用 DRM。
- **任务报 403 或地址过期：** 回到原页面重新播放，获取新的媒体请求，再按任务提示恢复或重新下载。
- **HLS 合并失败：** 检查 FFmpeg 是否可用。字幕问题先切回“不下载字幕”，确认普通下载是否正常。
- **升级后仍是旧版本，或提示不支持的 Tool 操作：** 停止旧 Bridge，更新程序文件，重新加载扩展，再到**设置 → 关于**核对版本。

如果以上方法仍无法解决，请[提交 Issue](https://github.com/ThinkerQAQ/vidotter/issues)，附上 VidOtter 版本、操作系统、复现步骤和脱敏后的错误信息，不要上传完整 `Data/`。

## 3. Configuration

在 **VidOtter → 设置**中修改并保存配置，文件位于安装根目录的 `Data/config.toml`。

### 3.1. 常用配置

- `download.output_dir`：下载目录，默认是 `Data/downloads/`。
- `download.concurrent`：下载并发，默认 `12`，范围 `1–64`。
- `download.quality`：默认自动选择，也可指定 `best` 或来源支持的高度，例如 `720`。
- `proxy.enabled`、`proxy.url`：默认关闭代理；支持 HTTP 代理，不支持代理凭据和 SOCKS。
- `interface.language`：界面语言，默认 `zh-CN`，可改为 `en`。
- `interface.open_mode`：默认 `popup`，可改为 `sidePanel`。

手动编辑前，先备份配置并停止 Bridge，修改后重启。请保留完整文件，不要用配置片段覆盖。

### 3.2. 升级与卸载

先暂停任务并备份 `Data/`，再按安装方式操作：

- **图形安装：** 使用 **VidOtter Maintenance**，按对应 Release 的说明升级。未配置在线更新源时，通过维护工具移除旧程序，再运行新安装器。
- **压缩包安装：** 用同一新发行版的 `bin/`、`extension/` 和随包 `tools/` 替换旧文件，保留 `Data/`，重新注册并加载扩展。

程序卸载会保留数据和下载文件；手动删除安装目录前需自行备份。

手动安装的注销命令为 Windows `.\bin\vidotterdev.exe uninstall`，Linux/macOS `./bin/vidotterdev uninstall`，之后在浏览器移除扩展。

## 4. Optional Features

### 4.1. Case 1：下载更多格式或网页中的视频

MP4/HLS 使用内置下载器。解析其他受支持的网页时，在设置页安装 yt-dlp；HLS 合并、音视频封装需要 FFmpeg。DASH 和分离音视频的支持取决于来源及工具。

### 4.2. Case 2：生成字幕或翻译

根据字幕来源选择处理方式：

- 使用原站字幕：需要 yt-dlp，且来源提供可获取的字幕。
- 本地语音识别：需要 FFmpeg、whisper.cpp 和语音模型。
- 字幕翻译：需要 llama.cpp 和翻译模型；使用语音识别结果时，还要指定原声语言。

在**设置 → 外部组件**检查所需工具，缺少时安装或配置；本地识别和翻译还需选择并下载模型，或填写已有模型的路径。组件就绪后，回到**资源嗅探**选择字幕处理方式并下载。完成后检查字幕文件、内容和时间轴。

### 4.3. Case 3：通过 ChatGPT 管理下载

桌面程序提供 STDIO MCP，启动参数为 `--mcp-stdio`，复用本地 Bridge。

先完成 Quick Start，确认扩展与 Bridge 已连接，再按以下步骤连接 ChatGPT：

1. 在 OpenAI Platform 创建自己的 Tunnel。
2. 打开**设置 → AI 集成 → ChatGPT MCP**，按需安装官方 Tunnel Client，填写 Tunnel ID，点击**配置并启动 Tunnel**。
3. 如果没有可复用的凭据，按本机终端提示输入具有 Tunnels Read + Use 权限的 Runtime API Key。
4. 在 ChatGPT 创建私有自定义 MCP 插件，连接方式选 Tunnel，填写同一个 Tunnel ID。
5. 在 ChatGPT 调用 `vidotter_status`，收到正常响应即连接成功。提交下载后，使用返回的任务 ID 查询结果。

### 4.4. Case 4：需要浏览器操作

如果 ChatGPT 已连接 Remote Browser Tool，直接复用现有连接操作页面，由 VidOtter 发现媒体和下载。

需要由 VidOtter 操作浏览器时，在设置中安装 [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) 组件。它使用独立发行包，只操作受限后台标签页；自动点击需在本机显式开启，无需另建 Tunnel。

### 4.5. Case 5：在 Android 使用

Android 是独立原生客户端，支持 Android 8.0 / API 26 及以上，通过粘贴或分享 MP4/HLS 直链下载。它不接入桌面任务、Native Messaging、yt-dlp 或 ChatGPT Tunnel。

先检查[最新 Release](https://github.com/ThinkerQAQ/vidotter/releases/latest) 是否提供匹配设备的 APK；若没有，再从源码构建：

1. 按 [5.1](#51-打开项目并验证) 克隆仓库并准备 Go 环境，以下命令均在仓库根目录执行。
2. 按 [Android 构建配置](mobile/android/app/build.gradle.kts)及[构建工具的环境检查](tools/vidotterdev/internal/maint/mobile_media.go)准备匹配的 JDK、Android SDK Platform、NDK 和 Platform-Tools。设置 `JAVA_HOME`、`ANDROID_SDK_ROOT` 为对应目录；需要指定 NDK 版本时，将 `ANDROID_NDK_HOME` 设为该版本目录。
3. 安装与 `bridge/go.mod` 一致的两个 Go mobile 工具，再构建 APK：

Linux/macOS：

```bash
vidotter_mobile_version="$(go -C bridge list -m -f '{{.Version}}' golang.org/x/mobile)"
go install "golang.org/x/mobile/cmd/gomobile@$vidotter_mobile_version"
go install "golang.org/x/mobile/cmd/gobind@$vidotter_mobile_version"
go -C tools/vidotterdev run . android-build
```

Windows PowerShell：

```powershell
$vidotterMobileVersion = go -C bridge list -m -f '{{.Version}}' golang.org/x/mobile
go install "golang.org/x/mobile/cmd/gomobile@$vidotterMobileVersion"
go install "golang.org/x/mobile/cmd/gobind@$vidotterMobileVersion"
go -C tools/vidotterdev run . android-build
```

`android-build` 会生成 Go 绑定，再调用仓库自带的 Gradle Wrapper。成功后：

1. 找到 `.build/artifacts/android/vidotter-android-arm64-debug.apk`，将它传到 ARM64 Android 设备。
2. 按系统提示允许从该来源安装 APK。
3. 打开 VidOtter，粘贴前面的示例 MP4 地址并下载，检查任务和下载文件。

## 5. Development

开发环境需要 Git、Go 和 Node.js。Go 工具链须满足 [`bridge/go.mod`](bridge/go.mod) 与 [`tools/vidotterdev/go.mod`](tools/vidotterdev/go.mod) 的声明；Node.js 版本与 [CI 配置](.github/workflows/ci.yml)保持一致。

### 5.1. 打开项目并验证

```bash
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` 执行两个 Go 模块的测试、扩展 JS 语法检查与脚本测试，成功后输出 `Go and browser checks passed`。`build` 将当前系统的 Bridge 编译到 `.build/out/`，开发安装见 [5.2](#52-安装开发版本)。

跨平台编译示例：

```bash
go -C tools/vidotterdev run . build --os windows --arch amd64
```

### 5.2. 安装开发版本

在源码仓库根目录执行以下命令，部署到 `.build/dev-install/` 并注册：

Linux/macOS：

```bash
vidotter_dev_dir="$PWD/.build/dev-install"
go -C tools/vidotterdev run . deploy --target "$vidotter_dev_dir"
cd "$vidotter_dev_dir"
./bin/vidotterdev install
```

Windows PowerShell：

```powershell
$vidotterDevDir = Join-Path (Get-Location).Path '.build\dev-install'
go -C tools/vidotterdev run . deploy --target $vidotterDevDir
Set-Location $vidotterDevDir
.\bin\vidotterdev.exe install
```

按 [2.3](#23-安装浏览器扩展) 加载这个部署目录的 `extension/`，再按 2.4–2.5 下载并检查文件。继续修改或打包源码前，先切回原来的源码仓库根目录。

### 5.3. 打包与 CI

统一使用 `tools/vidotterdev/` 构建、验证和打包。正式打包命令为 `go -C tools/vidotterdev run . package`，要求准备已校验的侧车工具、许可证及固定版本的 FFmpeg 源码包。`--allow-missing-tools` 仅用于不完整的开发包。

[CI](.github/workflows/ci.yml) 在 Windows、macOS 和 Linux 执行测试、构建及 README 链接检查；真实浏览器下载、图形安装器操作及 Android 设备需要另外验收。协作约定见 [AGENTS.md](AGENTS.md)。

## 6. License

原创源码采用 [Apache-2.0](LICENSE)，第三方组件遵循各自许可证，详见 [NOTICE](NOTICE) 和[第三方声明](THIRD_PARTY_NOTICES.md)。Windows 的 FFmpeg Slim 使用 LGPL-2.1-or-later，对应源码随同一 Release 提供；Qt Installer Framework 使用 GPL-3.0 与 Qt Company Exception。模型权重和下载的媒体不在项目源码许可范围内。

请只下载有权保存的媒体。VidOtter 不绕过 DRM、付费墙或访问控制。安装器的平台支持、签名及公证状态见对应 Release。

`Data/` 中可能包含任务来源、断点数据和本地认证信息，不要上传到公开仓库。浏览器会话由扩展处理，Bridge 仅在本机回环地址监听；私有 MCP 只连接可信客户端。
