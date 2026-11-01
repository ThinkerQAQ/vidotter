# VidOtter

**简体中文** · [English](README.md) · [下载发行包](https://github.com/ThinkerQAQ/vidotter/releases) · [反馈问题](https://github.com/ThinkerQAQ/vidotter/issues)

## Introduction（项目简介）

VidOtter 通过 **Chrome / Edge 扩展发现网页媒体**，再由**本地 Go Bridge** 下载，适合希望保存自己有权访问的视频、又不想手动复制临时媒体地址的用户。

- **日常下载：** 发现当前页面的 MP4/HLS、提交任务、查看进度和打开文件。DASH、分离音视频等格式可能需要额外工具，不能保证所有网站都兼容。
- **统一工作区：** 弹窗、侧边栏和本机 Web UI 共用下载任务、配置与历史。
- **按需扩展：** yt-dlp、站点字幕、本地语音识别、翻译、私有 ChatGPT MCP、受限浏览器自动化，均非首次 MP4 下载所必需。
- **独立 Android 客户端：** 接收或粘贴支持的媒体直链，并不是桌面 Bridge 的远程界面。

项目**不绕过 DRM、付费墙或访问控制**；发现资源不代表一定可下载。可选的 [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) 是独立维护的外部依赖，并非 VidOtter 核心。

![VidOtter 早期中文嗅探界面（v1.3.0）](assets/first-run-detect-zh.png)

*历史界面截图，仅帮助辨认功能入口；最新版本的布局、文字可能不同。*

## Quick Start（快速开始）

### 1. 前置条件

| 平台 | v1.6.3 安装器 / 压缩包 | 首次使用要求 |
| --- | --- | --- |
| Windows | **x64 图形 `.exe`**，或 ZIP | Edge 或 Chrome；不用安装 Go、Node.js 或获取管理员权限 |
| Linux | **x64 图形 `.run`**，或 x64/ARM64 TAR.GZ | Chrome/Chromium 或 Edge；HLS 合并及分离音轨处理需系统 FFmpeg |
| macOS | **Intel 图形 `.dmg`**，或 Intel/Apple Silicon TAR.GZ | Chrome/Chromium 或 Edge；HLS 合并及分离音轨处理需系统 FFmpeg |

**普通视频下载不需要 OpenAI 账号、API Key、Tunnel 或本地 AI 模型。** Windows 包附带适用于 VidOtter 任务的精简 FFmpeg；Linux/macOS 包不附带。首次建议下载普通、可直接播放的 MP4。

### 2. 下载安装（优先使用图形安装器）

从 [**v1.6.3 正式 Release**](https://github.com/ThinkerQAQ/vidotter/releases/latest) 下载与电脑匹配的文件：

- **Windows x64：** 下载 `VidOtter-Setup-v1.6.3-windows-amd64.exe` 并运行。
- **Linux x64：** 下载 `VidOtter-Setup-v1.6.3-linux-amd64.run`，赋予执行权限并运行。可在下载目录执行 `chmod +x VidOtter-Setup-v1.6.3-linux-amd64.run && ./VidOtter-Setup-v1.6.3-linux-amd64.run`。
- **macOS Intel：** 打开 `VidOtter-Setup-v1.6.3-macos-amd64.dmg`，运行其中的安装器。
- **macOS Apple Silicon / Linux ARM64：** 暂用**对应的 TAR.GZ 压缩包**，按照下面的手动方式安装；这两个架构尚未发布已验证的原生 GUI 安装器。

图形安装器会部署 Bridge 并注册 Native Messaging。点击**完成**会默认打开浏览器扩展激活说明和安装文件夹；**不会替用户静默授权本地扩展或启用 ChatGPT Tunnel**。首版安装器**尚未完成 Windows 代码签名和 Apple 公证**，系统可能出现发行者安全提示；运行前请核对来源与校验值。

**不想使用图形安装器，或目标平台没有原生 GUI 包：** 下载对应 ZIP/TAR.GZ，完整解压到长期使用的位置，保持 `bin/` 与 `extension/` 在同一个根目录。在**解压根目录**执行：

**Windows PowerShell**

```powershell
.\bin\vidotterdev.exe install
```

**Linux / macOS**

```bash
./bin/vidotterdev install
```

**手动安装验收：** 终端输出 `Native Messaging installed:` 和 `Host manifest:`，后者位于 `Data/native-host/`。若以后移动压缩包安装目录，需要在新位置重跑 `install`。两种安装方式都由浏览器按需唤起 Bridge，不需要终端一直运行。

### 3. 加载浏览器扩展（必需）

1. Edge 打开 `edge://extensions`，Chrome 打开 `chrome://extensions`。
2. 开启**开发者模式**，点击**加载解压缩的扩展（Load unpacked）**。
3. 选择安装目录内**直接包含 `manifest.json` 的 `extension/` 文件夹**（图形安装器会自动打开它），不要选择压缩包或整个软件根目录。
4. 固定 VidOtter 到工具栏，打开扩展，在**设置**中检查本地 Bridge / 环境是否正常。

VidOtter 当前采用本地扩展分发，**没有上架浏览器扩展商店**。浏览器的确认操作无法跳过。

如扩展实际 ID 与默认注册 ID 不同，从扩展管理页复制 32 位 ID，在解压根目录执行：

```powershell
# Windows：将占位符换成浏览器显示的 ID
.\bin\vidotterdev.exe install --extension-id YOUR_32_CHARACTER_ID
```

Linux/macOS 使用 `./bin/vidotterdev install --extension-id YOUR_32_CHARACTER_ID`。程序升级之后还要在扩展管理页点击**重新加载**；仅更新 Bridge 不会重启旧版浏览器后台脚本。

### 4. 完成一次真实下载

1. 在浏览器打开有权保存的视频并**开始播放**。可使用公开的 [MDN CC0 花朵 MP4](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4) 测试。
2. 保持**同一标签页**，打开 VidOtter，在**资源嗅探（Detect）**中选择完整 MP4 / HLS 播放清单，而非单独的媒体分片。
3. 第一次选择**不下载字幕（No subtitles）**，点击一次**下载**。
4. 在**下载任务（Downloads）**中查看新任务，等待显示**已完成（Completed）**，再通过打开文件入口检查，或到安装根目录的默认 `Data/downloads/` 检查文件。

**成功标准：** 扩展连接 Bridge、新任务确实创建、**实际下载的媒体文件能够打开**。只有绿灯或发现 URL，都不足以证明下载成功；真实浏览器播放测试仍需在你的机器上执行。

如需全屏工作区，从扩展点击**打开工作区**，进入 `http://127.0.0.1:17891/console/`。它通过扩展中继执行操作；仅打开这个本机网页不会自动获得 Bridge 操作权限。

### 5. 常见问题

| 现象 | 处理方式 |
| --- | --- |
| 找不到 Native Messaging Host / 环境异常 | 在正确的解压根目录重跑 `vidotterdev install`，核对扩展 ID，并在 `edge://extensions` / `chrome://extensions` 重新加载 VidOtter。 |
| 界面显示新功能，却报“不支持的 Tool 操作” | 可能还在使用旧版 Manifest V3 后台脚本。重新加载扩展并重新打开设置，不要删除 `Data/`。 |
| Bridge 一直无法就绪 | 检查本机端口 `127.0.0.1:17891` 是否冲突和 `Data/logs/` 日志；先确认进程身份再停止。 |
| 没有发现媒体 | 回到原视频标签页播放、拖动进度；有些网站没有可下载的 MP4/HLS 或使用 DRM。 |
| HTTP 403 / 媒体地址过期 | 查看任务错误，在合法访问的原页面重新产生媒体请求；部分任务能续用同一 ID，另一些需要重新下载。 |
| HLS 合并或字幕任务失败 | 核对 FFmpeg；本地识别、翻译还需要工具及模型。第一次先选“不下载字幕”。 |
| 升级后版本没有变化 | 停止旧 Bridge，整体替换同一发行版的程序文件，重新注册 Native Host，重新加载扩展，在**设置 → 关于**比对版本。 |

**升级与卸载：** 暂停任务，先备份 `Data/`。GUI 安装用户使用 **VidOtter Maintenance** 维护工具（Windows：开始菜单 → VidOtter → Uninstall VidOtter）。首版离线安装器**尚无已上线的远程更新源**；更换版本需先通过维护工具移除程序，再运行新安装包，保留原来的 `Data/` 和下载文件。ZIP/TAR.GZ 手动安装用户则替换同一新版本的 `bin/`、`extension/`、随包 `tools/`，**保留 `Data/`**，重新运行 `install` 并重载扩展。手动安装的注销命令是 Windows `./bin/vidotterdev.exe uninstall`（PowerShell 使用 `.\bin\vidotterdev.exe uninstall`），Linux/macOS `./bin/vidotterdev uninstall`，最后从浏览器移除扩展。程序卸载**不会主动清除 `Data/` 或下载文件**；如要手动删除整个安装目录，先备份。

## Configuration（配置说明）

Bridge 会在软件根目录生成 `Data/config.toml`。**首次使用没有任何必填配置。** 请优先通过 **VidOtter → 设置**修改；UI 会校验并保存完整的 TOML 文件。

| 字段 | 默认值 | 何时需要修改 |
| --- | --- | --- |
| `download.output_dir` | `Data/downloads/` | 修改下载目录 |
| `download.concurrent` | `12` | 下载并发，范围 1–64 |
| `download.quality` | 自动（`""`） | `best` 或来源支持的高度，如 `720` |
| `proxy.enabled`、`proxy.url` | `false`、空 | 需要显式 HTTP 代理时；不支持代理凭据与 SOCKS |
| `interface.language` | `zh-CN` | `en` / `zh-CN`，仅影响界面，不改变字幕语言 |
| `interface.open_mode` | `popup` | `popup` / `sidePanel` |

工具路径、嗅探类型、日志和模型等可在设置页继续管理。**不要用只有几个字段的片段覆盖 `config.toml`**：格式错误或字段不足可能让程序改用默认值。手工编辑须先备份、停止 Bridge，修改后重启并提交一项新任务核验。

`Data/bridge.json` 可能含本地认证 Token；`Data/jobs.json`、`Data/work/` 可能含敏感来源 URL 或断点数据。**不要原样上传 `Data/`。** Bridge 的 HTTP 服务必须留在本机回环接口，不能直接暴露公网。

## Optional Features（可选功能）

以下功能都**不影响首次 MP4 下载**：

- **网页解析与更多格式：** 通过设置页按需安装 yt-dlp；FFmpeg 负责音视频封装和合并。Windows v1.6.3 附带 FFmpeg Slim、whisper.cpp、llama.cpp 的可执行文件，**不附带模型权重**。Linux/macOS 若需要语音模型，应另行提供兼容的工具。
- **字幕：** 获取原站字幕需要 yt-dlp 和站点支持；本地转写需要 FFmpeg、whisper.cpp 与模型；字幕翻译需要 llama.cpp 和独立授权的模型。ASR 翻译时明确指定原声语言；不能只看视频状态，需检查字幕文件和时间轴。
- **私有 ChatGPT MCP：** 桌面程序内置 **13 个 MCP 工具**，通过 `--mcp-stdio` 复用 Bridge，普通下载并不依赖它。打开**设置 → AI 集成 → ChatGPT MCP**，先在 OpenAI Platform 创建自己的 Tunnel，再按需点击**安装官方 Tunnel Client**（校验官方 SHA-256）、输入 Tunnel ID 并点击**配置并启动 Tunnel**。若本机没有可复用的有效凭据，首次在**本机终端**输入受限 Runtime API Key（Tunnels Read + Use），不会通过浏览器扩展传输。设置页可启动、停止、重启用户级服务并检查 Ready。随后在 ChatGPT 创建**私有自定义 MCP 插件**，连接方式选 Tunnel、填入同一 ID，工具自动发现，调用 `vidotter_status` 验收。**本地 Tunnel Ready 不代表 ChatGPT 已注册成功。** 切勿公开密钥或本地配置文件。
- **受限浏览器自动化：** 可选安装独立维护的 [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp)，通过 VidOtter 设置进行集成。组件只允许管理受限后台标签页；自动点击需显式同意，不开放任意 JS、私人标签页或 Cookie。已有的 Windows 公开 MP4 冒烟测试**不能推导成所有网站都支持**。
- **Android（独立源码构建）：** Android 8.0 / API 26 及以上可使用应用内粘贴/分享 MP4、HLS 直链。它**不提供桌面 Native Messaging、桌面下载任务、yt-dlp 或 ChatGPT Tunnel**。桌面 Release 不含 Android APK。源码构建要求 JDK 17、Android SDK 35 / NDK、gomobile、Gradle Wrapper；配置好环境后依次执行 `go -C tools/vidotterdev run . android-bind` 和 `go -C tools/vidotterdev run . android-build`，手机设备验收另做。

## Development（开发与验证）

使用正式发行包的用户**不需要开发工具**。源码开发需要 Git、**Go 1.25+**（Bridge 的 `go.mod` 声明 `go 1.25.0`）及 Node.js（CI 使用 Node 22，用于 JS 测试）。

```bash
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` 运行两个 Go 模块测试、扩展 JS 语法检查与脚本测试；`build` 只将当前系统/架构的 Bridge 生成到 `.build/out/`，**不会注册 Native Messaging，也不是完整的发行包**。例如交叉编译：

```bash
go -C tools/vidotterdev run . build --os windows --arch amd64
```

独立的开发安装可从源码运行 `go -C tools/vidotterdev run . deploy --target /absolute/path/to/vidotter-dev`，再到**目标操作系统**的部署目录执行 `bin/vidotterdev install`。不要覆盖正在运行的 Windows Bridge 或用户 `Data/`。

正式发行打包命令是 `go -C tools/vidotterdev run . package`，但它**要求事先准备并校验** Windows FFmpeg / whisper / llama 侧车、许可证/源码声明及固定版本的 FFmpeg 源码包。`--allow-missing-tools` **只能用于不完整的开发包，不得当成完整 Release**。维护工具还提供 `licenses`、`tool-install`、`ffmpeg-native`、Android 构建命令。CI 在 **Windows、macOS、Linux** 上运行编译和测试，但不等同于三平台真实浏览器下载或 Android 手机验收。

核心链路：**扩展 → Native Messaging → 本地鉴权 Bridge → 下载工作进程 → `Data/`**。可选 MCP 通过 stdio 复用同一个 Bridge。开发协作约定见 [AGENTS.md](AGENTS.md)。

## License（许可证与合规）

VidOtter **原创源码**使用 [Apache-2.0](LICENSE)，更多法律义务见 [NOTICE](NOTICE) 和[第三方许可证说明](THIRD_PARTY_NOTICES.md)。**源码许可不能替用户获得第三方视频、图片、网站素材或模型的授权。**

Windows 包附带独立的 **FFmpeg Slim** 可执行文件，FFmpeg 代码适用 **LGPL-2.1-or-later**；v1.6.3 在同一 Release 中提供[对应的 FFmpeg-n8.1.2 源码包](https://github.com/ThinkerQAQ/vidotter/releases/download/v1.6.3/FFmpeg-n8.1.2-source.tar.gz)及许可/构建来源说明。其他版本及模型须核对各自许可。

只下载你有权保存的媒体。项目不隶属于视频内容平台，不绕过 DRM，也不代替用户判断版权。下载记录可能保留签名 URL；浏览器 Cookie 仅由扩展管理，不能写入公开日志或仓库。私有 Tunnel 可以允许 AI 使用本机工具，应只授权可信客户端，**不要将 Bridge 回环端口直接开放到公网**。首版 Qt GUI 安装器使用 Qt Installer Framework 4.7，适用 GPL-3.0 与 Qt Company Exception，详见第三方说明。目前未签名、公证，也尚未提供已上线的远程自动更新或 ARM64 原生 GUI 安装器。
