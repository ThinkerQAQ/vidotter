# Go 工程工具与维护流程

VidOtter 使用 `tools/vidotterdev/` 中的原生 Go 维护程序统一执行构建、验证、发行打包、安装与平台工具集成。项目无需额外的开发网关或外部 Project SDK。

## 目录职责

| 目录 | 职责 |
| --- | --- |
| `bridge/` | Go 下载引擎、本地鉴权 API、Native Messaging 和 Android Go 绑定 |
| `extension/` | Manifest V3、Popup、Side Panel 和 Web UI |
| `mobile/android/` | Kotlin 应用、官方 Gradle Wrapper |
| `tools/vidotterdev/` | 构建、验证、打包、部署、组件管理和 Android/FFmpeg 编排 |
| `docs/`、`docs/zh-CN/` | 英文与中文对照文档 |
| `.build/out/`、`.build/artifacts/`、`.build/cache/` | Git 忽略的构建产物与缓存 |
| 安装目录下的 `bin/`、`extension/`、`tools/`、`Data/` | 程序文件与持久化用户数据（部署不得清理 `Data/`） |

`bridge/` 仍是独立 Go 模块，避免破坏现有导入与 gomobile 绑定路径。

## 原生工程命令

在仓库根目录运行：

```sh
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
go -C tools/vidotterdev run . build --os windows --arch amd64
go -C tools/vidotterdev run . package
go -C tools/vidotterdev run . components --root /path/to/installed/vidotter
go -C tools/vidotterdev run . tool-install whisper
go -C tools/vidotterdev run . tool-install llama
go -C tools/vidotterdev run . android-bind
go -C tools/vidotterdev run . android-build
go -C tools/vidotterdev run . ffmpeg-native --source /path/to/FFmpeg
go -C tools/vidotterdev run . ffmpeg-android --source /path/to/FFmpeg
go -C tools/vidotterdev run . icons
```

`verify` 检查两个 Go 模块、JavaScript 语法和浏览器扩展测试，不代表真实浏览器、Windows 安装或 Android 真机验收。普通构建产物在 `.build/out/`，发行包在 `.build/artifacts/release/v<version>/`，Android 产物在 `.build/artifacts/android/`。

## 原生 LSP 与代码分析

- **Go：** 通过 LSP 客户端运行 `gopls serve`；`bridge/` 和 `tools/vidotterdev/` 是两个独立 Go 模块。优先使用符号、定义、引用和诊断能力。
- **浏览器代码：** 有语言服务器时使用编辑器提供的 JS/HTML/CSS 导航与诊断；没有时使用 `rg`、源码检查、`node --check` 和现有测试。
- **Markdown：** 通过 LSP 客户端运行 `marksman server`，查看标题、定义与链接引用。完整审查还要逐节阅读正文和表格。

LSP 服务器不是直接执行一次就能查询的命令，也无需为了它们额外实现一套网关。

## 发布与部署

`package` 生成 Windows amd64、Linux amd64/arm64、macOS amd64/arm64 发行包、SHA256 校验文件和 `components.json`。Windows 正式包要求 `dist/tools/` 中包含具备合法许可的 FFmpeg、Whisper、Llama 组件。缺少组件必须报错；`--allow-missing-tools` 仅允许开发用途的不完整包，不应用于正式发布。

从 WSL 部署 Windows 版之前，先停止 Bridge，再执行：

```sh
go -C tools/vidotterdev run . deploy --os windows --arch amd64 --target /mnt/c/software/Coding/vidotter
```

Windows 使用 `bin/vidotterdev.exe install` 注册 Native Messaging，Linux/macOS 使用 `./bin/vidotterdev install`；解除注册则执行 `uninstall`。安装和部署不得覆盖 `Data/`、已有下载文件和配置。

Android 保留官方 Gradle Wrapper、gomobile、SDK/NDK 和 JDK；FFmpeg 继续使用上游 configure/make 以及需要的平台工具链。没有执行成功的目标构建，不应声明已完成验收。

## 命令参数与前置条件

基础构建需要 Go 1.24+，`verify` 还需要 Node.js；首次使用见[开发教程](../tutorial/development.md)。以下参数由统一维护程序解析：

| 参数 | 用途 |
| --- | --- |
| `--root` | 明确仓库或安装根目录，否则从工作目录或程序位置向上查找 `extension/manifest.json` |
| `--os` / `--arch` | 构建或部署的目标系统与架构；macOS 使用 `darwin` |
| `--output` | 对支持的命令指定输出文件或目录 |
| `--target` | deploy 必填，明确要修改的安装目录 |
| `--extension-id` | install 接受的 32 位 a–p 扩展 ID |
| `--source` | FFmpeg 源码；package 接收其对应源码归档 |
| `--archive` | tool-install 使用的本地固定版本归档，仍校验 SHA-256 |

Go flag 解析在第一个位置参数处停止，因此使用 `tool-install --archive /path/to/archive.zip whisper`，不要把选项放在 `whisper` 后面。`build` 只生成 Bridge；`deploy` 还构建维护程序、复制扩展和文档，但不准备第三方工具，也不自动注册。

正式打包还需要 `.build/cache/source/FFmpeg-n8.1.2-source.tar.gz` 或 `package --source /path/to/archive`，归档 SHA-256 必须匹配代码固定值。只有 Windows 包复制 `dist/tools/`；Linux/macOS 包不附带 FFmpeg、Whisper、Llama。完整组件要求见[发行说明](../../../build/release/README.md)。`package` 不构建 Android APK，也不自动创建 GitHub Release。

## 依赖许可

`go -C tools/vidotterdev run . licenses` 通过固定版本上游工具，按桌面平台收集完整 Go 依赖许可与版本清单；`package` 自动执行。正式 Windows 打包会拒绝缺少 sidecar 许可与来源文件的产物。CI 在 Windows、macOS、Linux 执行现有 verify/build；浏览器与 Android 实机验收仍需单独进行。
