# Release 组件清单

VidOtter 的桌面发布包保留独立 sidecar，不把第三方可执行文件嵌入主程序：

- Windows 包提供 `tools/ffmpeg-slim.exe`；Linux/macOS 包不附带 FFmpeg，需单独安装；
- Windows 包在 `tools/whisper/` 中提供官方 CPU 版 `whisper-cli.exe` 及其运行时 DLL；模型文件不随包提供；
- Windows 包在 `tools/llama/` 中提供官方 CPU 版 `llama-server.exe` 及其运行时 DLL；翻译模型不随包提供；
- `yt-dlp` 是可选 Tool，缺失时由桌面端在用户点击后从官方 release 下载；
- `components.json` 记录交付方式、能力、随包文件 SHA-256 和许可证/源码信息；按需下载组件不记录本地开发目录中的同名文件。

Windows 组件需要准备好后才能创建正式发行包：

```powershell
go -C tools/vidotterdev run . tool-install whisper
go -C tools/vidotterdev run . tool-install llama
# package 已为每个暂存包生成 components.json；仅检查已有安装时才需要：
# go -C tools/vidotterdev run . components --root /path/to/installed/vidotter --os windows --arch amd64
```

`package` 固定生成五个桌面目标，目录布局一致，但组件交付能力不同；不能把 Windows 的随包工具假定为 Linux/macOS 也有。
Android/iOS 不使用该下载机制；原生库必须随 APK/AAB 或应用包发布。

创建 GitHub Release 资产：

```powershell
# 首先将官方 FFmpeg-n8.1.2-source.tar.gz 下载到
# .build/cache/source/FFmpeg-n8.1.2-source.tar.gz
# 并验证仓库记录的固定 SHA256
go -C tools/vidotterdev run . package
```

Go 维护程序会重新编译五个桌面目标，生成一个 Windows ZIP、四个 Linux/macOS TAR.GZ、FFmpeg 对应源码包和 `SHA256SUMS.txt`。

运行 `package` 前需准备 `dist/tools/ffmpeg-slim.exe` 及其许可、源码信息，安装 Whisper/Llama 不会生成 FFmpeg。`--allow-missing-tools` 仅用于明确不完整的开发包，不用于正式发布。完整命令见[维护指南](../../docs/zh-CN/architecture/maintenance.md)。
